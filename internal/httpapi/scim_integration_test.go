package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/identity"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestSCIMUserLifecycle(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:   "admin-key+admin",
		EnableJWT: true,
		Identity:  identity.NewMemoryBackend(),
	})
	h := srv.Handler

	stamp := time.Now().Format("150405.000")
	userName := "jdoe-" + stamp
	email := userName + "@example.com"

	rr := testutil.AuthRequest(h, http.MethodPost, "/scim/v2/Users", "admin-key", map[string]any{
		"schemas": []string{
			"urn:ietf:params:scim:schemas:core:2.0:User",
			"urn:ietf:params:scim:schemas:extension:one:2.0:Principal",
		},
		"userName":    userName,
		"externalId":  "emp-" + stamp,
		"displayName": "Jane Doe",
		"name":        map[string]any{"givenName": "Jane", "familyName": "Doe"},
		"emails":      []map[string]any{{"value": email, "primary": true, "type": "work"}},
		"active":      true,
		"urn:ietf:params:scim:schemas:extension:one:2.0:Principal": map[string]any{
			"principalType": "user",
			"roleApiNames":  []string{"StandardUser"},
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
	if created["userName"] != userName {
		t.Fatalf("userName=%v", created["userName"])
	}

	rr = testutil.AuthRequest(h, http.MethodGet, "/scim/v2/Users/"+id, "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("scim get %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(h, http.MethodGet, "/scim/v2/Users?filter=userName%20eq%20%22"+userName+"%22", "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("scim list %d %s", rr.Code, rr.Body.String())
	}
	var list map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &list)
	if int(list["totalResults"].(float64)) < 1 {
		t.Fatalf("expected >=1 result: %s", rr.Body.String())
	}

	rr = testutil.AuthRequest(h, http.MethodPatch, "/scim/v2/Users/"+id, "admin-key", map[string]any{
		"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": []map[string]any{
			{"op": "replace", "path": "active", "value": false},
		},
	})
	if rr.Code != 200 {
		t.Fatalf("scim deactivate %d %s", rr.Code, rr.Body.String())
	}
	var deactivated map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &deactivated)
	if deactivated["active"] != false {
		t.Fatalf("expected active=false: %v", deactivated["active"])
	}

	rr = testutil.AuthRequest(h, http.MethodPatch, "/scim/v2/Users/"+id, "admin-key", map[string]any{
		"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": []map[string]any{
			{"op": "replace", "path": "active", "value": true},
		},
	})
	if rr.Code != 200 {
		t.Fatalf("scim reactivate %d %s", rr.Code, rr.Body.String())
	}
}

func TestSCIMServicePrincipalExtension(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:   "admin-key+admin",
		EnableJWT: true,
		Identity:  identity.NewMemoryBackend(),
	})
	h := srv.Handler
	stamp := time.Now().Format("150405.000")
	userName := "billing-bot-" + stamp

	rr := testutil.AuthRequest(h, http.MethodPost, "/scim/v2/Users", "admin-key", map[string]any{
		"schemas": []string{
			"urn:ietf:params:scim:schemas:core:2.0:User",
			"urn:ietf:params:scim:schemas:extension:one:2.0:Principal",
		},
		"userName":    userName,
		"displayName": "Billing Bot",
		"active":      true,
		"urn:ietf:params:scim:schemas:extension:one:2.0:Principal": map[string]any{
			"principalType": "service",
			"roleApiNames":  []string{"StandardUser"},
		},
	})
	if rr.Code != 201 {
		t.Fatalf("scim create service %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	id, _ := created["id"].(string)
	ext, _ := created["urn:ietf:params:scim:schemas:extension:one:2.0:Principal"].(map[string]any)
	if ext == nil || ext["principalType"] != "service" {
		t.Fatalf("expected service extension: %v", created)
	}

	rr = testutil.AuthRequest(h, http.MethodDelete, "/scim/v2/Users/"+id, "admin-key", nil)
	if rr.Code != 204 {
		t.Fatalf("scim delete %d %s", rr.Code, rr.Body.String())
	}
	rr = testutil.AuthRequest(h, http.MethodGet, "/scim/v2/Users/"+id, "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("scim get after delete %d %s", rr.Code, rr.Body.String())
	}
	var after map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &after)
	if after["active"] != false {
		t.Fatalf("expected soft-deleted active=false, got %v", after["active"])
	}
}
