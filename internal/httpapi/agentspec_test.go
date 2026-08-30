package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/config"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/httpapi"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/seed"
)

func setupPlaybookTest(t *testing.T) (context.Context, *db.Pool, *httpapi.Server) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	meta := metadata.NewService(pool)
	if err := seed.Bootstrap(ctx, pool, meta, seed.Options{
		OwnerID:      "00000000-0000-4000-8000-000000000001",
		FeatureFlags: []string{"agents"},
		AutoSeed:     true,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	entries, err := authz.ParseAPIKeyEntries("admin+admin,clientonly:client")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		DefaultOwnerID: "00000000-0000-4000-8000-000000000001",
		APIKeyEntries:  entries,
		FeatureFlags:   []string{"agents"},
	}
	store := db.NewUserStore(pool)
	srv := httpapi.New(httpapi.Options{
		Config:   cfg,
		Resolver: &authz.Resolver{Entries: entries, DefaultOwnerID: cfg.DefaultOwnerID, Users: &db.AuthzUsers{Store: store}},
		Pool:     pool,
		Metadata: meta,
		SystemAz: &authz.SystemAuthz{Store: &db.AuthzSystemPerms{Store: db.NewSystemPermStore(pool)}},
	})
	return ctx, pool, srv
}

func TestClientAgentCatalogListsActiveSafeFields(t *testing.T) {
	_, pool, srv := setupPlaybookTest(t)
	const activeName = "ClientCatalogActive__c"
	const inactiveName = "ClientCatalogInactive__c"
	_, _ = pool.Exec(context.Background(), `DELETE FROM agent_playbooks WHERE api_name = ANY($1::text[])`, []string{activeName, inactiveName})
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM agent_playbooks WHERE api_name = ANY($1::text[])`, []string{activeName, inactiveName})
	})
	_, err := pool.Exec(context.Background(), `
INSERT INTO agent_playbooks (api_name, label, goal_template, instructions, require_approval, active, ownership, package_name,
  primary_section, harness_id, harness_version)
VALUES ($1,'Active agent','Help the user','private instructions',true,true,'custom','customer.default',
  'operate','harness.operate.query','1'),
       ($2,'Inactive agent','Do not list','private',false,false,'custom','customer.default',
  'operate','harness.operate.query','1')`, activeName, inactiveName)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/client/v1/agents/playbooks", nil)
	req.Header.Set("Authorization", "Bearer clientonly")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("catalog: %d %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(activeName)) || bytes.Contains(rec.Body.Bytes(), []byte(inactiveName)) {
		t.Fatalf("active filter mismatch: %s", rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private instructions")) || bytes.Contains(rec.Body.Bytes(), []byte("allowedTools")) {
		t.Fatalf("runtime catalog leaked definition internals: %s", rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"primarySection":"operate"`)) {
		t.Fatalf("runtime catalog missing primarySection: %s", rec.Body.String())
	}
}

