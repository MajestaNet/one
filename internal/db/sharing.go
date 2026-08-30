package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// DefaultAccess values for object OWD.
const (
	DefaultAccessPrivate         = "private"
	DefaultAccessPublicRead      = "public_read"
	DefaultAccessPublicReadWrite = "public_read_write"
)

// OrganizationSettings is the single-row install sharing latch.
type OrganizationSettings struct {
	RecordSharingEnabled   bool
	RecordSharingEnabledAt *time.Time
}

// ObjectSharingSettings is per-object OWD and rule enablement.
type ObjectSharingSettings struct {
	ObjectAPIName       string
	DefaultAccess       string
	SharingRulesEnabled bool
	UpdatedAt           time.Time
}

// SharingRule is a criteria-based sharing rule.
type SharingRule struct {
	ID                 string
	ObjectAPIName      string
	APIName            string
	Label              string
	Active             bool
	AccessLevel        string
	SharedToDataRoleID string
	Criteria           json.RawMessage
	SortOrder          int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// SharingStore persists sharing configuration and grants.
type SharingStore struct {
	pool *Pool
}

// NewSharingStore constructs a sharing store.
func NewSharingStore(pool *Pool) *SharingStore {
	return &SharingStore{pool: pool}
}

// GetOrganizationSettings loads install sharing settings.
func (s *SharingStore) GetOrganizationSettings(ctx context.Context) (*OrganizationSettings, error) {
	var out OrganizationSettings
	var enabledAt *time.Time
	err := s.pool.QueryRow(ctx, `
SELECT record_sharing_enabled, record_sharing_enabled_at
FROM organization_settings WHERE id = true`,
	).Scan(&out.RecordSharingEnabled, &enabledAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &OrganizationSettings{}, nil
		}
		return nil, err
	}
	out.RecordSharingEnabledAt = enabledAt
	return &out, nil
}

// EnableRecordSharing sets the irreversible sharing latch.
func (s *SharingStore) EnableRecordSharing(ctx context.Context) (*OrganizationSettings, error) {
	var enabledAt time.Time
	err := s.pool.QueryRow(ctx, `
UPDATE organization_settings
SET record_sharing_enabled = true,
    record_sharing_enabled_at = COALESCE(record_sharing_enabled_at, now())
WHERE id = true
RETURNING record_sharing_enabled, record_sharing_enabled_at`,
	).Scan(new(bool), &enabledAt)
	if err != nil {
		return nil, err
	}
	return &OrganizationSettings{RecordSharingEnabled: true, RecordSharingEnabledAt: &enabledAt}, nil
}

// ListObjectSharingSettings returns OWD for all objects.
func (s *SharingStore) ListObjectSharingSettings(ctx context.Context) ([]ObjectSharingSettings, error) {
	rows, err := s.pool.Query(ctx, `
SELECT object_api_name, default_access, sharing_rules_enabled, updated_at
FROM object_sharing_settings ORDER BY object_api_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ObjectSharingSettings
	for rows.Next() {
		var o ObjectSharingSettings
		if err := rows.Scan(&o.ObjectAPIName, &o.DefaultAccess, &o.SharingRulesEnabled, &o.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// GetObjectSharingSettings loads OWD for one object.
func (s *SharingStore) GetObjectSharingSettings(ctx context.Context, objectAPIName string) (*ObjectSharingSettings, error) {
	var o ObjectSharingSettings
	err := s.pool.QueryRow(ctx, `
SELECT object_api_name, default_access, sharing_rules_enabled, updated_at
FROM object_sharing_settings WHERE object_api_name = $1`, objectAPIName,
	).Scan(&o.ObjectAPIName, &o.DefaultAccess, &o.SharingRulesEnabled, &o.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &o, nil
}

// EnsureObjectSharingSettings inserts default OWD row for a new object.
func (s *SharingStore) EnsureObjectSharingSettings(ctx context.Context, objectAPIName string) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO object_sharing_settings (object_api_name, default_access, sharing_rules_enabled)
VALUES ($1, 'private', false)
ON CONFLICT (object_api_name) DO NOTHING`, objectAPIName)
	return err
}

