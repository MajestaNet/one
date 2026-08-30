package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/identity"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestPrincipalListIncludeDataAndFieldAudit(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:   "admin-key+admin",
		EnableJWT: true,
	})
	h := srv.Handler
	auth := func(method, path, bearer string, body any) *httptest.ResponseRecorder {
		return testutil.AuthRequest(h, method, path, bearer, body)
	}

	stamp := time.Now().Format("150405.000")
	fieldName := "CostCenter" + strings.ReplaceAll(stamp, ".", "") + "__c"
	rr := auth(http.MethodPost, "/metadata/v1/fields", "admin-key", map[string]any{
		"objectApiName": "User", "apiName": fieldName, "label": "Cost Center", "fieldType": "text",
	})
	if rr.Code != 201 {
		t.Fatalf("field %d %s", rr.Code, rr.Body.String())
	}

	email := "harden-" + stamp + "@example.com"
	rr = auth(http.MethodPost, "/client/v1/principals", "admin-key", map[string]any{
		"email": email, "displayName": "Harden User",
		"roleApiNames": []string{"StandardUser"},
		fieldName:      "CC-100",
	})
	if rr.Code != 201 {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	pid, _ := created["id"].(string)
	if pid == "" {
		t.Fatal("missing id")
	}

	rr = auth(http.MethodPatch, "/client/v1/principals/"+pid, "admin-key", map[string]any{
		fieldName:        "CC-SECRET",
		"employeeNumber": "E-" + stamp,
	})
	if rr.Code != 200 {
		t.Fatalf("patch %d %s", rr.Code, rr.Body.String())
	}

	details := latestAuditJSON(t, d, "identity.user.field.patch", pid)
	if !strings.Contains(details, fieldName) || !strings.Contains(details, "EmployeeNumber") {
		t.Fatalf("field patch audit missing apiNames: %s", details)
	}
	if strings.Contains(details, "CC-SECRET") || strings.Contains(details, "E-"+stamp) {
		t.Fatalf("field patch audit must not include values: %s", details)
	}

	rr = auth(http.MethodGet, "/client/v1/principals", "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("list %d %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "CC-SECRET") {
		t.Fatalf("default list must omit custom data: %s", rr.Body.String())
	}

	rr = auth(http.MethodGet, "/client/v1/principals?include=data", "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("list include=data %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "CC-SECRET") {
		t.Fatalf("include=data should return custom field: %s", rr.Body.String())
	}
}

func TestSCIMUserUpdateAuditFieldAPINames(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:   "admin-key+admin",
		EnableJWT: true,
		Identity:  identity.NewMemoryBackend(),
	})
	h := srv.Handler
	auth := func(method, path, bearer string, body any) *httptest.ResponseRecorder {
		return testutil.AuthRequest(h, method, path, bearer, body)
	}

	stamp := time.Now().Format("150405.000")
	fieldName := "CostCenter" + strings.ReplaceAll(stamp, ".", "") + "__c"
	rr := auth(http.MethodPost, "/metadata/v1/fields", "admin-key", map[string]any{
		"objectApiName": "User", "apiName": fieldName, "label": "Cost Center", "fieldType": "text",
	})
	if rr.Code != 201 {
		t.Fatalf("field %d %s", rr.Code, rr.Body.String())
	}

	userName := "scim-harden-" + stamp
	rr = auth(http.MethodPost, "/scim/v2/Users", "admin-key", map[string]any{
		"schemas": []string{
			"urn:ietf:params:scim:schemas:core:2.0:User",
			"urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom",
		},
		"userName":    userName,
		"displayName": "SCIM Harden",
		"emails":      []map[string]any{{"value": userName + "@example.com", "primary": true}},
		"active":      true,
		"urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom": map[string]any{
			fieldName: "CC-1",
		},
	})
	if rr.Code != 201 {
		t.Fatalf("scim create %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("missing id")
	}

	rr = auth(http.MethodPatch, "/scim/v2/Users/"+id, "admin-key", map[string]any{
		"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": []map[string]any{
			{
				"op":    "add",
				"path":  "urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom:" + fieldName,
				"value": "CC-OKTA-SECRET",
			},
		},
	})
	if rr.Code != 200 {
		t.Fatalf("scim patch %d %s", rr.Code, rr.Body.String())
	}

	details := latestAuditJSON(t, d, "scim.user.update", id)
	if !strings.Contains(details, fieldName) {
		t.Fatalf("scim update audit missing %s: %s", fieldName, details)
	}
	if strings.Contains(details, "CC-OKTA-SECRET") {
		t.Fatalf("scim update audit must not include values: %s", details)
	}
}

func latestAuditJSON(t *testing.T, d *testutil.Database, action, recordID string) string {
	t.Helper()
	var raw []byte
	err := d.Pool.QueryRow(t.Context(), `
SELECT details FROM audit_log
WHERE action = $1 AND record_id = $2::uuid
ORDER BY created_at DESC
LIMIT 1`, action, recordID).Scan(&raw)
	if err != nil {
		t.Fatalf("audit %s record %s: %v", action, recordID, err)
	}
	return string(raw)
}
