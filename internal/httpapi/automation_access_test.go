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

func setupAutomationAccessHTTP(t *testing.T) (context.Context, *db.Pool, *httpapi.Server) {
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
		OwnerID:  "00000000-0000-4000-8000-000000000001",
		AutoSeed: true,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	entries, err := authz.ParseAPIKeyEntries("admin+admin")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		DefaultOwnerID: "00000000-0000-4000-8000-000000000001",
		APIKeyEntries:  entries,
	}
	store := db.NewUserStore(pool)
	srv := httpapi.New(httpapi.Options{
		Config:       cfg,
		Resolver:     &authz.Resolver{Entries: entries, DefaultOwnerID: cfg.DefaultOwnerID, Users: &db.AuthzUsers{Store: store}},
		Pool:         pool,
		Metadata:     meta,
		SystemAz:     &authz.SystemAuthz{Store: &db.AuthzSystemPerms{Store: db.NewSystemPermStore(pool)}},
		AutomationAz: &authz.AutomationAuthz{Store: &db.AutomationPermStore{Pool: pool}},
	})
	return ctx, pool, srv
}

func TestAutomationAccessPermissionSetRoundTrip(t *testing.T) {
	ctx, pool, srv := setupAutomationAccessHTTP(t)
	const autoName = "HTTPAutoGrant__c"
	const psName = "HTTPAutoPS"

	_, _ = pool.Exec(ctx, `DELETE FROM automation_permissions WHERE automation_api_name=$1`, autoName)
	_, _ = pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
	_, _ = pool.Exec(ctx, `DELETE FROM permission_sets WHERE api_name=$1`, psName)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM automation_permissions WHERE automation_api_name=$1`, autoName)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
		_, _ = pool.Exec(ctx, `DELETE FROM permission_sets WHERE api_name=$1`, psName)
	})

	// Create automation → catalog stubs on every PS including Admin.
	body, _ := json.Marshal(map[string]any{
		"apiName": autoName, "label": "HTTP Auto", "objectApiName": "Account",
		"triggerEvent": "create", "actions": []any{},
	})
	req := httptest.NewRequest(http.MethodPost, "/metadata/v1/automations", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create automation: %d %s", rec.Code, rec.Body.String())
	}

	var adminCanRun bool
	err := pool.QueryRow(ctx, `
SELECT ap.can_run FROM automation_permissions ap
JOIN permission_sets ps ON ps.id = ap.permission_set_id
WHERE ps.api_name='Admin' AND ap.automation_api_name=$1`, autoName).Scan(&adminCanRun)
	if err != nil || !adminCanRun {
		t.Fatalf("Admin stub missing/false: err=%v canRun=%v", err, adminCanRun)
	}

	// Create PS with automationAccess grant list.
	psBody, _ := json.Marshal(map[string]any{
		"apiName": psName, "label": "HTTP Auto PS",
		"systemPermissions": []string{},
		"automationAccess": map[string]any{
			"allAutomations": false,
			"automations": []map[string]any{
				{"apiName": autoName, "canRun": true},
			},
		},
	})
	req = httptest.NewRequest(http.MethodPost, "/metadata/v1/permissions/sets", bytes.NewReader(psBody))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create PS: %d %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	aa, _ := created["automationAccess"].(map[string]any)
	if aa == nil {
		t.Fatalf("missing automationAccess on create response: %v", created)
	}
	if aa["allAutomations"] == true {
		t.Fatal("allAutomations should be false")
	}

	// GET returns automationAccess.
	req = httptest.NewRequest(http.MethodGet, "/metadata/v1/permissions/sets/"+psName, nil)
	req.Header.Set("Authorization", "Bearer admin")
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get PS: %d %s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	aa, _ = got["automationAccess"].(map[string]any)
	autos, _ := aa["automations"].([]any)
	foundGrant := false
	for _, raw := range autos {
		m, _ := raw.(map[string]any)
		if m["apiName"] == autoName && m["canRun"] == true {
			foundGrant = true
		}
	}
	if !foundGrant {
		t.Fatalf("expected canRun grant in GET: %v", aa)
	}

	// PATCH allAutomations.
	patch, _ := json.Marshal(map[string]any{
		"automationAccess": map[string]any{"allAutomations": true},
	})
	req = httptest.NewRequest(http.MethodPatch, "/metadata/v1/permissions/sets/"+psName, bytes.NewReader(patch))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch PS: %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	aa, _ = got["automationAccess"].(map[string]any)
	if aa["allAutomations"] != true {
		t.Fatalf("expected allAutomations true after patch: %v", aa)
	}

	var psID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM permission_sets WHERE api_name=$1`, psName).Scan(&psID); err != nil {
		t.Fatal(err)
	}
	az := &authz.AutomationAuthz{Store: &db.AutomationPermStore{Pool: pool}}
	ok, err := az.ActorCanRunAutomation(ctx, &authz.Actor{ID: "u", PermissionSetIDs: []string{psID}}, "NoSuchAuto")
	if err != nil || !ok {
		t.Fatalf("allAutomations should allow any apiName, ok=%v err=%v", ok, err)
	}
}
