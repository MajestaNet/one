package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/packages"
)

// ErrConflict is returned when creating a duplicate metadata row.
var ErrConflict = errors.New("conflict")

// ErrForbidden is returned for managed/customer ownership violations.
var ErrForbidden = errors.New("forbidden")

// ErrValidation is returned for invalid metadata create/update inputs.
var ErrValidation = errors.New("validation error")

// CreateOptions controls managed vs customer create semantics.
type CreateOptions struct {
	Role string // "" | "managed"
}

// DefaultCustomerPackage is the default package for customer metadata.
const DefaultCustomerPackage = "customer.default"

func isManagedPackageName(pkg *string) bool {
	return packages.IsManagedPackageName(pkg)
}

// AssertCustomerMutable rejects managed artifacts for customer mutations.
func AssertCustomerMutable(ownership, apiName, kind string) error {
	if ownership == "managed" {
		return fmt.Errorf("%w: cannot mutate managed %s %s", ErrForbidden, kind, apiName)
	}
	return nil
}

// InsertObject creates a new object (insert-only). Returns the created row.
func (s *Service) InsertObject(ctx context.Context, input ObjectDefinition, opts CreateOptions) (ObjectDefinition, error) {
	if strings.TrimSpace(input.APIName) == "" || strings.TrimSpace(input.Label) == "" {
		return ObjectDefinition{}, fmt.Errorf("%w: apiName and label are required", ErrValidation)
	}
	if input.StorageMode == "" {
		input.StorageMode = "flexible"
	}
	switch input.StorageMode {
	case db.StorageModeFlexible, db.StorageModeHighVolume:
	case db.StorageModeKernel:
		if opts.Role != "managed" {
			return ObjectDefinition{}, fmt.Errorf("%w: storageMode kernel is managed-only", ErrValidation)
		}
	default:
		return ObjectDefinition{}, fmt.Errorf("%w: unsupported storageMode: %s", ErrValidation, input.StorageMode)
	}
	if input.Features == nil {
		input.Features = map[string]bool{}
	}

	ownership := "custom"
	pkg := input.PackageName
	if opts.Role == "managed" {
		if !isManagedPackageName(pkg) {
			return ObjectDefinition{}, fmt.Errorf("%w: managed create requires a registered managed package", ErrForbidden)
		}
		ownership = "managed"
	} else {
		if input.Ownership == "managed" || isManagedPackageName(pkg) {
			return ObjectDefinition{}, fmt.Errorf("%w: cannot create managed metadata via API", ErrForbidden)
		}
		if pkg == nil {
			p := DefaultCustomerPackage
			pkg = &p
		}
	}

	featuresJSON, _ := json.Marshal(input.Features)
	var out ObjectDefinition
	var features []byte
	err := s.pool.QueryRow(ctx, `
INSERT INTO metadata_objects (api_name, label, plural_label, storage_mode, package_name, ownership, features)
VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb)
RETURNING id::text, api_name, label, plural_label, storage_mode, package_name, ownership, features`,
		input.APIName, input.Label, input.PluralLabel, input.StorageMode, pkg, ownership, string(featuresJSON),
	).Scan(&out.ID, &out.APIName, &out.Label, &out.PluralLabel, &out.StorageMode, &out.PackageName, &out.Ownership, &features)
	if err != nil {
		if isUniqueViolation(err) {
			return ObjectDefinition{}, fmt.Errorf("%w: object already exists: %s", ErrConflict, input.APIName)
		}
		return ObjectDefinition{}, err
	}
	out.Features = map[string]bool{}
	_ = json.Unmarshal(features, &out.Features)
	_ = s.BumpEpoch(ctx)
	s.cache.invalidate()
	if err := db.EnsureObjectInDataAccessCatalog(ctx, s.pool, out.APIName); err != nil {
		return ObjectDefinition{}, fmt.Errorf("ensure object data access catalog: %w", err)
	}
	if db.IsKernelStorage(out.StorageMode) {
		if err := db.EnsureUserObjectDescribeAccess(ctx, s.pool); err != nil {
			return ObjectDefinition{}, fmt.Errorf("ensure User describe access: %w", err)
		}
		return out, nil
	}
	if err := db.NewSharingStore(s.pool).EnsureObjectSharingSettings(ctx, out.APIName); err != nil {
		return ObjectDefinition{}, fmt.Errorf("ensure object sharing settings: %w", err)
	}
	if out.StorageMode == db.StorageModeHighVolume {
		if err := db.EnsureHighVolumePartition(ctx, s.pool, out.APIName); err != nil {
			return ObjectDefinition{}, fmt.Errorf("ensure high_volume partition: %w", err)
		}
	} else {
		if err := db.EnsureFlexiblePartition(ctx, s.pool, out.APIName); err != nil {
			return ObjectDefinition{}, fmt.Errorf("ensure flexible partition: %w", err)
		}
	}
	return out, nil
}

