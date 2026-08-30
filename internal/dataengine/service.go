package dataengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/jackc/pgx/v5"
)

// Service is the Go data-engine (CRUD + filter query + automation dispatch).
type Service struct {
	pool         *db.Pool
	meta         *metadata.Service
	ObjectAz     *authz.ObjectAuthz
	AutomationAz *authz.AutomationAuthz
	// Actions is optional; when set, sync guests can call invokeAction on the same write tx.
	Actions ActionInvoker
}

// ActionInvoker is the platform-action dispatcher used by sync guest automations.
// Implemented by internal/actions.Service (kept as an interface to avoid an import cycle).
type ActionInvoker interface {
	Invoke(ctx context.Context, actor *authz.Actor, apiName string, input map[string]any) (map[string]any, error)
}

// NewService constructs a data-engine service.
func NewService(pool *db.Pool, meta *metadata.Service) *Service {
	return &Service{pool: pool, meta: meta}
}

// QueryResult is POST /query response.
type QueryResult struct {
	Records    []SObjectRecord `json:"records"`
	TotalSize  int             `json:"totalSize"`
	Done       bool            `json:"done"`
	NextCursor string          `json:"nextCursor,omitempty"`
	QueryPlan  map[string]any  `json:"queryPlan,omitempty"`
}

