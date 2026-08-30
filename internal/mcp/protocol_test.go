package mcp_test

import (
	"testing"

	"github.com/MajestaNet/ide/internal/mcp"
)

func TestNegotiateProtocolVersion(t *testing.T) {
	if got := mcp.NegotiateProtocolVersion("2025-03-26"); got != "2025-03-26" {
		t.Fatalf("echo supported: got %q", got)
	}
	if got := mcp.NegotiateProtocolVersion("2024-11-05"); got != "2024-11-05" {
		t.Fatalf("echo legacy: got %q", got)
	}
	if got := mcp.NegotiateProtocolVersion("9999-01-01"); got != mcp.LatestProtocolVersion {
		t.Fatalf("fallback: got %q", got)
	}
}

func TestInitializeResultHasToolsCapability(t *testing.T) {
	res := mcp.InitializeResult(mcp.LatestProtocolVersion)
	caps, _ := res["capabilities"].(map[string]any)
	if caps == nil {
		t.Fatal("missing capabilities")
	}
	if _, ok := caps["tools"]; !ok {
		t.Fatal("missing tools capability")
	}
	info, _ := res["serverInfo"].(map[string]any)
	if info["name"] != mcp.ServerName {
		t.Fatalf("serverInfo.name=%v", info["name"])
	}
}

func TestIsLoopbackHost(t *testing.T) {
	cases := map[string]bool{
		"localhost":      true,
		"localhost:8080": true,
		"127.0.0.1":      true,
		"127.0.0.1:3000": true,
		"::1":            true,
		"[::1]":          true,
		"[::1]:8080":     true,
		"example.com":    false,
		"api.one":        false,
		"  LOCALHOST  ":  true,
	}
	for host, want := range cases {
		if got := mcp.IsLoopbackHost(host); got != want {
			t.Fatalf("IsLoopbackHost(%q)=%v want %v", host, got, want)
		}
	}
}

func TestIsNotification(t *testing.T) {
	if !mcp.IsNotification("notifications/initialized") {
		t.Fatal("expected notification")
	}
	if mcp.IsNotification("initialize") {
		t.Fatal("initialize is a request")
	}
}