// InsertField creates a new field (insert-only).
func (s *Service) InsertField(ctx context.Context, input FieldDefinition, opts CreateOptions) (FieldDefinition, error) {
	if input.ObjectAPIName == "" || input.APIName == "" || input.Label == "" || input.FieldType == "" {
		return FieldDefinition{}, fmt.Errorf("%w: objectApiName, apiName, label, fieldType are required", ErrValidation)
	}
	if _, err := s.requireObject(ctx, input.ObjectAPIName); err != nil {
		return FieldDefinition{}, err
	}
	if err := ValidateFieldTypeCreate(&input); err != nil {
		return FieldDefinition{}, err
	}

	ownership := "custom"
	pkg := input.PackageName
	if opts.Role == "managed" {
		if !isManagedPackageName(pkg) {
			return FieldDefinition{}, fmt.Errorf("%w: managed create requires a registered managed package", ErrForbidden)
		}
		ownership = "managed"
	} else {
		if input.Ownership == "managed" || isManagedPackageName(pkg) {
			return FieldDefinition{}, fmt.Errorf("%w: cannot create managed metadata via API", ErrForbidden)
		}
		if pkg == nil {
			p := DefaultCustomerPackage
			pkg = &p
		}
		if input.KernelColumn != nil && strings.TrimSpace(*input.KernelColumn) != "" {
			return FieldDefinition{}, fmt.Errorf("%w: kernelColumn is managed-only", ErrValidation)
		}
		input.KernelColumn = nil
	}

	if err := ApplyExternalIDRules(&input); err != nil {
		return FieldDefinition{}, err
	}
	if err := ApplySearchableRules(&input); err != nil {
		return FieldDefinition{}, err
	}

	// Defaults for lookup/MD/unique indexing
	indexed := input.Indexed
	if !indexed && (input.FieldType == FieldTypeLookup || input.FieldType == FieldTypeMasterDetail || input.FieldType == FieldTypeAutonumber || input.UniqueField || input.ExternalID) {
		indexed = true
	}
	filterable := input.Filterable
	sortable := input.Sortable
	if !filterable && (opts.Role == "managed" || input.ExternalID) {
		filterable = true
	}
	if input.ExternalID {
		filterable = true
	}
	if !sortable && opts.Role == "managed" {
		sortable = true
	}
	searchable := input.Searchable
	if searchable {
		filterable = true
	}

	defJSON := []byte("null")
	if len(input.DefaultValue) > 0 {
		defJSON = input.DefaultValue
	}
	var pickJSON *string
	if len(input.PicklistValues) > 0 {
		b, _ := json.Marshal(input.PicklistValues)
		s := string(b)
		pickJSON = &s
	}
	optsJSON := encodeFieldOptions(input)

	// Match Node: create relationship row before the field when lookup/MD.
	if (input.FieldType == FieldTypeLookup || input.FieldType == FieldTypeMasterDetail) && input.ReferenceTo != nil {
		if _, err := s.requireObject(ctx, *input.ReferenceTo); err != nil {
			return FieldDefinition{}, err
		}
		cascade := input.FieldType == FieldTypeMasterDetail
		relAPIName := input.ObjectAPIName + "_" + input.APIName
		if _, err := s.pool.Exec(ctx, `
INSERT INTO metadata_relationships (api_name, from_object, to_object, relationship_type, field_api_name, cascade_delete)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (api_name) DO NOTHING`,
			relAPIName, input.ObjectAPIName, *input.ReferenceTo, input.FieldType, input.APIName, cascade); err != nil {
			return FieldDefinition{}, err
		}
	}

	var out FieldDefinition
	var def []byte
	var pick []byte
	var rawOpts []byte
	err := s.pool.QueryRow(ctx, `
INSERT INTO metadata_fields (
  object_api_name, api_name, label, field_type,
  required, unique_field, external_id, indexed, filterable, sortable, searchable,
  default_value, length, precision, scale, picklist_values,
  reference_to, relationship_name, polymorphic_type_field, package_name, ownership, field_options, kernel_column
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13,$14,$15,$16::jsonb,$17,$18,$19,$20,$21,$22::jsonb,$23)
RETURNING id::text, object_api_name, api_name, label, field_type,
  required, unique_field, external_id, indexed, filterable, sortable, searchable,
  default_value, length, precision, scale, picklist_values,
  reference_to, relationship_name, polymorphic_type_field, package_name, ownership, field_options, kernel_column`,
		input.ObjectAPIName, input.APIName, input.Label, input.FieldType,
		input.Required, input.UniqueField, input.ExternalID, indexed, filterable, sortable, searchable,
		string(defJSON), input.Length, input.Precision, input.Scale, pickJSON,
		input.ReferenceTo, input.RelationshipName, input.PolymorphicTypeField, pkg, ownership, string(optsJSON), kernelColumnPtr(input.KernelColumn),
	).Scan(
		&out.ID, &out.ObjectAPIName, &out.APIName, &out.Label, &out.FieldType,
		&out.Required, &out.UniqueField, &out.ExternalID, &out.Indexed, &out.Filterable, &out.Sortable, &out.Searchable,
		&def, &out.Length, &out.Precision, &out.Scale, &pick,
		&out.ReferenceTo, &out.RelationshipName, &out.PolymorphicTypeField, &out.PackageName, &out.Ownership, &rawOpts, &out.KernelColumn,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return FieldDefinition{}, fmt.Errorf("%w: field already exists: %s.%s", ErrConflict, input.ObjectAPIName, input.APIName)
		}
		return FieldDefinition{}, err
	}
	if len(def) > 0 {
		out.DefaultValue = json.RawMessage(def)
	}
	if len(pick) > 0 && string(pick) != "null" {
		_ = json.Unmarshal(pick, &out.PicklistValues)
	}
	applyFieldOptions(&out, rawOpts)

	if out.FieldType == FieldTypeAutonumber {
		start := 1
		if out.AutonumberStart != nil {
			start = *out.AutonumberStart
		}
		if _, err := s.pool.Exec(ctx, `
INSERT INTO autonumber_sequences (object_api_name, field_api_name, next_value)
VALUES ($1,$2,$3)
ON CONFLICT (object_api_name, field_api_name) DO NOTHING`,
			out.ObjectAPIName, out.APIName, start); err != nil {
			return FieldDefinition{}, err
		}
	}

	_ = s.BumpEpoch(ctx)
	s.cache.invalidate()
	if err := db.EnsureFieldInDataAccessCatalog(ctx, s.pool, out.ObjectAPIName, out.APIName); err != nil {
		return FieldDefinition{}, fmt.Errorf("ensure field data access catalog: %w", err)
	}
	if out.Searchable {
		s.EnqueueSearchReindex(ctx, out.ObjectAPIName)
	}
	return out, nil
}

