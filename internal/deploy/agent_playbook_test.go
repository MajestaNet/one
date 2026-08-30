package deploy_test

import (
	"testing"

	"github.com/MajestaNet/ide/internal/deploy"
)

func TestDeployAgentPlaybookRoundTrip(t *testing.T) {
	ctx, pool, meta, _, engine := setupDeployTest(t)
	const pbName = "DeployAgentSpec__c"
	_, _ = pool.Exec(ctx, `DELETE FROM agent_playbooks WHERE api_name=$1`, pbName)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM agent_playbooks WHERE api_name=$1`, pbName)
	})

	_, err := pool.Exec(ctx, `
INSERT INTO agent_playbooks (
  api_name, label, goal_template, instructions, allowed_tools, object_scopes,
  require_approval, active, ownership, package_name,
  primary_section, harness_id, harness_version
) VALUES ($1,'Deploy Agent','goal','instr','["query"]'::jsonb,'[]'::jsonb,true,true,'custom','customer.default',
  'operate','harness.operate.query','1')`, pbName)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	snap, err := meta.ExportCustomerSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rawPB, ok := snap["agentPlaybooks"]
	if !ok {
		t.Fatal("snapshot missing agentPlaybooks")
	}
	found := false
	switch list := rawPB.(type) {
	case []map[string]any:
		for _, p := range list {
			if p["apiName"] == pbName {
				found = true
			}
		}
	case []any:
		for _, item := range list {
			m, _ := item.(map[string]any)
			if m["apiName"] == pbName {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("AgentSpec not in snapshot: %#v", rawPB)
	}

	bundle, err := engine.CreateBundleFromSnapshot(ctx, struct {
		Label               *string
		CreatedBy           *string
		ProductVersionRange string
	}{})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM agent_playbooks WHERE api_name=$1`, pbName)

	result, err := engine.PromoteBundle(ctx, struct {
		BundleID  string
		DryRun    bool
		CreatedBy *string
	}{BundleID: bundle.ID, DryRun: false})
	if err != nil {
		t.Fatal(err)
	}
	if result.Promotion.Status != "applied" {
		t.Fatalf("status=%s", result.Promotion.Status)
	}
	var ownership string
	err = pool.QueryRow(ctx, `SELECT ownership FROM agent_playbooks WHERE api_name=$1`, pbName).Scan(&ownership)
	if err != nil {
		t.Fatal(err)
	}
	if ownership != "custom" {
		t.Fatalf("ownership=%s", ownership)
	}
}

func skillPlaybookArtifact(pbName, skill string, automations []any) map[string]any {
	return map[string]any{
		"manifestVersion": 1,
		"ownership":       "custom",
		"objects":         []any{},
		"fields":          []any{},
		"validationRules": []any{},
		"automations":     automations,
		"agentPlaybooks": []any{
			map[string]any{
				"apiName":         pbName,
				"label":           "Skill agent",
				"goalTemplate":    "invoke",
				"instructions":    "invoke",
				"jobClass":        "skill",
				"allowedTools":    []any{},
				"objectScopes":    []any{},
				"allowedSkills":   []any{skill},
				"requireApproval": false,
				"active":          true,
				"ownership":       "custom",
			},
		},
		"permissionSets": []any{},
		"webhooks":       []any{},
		"tests":          []any{},
	}
}

func findUnknownSkill(report *deploy.ValidationReport) *deploy.ValidationIssue {
	if report == nil {
		return nil
	}
	for i := range report.Issues {
		if report.Issues[i].Code == "UNKNOWN_SKILL" && report.Issues[i].Severity == "error" {
			return &report.Issues[i]
		}
	}
	return nil
}

func TestValidateAllowedSkillsUnknownFailsClosed(t *testing.T) {
	ctx, _, meta, _, _ := setupDeployTest(t)
	raw := skillPlaybookArtifact("DeployUnknownSkill__c", "NoSuchSkill__c", []any{})
	art, err := deploy.ParseBundleArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	report, err := deploy.ValidateBundleArtifact(ctx, meta, art, "0.1.0", "*")
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatal("expected validation to fail closed for unknown skill")
	}
	if findUnknownSkill(report) == nil {
		t.Fatalf("expected UNKNOWN_SKILL issue, got %+v", report.Issues)
	}
}

func TestValidateAllowedSkillsBundleAutomationPasses(t *testing.T) {
	ctx, _, meta, _, _ := setupDeployTest(t)
	const skill = "BundleSkillOnly__c"
	raw := skillPlaybookArtifact("DeployBundleSkill__c", skill, []any{
		map[string]any{
			"apiName": skill, "label": "Bundle skill", "objectApiName": "Account",
			"triggerEvent": "manual", "active": true, "actions": []any{}, "ownership": "custom",
		},
	})
	art, err := deploy.ParseBundleArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	report, err := deploy.ValidateBundleArtifact(ctx, meta, art, "0.1.0", "*")
	if err != nil {
		t.Fatal(err)
	}
	if iss := findUnknownSkill(report); iss != nil {
		t.Fatalf("bundle automation must satisfy allowedSkills, got %+v", iss)
	}
}

func TestValidateAllowedSkillsInstallAutomationPasses(t *testing.T) {
	ctx, pool, meta, _, _ := setupDeployTest(t)
	const skill = "InstallSkillOnly__c"
	_, _ = pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name=$1`, skill)
	if _, err := pool.Exec(ctx, `
INSERT INTO metadata_automations (
  api_name, label, object_api_name, trigger_event, active, actions, ownership, package_name, runtime, execution
) VALUES ($1,'Install skill','Account','manual',false,'[]'::jsonb,'custom','customer.default','actions','async')`, skill); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name=$1`, skill)
	})

	raw := skillPlaybookArtifact("DeployInstallSkill__c", skill, []any{})
	art, err := deploy.ParseBundleArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	report, err := deploy.ValidateBundleArtifact(ctx, meta, art, "0.1.0", "*")
	if err != nil {
		t.Fatal(err)
	}
	if iss := findUnknownSkill(report); iss != nil {
		t.Fatalf("install metadata_automations must satisfy allowedSkills, got %+v", iss)
	}
}
