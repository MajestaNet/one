package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MajestaNet/ide/internal/agentharness"
	"github.com/MajestaNet/ide/internal/automation"
	"github.com/MajestaNet/ide/internal/canvas"
	"github.com/MajestaNet/ide/internal/metadata"
)

// ParseBundleArtifact parses and validates raw JSON into a BundleArtifact, applying defaults.
func ParseBundleArtifact(raw any) (*BundleArtifact, error) {
	// Dual-read tools → canvases before decode (ADR-021 ToolSpec rename).
	if m, ok := raw.(map[string]any); ok {
		tools, hasTools := m["tools"]
		canvases, hasCanvases := m["canvases"]
		if hasTools {
			if !hasCanvases || isEmptyJSONArray(canvases) {
				m["canvases"] = tools
			}
		}
	}

	// Re-encode to JSON for a clean decode pass.
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal artifact: %w", err)
	}
	var a BundleArtifact
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, fmt.Errorf("parse artifact: %w", err)
	}

	// Validate required fields.
	if a.ManifestVersion != 1 {
		return nil, fmt.Errorf("unsupported manifestVersion %d (expected 1)", a.ManifestVersion)
	}
	if a.Ownership == "" {
		a.Ownership = "custom"
	}
	if a.DefaultPackageName == "" {
		a.DefaultPackageName = DefaultCustomerPackage
	}

	// Apply defaults to objects.
	for i := range a.Objects {
		if a.Objects[i].StorageMode == "" {
			a.Objects[i].StorageMode = "flexible"
		}
		if a.Objects[i].Features == nil {
			a.Objects[i].Features = map[string]bool{}
		}
	}
	// Apply defaults to fields.
	for i := range a.Fields {
		if !a.Fields[i].Filterable {
			a.Fields[i].Filterable = true
		}
		if !a.Fields[i].Sortable {
			a.Fields[i].Sortable = true
		}
	}
	// Validation rules have no special defaults beyond what JSON unmarshalling provides.
	// Apply defaults to automations.
	for i := range a.Automations {
		if a.Automations[i].Actions == nil {
			a.Automations[i].Actions = []any{}
		}
		if a.Automations[i].Ownership == "" {
			a.Automations[i].Ownership = "custom"
		}
		if a.Automations[i].Runtime == "" {
			a.Automations[i].Runtime = "actions"
		}
		if a.Automations[i].Execution == "" {
			a.Automations[i].Execution = "async"
		}
	}
	if a.Sources == nil {
		a.Sources = map[string]string{}
	}
	for i := range a.AgentPlaybooks {
		if a.AgentPlaybooks[i].AllowedTools == nil {
			a.AgentPlaybooks[i].AllowedTools = []string{}
		}
		if a.AgentPlaybooks[i].ObjectScopes == nil {
			a.AgentPlaybooks[i].ObjectScopes = []string{}
		}
		if a.AgentPlaybooks[i].AllowedSkills == nil {
			a.AgentPlaybooks[i].AllowedSkills = []string{}
		}
		// Dual-read: prefer allowedToolSpecs, fall back to allowedCanvasSpecs (ADR-021).
		merged := a.AgentPlaybooks[i].AllowedToolSpecs
		if len(merged) == 0 {
			merged = a.AgentPlaybooks[i].AllowedCanvasSpecs
		}
		if merged == nil {
			merged = []string{}
		}
		a.AgentPlaybooks[i].AllowedToolSpecs = merged
		a.AgentPlaybooks[i].AllowedCanvasSpecs = merged
		if a.AgentPlaybooks[i].Ownership == "" {
			a.AgentPlaybooks[i].Ownership = "custom"
		}
		// Job-class harness (BP-064): jobClass is SoR when set; primarySection YAML still binds the section catalog.
		if strings.TrimSpace(a.AgentPlaybooks[i].JobClass) != "" {
			if binding, err := agentharness.BindSpec(a.AgentPlaybooks[i].JobClass, a.AgentPlaybooks[i].PrimarySection); err == nil {
				a.AgentPlaybooks[i].JobClass = binding.JobClass
				a.AgentPlaybooks[i].PrimarySection = binding.PrimarySection
				a.AgentPlaybooks[i].HarnessID = binding.HarnessID
				a.AgentPlaybooks[i].HarnessVersion = binding.HarnessVersion
				a.AgentPlaybooks[i].AllowedTools = agentharness.EnsureToolFloor(binding.ToolFloor, a.AgentPlaybooks[i].AllowedTools)
				a.AgentPlaybooks[i].RequireApproval = agentharness.EffectiveRequireApproval(
					binding.RequireApprovalDefault, a.AgentPlaybooks[i].RequireApproval)
			}
		} else {
			section := a.AgentPlaybooks[i].PrimarySection
			if section == "" {
				section = string(agentharness.SectionOperate)
			}
			if binding, err := agentharness.Bind(section); err == nil {
				a.AgentPlaybooks[i].PrimarySection = binding.PrimarySection
				a.AgentPlaybooks[i].HarnessID = binding.HarnessID
				a.AgentPlaybooks[i].HarnessVersion = binding.HarnessVersion
				a.AgentPlaybooks[i].AllowedTools = agentharness.EnsureToolFloor(binding.ToolFloor, a.AgentPlaybooks[i].AllowedTools)
				a.AgentPlaybooks[i].RequireApproval = agentharness.EffectiveRequireApproval(
					binding.RequireApprovalDefault, a.AgentPlaybooks[i].RequireApproval)
			} else {
				a.AgentPlaybooks[i].PrimarySection = section
			}
		}
	}
	for i := range a.Canvases {
		if a.Canvases[i].Ownership == "" {
			a.Canvases[i].Ownership = "custom"
		}
	}
	for i := range a.Experiences {
		if a.Experiences[i].Ownership == "" {
			a.Experiences[i].Ownership = "custom"
		}
	}
	for i := range a.PermissionSets {
		if a.PermissionSets[i].Ownership == "" {
			a.PermissionSets[i].Ownership = "custom"
		}
		if a.PermissionSets[i].SystemPermissions == nil {
			a.PermissionSets[i].SystemPermissions = []string{}
		}
		if a.PermissionSets[i].ObjectPermissions == nil {
			a.PermissionSets[i].ObjectPermissions = []SnapshotObjectPermission{}
		}
		if a.PermissionSets[i].FieldPermissions == nil {
			a.PermissionSets[i].FieldPermissions = []SnapshotFieldPermission{}
		}
		if a.PermissionSets[i].AutomationAccess == nil {
			a.PermissionSets[i].AutomationAccess = &SnapshotAutomationAccess{
				AllAutomations: a.PermissionSets[i].AllAutomations,
				Automations:    []SnapshotAutomationPermission{},
			}
		} else if a.PermissionSets[i].AutomationAccess.Automations == nil {
			a.PermissionSets[i].AutomationAccess.Automations = []SnapshotAutomationPermission{}
		}
	}
	for i := range a.Webhooks {
		if a.Webhooks[i].Ownership == "" {
			a.Webhooks[i].Ownership = "custom"
		}
		if a.Webhooks[i].EventTypes == nil {
			a.Webhooks[i].EventTypes = []string{"*"}
		}
	}
	for i := range a.Connectors {
		if a.Connectors[i].Ownership == "" {
			a.Connectors[i].Ownership = "custom"
		}
		if a.Connectors[i].AuthType == "" {
			a.Connectors[i].AuthType = "static_bearer"
		}
		if a.Connectors[i].AllowedMethods == nil {
			a.Connectors[i].AllowedMethods = []string{"GET", "POST"}
		}
		if a.Connectors[i].OAuthFlow == nil {
			a.Connectors[i].OAuthFlow = map[string]any{}
		}
	}
	for i := range a.Tests {
		if a.Tests[i].Ownership == "" {
			a.Tests[i].Ownership = "custom"
		}
		if a.Tests[i].Steps == nil {
			a.Tests[i].Steps = []any{}
		}
	}

	return &a, nil
}

