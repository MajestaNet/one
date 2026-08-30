package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MajestaNet/ide/internal/testutil"
)

func mcpCall(t *testing.T, h http.Handler, bearer, name string, args map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return testutil.AuthRequest(h, http.MethodPost, "/mcp", bearer, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
}

func TestMCPOrgValidateForbiddenWithoutDeployScope(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin,client-key:client",
	})
	rr := mcpCall(t, srv.Handler, "client-key", "org_validate", map[string]any{
		"artifact": map[string]any{"manifestVersion": 1},
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestMCPInvokeSkillDeniedWithoutAllowlistOrCanRun(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin,client-key:client",
	})
	const pbName = "MCPSkillDeny__c"
	_, _ = d.Pool.Exec(t.Context(), `DELETE FROM agent_playbooks WHERE api_name=$1`, pbName)
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM agent_playbooks WHERE api_name=$1`, pbName)
	})

	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/agents/playbooks", "admin-key", map[string]any{
		"apiName": pbName, "label": "Skill deny", "jobClass": "skill",
		"allowedSkills": []string{},
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create playbook: %d %s", rr.Code, rr.Body.String())
	}

	rr = mcpCall(t, srv.Handler, "admin-key", "invoke_skill", map[string]any{
		"apiName": "NoSuchSkill__c", "playbookApiName": pbName,
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 not in allowedSkills, got %d %s", rr.Code, rr.Body.String())
	}

	rr = mcpCall(t, srv.Handler, "client-key", "invoke_skill", map[string]any{
		"apiName": "NoSuchSkill__c",
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 canRun deny, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestMCPInstallVersionAuthenticated(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin",
	})
	rr := mcpCall(t, srv.Handler, "admin-key", "install_version", map[string]any{})
	if rr.Code != http.StatusOK {
		t.Fatalf("install_version: %d %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Result.Content) == 0 || resp.Result.Content[0].Text == "" {
		t.Fatalf("missing content: %s", rr.Body.String())
	}
}

func TestMCPInvokeSkillEnqueuesAutomationRun(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin",
	})
	const autoName = "MCPSkillHappy__c"
	const pbName = "MCPSkillHappyPB__c"
	cleanup := func() {
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM jobs WHERE payload->>'apiName'=$1`, autoName)
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM agent_playbooks WHERE api_name=$1`, pbName)
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
	}
	cleanup()
	t.Cleanup(cleanup)

	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/automations", "admin-key", map[string]any{
		"apiName": autoName, "label": "Happy skill", "objectApiName": "Account",
		"triggerEvent": "manual", "active": true, "runtime": "actions", "execution": "async",
		"actions": []any{},
	})
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("create automation: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/agents/playbooks", "admin-key", map[string]any{
		"apiName": pbName, "label": "Skill happy", "jobClass": "skill",
		"allowedSkills": []string{autoName},
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create playbook: %d %s", rr.Code, rr.Body.String())
	}

	actor, err := srv.Resolver.ResolveAPIKey("admin-key")
	if err != nil {
		t.Fatal(err)
	}

	rr = mcpCall(t, srv.Handler, "admin-key", "invoke_skill", map[string]any{
		"apiName": autoName, "playbookApiName": pbName,
		"input": map[string]any{"from": "mcp"},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("invoke_skill: %d %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Result.Content) == 0 {
		t.Fatalf("missing MCP content: %s", rr.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(resp.Result.Content[0].Text), &payload); err != nil {
		t.Fatalf("result text: %v %s", err, resp.Result.Content[0].Text)
	}
	jobID, _ := payload["id"].(string)
	if jobID == "" {
		t.Fatalf("missing job id: %s", resp.Result.Content[0].Text)
	}

	var jobType, apiName, actorID string
	err = d.Pool.QueryRow(t.Context(), `
SELECT job_type, payload->>'apiName', payload->>'actorId'
FROM jobs WHERE id=$1::uuid`, jobID).Scan(&jobType, &apiName, &actorID)
	if err != nil {
		t.Fatal(err)
	}
	if jobType != "automation.run" || apiName != autoName {
		t.Fatalf("job_type=%s apiName=%s", jobType, apiName)
	}
	if actorID != actor.ID {
		t.Fatalf("actorId=%s want %s", actorID, actor.ID)
	}
}

func TestMetadataPlaybookUnknownAllowedSkillRejected(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin",
	})
	const pbName = "MCPUnknownSkillPB__c"
	_, _ = d.Pool.Exec(t.Context(), `DELETE FROM agent_playbooks WHERE api_name=$1`, pbName)
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM agent_playbooks WHERE api_name=$1`, pbName)
	})

	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/agents/playbooks", "admin-key", map[string]any{
		"apiName": pbName, "label": "Unknown skill", "jobClass": "skill",
		"allowedSkills": []string{"NoSuch__c"},
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on unknown allowedSkills, got %d %s", rr.Code, rr.Body.String())
	}
	var errBody struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &errBody)
	if errBody.Error != "VALIDATION_ERROR" {
		t.Fatalf("expected VALIDATION_ERROR, got %s %s", errBody.Error, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/agents/playbooks", "admin-key", map[string]any{
		"apiName": pbName, "label": "Unknown skill", "jobClass": "skill",
		"allowedSkills": []string{},
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("empty allowedSkills create: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPatch, "/metadata/v1/agents/playbooks/"+pbName, "admin-key", map[string]any{
		"allowedSkills": []string{"NoSuch__c"},
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on PATCH unknown allowedSkills, got %d %s", rr.Code, rr.Body.String())
	}
}
