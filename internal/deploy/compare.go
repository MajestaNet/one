package deploy

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Diff entry kinds for local↔org compare (customer DX).
const (
	DiffAdd      = "add"
	DiffChange   = "change"
	DiffRemove   = "remove"
	DiffBaseline = "baseline"
)

// DiffEntry is one add/change/remove/baseline row in a DiffReport.
type DiffEntry struct {
	Kind    string `json:"kind"` // add | change | remove | baseline
	Path    string `json:"path"`
	Message string `json:"message,omitempty"`
}

// DiffReport compares a local packed artifact to the install customer snapshot.
type DiffReport struct {
	Entries []DiffEntry `json:"entries"`
	Counts  struct {
		Add      int `json:"add"`
		Change   int `json:"change"`
		Remove   int `json:"remove"`
		Baseline int `json:"baseline"`
	} `json:"counts"`
}

// CompareArtifacts diffs local (repo pack) against install (snapshot).
// Remove means customer-owned on install but missing locally (warn-only in DX v1).
// Baseline entries are informational managed-reference drift.
func CompareArtifacts(local, install *BundleArtifact) *DiffReport {
	report := &DiffReport{Entries: []DiffEntry{}}
	if local == nil {
		local = &BundleArtifact{}
	}
	if install == nil {
		install = &BundleArtifact{}
	}

	compareKeyed(report, "objects", objectKeys(local.Objects), objectKeys(install.Objects))
	compareKeyed(report, "fields", fieldKeys(local.Fields), fieldKeys(install.Fields))
	compareKeyed(report, "validationRules", ruleKeys(local.ValidationRules), ruleKeys(install.ValidationRules))
	compareKeyed(report, "automations", automationKeys(local.Automations), automationKeys(install.Automations))
	compareKeyed(report, "agentPlaybooks", playbookKeys(local.AgentPlaybooks), playbookKeys(install.AgentPlaybooks))
	compareKeyed(report, "canvases", canvasSpecKeys(local.Canvases), canvasSpecKeys(install.Canvases))
	compareKeyed(report, "experiences", experienceKeys(local.Experiences), experienceKeys(install.Experiences))
	compareKeyed(report, "permissionSets", permSetKeys(local.PermissionSets), permSetKeys(install.PermissionSets))
	compareKeyed(report, "webhooks", webhookKeys(local.Webhooks), webhookKeys(install.Webhooks))
	compareKeyed(report, "connectors", connectorKeys(local.Connectors), connectorKeys(install.Connectors))
	compareKeyed(report, "tests", testKeys(local.Tests), testKeys(install.Tests))
	compareKeyed(report, "dataRoles", dataRoleKeys(local.DataRoles), dataRoleKeys(install.DataRoles))
	compareKeyed(report, "objectSharingSettings", objectSharingKeys(local.ObjectSharingSettings), objectSharingKeys(install.ObjectSharingSettings))
	compareKeyed(report, "sharingRules", sharingRuleKeys(local.SharingRules), sharingRuleKeys(install.SharingRules))
	compareKeyed(report, "sources", sourceKeys(local.Sources), sourceKeys(install.Sources))

	compareBaseline(report, local.Baseline, install.Baseline)

	sort.Slice(report.Entries, func(i, j int) bool {
		if report.Entries[i].Path == report.Entries[j].Path {
			return report.Entries[i].Kind < report.Entries[j].Kind
		}
		return report.Entries[i].Path < report.Entries[j].Path
	})
	for _, e := range report.Entries {
		switch e.Kind {
		case DiffAdd:
			report.Counts.Add++
		case DiffChange:
			report.Counts.Change++
		case DiffRemove:
			report.Counts.Remove++
		case DiffBaseline:
			report.Counts.Baseline++
		}
	}
	return report
}

func compareKeyed(report *DiffReport, section string, local, install map[string]any) {
	seen := map[string]struct{}{}
	for k, lp := range local {
		seen[k] = struct{}{}
		path := section + "." + k
		ip, ok := install[k]
		if !ok {
			report.Entries = append(report.Entries, DiffEntry{
				Kind:    DiffAdd,
				Path:    path,
				Message: fmt.Sprintf("%s present locally, not on install", path),
			})
			continue
		}
		if !payloadEqual(lp, ip) {
			report.Entries = append(report.Entries, DiffEntry{
				Kind:    DiffChange,
				Path:    path,
				Message: fmt.Sprintf("%s differs between local and install", path),
			})
		}
	}
	for k := range install {
		if _, ok := seen[k]; ok {
			continue
		}
		path := section + "." + k
		report.Entries = append(report.Entries, DiffEntry{
			Kind:    DiffRemove,
			Path:    path,
			Message: fmt.Sprintf("%s on install, missing locally (delete-by-absence not applied in v1)", path),
		})
	}
}