// UpdateObjectSharingSettings patches OWD fields.
func (s *SharingStore) UpdateObjectSharingSettings(ctx context.Context, objectAPIName, defaultAccess string, sharingRulesEnabled *bool) (*ObjectSharingSettings, error) {
	if defaultAccess != "" {
		switch defaultAccess {
		case DefaultAccessPrivate, DefaultAccessPublicRead, DefaultAccessPublicReadWrite:
		default:
			return nil, fmt.Errorf("%w: invalid defaultAccess", ErrValidation)
		}
	}
	setParts := []string{"updated_at = now()"}
	args := []any{objectAPIName}
	if defaultAccess != "" {
		args = append(args, defaultAccess)
		setParts = append(setParts, fmt.Sprintf("default_access = $%d", len(args)))
	}
	if sharingRulesEnabled != nil {
		args = append(args, *sharingRulesEnabled)
		setParts = append(setParts, fmt.Sprintf("sharing_rules_enabled = $%d", len(args)))
	}
	q := fmt.Sprintf(`
UPDATE object_sharing_settings SET %s WHERE object_api_name = $1
RETURNING object_api_name, default_access, sharing_rules_enabled, updated_at`,
		strings.Join(setParts, ", "))
	var o ObjectSharingSettings
	err := s.pool.QueryRow(ctx, q, args...).Scan(&o.ObjectAPIName, &o.DefaultAccess, &o.SharingRulesEnabled, &o.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &o, nil
}

// CountActiveSharingRules counts active rules on an object.
func (s *SharingStore) CountActiveSharingRules(ctx context.Context, objectAPIName string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
SELECT count(*) FROM sharing_rules WHERE object_api_name = $1 AND active = true`, objectAPIName,
	).Scan(&n)
	return n, err
}

// ListSharingRules lists rules for an object.
func (s *SharingStore) ListSharingRules(ctx context.Context, objectAPIName string) ([]SharingRule, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, object_api_name, api_name, label, active, access_level,
       shared_to_data_role_id::text, criteria, sort_order, created_at, updated_at
FROM sharing_rules WHERE object_api_name = $1 ORDER BY sort_order, api_name`, objectAPIName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSharingRules(rows)
}

// GetSharingRule loads one rule by object + api name.
func (s *SharingStore) GetSharingRule(ctx context.Context, objectAPIName, apiName string) (*SharingRule, error) {
	row := s.pool.QueryRow(ctx, `
SELECT id::text, object_api_name, api_name, label, active, access_level,
       shared_to_data_role_id::text, criteria, sort_order, created_at, updated_at
FROM sharing_rules WHERE object_api_name = $1 AND api_name = $2`, objectAPIName, apiName)
	return scanSharingRule(row)
}

// CreateSharingRule inserts a sharing rule.
func (s *SharingStore) CreateSharingRule(ctx context.Context, rule SharingRule) (*SharingRule, error) {
	if rule.AccessLevel != "read" && rule.AccessLevel != "read_write" {
		return nil, fmt.Errorf("%w: accessLevel must be read or read_write", ErrValidation)
	}
	if len(rule.Criteria) == 0 {
		rule.Criteria = json.RawMessage(`{"filters":[]}`)
	}
	if err := validateSharingCriteria(rule.Criteria); err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
INSERT INTO sharing_rules (object_api_name, api_name, label, active, access_level, shared_to_data_role_id, criteria, sort_order)
VALUES ($1, $2, $3, $4, $5, $6::uuid, $7::jsonb, $8)
RETURNING id::text, object_api_name, api_name, label, active, access_level,
          shared_to_data_role_id::text, criteria, sort_order, created_at, updated_at`,
		rule.ObjectAPIName, rule.APIName, rule.Label, rule.Active, rule.AccessLevel,
		rule.SharedToDataRoleID, rule.Criteria, rule.SortOrder,
	)
	return scanSharingRule(row)
}

// UpdateSharingRule patches a sharing rule.
func (s *SharingStore) UpdateSharingRule(ctx context.Context, objectAPIName, apiName string, patch map[string]any) (*SharingRule, error) {
	existing, err := s.GetSharingRule(ctx, objectAPIName, apiName)
	if err != nil {
		return nil, err
	}
	label := existing.Label
	active := existing.Active
	accessLevel := existing.AccessLevel
	sharedTo := existing.SharedToDataRoleID
	criteria := existing.Criteria
	sortOrder := existing.SortOrder
	if v, ok := patch["label"].(string); ok && strings.TrimSpace(v) != "" {
		label = v
	}
	if v, ok := patch["active"].(bool); ok {
		active = v
	}
	if v, ok := patch["accessLevel"].(string); ok && v != "" {
		if v != "read" && v != "read_write" {
			return nil, fmt.Errorf("%w: accessLevel must be read or read_write", ErrValidation)
		}
		accessLevel = v
	}
	if v, ok := patch["sharedToDataRoleId"].(string); ok && v != "" {
		sharedTo = v
	}
	if v, ok := patch["criteria"]; ok {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid criteria", ErrValidation)
		}
		if err := validateSharingCriteria(b); err != nil {
			return nil, err
		}
		criteria = b
	}
	if v, ok := patch["sortOrder"].(float64); ok {
		sortOrder = int(v)
	}
	row := s.pool.QueryRow(ctx, `
UPDATE sharing_rules SET label=$3, active=$4, access_level=$5, shared_to_data_role_id=$6::uuid,
  criteria=$7::jsonb, sort_order=$8, updated_at=now()
WHERE object_api_name=$1 AND api_name=$2
RETURNING id::text, object_api_name, api_name, label, active, access_level,
          shared_to_data_role_id::text, criteria, sort_order, created_at, updated_at`,
		objectAPIName, apiName, label, active, accessLevel, sharedTo, criteria, sortOrder,
	)
	return scanSharingRule(row)
}

// DeleteSharingRule removes a rule.
func (s *SharingStore) DeleteSharingRule(ctx context.Context, objectAPIName, apiName string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sharing_rules WHERE object_api_name=$1 AND api_name=$2`, objectAPIName, apiName)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteRuleGrants removes materialized grants for a rule.
func (s *SharingStore) DeleteRuleGrants(ctx context.Context, ruleID string) error {
	_, err := s.pool.Exec(ctx, `
DELETE FROM record_access_grants WHERE row_cause = 'rule' AND source_id = $1::uuid`, ruleID)
	return err
}

// UpsertRuleGrant inserts or updates a materialized rule grant.
func (s *SharingStore) UpsertRuleGrant(ctx context.Context, recordID, objectAPIName, userID, accessLevel, ruleID string) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO record_access_grants (record_id, object_api_name, user_id, access_level, row_cause, source_id)
VALUES ($1::uuid, $2, $3::uuid, $4, 'rule', $5::uuid)
ON CONFLICT (object_api_name, record_id, user_id, row_cause, source_id) DO UPDATE SET access_level = EXCLUDED.access_level`,
		recordID, objectAPIName, userID, accessLevel, ruleID)
	return err
}

// HasRecordGrant reports whether the user has a materialized grant on the record.
func (s *SharingStore) HasRecordGrant(ctx context.Context, recordID, userID string, needWrite bool) (bool, error) {
	return s.HasRecordGrantForObject(ctx, "", recordID, userID, needWrite)
}

// HasRecordGrantForObject reports whether the user has a grant on (object, record).
// When objectAPIName is empty, falls back to record_id-only (legacy callers).
func (s *SharingStore) HasRecordGrantForObject(ctx context.Context, objectAPIName, recordID, userID string, needWrite bool) (bool, error) {
	if needWrite {
		return s.hasRecordGrantRW(ctx, objectAPIName, recordID, userID)
	}
	var n int
	var err error
	if objectAPIName != "" {
		err = s.pool.QueryRow(ctx, `
SELECT count(*) FROM record_access_grants
WHERE object_api_name = $1 AND record_id = $2::uuid AND user_id = $3::uuid`,
			objectAPIName, recordID, userID).Scan(&n)
	} else {
		err = s.pool.QueryRow(ctx, `
SELECT count(*) FROM record_access_grants
WHERE record_id = $1::uuid AND user_id = $2::uuid`, recordID, userID).Scan(&n)
	}
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *SharingStore) hasRecordGrantRW(ctx context.Context, objectAPIName, recordID, userID string) (bool, error) {
	var n int
	var err error
	if objectAPIName != "" {
		err = s.pool.QueryRow(ctx, `
SELECT count(*) FROM record_access_grants
WHERE object_api_name = $1 AND record_id = $2::uuid AND user_id = $3::uuid AND access_level = 'read_write'`,
			objectAPIName, recordID, userID).Scan(&n)
	} else {
		err = s.pool.QueryRow(ctx, `
SELECT count(*) FROM record_access_grants
WHERE record_id = $1::uuid AND user_id = $2::uuid AND access_level = 'read_write'`,
			recordID, userID).Scan(&n)
	}
	return n > 0, err
}

// HasRecordGrantReadWrite checks for read_write grant from any source.
func (s *SharingStore) HasRecordGrantReadWrite(ctx context.Context, recordID, userID string) (bool, error) {
	return s.hasRecordGrantRW(ctx, "", recordID, userID)
}

// ListActiveSharingRulesForObject returns active rules ordered for evaluation.
func (s *SharingStore) ListActiveSharingRulesForObject(ctx context.Context, objectAPIName string) ([]SharingRule, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, object_api_name, api_name, label, active, access_level,
       shared_to_data_role_id::text, criteria, sort_order, created_at, updated_at
FROM sharing_rules WHERE object_api_name = $1 AND active = true
ORDER BY sort_order, api_name`, objectAPIName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSharingRules(rows)
}