// Create inserts a record.
func (s *Service) Create(ctx context.Context, objectAPIName string, input map[string]any, actor *authz.Actor) (SObjectRecord, error) {
	if actor == nil {
		return nil, fmt.Errorf("%w: no actor", authz.ErrForbidden)
	}
	if txFrom(ctx) != nil {
		return s.createWith(ctx, objectAPIName, input, actor)
	}
	if err := s.ensureRecordPartitionsForWrite(ctx, objectAPIName, "create"); err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	ctx = withTx(ctx, tx)
	rec, err := s.createWith(ctx, objectAPIName, input, actor)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *Service) createWith(ctx context.Context, objectAPIName string, input map[string]any, actor *authz.Actor) (SObjectRecord, error) {
	desc, err := s.meta.Describe(ctx, objectAPIName)
	if err != nil {
		return nil, err
	}
	if err := rejectKernelStorage(objectAPIName, desc.StorageMode); err != nil {
		return nil, err
	}
	table, err := quoteTable(db.RecordsTableForStorageMode(desc.StorageMode))
	if err != nil {
		return nil, err
	}
	if err := RejectImmutableSystemFields(input); err != nil {
		return nil, err
	}
	ownerID, _ := optionalOwnerID(input)
	applyCommercialCreateDefaults(objectAPIName, input)
	data, err := NormalizeAndValidateFields(desc.Fields, input, "create")
	if err != nil {
		return nil, err
	}
	if err := s.allocateAutonumbers(ctx, objectAPIName, desc.Fields, data); err != nil {
		return nil, err
	}
	if err := EvaluateValidationRules(desc.ValidationRules, data); err != nil {
		return nil, err
	}
	if err := s.validateCommercialWrite(ctx, objectAPIName, data, "create"); err != nil {
		return nil, err
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	doc, title, subtitle := BuildSearchDocument(desc.Fields, data)

	q := s.querier(ctx)
	var (
		id              string
		createdAt       time.Time
		updatedAt       time.Time
		ownerScan       *string
		createdByOut    string
		lastModifiedOut string
	)
	err = q.QueryRow(ctx, fmt.Sprintf(`
INSERT INTO %s (object_api_name, owner_id, created_by_id, last_modified_by_id, data, search_document, search_title, search_subtitle)
VALUES ($1, $2::uuid, $3::uuid, $3::uuid, $4::jsonb, $5, $6, $7)
RETURNING id::text, owner_id::text, created_by_id::text, last_modified_by_id::text, created_at, updated_at`, table),
		objectAPIName, ownerID, actor.ID, dataJSON, doc, title, subtitle,
	).Scan(&id, &ownerScan, &createdByOut, &lastModifiedOut, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if err := s.afterWrite(ctx, actor, "create", objectAPIName, id, map[string]any{"data": data}); err != nil {
		return nil, err
	}
	return toSObject(id, ownerScan, createdByOut, lastModifiedOut, objectAPIName, createdAt, updatedAt, data, nil), nil
}

// Get loads a non-deleted record.
func (s *Service) Get(ctx context.Context, objectAPIName, id string) (SObjectRecord, error) {
	table, err := s.recordsTableForObject(ctx, objectAPIName)
	if err != nil {
		return nil, err
	}
	table, err = quoteTable(table)
	if err != nil {
		return nil, err
	}
	q := s.querier(ctx)
	var (
		ownerID          *string
		createdByID      string
		lastModifiedByID string
		createdAt        time.Time
		updatedAt        time.Time
		rawData          []byte
	)
	err = q.QueryRow(ctx, fmt.Sprintf(`
SELECT owner_id::text, created_by_id::text, last_modified_by_id::text, created_at, updated_at, data
FROM %s
WHERE id = $1::uuid AND object_api_name = $2`, table),
		id, objectAPIName,
	).Scan(&ownerID, &createdByID, &lastModifiedByID, &createdAt, &updatedAt, &rawData)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: Record not found: %s/%s", ErrNotFound, objectAPIName, id)
	}
	if err != nil {
		return nil, err
	}
	data, err := decodeJSONBMap(rawData)
	if err != nil {
		return nil, err
	}
	return toSObject(id, ownerID, createdByID, lastModifiedByID, objectAPIName, createdAt, updatedAt, data, nil), nil
}

// Update patches a record.
func (s *Service) Update(ctx context.Context, objectAPIName, id string, input map[string]any, actor *authz.Actor) (SObjectRecord, error) {
	if actor == nil {
		return nil, fmt.Errorf("%w: no actor", authz.ErrForbidden)
	}
	if txFrom(ctx) != nil {
		return s.updateWith(ctx, objectAPIName, id, input, actor)
	}
	if err := s.ensureRecordPartitionsForWrite(ctx, objectAPIName, "update"); err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	ctx = withTx(ctx, tx)
	rec, err := s.updateWith(ctx, objectAPIName, id, input, actor)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *Service) updateWith(ctx context.Context, objectAPIName, id string, input map[string]any, actor *authz.Actor) (SObjectRecord, error) {
	desc, err := s.meta.Describe(ctx, objectAPIName)
	if err != nil {
		return nil, err
	}
	table, err := quoteTable(db.RecordsTableForStorageMode(desc.StorageMode))
	if err != nil {
		return nil, err
	}
	if err := RejectImmutableSystemFields(input); err != nil {
		return nil, err
	}
	q := s.querier(ctx)
	var (
		ownerID     *string
		createdByID string
		createdAt   time.Time
		rawData     []byte
	)
	err = q.QueryRow(ctx, fmt.Sprintf(`
SELECT owner_id::text, created_by_id::text, created_at, data
FROM %s
WHERE id = $1::uuid AND object_api_name = $2`, table),
		id, objectAPIName,
	).Scan(&ownerID, &createdByID, &createdAt, &rawData)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: Record not found: %s/%s", ErrNotFound, objectAPIName, id)
	}
	if err != nil {
		return nil, err
	}
	existing, err := decodeJSONBMap(rawData)
	if err != nil {
		return nil, err
	}
	patch, err := NormalizeAndValidateFields(desc.Fields, input, "update")
	if err != nil {
		return nil, err
	}
	merged := map[string]any{}
	for k, v := range existing {
		merged[k] = v
	}
	for k, v := range patch {
		merged[k] = v
	}
	if err := EvaluateValidationRules(desc.ValidationRules, merged); err != nil {
		return nil, err
	}
	if err := s.validateCommercialWrite(ctx, objectAPIName, merged, "update"); err != nil {
		return nil, err
	}
	if newOwner, set := optionalOwnerID(input); set {
		ownerID = newOwner
	}
	mergedJSON, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	doc, title, subtitle := BuildSearchDocument(desc.Fields, merged)
	var (
		updatedAt       time.Time
		lastModifiedOut string
		ownerOut        *string
	)
	err = q.QueryRow(ctx, fmt.Sprintf(`
UPDATE %s
SET data = $1::jsonb, updated_at = now(), owner_id = $2::uuid, last_modified_by_id = $3::uuid,
    search_document = $4, search_title = $5, search_subtitle = $6
WHERE id = $7::uuid AND object_api_name = $8
RETURNING owner_id::text, last_modified_by_id::text, updated_at`, table),
		mergedJSON, ownerID, actor.ID, doc, title, subtitle, id, objectAPIName,
	).Scan(&ownerOut, &lastModifiedOut, &updatedAt)
	if err != nil {
		return nil, err
	}
	if err := s.afterWrite(ctx, actor, "update", objectAPIName, id, map[string]any{"patch": patch, "data": merged}); err != nil {
		return nil, err
	}
	return toSObject(id, ownerOut, createdByID, lastModifiedOut, objectAPIName, createdAt, updatedAt, merged, nil), nil
}

