package egress

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestHostAllowed(t *testing.T) {
	allow := []string{"api.partner.com", "*.example.com", ".corp.internal"}
	cases := []struct {
		host string
		ok   bool
	}{
		{"api.partner.com", true},
		{"foo.example.com", true},
		{"example.com", true},
		{"evil.com", false},
		{"corp.internal", true},
		{"x.corp.internal", true},
		{"", false},
	}
	for _, c := range cases {
		if got := HostAllowed(c.host, allow); got != c.ok {
			t.Fatalf("host %q: got %v want %v", c.host, got, c.ok)
		}
	}
	if HostAllowed("api.partner.com", nil) {
		t.Fatal("empty allowlist must deny")
	}
}

func TestAllowlistedHeader(t *testing.T) {
	if !AllowlistedHeader("Authorization") || !AllowlistedHeader("X-Customer-Id") {
		t.Fatal("expected allow")
	}
	if AllowlistedHeader("Cookie") || AllowlistedHeader("Host") {
		t.Fatal("expected deny")
	}
}

func TestJoinURL(t *testing.T) {
	got, err := JoinURL("https://api.example.com/v1/", "items")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://api.example.com/v1/items" {
		t.Fatalf("got %q", got)
	}
	if _, err := JoinURL("https://api.example.com", "https://evil.com"); err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestValidateURLRejectsHTTP(t *testing.T) {
	if err := ValidateURL("http://example.com"); err == nil {
		t.Fatal("expected reject")
	}
	if err := ValidateURL("https://localhost/x"); err == nil {
		t.Fatal("expected localhost reject")
	}
}

func TestSafeDialRejectsBlockedLiteralAtConnectTime(t *testing.T) {
	_, err := safeDialContext(context.Background(), "tcp", "127.0.0.1:443", false)
	if err == nil {
		t.Fatal("expected loopback address to be blocked")
	}
	if !strings.Contains(err.Error(), "blocked address") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateURLWithOptionsAllowsDevLocalHTTP(t *testing.T) {
	opts := ValidateOptions{AllowDevLocalHosts: true}
	for _, raw := range []string{
		"http://127.0.0.1:11434/v1",
		"http://localhost:11434/v1",
		"https://host.docker.internal:11434/v1",
	} {
		if err := ValidateURLWithOptions(raw, opts); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
	}
	if err := ValidateURLWithOptions("http://10.0.0.5:11434/v1", opts); err == nil {
		t.Fatal("expected non-dev-local private HTTP to remain rejected")
	}
	if err := ValidateURLWithOptions("http://example.com/v1", opts); err == nil {
		t.Fatal("expected public HTTP to remain rejected")
	}
	if err := ValidateURLWithOptions("http://metadata/v1", opts); err == nil {
		t.Fatal("expected metadata host to remain rejected")
	}
}

func TestSafeDialAllowsDevLocalLoopback(t *testing.T) {
	// Dial may fail with connection refused — that still means SSRF allowed the attempt.
	_, err := safeDialContext(context.Background(), "tcp", "127.0.0.1:1", true)
	if err != nil && strings.Contains(err.Error(), "blocked address") {
		t.Fatalf("dev-local dial should not SSRF-block loopback: %v", err)
	}
}

func TestNewSafeClientDisablesEnvironmentProxy(t *testing.T) {
	client := NewSafeClient(DefaultTimeout, nil)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("safe client must not allow a proxy to bypass guarded dialing")
	}
}