// InsertValidationRule creates a customer validation rule.
func (s *Service) InsertValidationRule(ctx context.Context, objectAPIName string, input ValidationRuleDefinition, opts CreateOptions) (ValidationRuleDefinition, error) {
	if objectAPIName == "" || input.APIName == "" || input.Label == "" || input.ErrorMessage == "" {
		return ValidationRuleDefinition{}, fmt.Errorf("objectApiName, apiName, label, errorMessage are required")
	}
	if _, err := s.requireObject(ctx, objectAPIName); err != nil {
		return ValidationRuleDefinition{}, err
	}
	ownership := "custom"
	pkg := input.PackageName
	if opts.Role == "managed" {
		if !isManagedPackageName(pkg) {
			return ValidationRuleDefinition{}, fmt.Errorf("%w: managed create requires package core|platform", ErrForbidden)
		}
		ownership = "managed"
	} else {
		if input.Ownership == "managed" || isManagedPackageName(pkg) {
			return ValidationRuleDefinition{}, fmt.Errorf("%w: cannot create managed metadata via API", ErrForbidden)
		}
		if pkg == nil {
			p := DefaultCustomerPackage
			pkg = &p
		}
	}
	active := input.Active
	expr := input.Expression
	if len(expr) == 0 {
		expr = json.RawMessage("null")
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO metadata_validation_rules (object_api_name, api_name, label, active, error_message, expression, package_name, ownership)
VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8)`,
		objectAPIName, input.APIName, input.Label, active, input.ErrorMessage, string(expr), pkg, ownership)
	if err != nil {
		if isUniqueViolation(err) {
			return ValidationRuleDefinition{}, fmt.Errorf("%w: validation rule already exists: %s.%s", ErrConflict, objectAPIName, input.APIName)
		}
		return ValidationRuleDefinition{}, err
	}
	_ = s.BumpEpoch(ctx)
	s.cache.invalidate()
	input.Ownership = ownership
	input.PackageName = pkg
	input.Active = active
	input.Expression = expr
	return input, nil
}

// EnsureObjectManaged creates a managed object if missing (seed).
// Deprecated: prefer SyncObjectManaged for product upgrades.
func (s *Service) EnsureObjectManaged(ctx context.Context, obj ObjectDefinition) error {
	return s.SyncObjectManaged(ctx, obj)
}

// EnsureFieldManaged creates a managed field if missing (seed).
// Deprecated: prefer SyncFieldManaged for product upgrades.
func (s *Service) EnsureFieldManaged(ctx context.Context, f FieldDefinition) error {
	return s.SyncFieldManaged(ctx, f)
}

// SyncObjectManaged inserts a managed object or updates product-owned attributes.
// Refuses to overwrite a colliding customer-owned apiName.
func (s *Service) SyncObjectManaged(ctx context.Context, obj ObjectDefinition) error {
	existing, err := s.GetObject(ctx, obj.APIName)
	if errors.Is(err, ErrNotFound) {
		_, err = s.InsertObject(ctx, obj, CreateOptions{Role: "managed"})
		return err
	}
	if err != nil {
		return err
	}
	if existing.Ownership != "managed" {
		return fmt.Errorf("%w: cannot sync managed object over customer-owned %s", ErrConflict, obj.APIName)
	}
	if obj.Features == nil {
		obj.Features = map[string]bool{}
	}
	featuresJSON, _ := json.Marshal(obj.Features)
	mode := obj.StorageMode
	if mode == "" {
		mode = "flexible"
	}
	_, err = s.pool.Exec(ctx, `
UPDATE metadata_objects
SET label=$2, plural_label=$3, features=$4::jsonb, package_name=$5, storage_mode=$6, updated_at=now()
WHERE api_name=$1 AND ownership='managed'`,
		obj.APIName, obj.Label, obj.PluralLabel, string(featuresJSON), obj.PackageName, mode)
	if err != nil {
		return err
	}
	if err := db.EnsureObjectInDataAccessCatalog(ctx, s.pool, obj.APIName); err != nil {
		return fmt.Errorf("ensure object data access catalog: %w", err)
	}
	if db.IsKernelStorage(mode) {
		if err := db.EnsureUserObjectDescribeAccess(ctx, s.pool); err != nil {
			return fmt.Errorf("ensure User describe access: %w", err)
		}
		_ = s.BumpEpoch(ctx)
		s.cache.invalidate()
		return nil
	}
	if mode == db.StorageModeHighVolume {
		if err := db.EnsureHighVolumePartition(ctx, s.pool, obj.APIName); err != nil {
			return err
		}
	} else {
		if err := db.EnsureFlexiblePartition(ctx, s.pool, obj.APIName); err != nil {
			return err
		}
	}
	_ = s.BumpEpoch(ctx)
	s.cache.invalidate()
	return nil
}

// SyncFieldManaged inserts a managed field or updates product-owned attributes.
// Does not change field_type, ownership, or package. Refuses customer-owned collisions.
func (s *Service) SyncFieldManaged(ctx context.Context, f FieldDefinition) error {
	if !f.Filterable {
		f.Filterable = true
	}
	if !f.Sortable {
		f.Sortable = true
	}
	existing, err := s.GetField(ctx, f.ObjectAPIName, f.APIName)
	if errors.Is(err, ErrNotFound) {
		_, err = s.InsertField(ctx, f, CreateOptions{Role: "managed"})
		return err
	}
	if err != nil {
		return err
	}
	if existing.Ownership != "managed" {
		return fmt.Errorf("%w: cannot sync managed field over customer-owned %s.%s", ErrConflict, f.ObjectAPIName, f.APIName)
	}

	indexed := f.Indexed
	if !indexed && (existing.FieldType == "lookup" || existing.FieldType == "master_detail" || f.UniqueField || f.ExternalID) {
		indexed = true
	}
	if err := ApplyExternalIDRules(&f); err != nil {
		return err
	}
	if err := ApplySearchableRules(&f); err != nil {
		return err
	}
	if f.ExternalID {
		indexed = true
		f.Filterable = true
	}
	if f.Searchable {
		f.Filterable = true
	}
	defJSON := []byte("null")
	if len(f.DefaultValue) > 0 {
		defJSON = f.DefaultValue
	}
	var pickJSON *string
	if len(f.PicklistValues) > 0 {
		b, _ := json.Marshal(f.PicklistValues)
		s := string(b)
		pickJSON = &s
	}

	optsJSON := encodeFieldOptions(f)

	_, err = s.pool.Exec(ctx, `
UPDATE metadata_fields
SET label=$3, required=$4, unique_field=$5, external_id=$6, indexed=$7, filterable=$8, sortable=$9, searchable=$10,
    default_value=$11::jsonb, length=$12, precision=$13, scale=$14, picklist_values=$15::jsonb,
    reference_to=$16, relationship_name=$17, polymorphic_type_field=$18, package_name=$19,
    kernel_column=$20, field_options=$21::jsonb, updated_at=now()
WHERE object_api_name=$1 AND api_name=$2 AND ownership='managed'`,
		f.ObjectAPIName, f.APIName, f.Label, f.Required, f.UniqueField, f.ExternalID, indexed, f.Filterable, f.Sortable, f.Searchable,
		string(defJSON), f.Length, f.Precision, f.Scale, pickJSON,
		f.ReferenceTo, f.RelationshipName, f.PolymorphicTypeField, f.PackageName, kernelColumnPtr(f.KernelColumn), string(optsJSON),
	)
	if err != nil {
		return err
	}
	if err := db.EnsureFieldInDataAccessCatalog(ctx, s.pool, f.ObjectAPIName, f.APIName); err != nil {
		return fmt.Errorf("ensure field data access catalog: %w", err)
	}
	_ = s.BumpEpoch(ctx)
	s.cache.invalidate()
	if existing.Searchable != f.Searchable {
		s.EnqueueSearchReindex(ctx, f.ObjectAPIName)
	}
	return nil
}

// EnqueueSearchReindex queues worker job search.reindex (all searchable objects).
// Coalesces into a single pending/running job so seed and metadata writes do not flood the queue.
// Metadata writes do not fail when enqueue is skipped or errors.
func (s *Service) EnqueueSearchReindex(ctx context.Context, objectAPIName string) {
	if s.pool == nil {
		return
	}
	// objectAPIName is accepted for callers; v1 coalesces to one all-objects job.
	_ = objectAPIName
	_, _ = s.pool.Exec(ctx, `
INSERT INTO jobs (job_type, payload)
SELECT 'search.reindex', '{}'::jsonb
WHERE NOT EXISTS (
  SELECT 1 FROM jobs
  WHERE job_type = 'search.reindex' AND status IN ('pending', 'running')
)`)
}

// RecordPackageInstall upserts package_installs for a managed package version (enabled=true).
func (s *Service) RecordPackageInstall(ctx context.Context, packageName, version string) error {
	if packageName == "" || version == "" {
		return fmt.Errorf("packageName and version are required")
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO package_installs (package_name, version, enabled, applied_at)
VALUES ($1, $2, true, now())
ON CONFLICT (package_name) DO UPDATE SET
  version = EXCLUDED.version,
  enabled = true,
  applied_at = now()`,
		packageName, version)
	return err
}