func scanSharingRules(rows pgx.Rows) ([]SharingRule, error) {
	var out []SharingRule
	for rows.Next() {
		var r SharingRule
		if err := rows.Scan(&r.ID, &r.ObjectAPIName, &r.APIName, &r.Label, &r.Active, &r.AccessLevel,
			&r.SharedToDataRoleID, &r.Criteria, &r.SortOrder, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanSharingRule(row pgx.Row) (*SharingRule, error) {
	var r SharingRule
	err := row.Scan(&r.ID, &r.ObjectAPIName, &r.APIName, &r.Label, &r.Active, &r.AccessLevel,
		&r.SharedToDataRoleID, &r.Criteria, &r.SortOrder, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// validateSharingCriteria requires at least one filter so rules cannot silently share all records.
func validateSharingCriteria(raw json.RawMessage) error {
	var body struct {
		Filters []json.RawMessage `json:"filters"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return fmt.Errorf("%w: invalid criteria JSON", ErrValidation)
	}
	if len(body.Filters) == 0 {
		return fmt.Errorf("%w: criteria.filters must contain at least one filter", ErrValidation)
	}
	return nil
}

// EnqueueSharingRecalc inserts a sharing.recalc job when record sharing is enabled.
func EnqueueSharingRecalc(ctx context.Context, pool *Pool, payload map[string]any) error {
	org, err := NewSharingStore(pool).GetOrganizationSettings(ctx)
	if err != nil || !org.RecordSharingEnabled {
		return nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `INSERT INTO jobs (job_type, payload) VALUES ('sharing.recalc', $1::jsonb)`, string(b))
	return err
}