func TestAgentSpecCRUDAndOwnership(t *testing.T) {
	_, pool, srv := setupPlaybookTest(t)
	const name = "TestAgentSpec__c"
	_, _ = pool.Exec(context.Background(), `DELETE FROM agent_playbooks WHERE api_name=$1`, name)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM agent_playbooks WHERE api_name=$1`, name)
	})

	body, _ := json.Marshal(map[string]any{
		"apiName": name, "label": "Test", "goalTemplate": "do {{x}}",
		"instructions": "You are a customer agent.", "requireApproval": true,
		"primarySection": "operate",
	})
	req := httptest.NewRequest(http.MethodPost, "/metadata/v1/agents/playbooks", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created["primarySection"] != "operate" || created["jobClass"] != "query" || created["harnessId"] != "harness.query.read" {
		t.Fatalf("harness binding missing: %s", rec.Body.String())
	}

	jobOnly, _ := json.Marshal(map[string]any{
		"apiName": name + "Job", "label": "Job class only", "jobClass": "customize",
	})
	reqJob := httptest.NewRequest(http.MethodPost, "/metadata/v1/agents/playbooks", bytes.NewReader(jobOnly))
	reqJob.Header.Set("Authorization", "Bearer admin")
	reqJob.Header.Set("Content-Type", "application/json")
	recJob := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recJob, reqJob)
	if recJob.Code != http.StatusCreated {
		t.Fatalf("create jobClass only: %d %s", recJob.Code, recJob.Body.String())
	}
	var createdJob map[string]any
	if err := json.Unmarshal(recJob.Body.Bytes(), &createdJob); err != nil {
		t.Fatal(err)
	}
	if createdJob["jobClass"] != "customize" || createdJob["primarySection"] != "build" {
		t.Fatalf("jobClass XOR fill: %s", recJob.Body.String())
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM agent_playbooks WHERE api_name=$1`, name+"Job")
	})

	dropFloor, _ := json.Marshal(map[string]any{"allowedTools": []string{"sobjects.write"}})
	reqDrop := httptest.NewRequest(http.MethodPatch, "/metadata/v1/agents/playbooks/"+name, bytes.NewReader(dropFloor))
	reqDrop.Header.Set("Authorization", "Bearer admin")
	reqDrop.Header.Set("Content-Type", "application/json")
	recDrop := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recDrop, reqDrop)
	if recDrop.Code != http.StatusOK {
		t.Fatalf("patch tools: %d %s", recDrop.Code, recDrop.Body.String())
	}
	if !bytes.Contains(recDrop.Body.Bytes(), []byte(`"sobjects.read"`)) || !bytes.Contains(recDrop.Body.Bytes(), []byte(`"search"`)) {
		t.Fatalf("PATCH dropped job-class floor: %s", recDrop.Body.String())
	}

	missingSection, _ := json.Marshal(map[string]any{
		"apiName": name + "NoSection", "label": "No section",
	})
	reqMissing := httptest.NewRequest(http.MethodPost, "/metadata/v1/agents/playbooks", bytes.NewReader(missingSection))
	reqMissing.Header.Set("Authorization", "Bearer admin")
	reqMissing.Header.Set("Content-Type", "application/json")
	recMissing := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recMissing, reqMissing)
	if recMissing.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without primarySection or jobClass, got %d %s", recMissing.Code, recMissing.Body.String())
	}

	harnessesReq := httptest.NewRequest(http.MethodGet, "/metadata/v1/agents/harnesses", nil)
	harnessesReq.Header.Set("Authorization", "Bearer admin")
	harnessesRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(harnessesRec, harnessesReq)
	if harnessesRec.Code != http.StatusOK {
		t.Fatalf("harnesses: %d %s", harnessesRec.Code, harnessesRec.Body.String())
	}
	if !bytes.Contains(harnessesRec.Body.Bytes(), []byte("harness.operate.query")) {
		t.Fatalf("harness catalog missing operate: %s", harnessesRec.Body.String())
	}
	if !bytes.Contains(harnessesRec.Body.Bytes(), []byte("harness.query.read")) {
		t.Fatalf("job-class catalog missing query: %s", harnessesRec.Body.String())
	}

	patch, _ := json.Marshal(map[string]any{"instructions": "updated"})
	req2 := httptest.NewRequest(http.MethodPatch, "/metadata/v1/agents/playbooks/"+name, bytes.NewReader(patch))
	req2.Header.Set("Authorization", "Bearer admin")
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rec2.Code, rec2.Body.String())
	}

	// Force managed ownership and ensure PATCH is rejected.
	_, err := pool.Exec(context.Background(), `UPDATE agent_playbooks SET ownership='managed' WHERE api_name=$1`, name)
	if err != nil {
		t.Fatal(err)
	}
	req3 := httptest.NewRequest(http.MethodPatch, "/metadata/v1/agents/playbooks/"+name, bytes.NewReader(patch))
	req3.Header.Set("Authorization", "Bearer admin")
	req3.Header.Set("Content-Type", "application/json")
	rec3 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for managed patch, got %d %s", rec3.Code, rec3.Body.String())
	}
}

func TestMCPGatedAndScopeDeny(t *testing.T) {
	entries, err := authz.ParseAPIKeyEntries("clientonly:client")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		DefaultOwnerID: "00000000-0000-4000-8000-000000000001",
		APIKeyEntries:  entries,
		FeatureFlags:   []string{"agents"},
	}
	srv := httpapi.New(httpapi.Options{
		Config:   cfg,
		Resolver: &authz.Resolver{Entries: entries, DefaultOwnerID: cfg.DefaultOwnerID},
	})
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{
			"name":      "get_object_metadata",
			"arguments": map[string]any{"apiName": "Account"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer clientonly")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestMCPDisabledWithoutAgentsFlag(t *testing.T) {
	entries, err := authz.ParseAPIKeyEntries("admin+admin")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		DefaultOwnerID: "00000000-0000-4000-8000-000000000001",
		APIKeyEntries:  entries,
		FeatureFlags:   []string{"other"},
	}
	srv := httpapi.New(httpapi.Options{
		Config:   cfg,
		Resolver: &authz.Resolver{Entries: entries, DefaultOwnerID: cfg.DefaultOwnerID},
	})
	req := httptest.NewRequest(http.MethodGet, "/mcp/tools", nil)
	req.Header.Set("Authorization", "Bearer admin")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when agents flag off, got %d", rec.Code)
	}
}