// SetPackageEnabled soft-enables or soft-disables a package install row.
func (s *Service) SetPackageEnabled(ctx context.Context, packageName string, enabled bool) error {
	if packageName == "" {
		return fmt.Errorf("packageName is required")
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE package_installs SET enabled = $2 WHERE package_name = $1`, packageName, enabled)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("package install not found: %s", packageName)
	}
	return nil
}

// GetPackageInstall returns version and enabled flag, or empty version if none.
func (s *Service) GetPackageInstall(ctx context.Context, packageName string) (version string, enabled bool, err error) {
	err = s.pool.QueryRow(ctx, `
SELECT version, enabled FROM package_installs WHERE package_name=$1`, packageName).Scan(&version, &enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	return version, enabled, err
}

// ListEnabledPackageInstalls returns non-core package names that are enabled.
func (s *Service) ListEnabledPackageInstalls(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
SELECT package_name FROM package_installs
WHERE enabled = true AND package_name <> 'core'
ORDER BY package_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func kernelColumnPtr(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}

// GetPackageInstallVersion returns the installed version, or empty if none.
func (s *Service) GetPackageInstallVersion(ctx context.Context, packageName string) (string, error) {
	version, _, err := s.GetPackageInstall(ctx, packageName)
	return version, err
}

// UpdateObject updates a customer-owned object definition.
func (s *Service) UpdateObject(ctx context.Context, apiName string, patch ObjectDefinition) (ObjectDefinition, error) {
	existing, err := s.GetObject(ctx, apiName)
	if err != nil {
		return ObjectDefinition{}, err
	}
	if err := AssertCustomerMutable(existing.Ownership, apiName, "object"); err != nil {
		return ObjectDefinition{}, err
	}
	if patch.Ownership == "managed" || isManagedPackageName(patch.PackageName) {
		return ObjectDefinition{}, fmt.Errorf("%w: cannot set managed ownership via API", ErrForbidden)
	}

	label := existing.Label
	if strings.TrimSpace(patch.Label) != "" {
		label = patch.Label
	}
	plural := existing.PluralLabel
	if strings.TrimSpace(patch.PluralLabel) != "" {
		plural = patch.PluralLabel
	}
	features := existing.Features
	if patch.Features != nil {
		features = patch.Features
	}
	pkg := existing.PackageName
	if patch.PackageName != nil {
		pkg = patch.PackageName
	}
	if pkg == nil {
		p := DefaultCustomerPackage
		pkg = &p
	}
	featuresJSON, _ := json.Marshal(features)
	_, err = s.pool.Exec(ctx, `
UPDATE metadata_objects
SET label=$2, plural_label=$3, features=$4::jsonb, package_name=$5, updated_at=now()
WHERE api_name=$1 AND ownership='custom'`,
		apiName, label, plural, string(featuresJSON), pkg)
	if err != nil {
		return ObjectDefinition{}, err
	}
	_ = s.BumpEpoch(ctx)
	s.cache.invalidate()
	return s.GetObject(ctx, apiName)
}

// DeleteObject deletes a customer-owned object when it has no fields or validation rules.
func (s *Service) DeleteObject(ctx context.Context, apiName string) error {
	existing, err := s.GetObject(ctx, apiName)
	if err != nil {
		return err
	}
	if err := AssertCustomerMutable(existing.Ownership, apiName, "object"); err != nil {
		return err
	}
	var fieldCount, ruleCount int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM metadata_fields WHERE object_api_name=$1`, apiName).Scan(&fieldCount); err != nil {
		return err
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM metadata_validation_rules WHERE object_api_name=$1`, apiName).Scan(&ruleCount); err != nil {
		return err
	}
	if fieldCount > 0 || ruleCount > 0 {
		return fmt.Errorf("%w: cannot delete object %s with %d fields and %d validation rules", ErrConflict, apiName, fieldCount, ruleCount)
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name=$1 AND ownership='custom'`, apiName)
	if err != nil {
		return err
	}
	if err := db.RemoveObjectFromDataAccessCatalog(ctx, s.pool, apiName); err != nil {
		return fmt.Errorf("remove object data access catalog: %w", err)
	}
	_ = s.BumpEpoch(ctx)
	s.cache.invalidate()
	return nil
}

