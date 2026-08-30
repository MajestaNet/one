package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/inference"
	"github.com/MajestaNet/ide/internal/testutil"
)

func setupInferenceHTTP(t *testing.T) (*testutil.Database, *testutil.TestServer) {
	t.Helper()
	d := testutil.RequireDatabase(t)
	testutil.LockInferenceConfig(t, d.Pool)
	resetInferenceInstall(t, d.Pool)
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin,meta-key:metadata,deploy-key:deploy,client-key:client",
	})
	t.Cleanup(func() { resetInferenceInstall(t, d.Pool) })
	return d, srv
}

func resetInferenceInstall(t *testing.T, pool *db.Pool) {
	t.Helper()
	ctx := context.Background()
	_, _ = pool.Exec(ctx, `
UPDATE install_inference_config
SET active_source='none', do_enabled=false, do_mode=NULL, default_provider_api_name=NULL, updated_at=now()
WHERE id=1`)
}

func inferenceAPIName(t *testing.T) string {
	t.Helper()
	name := "byo_" + strings.ReplaceAll(t.Name(), "/", "_")
	if len(name) > 48 {
		name = name[:48]
	}
	return name
}

func cleanupInferenceProvider(t *testing.T, pool *db.Pool, apiName string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_ = inference.DeleteProvider(ctx, pool, apiName)
		_ = db.DeleteInstallSecret(ctx, pool, "inference."+apiName)
	})
}

