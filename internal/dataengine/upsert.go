package dataengine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/jackc/pgx/v5/pgconn"
)

// UpsertResult is the outcome of Upsert.
type UpsertResult struct {
	Record  SObjectRecord `json:"record"`
	Created bool          `json:"created"`
}

// UpsertAuthz enforces object + record sharing + FLS for upsert (ADR-016).
type UpsertAuthz struct {
	AssertObjectAccess    func(ctx context.Context, actor *authz.Actor, objectAPIName string, action authz.CrudAction) error
	CanModifyRecord       func(ctx context.Context, actor *authz.Actor, recordID, ownerID, createdByID, objectAPIName string, modifyAll map[string]struct{}) (bool, error)
	GetModifyAllObjects   func(ctx context.Context, actor *authz.Actor) (map[string]struct{}, error)
	AssertEditableFields  func(ctx context.Context, actor *authz.Actor, objectAPIName string, data map[string]any) error
	StripUnreadableFields func(ctx context.Context, actor *authz.Actor, objectAPIName string, rec map[string]any) (map[string]any, error)
}

// NormalizeExternalIDValue coerces a value to the canonical string used for lookup.
func NormalizeExternalIDValue(v any) (string, error) {
	if v == nil {
		return "", validationErrorf("externalId value is required")
	}
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return "", validationErrorf("externalId value is required")
		}
		return s, nil
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t)), nil
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", t), "0"), "."), nil
	case int:
		return fmt.Sprintf("%d", t), nil
	case int64:
		return fmt.Sprintf("%d", t), nil
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		if s == "" || s == "<nil>" {
			return "", validationErrorf("externalId value is required")
		}
		return s, nil
	}
}

func requireExternalIDField(fields []metadata.FieldDefinition, fieldAPIName string) (metadata.FieldDefinition, error) {
	for _, f := range fields {
		if f.APIName == fieldAPIName {
			if !f.ExternalID {
				return metadata.FieldDefinition{}, validationErrorf("field %s is not marked externalId", fieldAPIName)
			}
			return f, nil
		}
	}
	return metadata.FieldDefinition{}, validationErrorf("unknown externalIdField %s", fieldAPIName)
}

