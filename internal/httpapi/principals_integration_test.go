package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/edge"
	"github.com/MajestaNet/ide/internal/identity"
	"github.com/MajestaNet/ide/internal/testutil"
)

// TestPrincipalCredentialClientHarness seeds a service principal, issues a secret,
// mints a Majesta One JWT, and calls Client API — the Admin UI → integration path.
func TestPrincipalCredentialClientHarness(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})

	memEdge := &edge.MemoryRoller{}
	memID := identity.NewMemoryBackend()
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:    "admin-key+admin",
		EnableJWT:  true,
		EdgeRoller: memEdge,
		Identity:   memID,
	})
	h := srv.Handler
	auth := func(method, path, bearer string, body any) *httptest.ResponseRecorder {
		return testutil.AuthRequest(h, method, path, bearer, body)
	}

	email := "mulesoft-" + time.Now().Format("150405.000") + "@example.com"
	rr := auth(http.MethodPost, "/client/v1/principals", "admin-key", map[string]any{
		"email": email, "displayName": "MuleSoft Orders", "principalType": "service",
		"roleApiNames": []string{"StandardUser"}, "permissionSetApiNames": []string{"Admin"},
	})
	if rr.Code != 201 {
		t.Fatalf("create principal %d %s", rr.Code, rr.Body.String())
	}
	var principal map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &principal)
	pid, _ := principal["id"].(string)
	if pid == "" {
		t.Fatal("missing id")
	}

	rr = auth(http.MethodPost, "/client/v1/principals/"+pid+"/credentials", "admin-key", map[string]any{
		"label": "mulesoft",
	})
	if rr.Code != 201 {
		t.Fatalf("issue credential %d %s", rr.Code, rr.Body.String())
	}
	var cred map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &cred)
	secret, _ := cred["clientSecret"].(string)
	if secret == "" {
		t.Fatal("missing clientSecret")
	}

	// Mint JWT as the integration
	tokBody, _ := json.Marshal(map[string]any{
		"grant_type": "client_credentials", "client_id": pid, "client_secret": secret,
	})
	tokReq := httptest.NewRequest(http.MethodPost, "/auth/v1/token", bytes.NewReader(tokBody))
	tokReq.Header.Set("Content-Type", "application/json")
	tokRR := httptest.NewRecorder()
	h.ServeHTTP(tokRR, tokReq)
	if tokRR.Code != 200 {
		t.Fatalf("token %d %s", tokRR.Code, tokRR.Body.String())
	}
	var tok map[string]any
	_ = json.Unmarshal(tokRR.Body.Bytes(), &tok)
	access, _ := tok["access_token"].(string)

	// Client call with JWT — no metadata scope needed
	rr = auth(http.MethodPost, "/client/v1/sobjects/Account", access, map[string]any{
		"Name": "From Integration " + time.Now().Format("150405"),
	})
	if rr.Code != 201 && rr.Code != 200 {
		t.Fatalf("client create %d %s", rr.Code, rr.Body.String())
	}

	// Integration JWT must not access Metadata
	rr = auth(http.MethodGet, "/metadata/v1/objects", access, nil)
	if rr.Code != 403 {
		t.Fatalf("expected 403 metadata, got %d", rr.Code)
	}

	// Exposure policy via Memory roller
	rr = auth(http.MethodPut, "/metadata/v1/install/exposure", "admin-key", map[string]any{
		"client":   map[string]any{"mode": "public", "cidrs": []string{}},
		"auth":     map[string]any{"mode": "public", "cidrs": []string{}},
		"metadata": map[string]any{"mode": "allowlist", "cidrs": []string{"10.0.0.0/8"}},
		"deploy":   map[string]any{"mode": "blocked", "cidrs": []string{}},
		"ops":      map[string]any{"mode": "blocked", "cidrs": []string{}},
	})
	if rr.Code != 200 {
		t.Fatalf("put exposure %d %s", rr.Code, rr.Body.String())
	}
	var exp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &exp)
	if exp["status"] != "applied" {
		t.Fatalf("status=%v body=%s", exp["status"], rr.Body.String())
	}
	if exp["rollerMode"] != "local" {
		t.Fatalf("rollerMode=%v", exp["rollerMode"])
	}
	if memEdge.Last.Metadata.Mode != edge.ModeAllowlist {
		t.Fatalf("memory roller not updated: %+v", memEdge.Last)
	}

	// List roles
	rr = auth(http.MethodGet, "/client/v1/roles", "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("roles %d %s", rr.Code, rr.Body.String())
	}
}
