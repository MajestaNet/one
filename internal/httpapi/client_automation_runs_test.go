package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/testutil"
	"github.com/MajestaNet/ide/internal/worker"
)

func TestClientAutomationInvokeAsync(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin,client-key:client,meta-key:metadata",
	})
	ctx := context.Background()
	const autoName = "ClientInvokeAsync__c"
	const psName = "ClientInvokePS"

	cleanup := func() {
		_, _ = d.Pool.Exec(ctx, `DELETE FROM jobs WHERE payload->>'apiName'=$1`, autoName)
		_, _ = d.Pool.Exec(ctx, `DELETE FROM automation_permissions WHERE automation_api_name=$1`, autoName)
		_, _ = d.Pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
		_, _ = d.Pool.Exec(ctx, `DELETE FROM permission_sets WHERE api_name=$1`, psName)
	}
	cleanup()
	t.Cleanup(cleanup)

	// Create via Metadata (admin).
	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/automations", "admin-key", map[string]any{
		"apiName": autoName, "label": "Invoke", "objectApiName": "Account",
		"triggerEvent": "manual", "active": true, "runtime": "actions", "execution": "async",
		"actions": []any{},
	})
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("create automation: %d %s", rr.Code, rr.Body.String())
	}

	// Grant can_run on a PS and assign to a non-admin client principal via API key...
	// API keys don't have PS — use admin for happy path, then deny with a scoped principal.
	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/automations/"+autoName+"/runs", "admin-key", map[string]any{
		"input": map[string]any{"hello": "world"},
	})
	if rr.Code != http.StatusAccepted {
		t.Fatalf("invoke: %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	jobID, _ := created["id"].(string)
	if jobID == "" {
		t.Fatal("missing job id")
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/automations/runs/"+jobID, "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get run: %d %s", rr.Code, rr.Body.String())
	}

	az := &authz.AutomationAuthz{Store: &db.AutomationPermStore{Pool: d.Pool}}
	objAz := &authz.ObjectAuthz{Store: &db.ObjectPermStore{Pool: d.Pool}}
	if _, err := worker.ProcessJobs(ctx, d.Pool, &worker.ProcessOptions{
		DataEngine:   srv.Data,
		AutomationAz: az,
		ObjectAz:     objAz,
		JobID:        jobID,
	}); err != nil {
		t.Fatalf("process: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/automations/runs/"+jobID, "admin-key", nil)
		var got map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &got)
		if got["status"] == "completed" {
			input, _ := got["input"].(map[string]any)
			if input["hello"] != "world" {
				t.Fatalf("input not preserved: %#v", input)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("job not completed: %s", rr.Body.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestClientAutomationInvokeForbidden(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin,client-key:client",
	})
	ctx := context.Background()
	const autoName = "ClientInvokeDeny__c"
	_, _ = d.Pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(ctx, `DELETE FROM jobs WHERE payload->>'apiName'=$1`, autoName)
		_, _ = d.Pool.Exec(ctx, `DELETE FROM automation_permissions WHERE automation_api_name=$1`, autoName)
		_, _ = d.Pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
	})

	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/automations", "admin-key", map[string]any{
		"apiName": autoName, "label": "Deny", "objectApiName": "Account",
		"triggerEvent": "manual", "active": true, "runtime": "actions", "execution": "async",
		"actions": []any{},
	})
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}

	// client-key has client scope but no PS / not admin → forbidden.
	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/automations/"+autoName+"/runs", "client-key", map[string]any{
		"input": map[string]any{},
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rr.Code, rr.Body.String())
	}
	var n int
	_ = d.Pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE payload->>'apiName'=$1`, autoName).Scan(&n)
	if n != 0 {
		t.Fatalf("job inserted on forbidden invoke: %d", n)
	}
}

func TestClientAutomationInvokeNotFoundInactive(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin",
	})
	ctx := context.Background()
	const autoName = "ClientInvokeInactive__c"
	_, _ = d.Pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
	})

	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/automations/DoesNotExist__c/runs", "admin-key", map[string]any{})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing: expected 404, got %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/automations", "admin-key", map[string]any{
		"apiName": autoName, "label": "Inactive", "objectApiName": "Account",
		"triggerEvent": "manual", "active": false, "runtime": "actions", "execution": "async",
		"actions": []any{},
	})
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		// Some installs may reject inactive create — insert directly.
		_, err := d.Pool.Exec(ctx, `
INSERT INTO metadata_automations (api_name, label, object_api_name, trigger_event, active, runtime, execution, actions)
VALUES ($1,'Inactive','Account','manual',false,'actions','async','[]'::jsonb)
ON CONFLICT (api_name) DO UPDATE SET active=false`, autoName)
		if err != nil && rr.Code >= 400 {
			t.Fatalf("create inactive: http=%d body=%s db=%v", rr.Code, rr.Body.String(), err)
		}
	} else {
		_, _ = d.Pool.Exec(ctx, `UPDATE metadata_automations SET active=false WHERE api_name=$1`, autoName)
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/automations/"+autoName+"/runs", "admin-key", map[string]any{})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("inactive: expected 404, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestClientAutomationCatalogAndMetadataScope(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin,deploy-key:deploy",
	})

	rr := testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/automations", "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("catalog: %d %s", rr.Code, rr.Body.String())
	}
	var catalog map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &catalog)
	list, _ := catalog["automations"].([]any)
	for _, item := range list {
		m, _ := item.(map[string]any)
		if _, ok := m["source"]; ok {
			t.Fatalf("catalog must not include source: %#v", m)
		}
	}

	// DeployBot role is deploy-only (see EnsureSystemRoles); mirrors scope gates in integration_test.
	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/automations", "deploy-key", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("deploy-only key should fail client scope, got %d %s", rr.Code, rr.Body.String())
	}
}