// GetByExternalID loads a record by an external-id field value.
func (s *Service) GetByExternalID(ctx context.Context, objectAPIName, externalIDField, externalIDValue string) (SObjectRecord, error) {
	desc, err := s.meta.Describe(ctx, objectAPIName)
	if err != nil {
		return nil, err
	}
	if err := rejectKernelStorage(objectAPIName, desc.StorageMode); err != nil {
		return nil, err
	}
	if _, err := requireExternalIDField(desc.Fields, externalIDField); err != nil {
		return nil, err
	}
	value, err := NormalizeExternalIDValue(externalIDValue)
	if err != nil {
		return nil, err
	}
	if _, err := assertSafeFieldName(externalIDField); err != nil {
		return nil, validationErrorf("Invalid externalIdField: %s", externalIDField)
	}
	table, err := quoteTable(db.RecordsTableForStorageMode(desc.StorageMode))
	if err != nil {
		return nil, err
	}
	q := s.querier(ctx)
	rows, err := q.Query(ctx, fmt.Sprintf(`
SELECT id::text, owner_id::text, created_by_id::text, last_modified_by_id::text, created_at, updated_at, data
FROM %s
WHERE object_api_name = $1 AND NULLIF(data ->> '%s', '') = $2
LIMIT 2`, table, escapeSQLLiteral(externalIDField)), objectAPIName, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type hit struct {
		id, createdBy, lastMod string
		owner                  *string
		createdAt, updatedAt   time.Time
		raw                    []byte
	}
	var hits []hit
	for rows.Next() {
		var h hit
		if err := rows.Scan(&h.id, &h.owner, &h.createdBy, &h.lastMod, &h.createdAt, &h.updatedAt, &h.raw); err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("%w: Record not found: %s/%s=%s", ErrNotFound, objectAPIName, externalIDField, value)
	}
	if len(hits) > 1 {
		return nil, fmt.Errorf("%w: DUPLICATE_EXTERNAL_ID: %s/%s=%s", ErrConflict, objectAPIName, externalIDField, value)
	}
	h := hits[0]
	data, err := decodeJSONBMap(h.raw)
	if err != nil {
		return nil, err
	}
	return toSObject(h.id, h.owner, h.createdBy, h.lastMod, objectAPIName, h.createdAt, h.updatedAt, data, nil), nil
}

// Upsert creates or updates a record keyed by an external-id field.
func (s *Service) Upsert(
	ctx context.Context,
	objectAPIName, externalIDField string,
	externalIDValue any,
	input map[string]any,
	actor *authz.Actor,
	az *UpsertAuthz,
) (*UpsertResult, error) {
	if txFrom(ctx) != nil {
		return s.upsertWith(ctx, objectAPIName, externalIDField, externalIDValue, input, actor, az)
	}
	if err := s.ensureRecordPartitionsForWrite(ctx, objectAPIName, "create"); err != nil {
		return nil, err
	}
	if err := s.ensureRecordPartitionsForWrite(ctx, objectAPIName, "update"); err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	txCtx := withTx(ctx, tx)
	result, err := s.upsertWith(txCtx, objectAPIName, externalIDField, externalIDValue, input, actor, az)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) upsertWith(
	ctx context.Context,
	objectAPIName, externalIDField string,
	externalIDValue any,
	input map[string]any,
	actor *authz.Actor,
	az *UpsertAuthz,
) (*UpsertResult, error) {
	if actor == nil {
		return nil, fmt.Errorf("%w: no actor", authz.ErrForbidden)
	}
	if input == nil {
		input = map[string]any{}
	}
	desc, err := s.meta.Describe(ctx, objectAPIName)
	if err != nil {
		return nil, err
	}
	if err := rejectKernelStorage(objectAPIName, desc.StorageMode); err != nil {
		return nil, err
	}
	if _, err := requireExternalIDField(desc.Fields, externalIDField); err != nil {
		return nil, err
	}
	value, err := NormalizeExternalIDValue(externalIDValue)
	if err != nil {
		return nil, err
	}
	if _, err := s.querier(ctx).Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, upsertLockKey(objectAPIName, externalIDField, value)); err != nil {
		return nil, err
	}

	existing, err := s.GetByExternalID(ctx, objectAPIName, externalIDField, value)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	body := cloneMap(input)
	delete(body, "Id")
	body[externalIDField] = value
	if owner, ok := body["OwnerId"]; ok {
		if az == nil || az.GetModifyAllObjects == nil {
			return nil, fmt.Errorf("%w: owner assignment authz required", authz.ErrForbidden)
		}
		modifyAll, err := az.GetModifyAllObjects(ctx, actor)
		if err != nil {
			return nil, err
		}
		var ownerID *string
		if value, ok := owner.(string); ok && value != "" {
			ownerID = &value
		}
		if err := authz.AssertOwnerIDWritable(actor, objectAPIName, ownerID, modifyAll); err != nil {
			return nil, err
		}
	}

	if existing != nil {
		if az == nil || az.AssertObjectAccess == nil || az.CanModifyRecord == nil || az.GetModifyAllObjects == nil {
			return nil, fmt.Errorf("%w: upsert authz required", authz.ErrForbidden)
		}
		if err := az.AssertObjectAccess(ctx, actor, objectAPIName, authz.ActionUpdate); err != nil {
			return nil, err
		}
		modifyAll, err := az.GetModifyAllObjects(ctx, actor)
		if err != nil {
			return nil, err
		}
		ownerID, _ := existing["OwnerId"].(string)
		createdByID, _ := existing["CreatedById"].(string)
		recID, _ := existing["Id"].(string)
		ok, err := az.CanModifyRecord(ctx, actor, recID, ownerID, createdByID, objectAPIName, modifyAll)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("%w: not allowed to update record", authz.ErrForbidden)
		}
		if az.AssertEditableFields != nil {
			if err := az.AssertEditableFields(ctx, actor, objectAPIName, body); err != nil {
				return nil, err
			}
		}
		updated, err := s.Update(ctx, objectAPIName, recID, body, actor)
		if err != nil {
			return nil, err
		}
		if az.StripUnreadableFields != nil {
			stripped, err := az.StripUnreadableFields(ctx, actor, objectAPIName, updated)
			if err != nil {
				return nil, fmt.Errorf("strip unreadable fields: %w", err)
			}
			updated = stripped
		}
		return &UpsertResult{Record: updated, Created: false}, nil
	}

	if az == nil || az.AssertObjectAccess == nil {
		return nil, fmt.Errorf("%w: upsert authz required", authz.ErrForbidden)
	}
	if err := az.AssertObjectAccess(ctx, actor, objectAPIName, authz.ActionCreate); err != nil {
		return nil, err
	}
	if az.AssertEditableFields != nil {
		if err := az.AssertEditableFields(ctx, actor, objectAPIName, body); err != nil {
			return nil, err
		}
	}
	created, err := s.Create(ctx, objectAPIName, body, actor)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("%w: DUPLICATE_EXTERNAL_ID: %s/%s=%s", ErrConflict, objectAPIName, externalIDField, value)
		}
		return nil, err
	}
	if az.StripUnreadableFields != nil {
		stripped, err := az.StripUnreadableFields(ctx, actor, objectAPIName, created)
		if err != nil {
			return nil, fmt.Errorf("strip unreadable fields: %w", err)
		}
		created = stripped
	}
	return &UpsertResult{Record: created, Created: true}, nil
}

// DeleteByExternalID deletes the record matching an external id.
func (s *Service) DeleteByExternalID(ctx context.Context, objectAPIName, externalIDField, externalIDValue string, actor *authz.Actor) error {
	existing, err := s.GetByExternalID(ctx, objectAPIName, externalIDField, externalIDValue)
	if err != nil {
		return err
	}
	recID, _ := existing["Id"].(string)
	return s.Delete(ctx, objectAPIName, recID, actor)
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func upsertLockKey(objectAPIName, externalIDField, value string) string {
	// hashtextextended takes text; Postgres rejects NUL bytes.
	return objectAPIName + "\x1f" + externalIDField + "\x1f" + value
}
