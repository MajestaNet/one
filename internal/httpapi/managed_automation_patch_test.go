package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/MajestaNet/ide/internal/packages"
	"github.com/MajestaNet/ide/internal/seed"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestPatchManagedAutomationActiveOnly(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin,builder-key:metadata",
	})
	ctx := context.Background()

	orig, ok := packages.Get("notes")
	if !ok {
		t.Fatal("notes module missing")
	}
	const autoName = "Notes_ToggleActiveBP069"
	mod := orig
	mod.Automations = []packages.AutomationDef{{
		APIName:       autoName,
		Label:         "Toggle fixture",
		Description:   "Fixture for PATCH active allowlist.",
		ObjectAPIName: "Note",
		TriggerEvent:  "create",
		Runtime:       "code",
		Execution:     "async",
		Source:        `export default async function run(ctx) { return { ok: true }; }`,
	}}
	packages.Register(mod)
	t.Cleanup(func() {
		packages.Register(orig)
		bg := context.Background()
		_, _ = d.Pool.Exec(bg, `DELETE FROM automation_permissions WHERE automation_api_name=$1`, autoName)
		_, _ = d.Pool.Exec(bg, `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
		_, _ = seed.DisablePackage(bg, d.Meta, "notes")
	})

	if _, err := seed.EnablePackage(ctx, d.Meta, "notes"); err != nil {
		t.Fatal(err)
	}

	rr := testutil.AuthRequest(srv.Handler, http.MethodPatch, "/metadata/v1/automations/"+autoName, "admin-key", map[string]any{
		"active": false,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("admin PATCH active: %d %s", rr.Code, rr.Body.String())
	}
	var patched map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &patched)
	if patched["active"] != false {
		t.Fatalf("active=%v", patched["active"])
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPatch, "/metadata/v1/automations/"+autoName, "admin-key", map[string]any{
		"label": "nope",
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("admin PATCH label: %d %s", rr.Code, rr.Body.String())
	}
	assertErrorCode(t, rr.Body.Bytes(), "FORBIDDEN")

	rr = testutil.AuthRequest(srv.Handler, http.MethodPatch, "/metadata/v1/automations/"+autoName, "builder-key", map[string]any{
		"active": true,
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("non-admin PATCH active: %d %s", rr.Code, rr.Body.String())
	}
	assertErrorCode(t, rr.Body.Bytes(), "FORBIDDEN")

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/metadata/v1/packages/notes", "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET package: %d %s", rr.Code, rr.Body.String())
	}
	var pkg map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &pkg)
	found := false
	for _, raw := range asSlice(pkg["automationApiNames"]) {
		if raw == autoName {
			found = true
		}
	}
	if !found {
		t.Fatalf("automationApiNames missing %s: %s", autoName, rr.Body.String())
	}
	foundRow := false
	for _, raw := range asSlice(pkg["automations"]) {
		row, _ := raw.(map[string]any)
		if row["apiName"] == autoName {
			foundRow = true
			if row["description"] != "Fixture for PATCH active allowlist." {
				t.Fatalf("package automation description=%v", row["description"])
			}
			if row["active"] != false {
				t.Fatalf("package automation active=%v", row["active"])
			}
			if row["installed"] != true {
				t.Fatalf("installed=%v", row["installed"])
			}
		}
	}
	if !foundRow {
		t.Fatalf("automations row missing: %s", rr.Body.String())
	}

	if _, err := seed.DisablePackage(ctx, d.Meta, "notes"); err != nil {
		t.Fatal(err)
	}
	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/metadata/v1/automations/"+autoName, "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET after pack disable: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/metadata/v1/packages/sales", "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET sales: %d %s", rr.Code, rr.Body.String())
	}
	var sales map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &sales)
	if _, ok := sales["automationApiNames"]; !ok {
		t.Fatalf("sales missing automationApiNames: %s", rr.Body.String())
	}
}
