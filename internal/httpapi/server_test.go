package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/config"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/httpapi"
)

func TestHealthAndAuth(t *testing.T) {
	cfg := &config.Config{
		Port:               8080,
		ProductVersion:     "0.1.0",
		CustomerID:         "t1",
		InstallID:          "i1",
		InstallRole:        "dev",
		APIKeyEntries:      mustKeys(t, "dev-admin-key+admin,dev-agent-key:client,dev-client-admin-key:client+admin,dev-metadata-key:metadata"),
		DefaultOwnerID:     "00000000-0000-4000-8000-000000000001",
		RequestBodyLimit:   1 << 20,
		RateLimitPerMinute: 0,
	}
	resolver := &authz.Resolver{
		Entries:        cfg.APIKeyEntries,
		DefaultOwnerID: cfg.DefaultOwnerID,
	}
	s := httpapi.New(httpapi.Options{Config: cfg, Resolver: resolver})
	h := s.Handler()

	t.Run("healthz", func(t *testing.T) {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rr.Code != 200 {
			t.Fatalf("status %d", rr.Code)
		}
		var body map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		if body["runtime"] != "go" {
			t.Fatalf("body=%v", body)
		}
	})

	t.Run("starting gate keeps healthz and blocks auth", func(t *testing.T) {
		s.SetReady(false)
		defer s.SetReady(true)

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rr.Code != 200 {
			t.Fatalf("healthz during start status %d", rr.Code)
		}

		rr = httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("readyz during start status %d body=%s", rr.Code, rr.Body.String())
		}
		var ready map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &ready)
		if ready["status"] != "starting" {
			t.Fatalf("readyz body=%v", ready)
		}

		rr = httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/v1/login", nil))
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("login during start status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("readyz skipped for typed-nil pool", func(t *testing.T) {
		var pool *db.Pool
		sNil := httpapi.New(httpapi.Options{Config: cfg, Resolver: resolver, DB: pool})
		rr := httptest.NewRecorder()
		sNil.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
		var body map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		if body["database"] != "skipped" {
			t.Fatalf("body=%v", body)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/client/v1/me", nil))
		if rr.Code != 401 {
			t.Fatalf("status %d", rr.Code)
		}
		if rr.Header().Get("WWW-Authenticate") == "" {
			t.Fatal("missing WWW-Authenticate challenge")
		}
	})

	t.Run("authorization requires bearer scheme", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/client/v1/me", nil)
		req.Header.Set("Authorization", "Basic dev-admin-key")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("x-api-key remains supported", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/client/v1/me", nil)
		req.Header.Set("X-API-Key", "dev-admin-key")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("public version omits install identity", func(t *testing.T) {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/version", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"customerId", "installId", "installRole"} {
			if _, ok := body[key]; ok {
				t.Fatalf("public version leaked %s: %v", key, body)
			}
		}
	})

	t.Run("login page has restrictive content policy", func(t *testing.T) {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/v1/login", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
		if got := rr.Header().Get("Content-Security-Policy"); got == "" {
			t.Fatal("missing Content-Security-Policy")
		}
	})

	t.Run("me with api key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/client/v1/me", nil)
		req.Header.Set("Authorization", "Bearer dev-admin-key")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != 200 {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("describe without metadata service", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/client/v1/describe", nil)
		req.Header.Set("Authorization", "Bearer dev-agent-key")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != 503 {
			t.Fatalf("status %d", rr.Code)
		}
	})

	t.Run("v1 alias routes exist", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
		req.Header.Set("Authorization", "Bearer dev-admin-key")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != 200 {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("metadata scope required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metadata/v1/objects", nil)
		req.Header.Set("Authorization", "Bearer dev-agent-key")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != 403 {
			t.Fatalf("status %d", rr.Code)
		}
	})

	t.Run("admin does not bypass family scope", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ops/v1/upgrades", nil)
		req.Header.Set("Authorization", "Bearer dev-client-admin-key")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("metadata mutation requires capability", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/metadata/v1/objects", nil)
		req.Header.Set("Authorization", "Bearer dev-metadata-key")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
	})
}

func mustKeys(t *testing.T, raw string) []authz.APIKeyEntry {
	t.Helper()
	e, err := authz.ParseAPIKeyEntries(raw)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