// FieldPatch is a partial update for customer-owned fields. Nil pointers keep existing values.
type FieldPatch struct {
	Label            *string          `json:"label"`
	Required         *bool            `json:"required"`
	UniqueField      *bool            `json:"uniqueField"`
	ExternalID       *bool            `json:"externalId"`
	Indexed          *bool            `json:"indexed"`
	Filterable       *bool            `json:"filterable"`
	Sortable         *bool            `json:"sortable"`
	Searchable       *bool            `json:"searchable"`
	DefaultValue     *json.RawMessage `json:"defaultValue"`
	Length           *int             `json:"length"`
	Precision        *int             `json:"precision"`
	Scale            *int             `json:"scale"`
	PicklistValues   *[]string        `json:"picklistValues"`
	ReferenceTo      *string          `json:"referenceTo"`
	RelationshipName *string          `json:"relationshipName"`
	PackageName      *string          `json:"packageName"`
	Ownership        *string          `json:"ownership"`
}

// UpdateField updates a customer-owned field definition from a partial patch.
func (s *Service) UpdateField(ctx context.Context, objectAPIName, apiName string, patch FieldPatch) (FieldDefinition, error) {
	existing, err := s.GetField(ctx, objectAPIName, apiName)
	if err != nil {
		return FieldDefinition{}, err
	}
	if err := AssertCustomerMutable(existing.Ownership, apiName, "field"); err != nil {
		return FieldDefinition{}, err
	}
	if patch.Ownership != nil && *patch.Ownership == "managed" {
		return FieldDefinition{}, fmt.Errorf("%w: cannot set managed ownership via API", ErrForbidden)
	}
	if isManagedPackageName(patch.PackageName) {
		return FieldDefinition{}, fmt.Errorf("%w: cannot set managed ownership via API", ErrForbidden)
	}

	label := existing.Label
	if patch.Label != nil && strings.TrimSpace(*patch.Label) != "" {
		label = *patch.Label
	}
	required := existing.Required
	if patch.Required != nil {
		required = *patch.Required
	}
	unique := existing.UniqueField
	if patch.UniqueField != nil {
		unique = *patch.UniqueField
	}
	externalID := existing.ExternalID
	if patch.ExternalID != nil {
		externalID = *patch.ExternalID
	}
	indexed := existing.Indexed
	if patch.Indexed != nil {
		indexed = *patch.Indexed
	}
	if !indexed && (existing.FieldType == "lookup" || existing.FieldType == "master_detail" || unique || externalID) {
		indexed = true
	}
	filterable := existing.Filterable
	if patch.Filterable != nil {
		filterable = *patch.Filterable
	}
	sortable := existing.Sortable
	if patch.Sortable != nil {
		sortable = *patch.Sortable
	}
	searchable := existing.Searchable
	if patch.Searchable != nil {
		searchable = *patch.Searchable
	}
	candidate := existing
	candidate.UniqueField = unique
	candidate.ExternalID = externalID
	candidate.Indexed = indexed
	candidate.Filterable = filterable
	candidate.Searchable = searchable
	if err := ApplyExternalIDRules(&candidate); err != nil {
		return FieldDefinition{}, err
	}
	if err := ApplySearchableRules(&candidate); err != nil {
		return FieldDefinition{}, err
	}
	unique = candidate.UniqueField
	externalID = candidate.ExternalID
	indexed = candidate.Indexed
	filterable = candidate.Filterable
	searchable = candidate.Searchable
	if externalID {
		indexed = true
		filterable = true
	}
	if searchable {
		filterable = true
	}
	length := existing.Length
	if patch.Length != nil {
		length = patch.Length
	}
	precision := existing.Precision
	if patch.Precision != nil {
		precision = patch.Precision
	}
	scale := existing.Scale
	if patch.Scale != nil {
		scale = patch.Scale
	}
	defJSON := []byte("null")
	if patch.DefaultValue != nil {
		if len(*patch.DefaultValue) > 0 {
			defJSON = *patch.DefaultValue
		}
	} else if len(existing.DefaultValue) > 0 {
		defJSON = existing.DefaultValue
	}
	picklist := existing.PicklistValues
	if patch.PicklistValues != nil {
		picklist = *patch.PicklistValues
	}
	var pickJSON *string
	if len(picklist) > 0 {
		b, _ := json.Marshal(picklist)
		s := string(b)
		pickJSON = &s
	}
	ref := existing.ReferenceTo
	if patch.ReferenceTo != nil {
		ref = patch.ReferenceTo
	}
	rel := existing.RelationshipName
	if patch.RelationshipName != nil {
		rel = patch.RelationshipName
	}
	pkg := existing.PackageName
	if patch.PackageName != nil {
		pkg = patch.PackageName
	}
	if pkg == nil {
		p := DefaultCustomerPackage
		pkg = &p
	}

	_, err = s.pool.Exec(ctx, `
UPDATE metadata_fields
SET label=$3, required=$4, unique_field=$5, external_id=$6, indexed=$7, filterable=$8, sortable=$9, searchable=$10,
    default_value=$11::jsonb, length=$12, precision=$13, scale=$14, picklist_values=$15::jsonb,
    reference_to=$16, relationship_name=$17, package_name=$18, updated_at=now()
WHERE object_api_name=$1 AND api_name=$2 AND ownership='custom'`,
		objectAPIName, apiName, label, required, unique, externalID, indexed, filterable, sortable, searchable,
		string(defJSON), length, precision, scale, pickJSON, ref, rel, pkg,
	)
	if err != nil {
		return FieldDefinition{}, err
	}
	_ = s.BumpEpoch(ctx)
	s.cache.invalidate()
	if existing.Searchable != searchable {
		s.EnqueueSearchReindex(ctx, objectAPIName)
	}
	return s.GetField(ctx, objectAPIName, apiName)
}

// DeleteField deletes a customer-owned field definition.
func (s *Service) DeleteField(ctx context.Context, objectAPIName, apiName string) error {
	existing, err := s.GetField(ctx, objectAPIName, apiName)
	if err != nil {
		return err
	}
	if err := AssertCustomerMutable(existing.Ownership, apiName, "field"); err != nil {
		return err
	}
	relAPIName := objectAPIName + "_" + apiName
	_, _ = s.pool.Exec(ctx, `DELETE FROM metadata_relationships WHERE api_name=$1`, relAPIName)
	ct, err := s.pool.Exec(ctx, `
DELETE FROM metadata_fields WHERE object_api_name=$1 AND api_name=$2 AND ownership='custom'`,
		objectAPIName, apiName)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := db.RemoveFieldFromDataAccessCatalog(ctx, s.pool, objectAPIName, apiName); err != nil {
		return fmt.Errorf("remove field data access catalog: %w", err)
	}
	_ = s.BumpEpoch(ctx)
	s.cache.invalidate()
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return !errors.Is(err, pgx.ErrNoRows) && strings.Contains(err.Error(), "duplicate key")
}