// ValidateBundleArtifact checks the artifact against the current metadata state.
func ValidateBundleArtifact(
	ctx context.Context,
	meta *metadata.Service,
	artifact *BundleArtifact,
	productVersion string,
	productVersionRange string,
) (*ValidationReport, error) {
	report := &ValidationReport{}
	report.Issues = []ValidationIssue{}

	// Resolve range: caller override > artifact field > "*"
	rangeStr := productVersionRange
	if rangeStr == "" && artifact.ProductVersionRange != nil {
		rangeStr = *artifact.ProductVersionRange
	}
	if rangeStr == "" {
		rangeStr = "*"
	}

	if !productVersionSatisfies(productVersion, rangeStr) {
		report.Issues = append(report.Issues, ValidationIssue{
			Severity: "error",
			Code:     "PRODUCT_VERSION",
			Message:  fmt.Sprintf("Install productVersion %s is outside bundle range %s", productVersion, rangeStr),
		})
	}

	if artifact.Ownership != "custom" {
		report.Issues = append(report.Issues, ValidationIssue{
			Severity: "error",
			Code:     "OWNERSHIP",
			Message:  "Bundle ownership must be custom",
		})
	}

	existingObjs, err := meta.ListObjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("list objects: %w", err)
	}
	objectByName := make(map[string]metadata.ObjectDefinition, len(existingObjs))
	for _, o := range existingObjs {
		objectByName[o.APIName] = o
	}

	// Validate objects.
	for _, obj := range artifact.Objects {
		if obj.Ownership != "custom" || isManagedPackageName(obj.PackageName) {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "MANAGED_IN_BUNDLE",
				Message:  fmt.Sprintf("Object %s is not customer-owned", obj.APIName),
				Path:     fmt.Sprintf("objects.%s", obj.APIName),
			})
			continue
		}
		if existing, ok := objectByName[obj.APIName]; ok {
			if isManagedOwnership(existing.Ownership) {
				report.Issues = append(report.Issues, ValidationIssue{
					Severity: "error",
					Code:     "MANAGED_CLASH",
					Message:  fmt.Sprintf("Cannot apply customer object over managed object %s", obj.APIName),
					Path:     fmt.Sprintf("objects.%s", obj.APIName),
				})
			}
		}
	}

	// Build set of objects in this bundle.
	bundleObjects := make(map[string]bool, len(artifact.Objects))
	for _, o := range artifact.Objects {
		bundleObjects[o.APIName] = true
	}

	// Validate fields.
	for _, field := range artifact.Fields {
		if field.Ownership != "custom" || isManagedPackageName(field.PackageName) {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "MANAGED_IN_BUNDLE",
				Message:  fmt.Sprintf("Field %s.%s is not customer-owned", field.ObjectAPIName, field.APIName),
				Path:     fmt.Sprintf("fields.%s.%s", field.ObjectAPIName, field.APIName),
			})
			continue
		}
		_, parentInDB := objectByName[field.ObjectAPIName]
		parentInBundle := bundleObjects[field.ObjectAPIName]
		if !parentInDB && !parentInBundle {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "MISSING_PARENT",
				Message:  fmt.Sprintf("Field %s.%s references unknown object", field.ObjectAPIName, field.APIName),
				Path:     fmt.Sprintf("fields.%s.%s", field.ObjectAPIName, field.APIName),
			})
			continue
		}
		existingField, err := meta.GetField(ctx, field.ObjectAPIName, field.APIName)
		if err == nil && isManagedOwnership(existingField.Ownership) {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "MANAGED_CLASH",
				Message:  fmt.Sprintf("Cannot overwrite managed field %s.%s", field.ObjectAPIName, field.APIName),
				Path:     fmt.Sprintf("fields.%s.%s", field.ObjectAPIName, field.APIName),
			})
		}
	}

	// Validate rules.
	for _, rule := range artifact.ValidationRules {
		if rule.Ownership != "custom" || isManagedPackageName(rule.PackageName) {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "MANAGED_IN_BUNDLE",
				Message:  fmt.Sprintf("Validation rule %s.%s is not customer-owned", rule.ObjectAPIName, rule.APIName),
				Path:     fmt.Sprintf("validationRules.%s", rule.APIName),
			})
		}
	}

	// Validate automations.
	for _, auto := range artifact.Automations {
		if auto.Ownership != "custom" || isManagedPackageName(auto.PackageName) {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "MANAGED_IN_BUNDLE",
				Message:  fmt.Sprintf("Automation %s is not customer-owned", auto.APIName),
				Path:     fmt.Sprintf("automations.%s", auto.APIName),
			})
		}
		src := auto.Source
		if (src == nil || *src == "") && auto.EntryFile != nil && artifact.Sources != nil {
			if body, ok := artifact.Sources[*auto.EntryFile]; ok {
				s := body
				src = &s
			}
		}
		if err := automation.ValidateDefinition(
			auto.APIName, auto.Runtime, auto.Execution, auto.TriggerEvent,
			auto.EntryFile, src, auto.RunAsPrincipalID, auto.Actions,
		); err != nil {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "AUTOMATION_INVALID",
				Message:  err.Error(),
				Path:     fmt.Sprintf("automations.%s", auto.APIName),
			})
		}
	}

	// Validate packed guest sources (import ban).
	for path, body := range artifact.Sources {
		if !strings.HasPrefix(path, "src/automations/") && !strings.HasPrefix(path, "tests/automations/") {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "SOURCE_PATH_INVALID",
				Message:  fmt.Sprintf("source path %q must be under src/automations/ or tests/automations/", path),
				Path:     "sources." + path,
			})
			continue
		}
		if err := automation.ValidateSourceImports(path, body); err != nil {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "AUTOMATION_IMPORT_FORBIDDEN",
				Message:  err.Error(),
				Path:     "sources." + path,
			})
		}
	}

	// Validate AgentSpecs.
	knownAutomations, err := knownAutomationAPINames(ctx, meta, artifact)
	if err != nil {
		return nil, fmt.Errorf("list automations: %w", err)
	}
	for _, pb := range artifact.AgentPlaybooks {
		if pb.Ownership != "custom" || isManagedPackageName(pb.PackageName) {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "MANAGED_IN_BUNDLE",
				Message:  fmt.Sprintf("AgentSpec %s is not customer-owned", pb.APIName),
				Path:     fmt.Sprintf("agentPlaybooks.%s", pb.APIName),
			})
		}
		if strings.TrimSpace(pb.JobClass) != "" {
			if _, err := agentharness.BindSpec(pb.JobClass, pb.PrimarySection); err != nil {
				report.Issues = append(report.Issues, ValidationIssue{
					Severity: "error",
					Code:     "INVALID_JOB_CLASS",
					Message:  fmt.Sprintf("AgentSpec %s: %v", pb.APIName, err),
					Path:     fmt.Sprintf("agentPlaybooks.%s.jobClass", pb.APIName),
				})
			}
		} else if _, err := agentharness.Bind(pb.PrimarySection); err != nil {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "INVALID_PRIMARY_SECTION",
				Message:  fmt.Sprintf("AgentSpec %s: %v", pb.APIName, err),
				Path:     fmt.Sprintf("agentPlaybooks.%s.primarySection", pb.APIName),
			})
		}
		report.Issues = append(report.Issues, unknownAllowedSkillIssues(pb, knownAutomations)...)
	}

	// Validate CanvasSpecs (YAML-only templates).
	for _, cs := range artifact.Canvases {
		path := fmt.Sprintf("canvases.%s", cs.APIName)
		if cs.Ownership != "custom" || isManagedPackageName(cs.PackageName) {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "MANAGED_IN_BUNDLE",
				Message:  fmt.Sprintf("CanvasSpec %s is not customer-owned", cs.APIName),
				Path:     path,
			})
		}
		if cs.APIName == "" || cs.Label == "" {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "CANVAS_SPEC_INVALID",
				Message:  "apiName and label are required",
				Path:     path,
			})
			continue
		}
		if err := canvas.ValidateSpecBody(cs.Layout, cs.Nodes, cs.DataBindings); err != nil {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "CANVAS_SPEC_INVALID",
				Message:  err.Error(),
				Path:     path,
			})
		}
	}

	// Validate Experiences (Client Experience config).
	for _, ex := range artifact.Experiences {
		path := fmt.Sprintf("experiences.%s", ex.APIName)
		if ex.Ownership != "custom" || isManagedPackageName(ex.PackageName) {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "MANAGED_IN_BUNDLE",
				Message:  fmt.Sprintf("Experience %s is not customer-owned", ex.APIName),
				Path:     path,
			})
		}
		if ex.APIName == "" || ex.Label == "" {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "EXPERIENCE_INVALID",
				Message:  "apiName and label are required",
				Path:     path,
			})
		}
	}

	for _, ps := range artifact.PermissionSets {
		if ps.Ownership != "custom" || isManagedPackageName(ps.PackageName) {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "MANAGED_IN_BUNDLE",
				Message:  fmt.Sprintf("Permission set %s is not customer-owned", ps.APIName),
				Path:     fmt.Sprintf("permissionSets.%s", ps.APIName),
			})
		}
	}

	for _, wh := range artifact.Webhooks {
		if wh.Ownership != "custom" || isManagedPackageName(wh.PackageName) {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "MANAGED_IN_BUNDLE",
				Message:  fmt.Sprintf("Webhook %s is not customer-owned", wh.APIName),
				Path:     fmt.Sprintf("webhooks.%s", wh.APIName),
			})
		}
	}

	for _, c := range artifact.Connectors {
		if c.Ownership != "custom" || isManagedPackageName(c.PackageName) {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "MANAGED_IN_BUNDLE",
				Message:  fmt.Sprintf("Connector %s is not customer-owned", c.APIName),
				Path:     fmt.Sprintf("connectors.%s", c.APIName),
			})
		}
		if strings.TrimSpace(c.BaseURL) == "" {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "CONNECTOR_BASE_URL",
				Message:  fmt.Sprintf("Connector %s requires baseUrl", c.APIName),
				Path:     fmt.Sprintf("connectors.%s.baseUrl", c.APIName),
			})
		}
	}

	for _, t := range artifact.Tests {
		if t.Ownership != "custom" || isManagedPackageName(t.PackageName) {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Code:     "MANAGED_IN_BUNDLE",
				Message:  fmt.Sprintf("Test suite %s is not customer-owned", t.APIName),
				Path:     fmt.Sprintf("tests.%s", t.APIName),
			})
		}
	}

	errorCount := 0
	for _, iss := range report.Issues {
		if iss.Severity == "error" {
			errorCount++
		}
	}
	report.OK = errorCount == 0
	report.Counts.Objects = len(artifact.Objects)
	report.Counts.Fields = len(artifact.Fields)
	report.Counts.ValidationRules = len(artifact.ValidationRules)
	report.Counts.Automations = len(artifact.Automations)
	report.Counts.AgentPlaybooks = len(artifact.AgentPlaybooks)
	report.Counts.Canvases = len(artifact.Canvases)
	report.Counts.Experiences = len(artifact.Experiences)
	report.Counts.PermissionSets = len(artifact.PermissionSets)
	report.Counts.Webhooks = len(artifact.Webhooks)
	report.Counts.Connectors = len(artifact.Connectors)
	report.Counts.Tests = len(artifact.Tests)

	return report, nil
}