func TestInferenceProviderCreateDoesNotEchoAPIKey(t *testing.T) {
	d, srv := setupInferenceHTTP(t)
	apiName := inferenceAPIName(t)
	cleanupInferenceProvider(t, d.Pool, apiName)

	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/inference/providers", "admin-key", map[string]any{
		"apiName": apiName, "label": "BYO", "baseUrl": "http://127.0.0.1:11434/v1",
		"apiKey": "sk-never-echo", "defaultModel": "test-model",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, "sk-never-echo") || strings.Contains(body, `"apiKey"`) {
		t.Fatalf("must not echo apiKey: %s", body)
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["hasSecret"] != true {
		t.Fatalf("hasSecret=%v", out["hasSecret"])
	}
	var n int
	err := d.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM install_secrets WHERE api_name=$1`, "inference."+apiName).Scan(&n)
	if err != nil || n != 1 {
		t.Fatalf("secret row: n=%d err=%v", n, err)
	}
}

func TestInferenceProvidersListOmitsSecrets(t *testing.T) {
	d, srv := setupInferenceHTTP(t)
	apiName := inferenceAPIName(t)
	cleanupInferenceProvider(t, d.Pool, apiName)

	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/inference/providers", "admin-key", map[string]any{
		"apiName": apiName, "label": "BYO", "baseUrl": "http://127.0.0.1:11434/v1",
		"apiKey": "sk-list-secret", "defaultModel": "m",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/metadata/v1/inference/providers", "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, `"apiKey"`) || strings.Contains(body, "ciphertext") || strings.Contains(body, "sk-list-secret") {
		t.Fatalf("list leaked secret material: %s", body)
	}
}

func TestPatchInferenceConfigRejectsDigitalOcean(t *testing.T) {
	_, srv := setupInferenceHTTP(t)
	rr := testutil.AuthRequest(srv.Handler, http.MethodPatch, "/metadata/v1/inference/config", "admin-key", map[string]any{
		"activeSource": "digitalocean",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestPatchInferenceConfigSetsBYO(t *testing.T) {
	d, srv := setupInferenceHTTP(t)
	apiName := inferenceAPIName(t)
	cleanupInferenceProvider(t, d.Pool, apiName)

	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/inference/providers", "admin-key", map[string]any{
		"apiName": apiName, "label": "BYO", "baseUrl": "http://127.0.0.1:11434/v1",
		"apiKey": "sk-byo", "defaultModel": "m",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPatch, "/metadata/v1/inference/config", "admin-key", map[string]any{
		"activeSource": "byo", "defaultProviderApiName": apiName,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["activeSource"] != "byo" {
		t.Fatalf("activeSource=%v", out["activeSource"])
	}
}

func TestPutCloudInferenceRequiresToken(t *testing.T) {
	_, srv := setupInferenceHTTP(t)
	srv.Config.DigitalOceanAPIToken = ""
	rr := testutil.AuthRequest(srv.Handler, http.MethodPut, "/deploy/v1/cloud/inference", "admin-key", map[string]any{
		"enabled": true, "mode": "standard",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d %s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != "DO_TOKEN_MISSING" {
		t.Fatalf("error=%v body=%s", out["error"], rr.Body.String())
	}
}

func TestPutCloudInferenceStandardModelID(t *testing.T) {
	_, srv := setupInferenceHTTP(t)
	srv.Config.DigitalOceanAPIToken = "dop_v1_test_token"
	t.Cleanup(func() { srv.Config.DigitalOceanAPIToken = "" })

	want, err := inference.ModelForMode(inference.ModeStandard)
	if err != nil {
		t.Fatal(err)
	}
	rr := testutil.AuthRequest(srv.Handler, http.MethodPut, "/deploy/v1/cloud/inference", "admin-key", map[string]any{
		"enabled": true, "mode": "standard",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["prepaid"] != true {
		t.Fatalf("prepaid=%v", out["prepaid"])
	}
	notice, _ := out["billingNotice"].(string)
	if strings.TrimSpace(notice) == "" {
		t.Fatal("empty billingNotice")
	}
	if out["modelId"] != want || out["doModelId"] != want {
		t.Fatalf("modelId=%v doModelId=%v want=%s", out["modelId"], out["doModelId"], want)
	}
	modes, _ := out["doModeModels"].(map[string]any)
	if modes["standard"] != want {
		t.Fatalf("doModeModels.standard=%v", modes["standard"])
	}

	get := testutil.AuthRequest(srv.Handler, http.MethodGet, "/deploy/v1/cloud/inference", "admin-key", nil)
	if get.Code != http.StatusOK {
		t.Fatalf("get: %d %s", get.Code, get.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(get.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["modelId"] != want || got["prepaid"] != true {
		t.Fatalf("get modelId=%v prepaid=%v", got["modelId"], got["prepaid"])
	}
}

func TestPutCloudInferenceDisablePreservesBYO(t *testing.T) {
	d, srv := setupInferenceHTTP(t)
	apiName := inferenceAPIName(t)
	cleanupInferenceProvider(t, d.Pool, apiName)

	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/inference/providers", "admin-key", map[string]any{
		"apiName": apiName, "label": "BYO", "baseUrl": "http://127.0.0.1:11434/v1",
		"apiKey": "sk-keep", "defaultModel": "m",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	rr = testutil.AuthRequest(srv.Handler, http.MethodPatch, "/metadata/v1/inference/config", "admin-key", map[string]any{
		"activeSource": "byo", "defaultProviderApiName": apiName,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPut, "/deploy/v1/cloud/inference", "admin-key", map[string]any{
		"enabled": false,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("put disable: %d %s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["activeSource"] != "byo" {
		t.Fatalf("disable wiped BYO: activeSource=%v", out["activeSource"])
	}
	if out["doEnabled"] != false {
		t.Fatalf("doEnabled=%v", out["doEnabled"])
	}
}

func TestDeleteDefaultProviderSetsSourceNone(t *testing.T) {
	d, srv := setupInferenceHTTP(t)
	apiName := inferenceAPIName(t)
	cleanupInferenceProvider(t, d.Pool, apiName)

	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/inference/providers", "admin-key", map[string]any{
		"apiName": apiName, "label": "BYO", "baseUrl": "http://127.0.0.1:11434/v1",
		"apiKey": "sk-del", "defaultModel": "m",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	rr = testutil.AuthRequest(srv.Handler, http.MethodPatch, "/metadata/v1/inference/config", "admin-key", map[string]any{
		"activeSource": "byo", "defaultProviderApiName": apiName,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodDelete, "/metadata/v1/inference/providers/"+apiName, "admin-key", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/metadata/v1/inference/config", "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("config: %d %s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["activeSource"] != "none" {
		t.Fatalf("activeSource=%v", out["activeSource"])
	}
}

func TestInferenceProviderMutateRequiresMetadataBuild(t *testing.T) {
	_, srv := setupInferenceHTTP(t)
	// Client-only keys have neither metadata scope nor metadata.build.
	// Metadata-scoped API keys are bound to MetadataCustomize (includes metadata.build).
	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/inference/providers", "client-key", map[string]any{
		"apiName": "forbidden_byo", "label": "no", "baseUrl": "http://127.0.0.1:11434/v1",
		"apiKey": "sk-no",
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestPutCloudInferenceRequiresAdmin(t *testing.T) {
	_, srv := setupInferenceHTTP(t)
	srv.Config.DigitalOceanAPIToken = "dop_v1_test_token"
	t.Cleanup(func() { srv.Config.DigitalOceanAPIToken = "" })
	rr := testutil.AuthRequest(srv.Handler, http.MethodPut, "/deploy/v1/cloud/inference", "deploy-key", map[string]any{
		"enabled": true, "mode": "standard",
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rr.Code, rr.Body.String())
	}
}
