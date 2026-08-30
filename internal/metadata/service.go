package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned when metadata is missing.
var ErrNotFound = errors.New("not found")

// Service is the metadata kernel read API (Phase 3).
type Service struct {
	pool  *db.Pool
	cache *Cache
	now   func() time.Time
}

// NewService constructs a metadata service.
func NewService(pool *db.Pool) *Service {
	return &Service{
		pool:  pool,
		cache: NewCache(DefaultCacheTTLMs),
		now:   time.Now,
	}
}

// Pool returns the underlying database pool (for seed/module side effects).
func (s *Service) Pool() *db.Pool {
	if s == nil {
		return nil
	}
	return s.pool
}

// ListObjects returns all metadata objects (read-through + cache populate).
func (s *Service) ListObjects(ctx context.Context) ([]ObjectDefinition, error) {
	if err := s.ensureCacheCoherent(ctx); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT id::text, api_name, label, plural_label, storage_mode, package_name, ownership, features
FROM metadata_objects ORDER BY api_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var objs []ObjectDefinition
	for rows.Next() {
		o, err := scanObject(rows)
		if err != nil {
			return nil, err
		}
		objs = append(objs, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.cache.setObjects(objs)
	return objs, nil
}

// Describe returns a full object describe.
func (s *Service) Describe(ctx context.Context, apiName string) (*DescribeObject, error) {
	if err := s.ensureCacheCoherent(ctx); err != nil {
		return nil, err
	}
	if d, ok := s.cache.getDescribe(apiName); ok {
		cp := d
		return &cp, nil
	}
	obj, err := s.requireObject(ctx, apiName)
	if err != nil {
		return nil, err
	}
	fields, err := s.GetFields(ctx, apiName)
	if err != nil {
		return nil, err
	}
	rules, err := s.listValidationRules(ctx, apiName)
	if err != nil {
		return nil, err
	}
	desc := DescribeObject{
		ObjectDefinition: obj,
		Fields:           fields,
		ValidationRules:  rules,
		Limits:           DefaultQueryLimits,
	}
	s.cache.setDescribe(desc)
	return &desc, nil
}

// DescribeGlobal returns the Client global describe payload.
func (s *Service) DescribeGlobal(ctx context.Context) (*GlobalDescribe, error) {
	objects, err := s.ListObjects(ctx)
	if err != nil {
		return nil, err
	}
	sobjects := make([]GlobalSObjectRef, 0, len(objects))
	for _, o := range objects {
		sobjects = append(sobjects, GlobalSObjectRef{
			Name:        o.APIName,
			Label:       o.Label,
			LabelPlural: o.PluralLabel,
			Custom:      o.Ownership == "custom",
			Ownership:   o.Ownership,
			PackageName: o.PackageName,
			StorageMode: o.StorageMode,
		})
	}
	return &GlobalDescribe{
		Encoding:     "UTF-8",
		MaxBatchSize: 2000,
		Limits:       DefaultQueryLimits,
		SObjects:     sobjects,
	}, nil
}

// GetFields returns fields for an object.
func (s *Service) GetFields(ctx context.Context, objectAPIName string) ([]FieldDefinition, error) {
	if err := s.ensureCacheCoherent(ctx); err != nil {
		return nil, err
	}
	if f, ok := s.cache.getFields(objectAPIName); ok {
		return f, nil
	}
	if _, err := s.requireObject(ctx, objectAPIName); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT `+fieldSelectColumns+`
FROM metadata_fields WHERE object_api_name = $1 ORDER BY api_name`, objectAPIName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var fields []FieldDefinition
	for rows.Next() {
		f, err := scanField(rows)
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.cache.setFields(objectAPIName, fields)
	return fields, nil
}

func (s *Service) listValidationRules(ctx context.Context, objectAPIName string) ([]ValidationRuleDefinition, error) {
	rows, err := s.pool.Query(ctx, `
SELECT api_name, label, active, error_message, expression, package_name, ownership
FROM metadata_validation_rules WHERE object_api_name = $1 ORDER BY api_name`, objectAPIName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []ValidationRuleDefinition
	for rows.Next() {
		var r ValidationRuleDefinition
		var pkg *string
		var ownership *string
		var expr []byte
		if err := rows.Scan(&r.APIName, &r.Label, &r.Active, &r.ErrorMessage, &expr, &pkg, &ownership); err != nil {
			return nil, err
		}
		r.Expression = json.RawMessage(expr)
		if r.Expression == nil {
			r.Expression = json.RawMessage("null")
		}
		r.PackageName = pkg
		if ownership != nil && *ownership != "" {
			r.Ownership = *ownership
		} else {
			r.Ownership = "custom"
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (s *Service) requireObject(ctx context.Context, apiName string) (ObjectDefinition, error) {
	if o, ok := s.cache.getObject(apiName); ok {
		return o, nil
	}
	row := s.pool.QueryRow(ctx, `
SELECT id::text, api_name, label, plural_label, storage_mode, package_name, ownership, features
FROM metadata_objects WHERE api_name = $1`, apiName)
	o, err := scanObject(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return ObjectDefinition{}, fmt.Errorf("%w: object not found: %s", ErrNotFound, apiName)
	}
	if err != nil {
		return ObjectDefinition{}, err
	}
	s.cache.setObject(o)
	return o, nil
}

func (s *Service) ensureCacheCoherent(ctx context.Context) error {
	nowMs := s.now().UnixMilli()
	if s.cache.isEpochCheckFresh(nowMs) {
		return nil
	}
	var epoch int64 = 1
	err := s.pool.QueryRow(ctx, `SELECT epoch FROM metadata_cache_epoch WHERE id = 1`).Scan(&epoch)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		epoch = 1
	}
	local := s.cache.getEpoch()
	if local != 0 && local != epoch {
		s.cache.invalidate()
	}
	s.cache.setEpoch(epoch, nowMs)
	return nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanObject(row scannable) (ObjectDefinition, error) {
	var o ObjectDefinition
	var pkg *string
	var ownership *string
	var features []byte
	if err := row.Scan(&o.ID, &o.APIName, &o.Label, &o.PluralLabel, &o.StorageMode, &pkg, &ownership, &features); err != nil {
		return o, err
	}
	o.PackageName = pkg
	if ownership != nil && *ownership != "" {
		o.Ownership = *ownership
	} else {
		o.Ownership = "custom"
	}
	o.Features = map[string]bool{}
	if len(features) > 0 && string(features) != "null" {
		_ = json.Unmarshal(features, &o.Features)
	}
	return o, nil
}

func scanField(row scannable) (FieldDefinition, error) {
	var f FieldDefinition
	var def []byte
	var pick []byte
	var opts []byte
	var pkg, ref, rel, polyType *string
	var ownership *string
	if err := row.Scan(
		&f.ID, &f.ObjectAPIName, &f.APIName, &f.Label, &f.FieldType,
		&f.Required, &f.UniqueField, &f.ExternalID, &f.Indexed, &f.Filterable, &f.Sortable, &f.Searchable,
		&def, &f.Length, &f.Precision, &f.Scale, &pick,
		&ref, &rel, &polyType, &pkg, &ownership, &opts, &f.KernelColumn,
	); err != nil {
		return f, err
	}
	if len(def) > 0 {
		f.DefaultValue = json.RawMessage(def)
	} else {
		f.DefaultValue = json.RawMessage("null")
	}
	if len(pick) > 0 && string(pick) != "null" {
		_ = json.Unmarshal(pick, &f.PicklistValues)
	}
	f.ReferenceTo = ref
	f.RelationshipName = rel
	f.PolymorphicTypeField = polyType
	f.PackageName = pkg
	if ownership != nil && *ownership != "" {
		f.Ownership = *ownership
	} else {
		f.Ownership = "custom"
	}
	applyFieldOptions(&f, opts)
	return f, nil
}

func applyFieldOptions(f *FieldDefinition, opts []byte) {
	if len(opts) == 0 || string(opts) == "null" || string(opts) == "{}" {
		return
	}
	var m map[string]any
	if err := json.Unmarshal(opts, &m); err != nil {
		return
	}
	if v, ok := m["autonumberFormat"].(string); ok && v != "" {
		f.AutonumberFormat = &v
	}
	switch v := m["autonumberStart"].(type) {
	case float64:
		n := int(v)
		f.AutonumberStart = &n
	case json.Number:
		if i, err := v.Int64(); err == nil {
			n := int(i)
			f.AutonumberStart = &n
		}
	}
}

func encodeFieldOptions(f FieldDefinition) []byte {
	m := map[string]any{}
	if f.AutonumberFormat != nil && *f.AutonumberFormat != "" {
		m["autonumberFormat"] = *f.AutonumberFormat
	}
	if f.AutonumberStart != nil {
		m["autonumberStart"] = *f.AutonumberStart
	}
	if len(m) == 0 {
		return []byte("{}")
	}
	b, _ := json.Marshal(m)
	return b
}

const fieldSelectColumns = `
id::text, object_api_name, api_name, label, field_type,
required, unique_field, external_id, indexed, filterable, sortable, searchable,
default_value, length, precision, scale, picklist_values,
reference_to, relationship_name, polymorphic_type_field, package_name, ownership, field_options, kernel_column`

// GetObject is an exported wrapper around requireObject.
func (s *Service) GetObject(ctx context.Context, apiName string) (ObjectDefinition, error) {
	return s.requireObject(ctx, apiName)
}

// GetField returns a single field definition for an object, or ErrNotFound.
func (s *Service) GetField(ctx context.Context, objectAPIName, fieldAPIName string) (FieldDefinition, error) {
	if err := s.ensureCacheCoherent(ctx); err != nil {
		return FieldDefinition{}, err
	}
	// Check cache first.
	if fields, ok := s.cache.getFields(objectAPIName); ok {
		for _, f := range fields {
			if f.APIName == fieldAPIName {
				return f, nil
			}
		}
		return FieldDefinition{}, fmt.Errorf("%w: field not found: %s.%s", ErrNotFound, objectAPIName, fieldAPIName)
	}
	// Fallback: query DB directly.
	row := s.pool.QueryRow(ctx, `
SELECT `+fieldSelectColumns+`
FROM metadata_fields WHERE object_api_name = $1 AND api_name = $2`,
		objectAPIName, fieldAPIName)
	f, err := scanField(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return FieldDefinition{}, fmt.Errorf("%w: field not found: %s.%s", ErrNotFound, objectAPIName, fieldAPIName)
	}
	return f, err
}

// InvalidateCache evicts all cached metadata and forces a fresh epoch check.
func (s *Service) InvalidateCache() {
	s.cache.invalidate()
	s.cache.ExpireEpochCheck()
}

// BumpEpoch increments the shared metadata_cache_epoch so all replicas invalidate.
func (s *Service) BumpEpoch(ctx context.Context) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE metadata_cache_epoch SET epoch = epoch + 1, updated_at = now() WHERE id = 1`)
	if err != nil {
		return err
	}
	s.InvalidateCache()
	return nil
}

// ExportCustomerSnapshot returns all customer-owned metadata objects, fields, validation
// rules, and automations as a snapshot map suitable for embedding in a BundleArtifact.
func (s *Service) ExportCustomerSnapshot(ctx context.Context) (map[string]any, error) {
	// Objects with ownership == 'custom'
	objectRows, err := s.pool.Query(ctx, `
SELECT id::text, api_name, label, plural_label, storage_mode, package_name, ownership, features
FROM metadata_objects WHERE ownership = 'custom' ORDER BY api_name`)
	if err != nil {
		return nil, err
	}
	defer objectRows.Close()
	var objects []map[string]any
	for objectRows.Next() {
		o, err := scanObject(objectRows)
		if err != nil {
			return nil, err
		}
		objects = append(objects, objectToMap(o))
	}
	if err := objectRows.Err(); err != nil {
		return nil, err
	}

	// Customer-owned fields on any object (including managed parents such as Account).
	fieldRows, err := s.pool.Query(ctx, `
SELECT `+fieldSelectColumns+`
FROM metadata_fields f
WHERE f.ownership = 'custom'
ORDER BY f.object_api_name, f.api_name`)
	if err != nil {
		return nil, err
	}
	defer fieldRows.Close()
	var fields []map[string]any
	for fieldRows.Next() {
		f, err := scanField(fieldRows)
		if err != nil {
			return nil, err
		}
		fields = append(fields, fieldToMap(f))
	}
	if err := fieldRows.Err(); err != nil {
		return nil, err
	}

	// Customer-owned validation rules (may target managed or customer objects).
	ruleRows, err := s.pool.Query(ctx, `
SELECT r.api_name, r.object_api_name, r.label, r.active, r.error_message, r.expression, r.package_name, r.ownership
FROM metadata_validation_rules r
WHERE r.ownership = 'custom'
ORDER BY r.object_api_name, r.api_name`)
	if err != nil {
		return nil, err
	}
	defer ruleRows.Close()
	var rules []map[string]any
	for ruleRows.Next() {
		var apiName, objectAPIName, label, errorMsg string
		var active bool
		var expr []byte
		var pkg, ownership *string
		if err := ruleRows.Scan(&apiName, &objectAPIName, &label, &active, &errorMsg, &expr, &pkg, &ownership); err != nil {
			return nil, err
		}
		var exprVal any
		if len(expr) > 0 {
			_ = json.Unmarshal(expr, &exprVal)
		}
		ownershipStr := "custom"
		if ownership != nil && *ownership != "" {
			ownershipStr = *ownership
		}
		r := map[string]any{
			"apiName":       apiName,
			"objectApiName": objectAPIName,
			"label":         label,
			"active":        active,
			"errorMessage":  errorMsg,
			"expression":    exprVal,
			"ownership":     ownershipStr,
		}
		if pkg != nil {
			r["packageName"] = *pkg
		}
		rules = append(rules, r)
	}
	if err := ruleRows.Err(); err != nil {
		return nil, err
	}

	// Customer-owned automations (may target managed or customer objects).
	autoRows, err := s.pool.Query(ctx, `
SELECT a.api_name, a.label, a.object_api_name, a.trigger_event, a.active, a.condition, a.actions,
       a.package_name, a.ownership, a.runtime, a.execution, a.entry_file, a.source, a.run_as_principal_id::text
FROM metadata_automations a
WHERE a.ownership = 'custom'
ORDER BY a.api_name`)
	if err != nil {
		return nil, err
	}
	defer autoRows.Close()
	var automations []map[string]any
	sources := map[string]string{}
	for autoRows.Next() {
		var apiName, label, objectAPIName, triggerEvent string
		var active bool
		var condition, actions []byte
		var pkg, ownership *string
		var runtime, execution string
		var entryFile, source, runAs *string
		if err := autoRows.Scan(
			&apiName, &label, &objectAPIName, &triggerEvent, &active, &condition, &actions,
			&pkg, &ownership, &runtime, &execution, &entryFile, &source, &runAs,
		); err != nil {
			return nil, err
		}
		var condVal, actionsVal any
		if len(condition) > 0 {
			_ = json.Unmarshal(condition, &condVal)
		}
		if len(actions) > 0 {
			_ = json.Unmarshal(actions, &actionsVal)
		}
		ownershipStr := "custom"
		if ownership != nil && *ownership != "" {
			ownershipStr = *ownership
		}
		a := map[string]any{
			"apiName":       apiName,
			"label":         label,
			"objectApiName": objectAPIName,
			"triggerEvent":  triggerEvent,
			"active":        active,
			"condition":     condVal,
			"actions":       actionsVal,
			"ownership":     ownershipStr,
			"runtime":       runtime,
			"execution":     execution,
		}
		if pkg != nil {
			a["packageName"] = *pkg
		}
		if entryFile != nil && *entryFile != "" {
			a["entryFile"] = *entryFile
		}
		if source != nil && *source != "" {
			a["source"] = *source
			if entryFile != nil && *entryFile != "" {
				sources[*entryFile] = *source
			}
		}
		if runAs != nil && *runAs != "" {
			a["runAsPrincipalId"] = *runAs
		}
		automations = append(automations, a)
	}
	if err := autoRows.Err(); err != nil {
		return nil, err
	}

	// Customer-owned AgentSpecs (agent_playbooks).
	pbRows, err := s.pool.Query(ctx, `
SELECT api_name, label, goal_template, COALESCE(instructions, ''), allowed_tools, object_scopes,
       COALESCE(allowed_skills, '[]'::jsonb), COALESCE(allowed_canvas_specs, '[]'::jsonb),
       require_approval, active, package_name, ownership,
       COALESCE(primary_section, ''), COALESCE(harness_id, ''), COALESCE(harness_version, ''),
       COALESCE(job_class, '')
FROM agent_playbooks
WHERE ownership = 'custom'
ORDER BY api_name`)
	if err != nil {
		return nil, err
	}
	defer pbRows.Close()
	var playbooks []map[string]any
	for pbRows.Next() {
		var apiName, label, goal, instructions, ownership string
		var primarySection, harnessID, harnessVersion, jobClass string
		var tools, scopes, skills, canvasSpecs []byte
		var requireApproval, active bool
		var pkg *string
		if err := pbRows.Scan(&apiName, &label, &goal, &instructions, &tools, &scopes, &skills, &canvasSpecs,
			&requireApproval, &active, &pkg, &ownership, &primarySection, &harnessID, &harnessVersion, &jobClass); err != nil {
			return nil, err
		}
		var toolsVal, scopesVal, skillsVal, canvasSpecsVal any
		if len(tools) > 0 {
			_ = json.Unmarshal(tools, &toolsVal)
		}
		if len(scopes) > 0 {
			_ = json.Unmarshal(scopes, &scopesVal)
		}
		if len(skills) > 0 {
			_ = json.Unmarshal(skills, &skillsVal)
		}
		if len(canvasSpecs) > 0 {
			_ = json.Unmarshal(canvasSpecs, &canvasSpecsVal)
		}
		if skillsVal == nil {
			skillsVal = []any{}
		}
		if canvasSpecsVal == nil {
			canvasSpecsVal = []any{}
		}
		ownershipStr := "custom"
		if ownership != "" {
			ownershipStr = ownership
		}
		p := map[string]any{
			"apiName": apiName, "label": label, "goalTemplate": goal,
			"instructions": instructions, "allowedTools": toolsVal, "objectScopes": scopesVal,
			"allowedSkills":    skillsVal,
			"allowedToolSpecs": canvasSpecsVal, "allowedCanvasSpecs": canvasSpecsVal,
			"requireApproval": requireApproval, "active": active, "ownership": ownershipStr,
			"primarySection": primarySection, "jobClass": jobClass,
			"harnessId": harnessID, "harnessVersion": harnessVersion,
		}
		if pkg != nil {
			p["packageName"] = *pkg
		}
		playbooks = append(playbooks, p)
	}
	if err := pbRows.Err(); err != nil {
		return nil, err
	}

	// Customer-owned ToolSpecs (storage: metadata_canvases; ADR-021).
	csRows, err := s.pool.Query(ctx, `
SELECT api_name, label, description, COALESCE(icon, ''), COALESCE(sort_order, 0),
       layout, nodes, data_bindings, active, package_name, ownership
FROM metadata_canvases
WHERE ownership = 'custom'
ORDER BY sort_order ASC, api_name`)
	if err != nil {
		return nil, err
	}
	defer csRows.Close()
	var canvasSpecs []map[string]any
	for csRows.Next() {
		var apiName, label, description, icon, ownership string
		var sortOrder int
		var layout, nodes, bindings []byte
		var active bool
		var pkg *string
		if err := csRows.Scan(&apiName, &label, &description, &icon, &sortOrder,
			&layout, &nodes, &bindings, &active, &pkg, &ownership); err != nil {
			return nil, err
		}
		var layoutVal, nodesVal, bindingsVal any
		_ = json.Unmarshal(layout, &layoutVal)
		_ = json.Unmarshal(nodes, &nodesVal)
		_ = json.Unmarshal(bindings, &bindingsVal)
		if bindingsVal == nil {
			bindingsVal = []any{}
		}
		ownershipStr := "custom"
		if ownership != "" {
			ownershipStr = ownership
		}
		c := map[string]any{
			"apiName": apiName, "label": label, "description": description,
			"icon": icon, "sortOrder": sortOrder,
			"layout": layoutVal, "nodes": nodesVal, "dataBindings": bindingsVal,
			"active": active, "ownership": ownershipStr,
		}
		if pkg != nil {
			c["packageName"] = *pkg
		}
		canvasSpecs = append(canvasSpecs, c)
	}
	if err := csRows.Err(); err != nil {
		return nil, err
	}

	if objects == nil {
		objects = []map[string]any{}
	}
	if fields == nil {
		fields = []map[string]any{}
	}
	if rules == nil {
		rules = []map[string]any{}
	}
	if automations == nil {
		automations = []map[string]any{}
	}
	if playbooks == nil {
		playbooks = []map[string]any{}
	}
	if canvasSpecs == nil {
		canvasSpecs = []map[string]any{}
	}

	// Non-system permission sets (definitions only; assignments excluded).
	psRows, err := s.pool.Query(ctx, `
SELECT id::text, api_name, label, description, COALESCE(system_permissions, '[]'::jsonb), all_automations
FROM permission_sets
WHERE is_system = false
ORDER BY api_name`)
	if err != nil {
		return nil, err
	}
	defer psRows.Close()
	var permissionSets []map[string]any
	for psRows.Next() {
		var id, apiName, label string
		var desc *string
		var sysRaw []byte
		var allAutomations bool
		if err := psRows.Scan(&id, &apiName, &label, &desc, &sysRaw, &allAutomations); err != nil {
			return nil, err
		}
		var sysPerms []string
		_ = json.Unmarshal(sysRaw, &sysPerms)
		if sysPerms == nil {
			sysPerms = []string{}
		}
		objPerms := []map[string]any{}
		opRows, err := s.pool.Query(ctx, `
SELECT object_api_name, can_create, can_read, can_update, can_delete, view_all, modify_all
FROM object_permissions WHERE permission_set_id=$1::uuid ORDER BY object_api_name`, id)
		if err != nil {
			return nil, err
		}
		for opRows.Next() {
			var obj string
			var c, r, u, d, va, ma bool
			if err := opRows.Scan(&obj, &c, &r, &u, &d, &va, &ma); err != nil {
				opRows.Close()
				return nil, err
			}
			objPerms = append(objPerms, map[string]any{
				"objectApiName": obj, "canCreate": c, "canRead": r, "canUpdate": u,
				"canDelete": d, "viewAll": va, "modifyAll": ma,
			})
		}
		opRows.Close()
		if err := opRows.Err(); err != nil {
			return nil, err
		}
		fieldPerms := []map[string]any{}
		fpRows, err := s.pool.Query(ctx, `
SELECT object_api_name, field_api_name, can_read, can_edit
FROM field_permissions WHERE permission_set_id=$1::uuid
ORDER BY object_api_name, field_api_name`, id)
		if err != nil {
			return nil, err
		}
		for fpRows.Next() {
			var obj, field string
			var canRead, canEdit bool
			if err := fpRows.Scan(&obj, &field, &canRead, &canEdit); err != nil {
				fpRows.Close()
				return nil, err
			}
			fieldPerms = append(fieldPerms, map[string]any{
				"objectApiName": obj, "fieldApiName": field, "canRead": canRead, "canEdit": canEdit,
			})
		}
		fpRows.Close()
		if err := fpRows.Err(); err != nil {
			return nil, err
		}
		autoSection, err := db.LoadAutomationAccessSection(ctx, s.pool, id)
		if err != nil {
			return nil, err
		}
		autoEntries := []map[string]any{}
		for _, e := range autoSection.Automations {
			autoEntries = append(autoEntries, map[string]any{"apiName": e.APIName, "canRun": e.CanRun})
		}
		ps := map[string]any{
			"apiName": apiName, "label": label, "ownership": "custom",
			"systemPermissions": sysPerms, "objectPermissions": objPerms, "fieldPermissions": fieldPerms,
			"allAutomations": allAutomations,
			"automationAccess": map[string]any{
				"allAutomations": allAutomations,
				"automations":    autoEntries,
			},
		}
		if desc != nil {
			ps["description"] = *desc
		}
		permissionSets = append(permissionSets, ps)
	}
	if err := psRows.Err(); err != nil {
		return nil, err
	}

	// Webhooks (never export secrets).
	whRows, err := s.pool.Query(ctx, `
SELECT api_name, url, event_types, active
FROM webhooks ORDER BY api_name`)
	if err != nil {
		return nil, err
	}
	defer whRows.Close()
	var webhooks []map[string]any
	for whRows.Next() {
		var apiName, url string
		var eventsRaw []byte
		var active bool
		if err := whRows.Scan(&apiName, &url, &eventsRaw, &active); err != nil {
			return nil, err
		}
		var events []string
		_ = json.Unmarshal(eventsRaw, &events)
		if events == nil {
			events = []string{"*"}
		}
		webhooks = append(webhooks, map[string]any{
			"apiName": apiName, "url": url, "eventTypes": events, "active": active, "ownership": "custom",
		})
	}
	if err := whRows.Err(); err != nil {
		return nil, err
	}

	// Connectors (never export secret ciphertext or OAuth tokens).
	connRows, err := s.pool.Query(ctx, `
SELECT api_name, label, base_url, secret_ref, allowed_methods, COALESCE(path_prefix,''), active,
       COALESCE(auth_type,'static_bearer'), COALESCE(oauth_flow,'{}'::jsonb)
FROM install_connectors ORDER BY api_name`)
	if err != nil {
		return nil, err
	}
	defer connRows.Close()
	var connectors []map[string]any
	for connRows.Next() {
		var apiName, label, baseURL, pathPrefix, authType string
		var secretRef *string
		var methodsRaw, flowRaw []byte
		var active bool
		if err := connRows.Scan(&apiName, &label, &baseURL, &secretRef, &methodsRaw, &pathPrefix, &active, &authType, &flowRaw); err != nil {
			return nil, err
		}
		var methods []string
		_ = json.Unmarshal(methodsRaw, &methods)
		var flow map[string]any
		_ = json.Unmarshal(flowRaw, &flow)
		m := map[string]any{
			"apiName": apiName, "label": label, "baseUrl": baseURL,
			"allowedMethods": methods, "pathPrefix": pathPrefix, "active": active,
			"authType": authType, "oauthFlow": flow, "ownership": "custom",
		}
		if secretRef != nil {
			m["secretRef"] = *secretRef
		}
		connectors = append(connectors, m)
	}
	if err := connRows.Err(); err != nil {
		return nil, err
	}

	if permissionSets == nil {
		permissionSets = []map[string]any{}
	}
	if webhooks == nil {
		webhooks = []map[string]any{}
	}
	if connectors == nil {
		connectors = []map[string]any{}
	}
	if automations == nil {
		automations = []map[string]any{}
	}

	dataRoles, err := exportDataRoles(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	objectSharing, err := exportObjectSharingSettings(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	sharingRules, err := exportSharingRules(ctx, s.pool)
	if err != nil {
		return nil, err
	}

	baseline, err := s.ExportManagedBaseline(ctx, "", "")
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"manifestVersion":       1,
		"ownership":             "custom",
		"objects":               objects,
		"fields":                fields,
		"validationRules":       rules,
		"automations":           automations,
		"agentPlaybooks":        playbooks,
		"tools":                 canvasSpecs,
		"canvases":              canvasSpecs,
		"permissionSets":        permissionSets,
		"webhooks":              webhooks,
		"connectors":            connectors,
		"sources":               sources,
		"dataRoles":             dataRoles,
		"objectSharingSettings": objectSharing,
		"sharingRules":          sharingRules,
		"baseline":              baseline,
	}, nil
}

// ExportManagedBaseline returns managed object/field definitions for the
// read-only .one/baseline tree (never packed or promoted).
func (s *Service) ExportManagedBaseline(ctx context.Context, productVersion, sourceInstallID string) (map[string]any, error) {
	objectRows, err := s.pool.Query(ctx, `
SELECT id::text, api_name, label, plural_label, storage_mode, package_name, ownership, features
FROM metadata_objects WHERE ownership = 'managed' ORDER BY api_name`)
	if err != nil {
		return nil, err
	}
	defer objectRows.Close()
	var objects []map[string]any
	for objectRows.Next() {
		o, err := scanObject(objectRows)
		if err != nil {
			return nil, err
		}
		objects = append(objects, objectToMap(o))
	}
	if err := objectRows.Err(); err != nil {
		return nil, err
	}

	fieldRows, err := s.pool.Query(ctx, `
SELECT `+fieldSelectColumns+`
FROM metadata_fields f
WHERE f.ownership = 'managed'
ORDER BY f.object_api_name, f.api_name`)
	if err != nil {
		return nil, err
	}
	defer fieldRows.Close()
	var fields []map[string]any
	for fieldRows.Next() {
		f, err := scanField(fieldRows)
		if err != nil {
			return nil, err
		}
		fields = append(fields, fieldToMap(f))
	}
	if err := fieldRows.Err(); err != nil {
		return nil, err
	}
	if objects == nil {
		objects = []map[string]any{}
	}
	if fields == nil {
		fields = []map[string]any{}
	}
	out := map[string]any{
		"productVersion": productVersion,
		"generatedAt":    time.Now().UTC().Format(time.RFC3339),
		"objects":        objects,
		"fields":         fields,
	}
	if sourceInstallID != "" {
		out["sourceInstallId"] = sourceInstallID
	}
	return out, nil
}

func exportDataRoles(ctx context.Context, pool *db.Pool) ([]map[string]any, error) {
	roles, err := db.NewDataRoleStore(pool).ListDataRoles(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(roles))
	for _, r := range roles {
		if r.IsSystem {
			continue
		}
		m := map[string]any{"apiName": r.APIName, "label": r.Label}
		if r.ParentDataRoleID != nil {
			if parent, err := db.NewDataRoleStore(pool).GetDataRoleByID(ctx, *r.ParentDataRoleID); err == nil {
				m["parentDataRoleApiName"] = parent.APIName
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func exportObjectSharingSettings(ctx context.Context, pool *db.Pool) ([]map[string]any, error) {
	list, err := db.NewSharingStore(pool).ListObjectSharingSettings(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list))
	for _, o := range list {
		out = append(out, map[string]any{
			"objectApiName":       o.ObjectAPIName,
			"defaultAccess":       o.DefaultAccess,
			"sharingRulesEnabled": o.SharingRulesEnabled,
		})
	}
	return out, nil
}

func exportSharingRules(ctx context.Context, pool *db.Pool) ([]map[string]any, error) {
	rows, err := pool.Query(ctx, `
SELECT sr.object_api_name, sr.api_name, sr.label, sr.active, sr.access_level, dr.api_name, sr.criteria, sr.sort_order
FROM sharing_rules sr
JOIN data_roles dr ON dr.id = sr.shared_to_data_role_id
ORDER BY sr.object_api_name, sr.sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var objectAPIName, apiName, label, accessLevel, roleAPIName string
		var active bool
		var criteria []byte
		var sortOrder int
		if err := rows.Scan(&objectAPIName, &apiName, &label, &active, &accessLevel, &roleAPIName, &criteria, &sortOrder); err != nil {
			return nil, err
		}
		var crit any
		_ = json.Unmarshal(criteria, &crit)
		out = append(out, map[string]any{
			"objectApiName":           objectAPIName,
			"apiName":                 apiName,
			"label":                   label,
			"active":                  active,
			"accessLevel":             accessLevel,
			"sharedToDataRoleApiName": roleAPIName,
			"criteria":                crit,
			"sortOrder":               sortOrder,
		})
	}
	return out, rows.Err()
}

// CreateObject upserts a metadata object row (customer-owned, for apply).
func (s *Service) CreateObject(ctx context.Context, obj ObjectDefinition) error {
	featuresJSON, err := json.Marshal(obj.Features)
	if err != nil {
		featuresJSON = []byte("{}")
	}
	mode := obj.StorageMode
	if mode == "" {
		mode = db.StorageModeFlexible
	}
	if db.IsKernelStorage(mode) {
		return fmt.Errorf("%w: storageMode kernel is managed-only", ErrValidation)
	}
	_, err = s.pool.Exec(ctx, `
INSERT INTO metadata_objects (api_name, label, plural_label, storage_mode, package_name, ownership, features)
VALUES ($1, $2, $3, $4, $5, 'custom', $6)
ON CONFLICT (api_name) DO UPDATE
  SET label        = EXCLUDED.label,
      plural_label = EXCLUDED.plural_label,
      storage_mode = EXCLUDED.storage_mode,
      package_name = EXCLUDED.package_name,
      features     = EXCLUDED.features,
      ownership    = 'custom',
      updated_at   = now()`,
		obj.APIName, obj.Label, obj.PluralLabel, mode, obj.PackageName, string(featuresJSON))
	if err != nil {
		return err
	}
	if mode == db.StorageModeHighVolume {
		if err := db.EnsureHighVolumePartition(ctx, s.pool, obj.APIName); err != nil {
			return fmt.Errorf("ensure high_volume partition: %w", err)
		}
	} else {
		if err := db.EnsureFlexiblePartition(ctx, s.pool, obj.APIName); err != nil {
			return fmt.Errorf("ensure flexible partition: %w", err)
		}
	}
	if err := db.EnsureObjectInDataAccessCatalog(ctx, s.pool, obj.APIName); err != nil {
		return fmt.Errorf("ensure object data access catalog: %w", err)
	}
	return db.NewSharingStore(s.pool).EnsureObjectSharingSettings(ctx, obj.APIName)
}

// CreateField upserts a metadata field row (customer-owned, for apply).
func (s *Service) CreateField(ctx context.Context, f FieldDefinition) error {
	defJSON, _ := json.Marshal(f.DefaultValue)
	var pickJSON []byte
	if len(f.PicklistValues) > 0 {
		pickJSON, _ = json.Marshal(f.PicklistValues)
	}
	optsJSON := encodeFieldOptions(f)
	if err := ApplyExternalIDRules(&f); err != nil {
		return err
	}
	if err := ApplySearchableRules(&f); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO metadata_fields (
  object_api_name, api_name, label, field_type,
  required, unique_field, external_id, indexed, filterable, sortable, searchable,
  default_value, length, precision, scale, picklist_values,
  reference_to, relationship_name, polymorphic_type_field, package_name, ownership, field_options
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,'custom',$21)
ON CONFLICT (object_api_name, api_name) DO UPDATE
  SET label             = EXCLUDED.label,
      field_type        = EXCLUDED.field_type,
      required          = EXCLUDED.required,
      unique_field      = EXCLUDED.unique_field,
      external_id       = EXCLUDED.external_id,
      indexed           = EXCLUDED.indexed,
      filterable        = EXCLUDED.filterable,
      sortable          = EXCLUDED.sortable,
      searchable        = EXCLUDED.searchable,
      default_value     = EXCLUDED.default_value,
      length            = EXCLUDED.length,
      precision         = EXCLUDED.precision,
      scale             = EXCLUDED.scale,
      picklist_values   = EXCLUDED.picklist_values,
      reference_to      = EXCLUDED.reference_to,
      relationship_name = EXCLUDED.relationship_name,
      polymorphic_type_field = EXCLUDED.polymorphic_type_field,
      package_name      = EXCLUDED.package_name,
      ownership         = 'custom',
      field_options     = EXCLUDED.field_options,
      updated_at        = now()`,
		f.ObjectAPIName, f.APIName, f.Label, f.FieldType,
		f.Required, f.UniqueField, f.ExternalID, f.Indexed, f.Filterable, f.Sortable, f.Searchable,
		string(defJSON), f.Length, f.Precision, f.Scale, nullIfEmpty(pickJSON),
		f.ReferenceTo, f.RelationshipName, f.PolymorphicTypeField, f.PackageName, string(optsJSON))
	if err != nil {
		return err
	}
	if f.Searchable {
		s.EnqueueSearchReindex(ctx, f.ObjectAPIName)
	}
	return nil
}

// CreateValidationRule inserts a new validation rule row (customer-owned, for apply).
// Callers should check existence and call UpdateValidationRule instead if it already exists.
func (s *Service) CreateValidationRule(ctx context.Context, r ValidationRuleDefinition, objectAPIName string) error {
	exprJSON, _ := json.Marshal(r.Expression)
	_, err := s.pool.Exec(ctx, `
INSERT INTO metadata_validation_rules (object_api_name, api_name, label, active, error_message, expression, package_name, ownership)
VALUES ($1,$2,$3,$4,$5,$6,$7,'custom')`,
		objectAPIName, r.APIName, r.Label, r.Active, r.ErrorMessage, string(exprJSON), r.PackageName)
	return err
}

// UpdateValidationRule updates an existing validation rule (customer-owned, for apply).
func (s *Service) UpdateValidationRule(ctx context.Context, r ValidationRuleDefinition, objectAPIName string) error {
	exprJSON, _ := json.Marshal(r.Expression)
	_, err := s.pool.Exec(ctx, `
UPDATE metadata_validation_rules
SET label=$3, active=$4, error_message=$5, expression=$6, package_name=$7, ownership='custom'
WHERE object_api_name=$1 AND api_name=$2`,
		objectAPIName, r.APIName, r.Label, r.Active, r.ErrorMessage, string(exprJSON), r.PackageName)
	return err
}

// CacheForTest exposes cache helpers for tests.
func (s *Service) CacheForTest() *Cache { return s.cache }

func nullIfEmpty(b []byte) *string {
	if len(b) == 0 {
		return nil
	}
	s := string(b)
	return &s
}

func objectToMap(o ObjectDefinition) map[string]any {
	m := map[string]any{
		"apiName":     o.APIName,
		"label":       o.Label,
		"pluralLabel": o.PluralLabel,
		"storageMode": o.StorageMode,
		"ownership":   o.Ownership,
		"features":    o.Features,
	}
	if o.PackageName != nil {
		m["packageName"] = *o.PackageName
	}
	return m
}

func fieldToMap(f FieldDefinition) map[string]any {
	m := map[string]any{
		"objectApiName": f.ObjectAPIName,
		"apiName":       f.APIName,
		"label":         f.Label,
		"fieldType":     f.FieldType,
		"required":      f.Required,
		"uniqueField":   f.UniqueField,
		"externalId":    f.ExternalID,
		"filterable":    f.Filterable,
		"sortable":      f.Sortable,
		"searchable":    f.Searchable,
		"ownership":     f.Ownership,
	}
	if f.Indexed {
		m["indexed"] = f.Indexed
	}
	if f.DefaultValue != nil {
		var v any
		_ = json.Unmarshal(f.DefaultValue, &v)
		m["defaultValue"] = v
	}
	if f.Length != nil {
		m["length"] = *f.Length
	}
	if f.Precision != nil {
		m["precision"] = *f.Precision
	}
	if f.Scale != nil {
		m["scale"] = *f.Scale
	}
	if len(f.PicklistValues) > 0 {
		m["picklistValues"] = f.PicklistValues
	}
	if f.ReferenceTo != nil {
		m["referenceTo"] = *f.ReferenceTo
	}
	if f.RelationshipName != nil {
		m["relationshipName"] = *f.RelationshipName
	}
	if f.PolymorphicTypeField != nil {
		m["polymorphicTypeField"] = *f.PolymorphicTypeField
	}
	if f.AutonumberFormat != nil {
		m["autonumberFormat"] = *f.AutonumberFormat
	}
	if f.AutonumberStart != nil {
		m["autonumberStart"] = *f.AutonumberStart
	}
	if f.PackageName != nil {
		m["packageName"] = *f.PackageName
	}
	if f.KernelColumn != nil && *f.KernelColumn != "" {
		m["kernelColumn"] = *f.KernelColumn
	}
	return m
}
