package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/testutil"
)

func TestHTTPFamiliesIntegration(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})

	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin,agent-key:client,meta-key:metadata,deploy-key:deploy",
	})
	h := srv.Handler
	auth := func(method, path, key string, body any) *httptest.ResponseRecorder {
		return testutil.AuthRequest(h, method, path, key, body)
	}

	t.Run("client create and query", func(t *testing.T) {
		rr := auth(http.MethodPost, "/client/v1/sobjects/Account", "admin-key", map[string]any{
			"Name": "Acme " + time.Now().Format("150405.000"),
		})
		if rr.Code != 201 && rr.Code != 200 {
			t.Fatalf("create status %d body=%s", rr.Code, rr.Body.String())
		}
		var created map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &created)
		id, _ := created["Id"].(string)
		if id == "" {
			t.Fatalf("missing id: %s", rr.Body.String())
		}

		rr = auth(http.MethodPost, "/client/v1/query", "admin-key", map[string]any{
			"object": "Account",
			"limit":  5,
		})
		if rr.Code != 200 {
			t.Fatalf("query status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("contact without account", func(t *testing.T) {
		rr := auth(http.MethodPost, "/client/v1/sobjects/Contact", "admin-key", map[string]any{
			"LastName": "Solo" + time.Now().Format("150405"),
			"Email":    "solo@example.com",
		})
		if rr.Code != 201 && rr.Code != 200 {
			t.Fatalf("create contact without AccountId status %d body=%s", rr.Code, rr.Body.String())
		}
		var created map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &created)
		if id, _ := created["Id"].(string); id == "" {
			t.Fatalf("missing contact id: %s", rr.Body.String())
		}
	})

	t.Run("metadata list and write", func(t *testing.T) {
		rr := auth(http.MethodGet, "/metadata/v1/objects", "meta-key", nil)
		if rr.Code != 200 {
			t.Fatalf("list status %d body=%s", rr.Code, rr.Body.String())
		}
		name := "HttpObj" + time.Now().Format("150405")
		// Admin still works.
		rr = auth(http.MethodPost, "/metadata/v1/objects", "admin-key", map[string]any{
			"apiName": name,
			"label":   name,
		})
		if rr.Code != 201 && rr.Code != 200 {
			t.Fatalf("create object status %d body=%s", rr.Code, rr.Body.String())
		}
		// Non-admin meta-key gets MetadataDeveloper + metadata.customize via API key binding.
		name2 := "HttpBuilder" + time.Now().Format("150405")
		rr = auth(http.MethodPost, "/metadata/v1/objects", "meta-key", map[string]any{
			"apiName": name2,
			"label":   name2,
		})
		if rr.Code != 201 && rr.Code != 200 {
			t.Fatalf("builder create object status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("deploy environment and promote capability", func(t *testing.T) {
		rr := auth(http.MethodGet, "/deploy/v1/environment", "deploy-key", nil)
		if rr.Code != 200 {
			t.Fatalf("env status %d body=%s", rr.Code, rr.Body.String())
		}
		// deploy-key gets DeployBot + deploy.promote; create empty-ish bundle request may 400 but not 403 CAPABILITY
		rr = auth(http.MethodPost, "/deploy/v1/bundles", "deploy-key", map[string]any{})
		if rr.Code == http.StatusForbidden {
			t.Fatalf("deploy-key should have deploy.promote, got 403: %s", rr.Body.String())
		}
	})

	t.Run("scope gates", func(t *testing.T) {
		if rr := auth(http.MethodGet, "/metadata/v1/objects", "agent-key", nil); rr.Code != 403 {
			t.Fatalf("agent metadata want 403 got %d", rr.Code)
		}
		if rr := auth(http.MethodGet, "/deploy/v1/environment", "agent-key", nil); rr.Code != 403 {
			t.Fatalf("agent deploy want 403 got %d", rr.Code)
		}
	})

	t.Run("openapi removed", func(t *testing.T) {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
		if rr.Code != 404 {
			t.Fatalf("want 404 got %d", rr.Code)
		}
	})
}