// Delete hard-deletes a record (no soft-delete / deleted_at).
func (s *Service) Delete(ctx context.Context, objectAPIName, id string, actor *authz.Actor) error {
	if actor == nil {
		return fmt.Errorf("%w: no actor", authz.ErrForbidden)
	}
	if txFrom(ctx) != nil {
		return s.deleteWith(ctx, objectAPIName, id, actor)
	}
	if err := s.ensureRecordPartitionsForWrite(ctx, objectAPIName, "delete"); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	ctx = withTx(ctx, tx)
	if err := s.deleteWith(ctx, objectAPIName, id, actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) deleteWith(ctx context.Context, objectAPIName, id string, actor *authz.Actor) error {
	if objectAPIName == "OrderLine" {
		existing, err := s.Get(ctx, objectAPIName, id)
		if err != nil {
			return err
		}
		if err := s.validateCommercialWrite(ctx, objectAPIName, existing, "delete"); err != nil {
			return err
		}
	}
	table, err := s.recordsTableForObject(ctx, objectAPIName)
	if err != nil {
		return err
	}
	table, err = quoteTable(table)
	if err != nil {
		return err
	}
	q := s.querier(ctx)
	tag, err := q.Exec(ctx, fmt.Sprintf(`
DELETE FROM %s WHERE id = $1::uuid AND object_api_name = $2`, table),
		id, objectAPIName,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: Record not found: %s/%s", ErrNotFound, objectAPIName, id)
	}
	// Drop any sharing grants for this record identity.
	if _, err := q.Exec(ctx, `
DELETE FROM record_access_grants WHERE object_api_name = $1 AND record_id = $2::uuid`,
		objectAPIName, id); err != nil {
		return err
	}
	return s.afterWrite(ctx, actor, "delete", objectAPIName, id, map[string]any{})
}

// Query runs a filter/sort/keyset page with optional parent/child relationships.
func (s *Service) Query(ctx context.Context, raw json.RawMessage, vis QueryVisibility) (*QueryResult, error) {
	req, err := ParseQueryRequest(raw)
	if err != nil {
		return nil, err
	}
	obj, err := s.meta.GetObject(ctx, req.Object)
	if err != nil {
		return nil, err
	}
	if err := rejectKernelStorage(req.Object, obj.StorageMode); err != nil {
		return nil, err
	}
	fields, err := s.meta.GetFields(ctx, req.Object)
	if err != nil {
		return nil, err
	}
	if err := assertHighVolumeQueryGuardrails(obj.StorageMode, req, fields); err != nil {
		return nil, err
	}
	if err := assertFlexibleQueryGuardrails(obj.StorageMode, req, fields); err != nil {
		return nil, err
	}
	primaryTable := db.RecordsTableForStorageMode(obj.StorageMode)
	built, err := buildPrimarySelectSQL(req, fields, primaryTable, func(related string) (string, error) {
		return s.recordsTableForObject(ctx, related)
	}, vis)
	if err != nil {
		return nil, err
	}
	parentJoins := make([]joinPlan, 0)
	childJoins := make([]joinPlan, 0)
	for _, j := range built.Joins {
		if j.Type == "parent" {
			parentJoins = append(parentJoins, j)
		} else {
			childJoins = append(childJoins, j)
		}
	}

	q := s.querier(ctx)
	rows, err := q.Query(ctx, built.Text, built.Args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var (
		records     []SObjectRecord
		lastCreated time.Time
		lastID      string
	)
	for rows.Next() {
		var (
			id, createdByID, lastModifiedByID, objName string
			ownerID                                    *string
			createdAt, updatedAt                       time.Time
			rawData                                    []byte
		)
		dest := make([]any, 8+2*len(parentJoins))
		dest[0], dest[1], dest[2], dest[3] = &id, &ownerID, &createdByID, &lastModifiedByID
		dest[4], dest[5], dest[6], dest[7] = &createdAt, &updatedAt, &objName, &rawData
		parentDataRaw := make([][]byte, len(parentJoins))
		parentIDs := make([]*string, len(parentJoins))
		for i := range parentJoins {
			dest[8+2*i] = &parentDataRaw[i]
			dest[8+2*i+1] = &parentIDs[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		data, err := decodeJSONBMap(rawData)
		if err != nil {
			return nil, err
		}
		rec := toSObject(id, ownerID, createdByID, lastModifiedByID, objName, createdAt, updatedAt, data, req.Select)
		for i, join := range parentJoins {
			if parentIDs[i] != nil && *parentIDs[i] != "" {
				pdata, err := decodeJSONBMap(parentDataRaw[i])
				if err != nil {
					return nil, err
				}
				nested := map[string]any{"Id": *parentIDs[i]}
				for k, v := range projectData(pdata, join.Select) {
					nested[k] = v
				}
				rec[join.Alias] = nested
			} else {
				rec[join.Alias] = nil
			}
		}
		records = append(records, rec)
		lastCreated = createdAt
		lastID = id
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if records == nil {
		records = []SObjectRecord{}
	}

	if len(childJoins) > 0 && len(records) > 0 {
		parentIDs := make([]string, len(records))
		for i, r := range records {
			parentIDs[i], _ = r["Id"].(string)
		}
		for _, join := range childJoins {
			text, args, err := buildChildSelectSQL(join, parentIDs, vis)
			if err != nil {
				return nil, err
			}
			if text == "" {
				continue
			}
			childRows, err := q.Query(ctx, text, args...)
			if err != nil {
				return nil, err
			}
			byParent := map[string][]SObjectRecord{}
			for childRows.Next() {
				var (
					cid, createdByID, lastModifiedByID, cObj, parentID string
					cOwner                                             *string
					cCreated, cUpdated                                 time.Time
					cData                                              []byte
				)
				if err := childRows.Scan(&cid, &cOwner, &createdByID, &lastModifiedByID, &cCreated, &cUpdated, &cObj, &cData, &parentID); err != nil {
					childRows.Close()
					return nil, err
				}
				cdata, err := decodeJSONBMap(cData)
				if err != nil {
					childRows.Close()
					return nil, err
				}
				byParent[parentID] = append(byParent[parentID], toSObject(cid, cOwner, createdByID, lastModifiedByID, cObj, cCreated, cUpdated, cdata, join.Select))
			}
			childRows.Close()
			if err := childRows.Err(); err != nil {
				return nil, err
			}
			for _, parent := range records {
				pid, _ := parent["Id"].(string)
				if list, ok := byParent[pid]; ok {
					parent[join.Alias] = list
				} else {
					parent[join.Alias] = []SObjectRecord{}
				}
			}
		}
	}

	var nextCursor string
	done := true
	// Keyset cursors are only correct for default CreatedAt/id ordering.
	if len(req.Sort) == 0 && len(records) == built.PageLimit && lastID != "" {
		nextCursor = encodeKeysetCursor(lastCreated.UTC().Format(time.RFC3339Nano), lastID)
		done = false
	}
	hints, err := s.buildIndexHints(ctx, req.Object, req, fields)
	if err != nil {
		return nil, err
	}
	warnings := make([]string, 0, len(hints))
	for _, h := range hints {
		warnings = append(warnings, h.FieldAPIName+": "+h.Reason)
	}
	locatorCeiling := Limits.LocatorMaxRows
	if obj.StorageMode == db.StorageModeHighVolume && req.Mode == "locator" {
		locatorCeiling = HighVolumeLocatorMaxRows
	}
	plan := map[string]any{
		"joins":          len(built.Joins),
		"mode":           req.Mode,
		"indexHints":     hints,
		"warnings":       warnings,
		"locatorMaxRows": locatorCeiling,
	}
	return &QueryResult{
		Records:    records,
		TotalSize:  len(records),
		Done:       done,
		NextCursor: nextCursor,
		QueryPlan:  plan,
	}, nil
}

// CompositeSubrequest is one entry in POST /composite.
type CompositeSubrequest struct {
	Method      string         `json:"method"`
	Object      string         `json:"object"`
	ID          string         `json:"id,omitempty"`
	Body        map[string]any `json:"body,omitempty"`
	ReferenceID string         `json:"referenceId"`
}

// CompositeResult is the composite response wrapper.
type CompositeResult struct {
	CompositeResponse []map[string]any `json:"compositeResponse"`
}

// CompositeAuthz enforces object + record sharing AuthZ for composite subrequests (ADR-016).
// Required for Client composite; without it GET/PATCH/DELETE are denied.
type CompositeAuthz struct {
	AssertObjectAccess    func(ctx context.Context, actor *authz.Actor, objectAPIName string, action authz.CrudAction) error
	CanViewRecord         func(ctx context.Context, actor *authz.Actor, recordID, ownerID, createdByID, objectAPIName string, viewAll map[string]struct{}) (bool, error)
	CanModifyRecord       func(ctx context.Context, actor *authz.Actor, recordID, ownerID, createdByID, objectAPIName string, modifyAll map[string]struct{}) (bool, error)
	GetViewAllObjects     func(ctx context.Context, actor *authz.Actor) (map[string]struct{}, error)
	GetModifyAllObjects   func(ctx context.Context, actor *authz.Actor) (map[string]struct{}, error)
	AssertEditableFields  func(ctx context.Context, actor *authz.Actor, objectAPIName string, data map[string]any) error
	StripUnreadableFields func(ctx context.Context, actor *authz.Actor, objectAPIName string, rec map[string]any) (map[string]any, error)
}

// Composite runs sequential subrequests with @{ref.Field} substitution and AuthZ.
func (s *Service) Composite(ctx context.Context, requests []CompositeSubrequest, actor *authz.Actor, az *CompositeAuthz) (*CompositeResult, error) {
	results := make([]map[string]any, 0, len(requests))
	refs := map[string]SObjectRecord{}
	for _, req := range requests {
		body := req.Body
		if body == nil {
			body = map[string]any{}
		}
		raw, _ := json.Marshal(body)
		substituted := string(raw)
		for refID, rec := range refs {
			for field, val := range rec {
				token := "@{" + refID + "." + field + "}"
				substituted = strings.ReplaceAll(substituted, token, fmt.Sprint(val))
			}
		}
		_ = json.Unmarshal([]byte(substituted), &body)

		entry := map[string]any{"referenceId": req.ReferenceID}
		switch strings.ToUpper(req.Method) {
		case "POST":
			if az == nil || az.AssertObjectAccess == nil {
				entry["status"] = 403
				entry["body"] = map[string]any{"error": "forbidden"}
				results = append(results, entry)
				continue
			}
			if err := az.AssertObjectAccess(ctx, actor, req.Object, authz.ActionCreate); err != nil {
				entry["status"] = 403
				entry["body"] = map[string]any{"error": err.Error()}
				results = append(results, entry)
				continue
			}
			if err := assertCompositeOwnerWrite(ctx, az, actor, req.Object, body); err != nil {
				entry["status"] = 403
				entry["body"] = map[string]any{"error": err.Error()}
				results = append(results, entry)
				continue
			}
			if az.AssertEditableFields != nil {
				if err := az.AssertEditableFields(ctx, actor, req.Object, body); err != nil {
					entry["status"] = 403
					entry["body"] = map[string]any{"error": err.Error()}
					results = append(results, entry)
					continue
				}
			}
			created, err := s.Create(ctx, req.Object, body, actor)
			if err != nil {
				entry["status"] = 400
				entry["body"] = map[string]any{"error": err.Error()}
			} else {
				created, err = stripCompositeRecord(ctx, az, actor, req.Object, created)
				if err != nil {
					entry["status"] = 500
					entry["body"] = map[string]any{"error": "field visibility evaluation failed"}
					break
				}
				refs[req.ReferenceID] = created
				entry["status"] = 201
				entry["body"] = created
			}
		case "GET":
			if req.ID == "" {
				entry["status"] = 400
				entry["body"] = map[string]any{"error": "Invalid composite subrequest"}
			} else if az == nil || az.AssertObjectAccess == nil || az.CanViewRecord == nil || az.GetViewAllObjects == nil {
				entry["status"] = 403
				entry["body"] = map[string]any{"error": "forbidden"}
			} else if err := az.AssertObjectAccess(ctx, actor, req.Object, authz.ActionRead); err != nil {
				entry["status"] = 403
				entry["body"] = map[string]any{"error": err.Error()}
			} else {
				got, err := s.Get(ctx, req.Object, req.ID)
				if err != nil {
					entry["status"] = 400
					entry["body"] = map[string]any{"error": err.Error()}
				} else {
					viewAll, err := az.GetViewAllObjects(ctx, actor)
					if err != nil {
						entry["status"] = 400
						entry["body"] = map[string]any{"error": err.Error()}
					} else {
						ownerID, _ := got["OwnerId"].(string)
						createdByID, _ := got["CreatedById"].(string)
						recID, _ := got["Id"].(string)
						ok, err := az.CanViewRecord(ctx, actor, recID, ownerID, createdByID, req.Object, viewAll)
						if err != nil {
							entry["status"] = 400
							entry["body"] = map[string]any{"error": err.Error()}
						} else if !ok {
							entry["status"] = 403
							entry["body"] = map[string]any{"error": "forbidden"}
						} else {
							got, err = stripCompositeRecord(ctx, az, actor, req.Object, got)
							if err != nil {
								entry["status"] = 500
								entry["body"] = map[string]any{"error": "field visibility evaluation failed"}
							} else {
								refs[req.ReferenceID] = got
								entry["status"] = 200
								entry["body"] = got
							}
						}
					}
				}
			}
		case "PATCH":
			if req.ID == "" {
				entry["status"] = 400
				entry["body"] = map[string]any{"error": "Invalid composite subrequest"}
			} else if az == nil || az.AssertObjectAccess == nil || az.CanModifyRecord == nil || az.GetModifyAllObjects == nil {
				entry["status"] = 403
				entry["body"] = map[string]any{"error": "forbidden"}
			} else if err := az.AssertObjectAccess(ctx, actor, req.Object, authz.ActionUpdate); err != nil {
				entry["status"] = 403
				entry["body"] = map[string]any{"error": err.Error()}
			} else {
				existing, err := s.Get(ctx, req.Object, req.ID)
				if err != nil {
					entry["status"] = 400
					entry["body"] = map[string]any{"error": err.Error()}
				} else {
					modifyAll, err := az.GetModifyAllObjects(ctx, actor)
					if err != nil {
						entry["status"] = 400
						entry["body"] = map[string]any{"error": err.Error()}
					} else {
						ownerID, _ := existing["OwnerId"].(string)
						createdByID, _ := existing["CreatedById"].(string)
						ok, err := az.CanModifyRecord(ctx, actor, req.ID, ownerID, createdByID, req.Object, modifyAll)
						if err != nil {
							entry["status"] = 400
							entry["body"] = map[string]any{"error": err.Error()}
						} else if !ok {
							entry["status"] = 403
							entry["body"] = map[string]any{"error": "forbidden"}
						} else if err := assertCompositeOwnerWrite(ctx, az, actor, req.Object, body); err != nil {
							entry["status"] = 403
							entry["body"] = map[string]any{"error": err.Error()}
						} else if az.AssertEditableFields != nil {
							if err := az.AssertEditableFields(ctx, actor, req.Object, body); err != nil {
								entry["status"] = 403
								entry["body"] = map[string]any{"error": err.Error()}
							} else {
								updated, err := s.Update(ctx, req.Object, req.ID, body, actor)
								if err != nil {
									entry["status"] = 400
									entry["body"] = map[string]any{"error": err.Error()}
								} else {
									updated, err = stripCompositeRecord(ctx, az, actor, req.Object, updated)
									if err != nil {
										entry["status"] = 500
										entry["body"] = map[string]any{"error": "field visibility evaluation failed"}
									} else {
										refs[req.ReferenceID] = updated
										entry["status"] = 200
										entry["body"] = updated
									}
								}
							}
						} else {
							updated, err := s.Update(ctx, req.Object, req.ID, body, actor)
							if err != nil {
								entry["status"] = 400
								entry["body"] = map[string]any{"error": err.Error()}
							} else {
								updated, err = stripCompositeRecord(ctx, az, actor, req.Object, updated)
								if err != nil {
									entry["status"] = 500
									entry["body"] = map[string]any{"error": "field visibility evaluation failed"}
								} else {
									refs[req.ReferenceID] = updated
									entry["status"] = 200
									entry["body"] = updated
								}
							}
						}
					}
				}
			}
		case "DELETE":
			if req.ID == "" {
				entry["status"] = 400
				entry["body"] = map[string]any{"error": "Invalid composite subrequest"}
			} else if az == nil || az.AssertObjectAccess == nil || az.CanModifyRecord == nil || az.GetModifyAllObjects == nil {
				entry["status"] = 403
				entry["body"] = map[string]any{"error": "forbidden"}
			} else if err := az.AssertObjectAccess(ctx, actor, req.Object, authz.ActionDelete); err != nil {
				entry["status"] = 403
				entry["body"] = map[string]any{"error": err.Error()}
			} else {
				existing, err := s.Get(ctx, req.Object, req.ID)
				if err != nil {
					entry["status"] = 400
					entry["body"] = map[string]any{"error": err.Error()}
				} else {
					modifyAll, err := az.GetModifyAllObjects(ctx, actor)
					if err != nil {
						entry["status"] = 400
						entry["body"] = map[string]any{"error": err.Error()}
					} else {
						ownerID, _ := existing["OwnerId"].(string)
						createdByID, _ := existing["CreatedById"].(string)
						ok, err := az.CanModifyRecord(ctx, actor, req.ID, ownerID, createdByID, req.Object, modifyAll)
						if err != nil {
							entry["status"] = 400
							entry["body"] = map[string]any{"error": err.Error()}
						} else if !ok {
							entry["status"] = 403
							entry["body"] = map[string]any{"error": "forbidden"}
						} else if err := s.Delete(ctx, req.Object, req.ID, actor); err != nil {
							entry["status"] = 400
							entry["body"] = map[string]any{"error": err.Error()}
						} else {
							entry["status"] = 204
							entry["body"] = nil
						}
					}
				}
			}
		case "UPSERT":
			extField, _ := body["externalIdField"].(string)
			extField = strings.TrimSpace(extField)
			extVal, hasExt := body["externalId"]
			if extField == "" || !hasExt {
				entry["status"] = 400
				entry["body"] = map[string]any{"error": "UPSERT requires externalIdField and externalId"}
			} else {
				delete(body, "externalIdField")
				delete(body, "externalId")
				upsertAz := &UpsertAuthz{}
				if az != nil {
					upsertAz.AssertObjectAccess = az.AssertObjectAccess
					upsertAz.CanModifyRecord = az.CanModifyRecord
					upsertAz.GetModifyAllObjects = az.GetModifyAllObjects
					upsertAz.AssertEditableFields = az.AssertEditableFields
					upsertAz.StripUnreadableFields = az.StripUnreadableFields
				}
				result, err := s.Upsert(ctx, req.Object, extField, extVal, body, actor, upsertAz)
				if err != nil {
					entry["status"] = 400
					entry["body"] = map[string]any{"error": err.Error()}
				} else {
					refs[req.ReferenceID] = result.Record
					if result.Created {
						entry["status"] = 201
					} else {
						entry["status"] = 200
					}
					out := map[string]any{}
					for k, v := range result.Record {
						out[k] = v
					}
					out["created"] = result.Created
					entry["body"] = out
				}
			}
		default:
			entry["status"] = 400
			entry["body"] = map[string]any{"error": "Invalid composite subrequest"}
		}
		results = append(results, entry)
	}
	return &CompositeResult{CompositeResponse: results}, nil
}

func stripCompositeRecord(ctx context.Context, az *CompositeAuthz, actor *authz.Actor, object string, rec SObjectRecord) (SObjectRecord, error) {
	if az == nil || az.StripUnreadableFields == nil {
		return rec, nil
	}
	stripped, err := az.StripUnreadableFields(ctx, actor, object, rec)
	if err != nil {
		return nil, fmt.Errorf("strip unreadable fields: %w", err)
	}
	return SObjectRecord(stripped), nil
}

func assertCompositeOwnerWrite(ctx context.Context, az *CompositeAuthz, actor *authz.Actor, object string, body map[string]any) error {
	v, ok := body["OwnerId"]
	if !ok {
		return nil
	}
	if az == nil || az.GetModifyAllObjects == nil {
		return fmt.Errorf("%w: owner assignment authz required", authz.ErrForbidden)
	}
	modifyAll, err := az.GetModifyAllObjects(ctx, actor)
	if err != nil {
		return err
	}
	var ownerID *string
	if value, ok := v.(string); ok && value != "" {
		ownerID = &value
	}
	return authz.AssertOwnerIDWritable(actor, object, ownerID, modifyAll)
}

// BulkInsertResult is POST /bulk/:object response.
type BulkInsertResult struct {
	Created int              `json:"created"`
	Records []SObjectRecord  `json:"records"`
	Errors  []map[string]any `json:"errors"`
}

// BulkInsert creates many records, collecting per-row errors.
func (s *Service) BulkInsert(ctx context.Context, objectAPIName string, rows []map[string]any, actor *authz.Actor) (*BulkInsertResult, error) {
	created := make([]SObjectRecord, 0, len(rows))
	errorsOut := make([]map[string]any, 0)
	for i, row := range rows {
		rec, err := s.Create(ctx, objectAPIName, row, actor)
		if err != nil {
			errorsOut = append(errorsOut, map[string]any{"index": i, "message": err.Error()})
			continue
		}
		created = append(created, rec)
	}
	return &BulkInsertResult{Created: len(created), Records: created, Errors: errorsOut}, nil
}

func stringsTitle(s string) string {
	if s == "" {
		return s
	}
	return stringsToUpperFirst(s)
}

func stringsToUpperFirst(s string) string {
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] = r[0] - ('a' - 'A')
	}
	return string(r)
}
