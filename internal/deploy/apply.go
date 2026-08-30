package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/MajestaNet/ide/internal/agentharness"
	"github.com/MajestaNet/ide/internal/canvas"
	"github.com/MajestaNet/ide/internal/connectoroauth"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/jackc/pgx/v5"
)

// ApplyBundleArtifact upserts all objects/fields/rules/automations from the artifact
// into the local database, then bumps the metadata cache epoch.
func ApplyBundleArtifact(
	ctx context.Context,
	pool *db.Pool,
	meta *metadata.Service,
	artifact *BundleArtifact,
	dryRun bool,
) (*ApplyReport, error) {
	report := &ApplyReport{
		Actions: []ApplyAction{},
	}

	// Objects
	for _, obj := range artifact.Objects {
		existing, err := meta.GetObject(ctx, obj.APIName)
		if err != nil && !errors.Is(err, metadata.ErrNotFound) {
			return nil, fmt.Errorf("get object %s: %w", obj.APIName, err)
		}
		if errors.Is(err, metadata.ErrNotFound) {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "object", APIName: obj.APIName, Action: "created",
			})
			if !dryRun {
				pkgName := obj.PackageName
				if pkgName == nil {
					s := DefaultCustomerPackage
					pkgName = &s
				}
				if err := meta.CreateObject(ctx, metadata.ObjectDefinition{
					APIName:     obj.APIName,
					Label:       obj.Label,
					PluralLabel: obj.PluralLabel,
					StorageMode: obj.StorageMode,
					PackageName: pkgName,
					Ownership:   "custom",
					Features:    obj.Features,
				}); err != nil {
					return nil, fmt.Errorf("create object %s: %w", obj.APIName, err)
				}
			}
		} else {
			needsUpdate := existing.Label != obj.Label ||
				existing.PluralLabel != obj.PluralLabel ||
				existing.StorageMode != obj.StorageMode
			if needsUpdate {
				report.Actions = append(report.Actions, ApplyAction{
					Kind: "object", APIName: obj.APIName, Action: "updated",
				})
				if !dryRun {
					pkgName := obj.PackageName
					if pkgName == nil {
						s := DefaultCustomerPackage
						pkgName = &s
					}
					if err := meta.CreateObject(ctx, metadata.ObjectDefinition{
						APIName:     obj.APIName,
						Label:       obj.Label,
						PluralLabel: obj.PluralLabel,
						StorageMode: obj.StorageMode,
						PackageName: pkgName,
						Ownership:   "custom",
						Features:    obj.Features,
					}); err != nil {
						return nil, fmt.Errorf("update object %s: %w", obj.APIName, err)
					}
				}
			} else {
				report.Actions = append(report.Actions, ApplyAction{
					Kind: "object", APIName: obj.APIName, Action: "skipped",
				})
			}
		}
	}

	// Refresh cache after object changes so field parent lookups succeed.
	if !dryRun {
		meta.InvalidateCache()
	}

	// Fields
	for _, field := range artifact.Fields {
		existing, err := meta.GetField(ctx, field.ObjectAPIName, field.APIName)
		if err != nil && !errors.Is(err, metadata.ErrNotFound) {
			return nil, fmt.Errorf("get field %s.%s: %w", field.ObjectAPIName, field.APIName, err)
		}
		if errors.Is(err, metadata.ErrNotFound) {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "field", APIName: field.APIName, ObjectAPIName: field.ObjectAPIName, Action: "created",
			})
			if !dryRun {
				pkgName := field.PackageName
				if pkgName == nil {
					s := DefaultCustomerPackage
					pkgName = &s
				}
				var defRaw json.RawMessage
				if field.DefaultValue != nil {
					b, _ := json.Marshal(field.DefaultValue)
					defRaw = b
				} else {
					defRaw = json.RawMessage("null")
				}
				indexed := false
				if field.Indexed != nil {
					indexed = *field.Indexed
				}
				searchable := false
				if field.Searchable != nil {
					searchable = *field.Searchable
				}
				if err := meta.CreateField(ctx, metadata.FieldDefinition{
					ObjectAPIName:        field.ObjectAPIName,
					APIName:              field.APIName,
					Label:                field.Label,
					FieldType:            field.FieldType,
					Required:             field.Required,
					UniqueField:          field.UniqueField,
					ExternalID:           field.ExternalID,
					Indexed:              indexed,
					Filterable:           field.Filterable,
					Sortable:             field.Sortable,
					Searchable:           searchable,
					DefaultValue:         defRaw,
					Length:               field.Length,
					Precision:            field.Precision,
					Scale:                field.Scale,
					PicklistValues:       field.PicklistValues,
					ReferenceTo:          field.ReferenceTo,
					RelationshipName:     field.RelationshipName,
					PolymorphicTypeField: field.PolymorphicTypeField,
					PackageName:          pkgName,
					Ownership:            "custom",
				}); err != nil {
					return nil, fmt.Errorf("create field %s.%s: %w", field.ObjectAPIName, field.APIName, err)
				}
			}
		} else {
			needsUpdate := existing.Label != field.Label ||
				existing.Required != field.Required ||
				existing.FieldType != field.FieldType
			if field.Searchable != nil && existing.Searchable != *field.Searchable {
				needsUpdate = true
			}
			if needsUpdate {
				report.Actions = append(report.Actions, ApplyAction{
					Kind: "field", APIName: field.APIName, ObjectAPIName: field.ObjectAPIName, Action: "updated",
				})
				if !dryRun {
					pkgName := field.PackageName
					if pkgName == nil {
						s := DefaultCustomerPackage
						pkgName = &s
					}
					var defRaw json.RawMessage
					if field.DefaultValue != nil {
						b, _ := json.Marshal(field.DefaultValue)
						defRaw = b
					} else {
						defRaw = existing.DefaultValue
					}
					indexed := existing.Indexed
					if field.Indexed != nil {
						indexed = *field.Indexed
					}
					searchable := existing.Searchable
					if field.Searchable != nil {
						searchable = *field.Searchable
					}
					if err := meta.CreateField(ctx, metadata.FieldDefinition{
						ObjectAPIName:        field.ObjectAPIName,
						APIName:              field.APIName,
						Label:                field.Label,
						FieldType:            field.FieldType,
						Required:             field.Required,
						UniqueField:          field.UniqueField,
						ExternalID:           field.ExternalID,
						Indexed:              indexed,
						Filterable:           field.Filterable,
						Sortable:             field.Sortable,
						Searchable:           searchable,
						DefaultValue:         defRaw,
						Length:               field.Length,
						Precision:            field.Precision,
						Scale:                field.Scale,
						PicklistValues:       field.PicklistValues,
						ReferenceTo:          field.ReferenceTo,
						RelationshipName:     field.RelationshipName,
						PolymorphicTypeField: field.PolymorphicTypeField,
						PackageName:          pkgName,
						Ownership:            "custom",
					}); err != nil {
						return nil, fmt.Errorf("update field %s.%s: %w", field.ObjectAPIName, field.APIName, err)
					}
				}
			} else {
				report.Actions = append(report.Actions, ApplyAction{
					Kind: "field", APIName: field.APIName, ObjectAPIName: field.ObjectAPIName, Action: "skipped",
				})
			}
		}
	}

	// Validation rules
	for _, rule := range artifact.ValidationRules {
		var existingID string
		err := pool.QueryRow(ctx,
			`SELECT id::text FROM metadata_validation_rules WHERE object_api_name=$1 AND api_name=$2`,
			rule.ObjectAPIName, rule.APIName,
		).Scan(&existingID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check rule %s.%s: %w", rule.ObjectAPIName, rule.APIName, err)
		}

		pkgName := rule.PackageName
		if pkgName == nil {
			s := DefaultCustomerPackage
			pkgName = &s
		}
		rd := metadata.ValidationRuleDefinition{
			APIName:      rule.APIName,
			Label:        rule.Label,
			Active:       rule.Active,
			ErrorMessage: rule.ErrorMessage,
			PackageName:  pkgName,
			Ownership:    "custom",
		}
		if rule.Expression != nil {
			b, _ := json.Marshal(rule.Expression)
			rd.Expression = b
		}

		if errors.Is(err, pgx.ErrNoRows) {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "validationRule", APIName: rule.APIName, ObjectAPIName: rule.ObjectAPIName, Action: "created",
			})
			if !dryRun {
				if err := meta.CreateValidationRule(ctx, rd, rule.ObjectAPIName); err != nil {
					return nil, fmt.Errorf("create rule %s.%s: %w", rule.ObjectAPIName, rule.APIName, err)
				}
			}
		} else {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "validationRule", APIName: rule.APIName, ObjectAPIName: rule.ObjectAPIName, Action: "updated",
			})
			if !dryRun {
				if err := meta.UpdateValidationRule(ctx, rd, rule.ObjectAPIName); err != nil {
					return nil, fmt.Errorf("update rule %s.%s: %w", rule.ObjectAPIName, rule.APIName, err)
				}
			}
		}
	}

	// Automations
	for _, auto := range artifact.Automations {
		var existingID string
		err := pool.QueryRow(ctx,
			`SELECT id::text FROM metadata_automations WHERE api_name=$1`, auto.APIName,
		).Scan(&existingID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check automation %s: %w", auto.APIName, err)
		}

		pkgName := auto.PackageName
		if pkgName == nil {
			s := DefaultCustomerPackage
			pkgName = &s
		}
		condJSON := []byte("null")
		if auto.Condition != nil {
			condJSON, _ = json.Marshal(auto.Condition)
		}
		actJSON, _ := json.Marshal(auto.Actions)
		runtime := auto.Runtime
		if runtime == "" {
			runtime = "actions"
		}
		execution := auto.Execution
		if execution == "" {
			execution = "async"
		}
		var entryFile, source, runAs any
		if auto.EntryFile != nil {
			entryFile = *auto.EntryFile
		}
		src := auto.Source
		if (src == nil || *src == "") && auto.EntryFile != nil && artifact.Sources != nil {
			if body, ok := artifact.Sources[*auto.EntryFile]; ok {
				s := body
				src = &s
			}
		}
		if src != nil {
			source = *src
		}
		if auto.RunAsPrincipalID != nil {
			runAs = *auto.RunAsPrincipalID
		}

		if errors.Is(err, pgx.ErrNoRows) {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "automation", APIName: auto.APIName, Action: "created",
			})
			if !dryRun {
				_, err := pool.Exec(ctx, `
INSERT INTO metadata_automations (
  api_name, label, object_api_name, trigger_event, active, condition, actions,
  package_name, ownership, runtime, execution, entry_file, source, run_as_principal_id
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'custom',$9,$10,$11,$12,$13)`,
					auto.APIName, auto.Label, auto.ObjectAPIName, auto.TriggerEvent,
					auto.Active, string(condJSON), string(actJSON), *pkgName,
					runtime, execution, entryFile, source, runAs)
				if err != nil {
					return nil, fmt.Errorf("create automation %s: %w", auto.APIName, err)
				}
				if err := db.EnsureAutomationInAccessCatalog(ctx, pool, auto.APIName); err != nil {
					return nil, fmt.Errorf("automation access catalog %s: %w", auto.APIName, err)
				}
			}
		} else {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "automation", APIName: auto.APIName, Action: "updated",
			})
			if !dryRun {
				_, err := pool.Exec(ctx, `
UPDATE metadata_automations
SET label=$2, object_api_name=$3, trigger_event=$4, active=$5, condition=$6, actions=$7,
    package_name=$8, ownership='custom', runtime=$9, execution=$10, entry_file=$11, source=$12,
    run_as_principal_id=$13, updated_at=now()
WHERE api_name=$1`,
					auto.APIName, auto.Label, auto.ObjectAPIName, auto.TriggerEvent,
					auto.Active, string(condJSON), string(actJSON), *pkgName,
					runtime, execution, entryFile, source, runAs)
				if err != nil {
					return nil, fmt.Errorf("update automation %s: %w", auto.APIName, err)
				}
				if err := db.EnsureAutomationInAccessCatalog(ctx, pool, auto.APIName); err != nil {
					return nil, fmt.Errorf("automation access catalog %s: %w", auto.APIName, err)
				}
			}
		}
	}

	// AgentSpecs (agent_playbooks)
	for _, pb := range artifact.AgentPlaybooks {
		var existingID string
		err := pool.QueryRow(ctx,
			`SELECT id::text FROM agent_playbooks WHERE api_name=$1`, pb.APIName,
		).Scan(&existingID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check agentPlaybook %s: %w", pb.APIName, err)
		}

		pkgName := pb.PackageName
		if pkgName == nil {
			s := DefaultCustomerPackage
			pkgName = &s
		}
		tools := pb.AllowedTools
		if tools == nil {
			tools = []string{}
		}
		scopes := pb.ObjectScopes
		if scopes == nil {
			scopes = []string{}
		}
		skills := pb.AllowedSkills
		if skills == nil {
			skills = []string{}
		}
		// Dual-read ToolSpec allowlist (ADR-021).
		canvasSpecs := pb.AllowedToolSpecs
		if len(canvasSpecs) == 0 {
			canvasSpecs = pb.AllowedCanvasSpecs
		}
		if canvasSpecs == nil {
			canvasSpecs = []string{}
		}
		section := pb.PrimarySection
		if section == "" && strings.TrimSpace(pb.JobClass) == "" {
			section = string(agentharness.SectionOperate)
		}
		var binding agentharness.Binding
		var bindErr error
		storedJobClass := ""
		if strings.TrimSpace(pb.JobClass) != "" {
			binding, bindErr = agentharness.BindSpec(pb.JobClass, pb.PrimarySection)
			if bindErr != nil {
				return nil, fmt.Errorf("agentPlaybook %s jobClass: %w", pb.APIName, bindErr)
			}
			storedJobClass = binding.JobClass
		} else {
			binding, bindErr = agentharness.Bind(section)
			if bindErr != nil {
				return nil, fmt.Errorf("agentPlaybook %s primarySection: %w", pb.APIName, bindErr)
			}
		}
		tools = agentharness.EnsureToolFloor(binding.ToolFloor, tools)
		requireApproval := agentharness.EffectiveRequireApproval(binding.RequireApprovalDefault, pb.RequireApproval)
		toolsJSON, _ := json.Marshal(tools)
		scopesJSON, _ := json.Marshal(scopes)
		skillsJSON, _ := json.Marshal(skills)
		canvasSpecsJSON, _ := json.Marshal(canvasSpecs)

		if errors.Is(err, pgx.ErrNoRows) {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "agentPlaybook", APIName: pb.APIName, Action: "created",
			})
			if !dryRun {
				_, err := pool.Exec(ctx, `
INSERT INTO agent_playbooks (
  api_name, label, goal_template, instructions, allowed_tools, object_scopes, allowed_skills,
  allowed_canvas_specs, require_approval, active, package_name, ownership,
  primary_section, harness_id, harness_version, job_class
)
VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7::jsonb,$8::jsonb,$9,$10,$11,'custom',$12,$13,$14,$15)`,
					pb.APIName, pb.Label, pb.GoalTemplate, pb.Instructions,
					string(toolsJSON), string(scopesJSON), string(skillsJSON), string(canvasSpecsJSON),
					requireApproval, pb.Active, *pkgName,
					nullableText(binding.PrimarySection), binding.HarnessID, binding.HarnessVersion,
					nullableText(storedJobClass))
				if err != nil {
					return nil, fmt.Errorf("create agentPlaybook %s: %w", pb.APIName, err)
				}
			}
		} else {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "agentPlaybook", APIName: pb.APIName, Action: "updated",
			})
			if !dryRun {
				_, err := pool.Exec(ctx, `
UPDATE agent_playbooks
SET label=$2, goal_template=$3, instructions=$4, allowed_tools=$5::jsonb, object_scopes=$6::jsonb,
    allowed_skills=$7::jsonb, allowed_canvas_specs=$8::jsonb, require_approval=$9, active=$10,
    package_name=$11, ownership='custom', primary_section=$12, harness_id=$13, harness_version=$14,
    job_class=$15, updated_at=now()
WHERE api_name=$1 AND ownership='custom'`,
					pb.APIName, pb.Label, pb.GoalTemplate, pb.Instructions,
					string(toolsJSON), string(scopesJSON), string(skillsJSON), string(canvasSpecsJSON),
					requireApproval, pb.Active, *pkgName,
					nullableText(binding.PrimarySection), binding.HarnessID, binding.HarnessVersion,
					nullableText(storedJobClass))
				if err != nil {
					return nil, fmt.Errorf("update agentPlaybook %s: %w", pb.APIName, err)
				}
			}
		}
	}

	// CanvasSpecs (metadata_canvases)
	for _, cs := range artifact.Canvases {
		var existingID string
		err := pool.QueryRow(ctx,
			`SELECT id::text FROM metadata_canvases WHERE api_name=$1`, cs.APIName,
		).Scan(&existingID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check canvasSpec %s: %w", cs.APIName, err)
		}
		pkgName := cs.PackageName
		if pkgName == nil {
			s := DefaultCustomerPackage
			pkgName = &s
		}
		layout := cs.Layout
		if len(layout) == 0 {
			layout = json.RawMessage(`{"mode":"sections"}`)
		}
		nodes := cs.Nodes
		if len(nodes) == 0 {
			nodes = json.RawMessage(`[]`)
		}
		if sanitized, err := canvas.SanitizeNodesJSON(nodes); err != nil {
			return nil, fmt.Errorf("sanitize toolSpec %s nodes: %w", cs.APIName, err)
		} else {
			nodes = sanitized
		}
		bindings := cs.DataBindings
		if bindings == nil {
			bindings = json.RawMessage(`[]`)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "toolSpec", APIName: cs.APIName, Action: "created",
			})
			if !dryRun {
				_, err := pool.Exec(ctx, `
INSERT INTO metadata_canvases (
  api_name, label, description, icon, sort_order, layout, nodes, data_bindings, active, package_name, ownership
)
VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8::jsonb,$9,$10,'custom')`,
					cs.APIName, cs.Label, cs.Description, cs.Icon, cs.SortOrder,
					string(layout), string(nodes), string(bindings), cs.Active, *pkgName)
				if err != nil {
					return nil, fmt.Errorf("create toolSpec %s: %w", cs.APIName, err)
				}
			}
		} else {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "toolSpec", APIName: cs.APIName, Action: "updated",
			})
			if !dryRun {
				_, err := pool.Exec(ctx, `
UPDATE metadata_canvases
SET label=$2, description=$3, icon=$4, sort_order=$5, layout=$6::jsonb, nodes=$7::jsonb, data_bindings=$8::jsonb,
    active=$9, package_name=$10, ownership='custom', updated_at=now()
WHERE api_name=$1 AND ownership='custom'`,
					cs.APIName, cs.Label, cs.Description, cs.Icon, cs.SortOrder,
					string(layout), string(nodes), string(bindings), cs.Active, *pkgName)
				if err != nil {
					return nil, fmt.Errorf("update toolSpec %s: %w", cs.APIName, err)
				}
			}
		}
	}

	// Experiences (metadata_experiences)
	for _, ex := range artifact.Experiences {
		var existingID string
		err := pool.QueryRow(ctx,
			`SELECT id::text FROM metadata_experiences WHERE api_name=$1`, ex.APIName,
		).Scan(&existingID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check experience %s: %w", ex.APIName, err)
		}
		pkgName := ex.PackageName
		if pkgName == nil {
			s := DefaultCustomerPackage
			pkgName = &s
		}
		originsJSON, _ := json.Marshal(ex.AllowedOrigins)
		if originsJSON == nil {
			originsJSON = []byte("[]")
		}
		if errors.Is(err, pgx.ErrNoRows) {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "experience", APIName: ex.APIName, Action: "created",
			})
			if !dryRun {
				_, err := pool.Exec(ctx, `
INSERT INTO metadata_experiences (
  api_name, label, description, home_url, connected_app_api_name, allowed_origins, active, package_name, ownership
)
VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8,'custom')`,
					ex.APIName, ex.Label, ex.Description, ex.HomeURL, ex.ConnectedAppAPIName,
					string(originsJSON), ex.Active, *pkgName)
				if err != nil {
					return nil, fmt.Errorf("create experience %s: %w", ex.APIName, err)
				}
			}
		} else {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "experience", APIName: ex.APIName, Action: "updated",
			})
			if !dryRun {
				_, err := pool.Exec(ctx, `
UPDATE metadata_experiences
SET label=$2, description=$3, home_url=$4, connected_app_api_name=$5, allowed_origins=$6::jsonb,
    active=$7, package_name=$8, ownership='custom', updated_at=now()
WHERE api_name=$1 AND ownership='custom'`,
					ex.APIName, ex.Label, ex.Description, ex.HomeURL, ex.ConnectedAppAPIName,
					string(originsJSON), ex.Active, *pkgName)
				if err != nil {
					return nil, fmt.Errorf("update experience %s: %w", ex.APIName, err)
				}
			}
		}
	}

	// Permission sets (non-system definitions).
	for _, ps := range artifact.PermissionSets {
		sysJSON, _ := json.Marshal(ps.SystemPermissions)
		if sysJSON == nil {
			sysJSON = []byte("[]")
		}
		var existingID string
		var isSystem bool
		err := pool.QueryRow(ctx,
			`SELECT id::text, is_system FROM permission_sets WHERE api_name=$1`, ps.APIName,
		).Scan(&existingID, &isSystem)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check permission set %s: %w", ps.APIName, err)
		}
		if err == nil && isSystem {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "permissionSet", APIName: ps.APIName, Action: "skipped",
			})
			continue
		}
		if errors.Is(err, pgx.ErrNoRows) {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "permissionSet", APIName: ps.APIName, Action: "created",
			})
			if !dryRun {
				err := pool.QueryRow(ctx, `
INSERT INTO permission_sets (api_name, label, description, system_permissions, is_system)
VALUES ($1,$2,$3,$4::jsonb,false) RETURNING id::text`,
					ps.APIName, ps.Label, ps.Description, string(sysJSON)).Scan(&existingID)
				if err != nil {
					return nil, fmt.Errorf("create permission set %s: %w", ps.APIName, err)
				}
			}
		} else {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "permissionSet", APIName: ps.APIName, Action: "updated",
			})
			if !dryRun {
				_, err := pool.Exec(ctx, `
UPDATE permission_sets SET label=$2, description=$3, system_permissions=$4::jsonb WHERE id=$1::uuid AND is_system=false`,
					existingID, ps.Label, ps.Description, string(sysJSON))
				if err != nil {
					return nil, fmt.Errorf("update permission set %s: %w", ps.APIName, err)
				}
				_, _ = pool.Exec(ctx, `DELETE FROM object_permissions WHERE permission_set_id=$1::uuid`, existingID)
				_, _ = pool.Exec(ctx, `DELETE FROM field_permissions WHERE permission_set_id=$1::uuid`, existingID)
				_, _ = pool.Exec(ctx, `DELETE FROM automation_permissions WHERE permission_set_id=$1::uuid`, existingID)
			}
		}
		if !dryRun && existingID != "" {
			allAutos := ps.AllAutomations
			if ps.AutomationAccess != nil {
				allAutos = ps.AutomationAccess.AllAutomations
			}
			if _, err := pool.Exec(ctx, `
UPDATE permission_sets SET all_automations=$2 WHERE id=$1::uuid AND is_system=false`,
				existingID, allAutos); err != nil {
				return nil, fmt.Errorf("set all_automations %s: %w", ps.APIName, err)
			}
			for _, op := range ps.ObjectPermissions {
				_, err := pool.Exec(ctx, `
INSERT INTO object_permissions (permission_set_id, object_api_name, can_create, can_read, can_update, can_delete, view_all, modify_all)
VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8)`,
					existingID, op.ObjectAPIName, op.CanCreate, op.CanRead, op.CanUpdate, op.CanDelete, op.ViewAll, op.ModifyAll)
				if err != nil {
					return nil, fmt.Errorf("object perm %s.%s: %w", ps.APIName, op.ObjectAPIName, err)
				}
			}
			for _, fp := range ps.FieldPermissions {
				_, err := pool.Exec(ctx, `
INSERT INTO field_permissions (permission_set_id, object_api_name, field_api_name, can_read, can_edit)
VALUES ($1::uuid,$2,$3,$4,$5)`,
					existingID, fp.ObjectAPIName, fp.FieldAPIName, fp.CanRead, fp.CanEdit)
				if err != nil {
					return nil, fmt.Errorf("field perm %s.%s.%s: %w", ps.APIName, fp.ObjectAPIName, fp.FieldAPIName, err)
				}
			}
			autoEntries := []db.AutomationAccessEntry{}
			if ps.AutomationAccess != nil {
				for _, a := range ps.AutomationAccess.Automations {
					autoEntries = append(autoEntries, db.AutomationAccessEntry{APIName: a.APIName, CanRun: a.CanRun})
				}
			}
			if err := db.UpsertAutomationAccessEntries(ctx, pool, existingID, autoEntries); err != nil {
				return nil, fmt.Errorf("automation access %s: %w", ps.APIName, err)
			}
			// Fill deny stubs for objects/automations not listed in the artifact (preserves explicit grants).
			if err := db.BackfillPermissionSetDataAccess(ctx, pool, existingID); err != nil {
				return nil, fmt.Errorf("backfill data access %s: %w", ps.APIName, err)
			}
			if err := db.BackfillPermissionSetAutomationAccess(ctx, pool, existingID); err != nil {
				return nil, fmt.Errorf("backfill automation access %s: %w", ps.APIName, err)
			}
		}
	}

	// Webhooks (URL/events/active only; secrets preserved if already set).
	for _, wh := range artifact.Webhooks {
		eventsJSON, _ := json.Marshal(wh.EventTypes)
		if eventsJSON == nil {
			eventsJSON = []byte(`["*"]`)
		}
		var existingID string
		err := pool.QueryRow(ctx, `SELECT id::text FROM webhooks WHERE api_name=$1`, wh.APIName).Scan(&existingID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check webhook %s: %w", wh.APIName, err)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "webhook", APIName: wh.APIName, Action: "created",
			})
			if !dryRun {
				_, err := pool.Exec(ctx, `
INSERT INTO webhooks (api_name, url, event_types, active) VALUES ($1,$2,$3::jsonb,$4)`,
					wh.APIName, wh.URL, string(eventsJSON), wh.Active)
				if err != nil {
					return nil, fmt.Errorf("create webhook %s: %w", wh.APIName, err)
				}
			}
		} else {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "webhook", APIName: wh.APIName, Action: "updated",
			})
			if !dryRun {
				_, err := pool.Exec(ctx, `
UPDATE webhooks SET url=$2, event_types=$3::jsonb, active=$4 WHERE api_name=$1`,
					wh.APIName, wh.URL, string(eventsJSON), wh.Active)
				if err != nil {
					return nil, fmt.Errorf("update webhook %s: %w", wh.APIName, err)
				}
			}
		}
	}

	// Connectors (defs + OAuth flow specs + secret ref names; tokens/secrets preserved on target).
	for _, c := range artifact.Connectors {
		authType := c.AuthType
		if authType == "" {
			authType = "static_bearer"
		}
		flowJSON, _ := json.Marshal(c.OAuthFlow)
		if flowJSON == nil {
			flowJSON = []byte(`{}`)
		}
		methodsJSON, _ := json.Marshal(c.AllowedMethods)
		if methodsJSON == nil {
			methodsJSON = []byte(`["GET","POST"]`)
		}
		var existing string
		err := pool.QueryRow(ctx, `SELECT api_name FROM install_connectors WHERE api_name=$1`, c.APIName).Scan(&existing)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check connector %s: %w", c.APIName, err)
		}
		action := "updated"
		if errors.Is(err, pgx.ErrNoRows) {
			action = "created"
		}
		report.Actions = append(report.Actions, ApplyAction{
			Kind: "connector", APIName: c.APIName, Action: action,
		})
		if !dryRun {
			conn := db.InstallConnector{
				APIName: c.APIName, Label: c.Label, BaseURL: c.BaseURL,
				SecretRef: c.SecretRef, PathPrefix: c.PathPrefix, Active: c.Active,
				AuthType: authType,
			}
			_ = json.Unmarshal(methodsJSON, &conn.AllowedMethods)
			_ = json.Unmarshal(flowJSON, &conn.OAuthFlow)
			// Preserve target secret_ref when bundle omits it.
			if conn.SecretRef == nil && action == "updated" {
				if cur, gerr := db.GetInstallConnector(ctx, pool, c.APIName); gerr == nil {
					conn.SecretRef = cur.SecretRef
					prevHash := connectoroauth.ConfigHash(cur.AuthType, cur.OAuthFlow, secretRefOrEmpty(cur.SecretRef))
					newHash := connectoroauth.ConfigHash(authType, conn.OAuthFlow, secretRefOrEmpty(conn.SecretRef))
					if prevHash != newHash {
						_ = db.DeleteInstallConnectorOAuthToken(ctx, pool, c.APIName)
					}
				}
			} else if action == "updated" {
				if cur, gerr := db.GetInstallConnector(ctx, pool, c.APIName); gerr == nil {
					prevHash := connectoroauth.ConfigHash(cur.AuthType, cur.OAuthFlow, secretRefOrEmpty(cur.SecretRef))
					newHash := connectoroauth.ConfigHash(authType, conn.OAuthFlow, secretRefOrEmpty(conn.SecretRef))
					if prevHash != newHash {
						_ = db.DeleteInstallConnectorOAuthToken(ctx, pool, c.APIName)
					}
				}
			}
			if err := db.UpsertInstallConnector(ctx, pool, conn); err != nil {
				return nil, fmt.Errorf("upsert connector %s: %w", c.APIName, err)
			}
		}
	}

	// Customer test suites.
	for _, t := range artifact.Tests {
		stepsJSON, _ := json.Marshal(t.Steps)
		if stepsJSON == nil {
			stepsJSON = []byte("[]")
		}
		pkgName := t.PackageName
		if pkgName == nil {
			s := DefaultCustomerPackage
			pkgName = &s
		}
		var existingID string
		var ownership string
		err := pool.QueryRow(ctx,
			`SELECT id::text, ownership FROM customer_tests WHERE api_name=$1`, t.APIName,
		).Scan(&existingID, &ownership)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check test %s: %w", t.APIName, err)
		}
		if err == nil && ownership == "managed" {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "test", APIName: t.APIName, Action: "skipped",
			})
			continue
		}
		if errors.Is(err, pgx.ErrNoRows) {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "test", APIName: t.APIName, Action: "created",
			})
			if !dryRun {
				_, err := pool.Exec(ctx, `
INSERT INTO customer_tests (api_name, label, description, active, steps, package_name, ownership)
VALUES ($1,$2,$3,$4,$5::jsonb,$6,'custom')`,
					t.APIName, t.Label, t.Description, t.Active, string(stepsJSON), *pkgName)
				if err != nil {
					return nil, fmt.Errorf("create test %s: %w", t.APIName, err)
				}
			}
		} else {
			report.Actions = append(report.Actions, ApplyAction{
				Kind: "test", APIName: t.APIName, Action: "updated",
			})
			if !dryRun {
				_, err := pool.Exec(ctx, `
UPDATE customer_tests
SET label=$2, description=$3, active=$4, steps=$5::jsonb, package_name=$6, ownership='custom', updated_at=now()
WHERE api_name=$1 AND ownership='custom'`,
					t.APIName, t.Label, t.Description, t.Active, string(stepsJSON), *pkgName)
				if err != nil {
					return nil, fmt.Errorf("update test %s: %w", t.APIName, err)
				}
			}
		}
	}

	// Persist guest sources (automation + unit test TS) for Deploy test steps / snapshot.
	if err := UpsertCustomerSources(ctx, pool, artifact.Sources, dryRun); err != nil {
		return nil, err
	}
	if len(artifact.Sources) > 0 && !dryRun {
		report.Actions = append(report.Actions, ApplyAction{
			Kind: "sources", APIName: fmt.Sprintf("%d files", len(artifact.Sources)), Action: "updated",
		})
	}

	// Bump cache epoch after all writes.
	if !dryRun {
		if err := meta.BumpEpoch(ctx); err != nil {
			return nil, fmt.Errorf("bump epoch: %w", err)
		}
		if err := applySharingMetadata(ctx, pool, sharingSnapshotFromArtifact(artifact), dryRun); err != nil {
			return nil, fmt.Errorf("apply sharing metadata: %w", err)
		}
	}

	for _, a := range report.Actions {
		switch a.Action {
		case "created":
			report.Created++
		case "updated":
			report.Updated++
		case "skipped":
			report.Skipped++
		}
	}
	return report, nil
}

func secretRefOrEmpty(ref *string) string {
	if ref == nil {
		return ""
	}
	return *ref
}

func nullableText(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