func compareBaseline(report *DiffReport, local, install *ManagedBaseline) {
	if local == nil && install == nil {
		return
	}
	locObjs := map[string]any{}
	instObjs := map[string]any{}
	if local != nil {
		for _, o := range local.Objects {
			locObjs[o.APIName] = normalizeForCompare(o)
		}
	}
	if install != nil {
		for _, o := range install.Objects {
			instObjs[o.APIName] = normalizeForCompare(o)
		}
	}
	compareBaselineKeyed(report, "baseline.objects", locObjs, instObjs)

	locFields := map[string]any{}
	instFields := map[string]any{}
	if local != nil {
		for _, f := range local.Fields {
			locFields[f.ObjectAPIName+"."+f.APIName] = normalizeForCompare(f)
		}
	}
	if install != nil {
		for _, f := range install.Fields {
			instFields[f.ObjectAPIName+"."+f.APIName] = normalizeForCompare(f)
		}
	}
	compareBaselineKeyed(report, "baseline.fields", locFields, instFields)
}

func compareBaselineKeyed(report *DiffReport, section string, local, install map[string]any) {
	seen := map[string]struct{}{}
	for k, lp := range local {
		seen[k] = struct{}{}
		path := section + "." + k
		ip, ok := install[k]
		if !ok || !payloadEqual(lp, ip) {
			report.Entries = append(report.Entries, DiffEntry{
				Kind:    DiffBaseline,
				Path:    path,
				Message: "managed baseline drift (informational; not packed or promoted)",
			})
		}
	}
	for k := range install {
		if _, ok := seen[k]; ok {
			continue
		}
		report.Entries = append(report.Entries, DiffEntry{
			Kind:    DiffBaseline,
			Path:    section + "." + k,
			Message: "managed baseline present on install only (informational)",
		})
	}
}

func payloadEqual(a, b any) bool {
	ca, err := canonicalJSON(normalizeForCompare(a))
	if err != nil {
		return false
	}
	cb, err := canonicalJSON(normalizeForCompare(b))
	if err != nil {
		return false
	}
	return ca == cb
}

// normalizeForCompare strips volatile identity fields before payload equality.
func normalizeForCompare(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return v
	}
	return stripVolatile(raw)
}

func stripVolatile(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, child := range val {
			switch k {
			case "id", "ID", "createdAt", "sourceInstallId", "sourceInstallRole", "customerId":
				continue
			default:
				out[k] = stripVolatile(child)
			}
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = stripVolatile(item)
		}
		return out
	default:
		return v
	}
}

func objectKeys(items []SnapshotObject) map[string]any {
	out := make(map[string]any, len(items))
	for _, it := range items {
		out[it.APIName] = it
	}
	return out
}

func fieldKeys(items []SnapshotField) map[string]any {
	out := make(map[string]any, len(items))
	for _, it := range items {
		out[it.ObjectAPIName+"."+it.APIName] = it
	}
	return out
}

func ruleKeys(items []SnapshotRule) map[string]any {
	out := make(map[string]any, len(items))
	for _, it := range items {
		out[it.ObjectAPIName+"."+it.APIName] = it
	}
	return out
}

func automationKeys(items []SnapshotAutomation) map[string]any {
	out := make(map[string]any, len(items))
	for _, it := range items {
		out[it.APIName] = it
	}
	return out
}

func playbookKeys(items []SnapshotAgentPlaybook) map[string]any {
	out := make(map[string]any, len(items))
	for _, it := range items {
		out[it.APIName] = it
	}
	return out
}

func canvasSpecKeys(items []SnapshotCanvasSpec) map[string]any {
	out := make(map[string]any, len(items))
	for _, it := range items {
		out[it.APIName] = it
	}
	return out
}

func experienceKeys(items []SnapshotExperience) map[string]any {
	out := make(map[string]any, len(items))
	for _, it := range items {
		out[it.APIName] = it
	}
	return out
}

func permSetKeys(items []SnapshotPermissionSet) map[string]any {
	out := make(map[string]any, len(items))
	for _, it := range items {
		out[it.APIName] = it
	}
	return out
}

func webhookKeys(items []SnapshotWebhook) map[string]any {
	out := make(map[string]any, len(items))
	for _, it := range items {
		out[it.APIName] = it
	}
	return out
}

func connectorKeys(items []SnapshotConnector) map[string]any {
	out := make(map[string]any, len(items))
	for _, it := range items {
		out[it.APIName] = it
	}
	return out
}

func testKeys(items []SnapshotTestSuite) map[string]any {
	out := make(map[string]any, len(items))
	for _, it := range items {
		out[it.APIName] = it
	}
	return out
}

func dataRoleKeys(items []SnapshotDataRole) map[string]any {
	out := make(map[string]any, len(items))
	for _, it := range items {
		out[it.APIName] = it
	}
	return out
}

func objectSharingKeys(items []SnapshotObjectSharing) map[string]any {
	out := make(map[string]any, len(items))
	for _, it := range items {
		out[it.ObjectAPIName] = it
	}
	return out
}

func sharingRuleKeys(items []SnapshotSharingRule) map[string]any {
	out := make(map[string]any, len(items))
	for _, it := range items {
		out[it.ObjectAPIName+"."+it.APIName] = it
	}
	return out
}

func sourceKeys(sources map[string]string) map[string]any {
	out := make(map[string]any, len(sources))
	for k, v := range sources {
		out[k] = v
	}
	return out
}
