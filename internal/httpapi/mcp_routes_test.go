package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/config"
	"github.com/MajestaNet/ide/internal/httpapi"
	"github.com/MajestaNet/ide/internal/mcp"
)

func mcpTestServer(t *testing.T, flags ...string) *httpapi.Server {
	t.Helper()
	entries, err := authz.ParseAPIKeyEntries("admin+admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) == 0 {
		flags = []string{"agents"}
	}
	cfg := &config.Config{
		DefaultOwnerID: "00000000-0000-4000-8000-000000000001",
		APIKeyEntries:  entries,
		FeatureFlags:   flags,
	}
	return httpapi.New(httpapi.Options{
		Config:   cfg,
		Resolver: &authz.Resolver{Entries: entries, DefaultOwnerID: cfg.DefaultOwnerID},
	})
}

func TestMCPInitializeAndToolsList(t *testing.T) {
	srv := mcpTestServer(t)
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "0"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("initialize: got %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type=%q", rec.Header().Get("Content-Type"))
	}
	var initResp struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &initResp); err != nil {
		t.Fatal(err)
	}
	if initResp.Result.ProtocolVersion != mcp.LatestProtocolVersion {
		t.Fatalf("protocolVersion=%q", initResp.Result.ProtocolVersion)
	}

	// notifications/initialized → 202
	notify, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "method": "notifications/initialized",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(notify))
	req2.Header.Set("Authorization", "Bearer admin")
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Accept", "application/json")
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusAccepted {
		t.Fatalf("initialized notify: got %d", rec2.Code)
	}

	listPayload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{},
	})
	req3 := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(listPayload))
	req3.Header.Set("Authorization", "Bearer admin")
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Accept", "application/json, text/event-stream")
	rec3 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("tools/list: got %d %s", rec3.Code, rec3.Body.String())
	}
	var listResp struct {
		Result struct {
			Tools []mcp.ToolDesc `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec3.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Result.Tools) == 0 {
		t.Fatal("expected tools")
	}
}

func TestMCPGetAndDeleteMethodNotAllowed(t *testing.T) {
	srv := mcpTestServer(t)
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		req := httptest.NewRequest(method, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer admin")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s /mcp: got %d", method, rec.Code)
		}
	}
}

func TestMCPRejectsOversizedBody(t *testing.T) {
	srv := mcpTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(bytes.Repeat([]byte(" "), (1<<20)+1)))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestMCPLoopbackOriginReject(t *testing.T) {
	srv := mcpTestServer(t)
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "ping", "params": map[string]any{},
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(payload))
	req.Host = "localhost:8080"
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rec.Code, rec.Body.String())
	}
}
