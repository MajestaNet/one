package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/config"
	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/httpapi"
)

func revisionTestHandler(t *testing.T, min, current int) http.Handler {
	t.Helper()
	cfg := &config.Config{
		Port:               8080,
		ProductVersion:     "0.4.0",
		APIRevisionMin:     min,
		APIRevisionCurrent: current,
		CustomerID:         "t1",
		InstallID:          "i1",
		InstallRole:        "dev",
		APIKeyEntries:      mustKeys(t, "dev-admin-key+admin"),
		DefaultOwnerID:     "00000000-0000-4000-8000-000000000001",
		RequestBodyLimit:   1 << 20,
		RateLimitPerMinute: 0,
		AuthJWTSigningKey:  "test-signing-key-32bytes-minimum!!",
		AuthJWTIssuer:      "http://localhost:8080/auth/v1",
		AuthJWTTTLSeconds:  3600,
		AuthJWTEnabled:     true,
	}
	resolver := &authz.Resolver{Entries: cfg.APIKeyEntries, DefaultOwnerID: cfg.DefaultOwnerID}
	eng := deploy.NewDeployEngine(nil, nil, nil, deploy.Options{
		InstallID:          cfg.InstallID,
		InstallRole:        cfg.InstallRole,
		ProductVersion:     cfg.ProductVersion,
		APIRevisionMin:     min,
		APIRevisionCurrent: current,
		CustomerID:         cfg.CustomerID,
	})
	return httpapi.New(httpapi.Options{Config: cfg, Resolver: resolver, Deploy: eng}).Handler()
}

func TestAPIRevisionMiddleware(t *testing.T) {
	h := revisionTestHandler(t, 12, 14)

	t.Run("version includes apiRevision", func(t *testing.T) {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/version", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
		var body map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		rev, ok := body["apiRevision"].(map[string]any)
		if !ok {
			t.Fatalf("apiRevision missing: %v", body)
		}
		if rev["min"] != float64(12) || rev["current"] != float64(14) || rev["recommended"] != float64(14) {
			t.Fatalf("apiRevision=%v", rev)
		}
		if body["httpApi"] == nil {
			t.Fatal("httpApi missing")
		}
	})

	t.Run("me echoes apiRevision", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/client/v1/me", nil)
		req.Header.Set("X-API-Key", "dev-admin-key")
		req.Header.Set("One-API-Revision", "12")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
		var body map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		rev, ok := body["apiRevision"].(map[string]any)
		if !ok {
			t.Fatalf("apiRevision missing: %v", body)
		}
		if rev["current"] != float64(14) {
			t.Fatalf("apiRevision=%v", rev)
		}
	})

	t.Run("deploy environment includes apiRevision", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/deploy/v1/environment", nil)
		req.Header.Set("X-API-Key", "dev-admin-key")
		req.Header.Set("One-API-Revision", "14")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
		var body map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		rev, ok := body["apiRevision"].(map[string]any)
		if !ok {
			t.Fatalf("apiRevision missing: %v", body)
		}
		if rev["min"] != float64(12) || rev["current"] != float64(14) || rev["recommended"] != float64(14) {
			t.Fatalf("apiRevision=%v", rev)
		}
	})

	t.Run("default pin uses current", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/client/v1/me", nil)
		req.Header.Set("X-API-Key", "dev-admin-key")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("pin below min rejected with cta", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/client/v1/me", nil)
		req.Header.Set("X-API-Key", "dev-admin-key")
		req.Header.Set("One-API-Revision", "11")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
		var body map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		if body["error"] != "API_REVISION_UNSUPPORTED" {
			t.Fatalf("body=%v", body)
		}
		cta, _ := body["cta"].(string)
		if cta == "" {
			t.Fatalf("missing cta: %v", body)
		}
	})

	t.Run("pin above current rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/client/v1/me", nil)
		req.Header.Set("X-API-Key", "dev-admin-key")
		req.Header.Set("One-API-Revision", "15")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("path alias rN rewrites onto family route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/client/v1/r12/me", nil)
		req.Header.Set("X-API-Key", "dev-admin-key")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("path alias out of window rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/client/v1/r11/me", nil)
		req.Header.Set("X-API-Key", "dev-admin-key")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("header wins over path alias", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/client/v1/r12/me", nil)
		req.Header.Set("X-API-Key", "dev-admin-key")
		req.Header.Set("One-API-Revision", "14")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("auth family omitted pin defaults to current", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/v1/login", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("auth family rejects out of window pin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/v1/login", nil)
		req.Header.Set("One-API-Revision", "11")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("scim stays revision-agnostic", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
		req.Header.Set("One-API-Revision", "11")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code == http.StatusBadRequest {
			t.Fatalf("SCIM must not enforce API revision, body=%s", rr.Body.String())
		}
	})

	t.Run("healthz skips revision", func(t *testing.T) {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d", rr.Code)
		}
	})
}

func TestAPIRevisionCompatMatrix(t *testing.T) {
	// Env-simulated last-N matrix: one binary, overlapping windows.
	type row struct {
		min, current, pin int
		want              int
	}
	cases := []row{
		{12, 14, 14, http.StatusOK},
		{12, 14, 12, http.StatusOK},
		{12, 14, 11, http.StatusBadRequest},
		{12, 14, 15, http.StatusBadRequest},
		{1, 1, 1, http.StatusOK},
		{13, 15, 12, http.StatusBadRequest},
		{13, 15, 13, http.StatusOK},
	}
	for _, tc := range cases {
		h := revisionTestHandler(t, tc.min, tc.current)
		req := httptest.NewRequest(http.MethodGet, "/client/v1/me", nil)
		req.Header.Set("X-API-Key", "dev-admin-key")
		req.Header.Set("One-API-Revision", strconv.Itoa(tc.pin))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != tc.want {
			t.Fatalf("window=[%d,%d] pin=%d status=%d want=%d body=%s",
				tc.min, tc.current, tc.pin, rr.Code, tc.want, rr.Body.String())
		}
	}
}

func TestMCPAPIRevision(t *testing.T) {
	ping, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "ping", "params": map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}

	mcpPOST := func(h http.Handler, path string, revHeader string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(ping))
		req.Header.Set("X-API-Key", "dev-admin-key")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		if revHeader != "" {
			req.Header.Set("One-API-Revision", revHeader)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	t.Run("out of window pin rejected", func(t *testing.T) {
		h := revisionTestHandler(t, 12, 14)
		rr := mcpPOST(h, "/mcp", "99")
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
		var body map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		if body["error"] != "API_REVISION_UNSUPPORTED" {
			t.Fatalf("body=%v", body)
		}
	})

	t.Run("omitted header defaults to current JSON-RPC", func(t *testing.T) {
		h := revisionTestHandler(t, 12, 14)
		rr := mcpPOST(h, "/mcp", "")
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
		var body map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		if body["jsonrpc"] != "2.0" {
			t.Fatalf("expected JSON-RPC, body=%v", body)
		}
	})

	t.Run("path alias r1 rewrites onto handleMCP", func(t *testing.T) {
		h := revisionTestHandler(t, 1, 1)
		rr := mcpPOST(h, "/mcp/r1", "")
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
		var body map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		if body["jsonrpc"] != "2.0" {
			t.Fatalf("expected JSON-RPC, body=%v", body)
		}
	})
}
