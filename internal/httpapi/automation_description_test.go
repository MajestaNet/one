package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/testutil"
)

func TestAutomationDescriptionRoundTrip(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin",
	})
	const autoName = "DescRoundTrip_BP069"
	cleanup := func() {
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM automation_permissions WHERE automation_api_name=$1`, autoName)
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
	}
	cleanup()
	t.Cleanup(cleanup)

	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/automations", "admin-key", map[string]any{
		"apiName": autoName, "label": "Desc round trip", "objectApiName": "Account",
		"triggerEvent": "create", "active": true, "runtime": "actions", "execution": "async",
		"actions":     []any{},
		"description": "Creates a follow-up when an Account is inserted.",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST: %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created["description"] != "Creates a follow-up when an Account is inserted." {
		t.Fatalf("POST description=%v", created["description"])
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/metadata/v1/automations/"+autoName, "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET-by-name: %d %s", rr.Code, rr.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["description"] != "Creates a follow-up when an Account is inserted." {
		t.Fatalf("GET description=%v", got["description"])
	}
	if got["apiName"] != autoName {
		t.Fatalf("GET apiName=%v", got["apiName"])
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPatch, "/metadata/v1/automations/"+autoName, "admin-key", map[string]any{
		"description": "Updated functional description.",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH: %d %s", rr.Code, rr.Body.String())
	}
	var patched map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched["description"] != "Updated functional description." {
		t.Fatalf("PATCH description=%v", patched["description"])
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/metadata/v1/automations", "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rr.Code, rr.Body.String())
	}
	var listed map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, raw := range asSlice(listed["automations"]) {
		row, _ := raw.(map[string]any)
		if row["apiName"] == autoName {
			found = true
			if row["description"] != "Updated functional description." {
				t.Fatalf("list description=%v", row["description"])
			}
		}
	}
	if !found {
		t.Fatal("list missing created automation")
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPatch, "/metadata/v1/automations/"+autoName, "admin-key", map[string]any{
		"description": strings.Repeat("x", 501),
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("oversize PATCH: %d %s", rr.Code, rr.Body.String())
	}
	assertErrorCode(t, rr.Body.Bytes(), "VALIDATION_ERROR")

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/automations", "admin-key", map[string]any{
		"apiName": "TooLongDesc_BP069", "label": "Too long", "objectApiName": "Account",
		"triggerEvent": "create", "description": strings.Repeat("y", 501),
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("oversize POST: %d %s", rr.Code, rr.Body.String())
	}
	assertErrorCode(t, rr.Body.Bytes(), "VALIDATION_ERROR")

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/metadata/v1/automations/DoesNotExist_BP069", "admin-key", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing GET: %d %s", rr.Code, rr.Body.String())
	}
	assertErrorCode(t, rr.Body.Bytes(), "NOT_FOUND")
}