// knownAutomationAPINames is the union of this bundle's automations and install
// metadata_automations (when validate has a DB). Names need not be active.
func knownAutomationAPINames(ctx context.Context, meta *metadata.Service, artifact *BundleArtifact) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	if artifact != nil {
		for _, a := range artifact.Automations {
			name := strings.TrimSpace(a.APIName)
			if name != "" {
				out[name] = struct{}{}
			}
		}
	}
	if meta == nil || meta.Pool() == nil {
		return out, nil
	}
	rows, err := meta.Pool().Query(ctx, `SELECT api_name FROM metadata_automations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		name = strings.TrimSpace(name)
		if name != "" {
			out[name] = struct{}{}
		}
	}
	return out, rows.Err()
}

// unknownAllowedSkillIssues fails closed when an AgentSpec names a skill that is
// neither in the bundle automations nor on the install. Empty allowedSkills is valid.
func unknownAllowedSkillIssues(pb SnapshotAgentPlaybook, known map[string]struct{}) []ValidationIssue {
	var issues []ValidationIssue
	for i, raw := range pb.AllowedSkills {
		name := strings.TrimSpace(raw)
		path := fmt.Sprintf("agentPlaybooks.%s.allowedSkills[%d]", pb.APIName, i)
		if name == "" {
			issues = append(issues, ValidationIssue{
				Severity: "error",
				Code:     "UNKNOWN_SKILL",
				Message:  fmt.Sprintf("AgentSpec %s allowedSkills entries must be non-empty automation apiNames", pb.APIName),
				Path:     path,
			})
			continue
		}
		if _, ok := known[name]; ok {
			continue
		}
		issues = append(issues, ValidationIssue{
			Severity: "error",
			Code:     "UNKNOWN_SKILL",
			Message:  fmt.Sprintf("AgentSpec %s allowedSkills %q is not a known automation in this bundle or on the install", pb.APIName, name),
			Path:     path,
		})
	}
	return issues
}

func isEmptyJSONArray(v any) bool {
	if v == nil {
		return true
	}
	switch t := v.(type) {
	case []any:
		return len(t) == 0
	case []map[string]any:
		return len(t) == 0
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return false
		}
		s := strings.TrimSpace(string(b))
		return s == "null" || s == "[]"
	}
}
