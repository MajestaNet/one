package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/identity"
	"github.com/MajestaNet/ide/internal/integration"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestIntegrationsConnectedAppsHarness(t *testing.T) {
	d := testutil.RequireDatabase(t)
	_, _ = d.Pool.Exec(t.Context(), `DELETE FROM identity_links WHERE provider='memory'`)
	_, _ = d.Pool.Exec(t.Context(), `DELETE FROM integration_configs WHERE api_name LIKE 'acme.orders.%'`)
	memID := identity.NewMemoryBackend()
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{
		Identity:      memID,
		EncryptionKey: "test-enc-key-for-integrations!!",
	})

	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:       "admin-key+admin",
		EnableJWT:     true,
		Identity:      memID,
		JWTSigningKey: "test-enc-key-for-integrations!!",
	})
	srv.Config.WebhookEncryptionKey = "test-enc-key-for-integrations!!"
	h := srv.Handler
	auth := func(method, path, bearer string, body any) *httptest.ResponseRecorder {
		return testutil.AuthRequest(h, method, path, bearer, body)
	}

	// OOTB Control IDE integration from seed.
	rr := auth(http.MethodGet, "/client/v1/integrations/"+integration.APINameControlIDE, "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("get control ide %d %s", rr.Code, rr.Body.String())
	}
	var ide map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &ide)
	if ide["clientKind"] != "public" {
		t.Fatalf("expected public, got %v", ide["clientKind"])
	}
	if ide["ownership"] != "managed" {
		t.Fatalf("expected managed, got %v", ide["ownership"])
	}
	if ide["pkceRequired"] != true {
		t.Fatalf("expected pkceRequired")
	}

	// Managed cannot be deleted.
	rr = auth(http.MethodDelete, "/client/v1/integrations/"+integration.APINameControlIDE, "admin-key", nil)
	if rr.Code != 403 {
		t.Fatalf("delete managed want 403 got %d %s", rr.Code, rr.Body.String())
	}

	// Create confidential M2M integration.
	apiName := "acme.orders." + time.Now().Format("150405.000")
	rr = auth(http.MethodPost, "/client/v1/integrations", "admin-key", map[string]any{
		"apiName":      apiName,
		"label":        "Acme Orders",
		"clientKind":   "confidential",
		"oauthFlows":   []string{"client_credentials"},
		"roleApiNames": []string{"StandardUser"},
	})
	if rr.Code != 201 {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	secret, _ := created["oneClientSecret"].(string)
	pid, _ := created["principalId"].(string)
	if secret == "" || pid == "" {
		t.Fatalf("missing secret/principal: %v", created)
	}
	if created["hasOneSecret"] != true {
		t.Fatal("expected hasOneSecret")
	}
	// Identity app client is linked via identity_links (not integration_configs columns).
	if len(memID.Clients) == 0 {
		t.Fatal("expected identity app client from memory backend")
	}

	// Mint JWT with one client credentials.
	rr = auth(http.MethodPost, "/auth/v1/token", "", map[string]any{
		"grant_type": "client_credentials", "client_id": pid, "client_secret": secret,
	})
	if rr.Code != 200 {
		t.Fatalf("token %d %s", rr.Code, rr.Body.String())
	}
	var tok map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &tok)
	access, _ := tok["access_token"].(string)
	if access == "" {
		t.Fatal("missing access_token")
	}

	// Reveal returns the same secret.
	rr = auth(http.MethodPost, "/client/v1/integrations/"+apiName+"/secrets/reveal", "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("reveal %d %s", rr.Code, rr.Body.String())
	}
	var revealed map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &revealed)
	if revealed["oneClientSecret"] != secret {
		t.Fatalf("reveal mismatch: %v vs %s", revealed["oneClientSecret"], secret)
	}

	// Rotate issues a new secret; old one fails.
	rr = auth(http.MethodPost, "/client/v1/integrations/"+apiName+"/secrets/rotate", "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("rotate %d %s", rr.Code, rr.Body.String())
	}
	var rotated map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &rotated)
	newSecret, _ := rotated["oneClientSecret"].(string)
	if newSecret == "" || newSecret == secret {
		t.Fatalf("expected new secret, got %q", newSecret)
	}
	rr = auth(http.MethodPost, "/auth/v1/token", "", map[string]any{
		"grant_type": "client_credentials", "client_id": pid, "client_secret": secret,
	})
	if rr.Code == 200 {
		t.Fatal("old secret should not mint")
	}
	rr = auth(http.MethodPost, "/auth/v1/token", "", map[string]any{
		"grant_type": "client_credentials", "client_id": pid, "client_secret": newSecret,
	})
	if rr.Code != 200 {
		t.Fatalf("new secret token %d %s", rr.Code, rr.Body.String())
	}

	// Patch callbacks on managed IDE.
	rr = auth(http.MethodPatch, "/client/v1/integrations/"+integration.APINameControlIDE, "admin-key", map[string]any{
		"callbackUrls": []string{"http://127.0.0.1:9999/oauth/callback"},
	})
	if rr.Code != 200 {
		t.Fatalf("patch ide %d %s", rr.Code, rr.Body.String())
	}

	// Delete customer integration.
	rr = auth(http.MethodDelete, "/client/v1/integrations/"+apiName, "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("delete %d %s", rr.Code, rr.Body.String())
	}
	rr = auth(http.MethodGet, "/client/v1/integrations/"+apiName, "admin-key", nil)
	if rr.Code != 404 {
		t.Fatalf("get deleted want 404 got %d", rr.Code)
	}

	// List includes control IDE.
	rr = auth(http.MethodGet, "/client/v1/integrations", "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("list %d %s", rr.Code, rr.Body.String())
	}

	// Public Experience client defaults to client scope + StandardUser role.
	portalName := "acme.portal." + time.Now().Format("150405.000")
	rr = auth(http.MethodPost, "/client/v1/integrations", "admin-key", map[string]any{
		"apiName":      portalName,
		"label":        "Acme Portal",
		"clientKind":   "public",
		"oauthFlows":   []string{"authorization_code"},
		"callbackUrls": []string{"http://127.0.0.1:3000/oauth/callback"},
	})
	if rr.Code != 201 {
		t.Fatalf("create public portal %d %s", rr.Code, rr.Body.String())
	}
	var portal map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &portal)
	hint, _ := portal["allowedScopesHint"].([]any)
	if len(hint) != 1 || hint[0] != "client" {
		t.Fatalf("expected [client] scopes hint, got %v", portal["allowedScopesHint"])
	}

	// Reject metadata scope on public client.
	rr = auth(http.MethodPost, "/client/v1/integrations", "admin-key", map[string]any{
		"apiName":           "bad.portal." + time.Now().Format("150405.000"),
		"label":             "Bad Portal",
		"clientKind":        "public",
		"oauthFlows":        []string{"authorization_code"},
		"callbackUrls":      []string{"http://127.0.0.1:3000/oauth/callback"},
		"allowedScopesHint": []string{"client", "metadata"},
	})
	if rr.Code != 400 {
		t.Fatalf("public+metadata want 400 got %d %s", rr.Code, rr.Body.String())
	}
}
