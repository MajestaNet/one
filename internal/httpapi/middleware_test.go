package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/config"
)

func TestClientKeyIgnoresForwardingHeadersFromUntrustedPeer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/client/v1/me", nil)
	req.RemoteAddr = "203.0.113.9:43210"
	req.Header.Set("X-Forwarded-For", "198.51.100.1")
	req.Header.Set("X-Real-IP", "198.51.100.2")

	if got := clientKey(req); got != "203.0.113.9" {
		t.Fatalf("clientKey=%q, want direct peer", got)
	}
}

func TestClientKeyUsesValidatedForwardingHeaderFromPrivateProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/client/v1/me", nil)
	req.RemoteAddr = "10.0.0.8:43210"
	req.Header.Set("X-Forwarded-For", "192.0.2.10, 198.51.100.7")

	if got := clientKey(req); got != "198.51.100.7" {
		t.Fatalf("clientKey=%q, want rightmost forwarded client", got)
	}

	req.Header.Set("X-Forwarded-For", "not-an-ip")
	req.Header.Set("X-Real-IP", "also-not-an-ip")
	if got := clientKey(req); got != "10.0.0.8" {
		t.Fatalf("clientKey=%q, want proxy peer fallback", got)
	}
}

func TestRateLimiterPrunesExpiredClientBuckets(t *testing.T) {
	rl := newRateLimiter(10)
	rl.hits["expired"] = []time.Time{time.Now().Add(-2 * time.Minute)}
	rl.lastCleanup = time.Now().Add(-2 * time.Minute)

	if !rl.allow("active") {
		t.Fatal("first request should be allowed")
	}
	if _, ok := rl.hits["expired"]; ok {
		t.Fatal("expired client bucket was not pruned")
	}
}

func TestClassifyAdmissionPath(t *testing.T) {
	cases := map[string]string{
		"/healthz":                    "",
		"/version":                    "",
		"/client/v1/sobjects/Account": laneClient,
		"/client/v1/r12/me":           laneClient,
		"/v1/sobjects/Account":        laneClient,
		"/v1/objects":                 laneMetadata,
		"/v1/sharing/settings":        laneMetadata,
		"/v1/agents/playbooks":        laneMetadata,
		"/v1/agents/runs":             laneClient,
		"/metadata/v1/objects":        laneMetadata,
		"/deploy/v1/promotions":       laneDeploy,
		"/ops/v1/roll":                laneOps,
		"/auth/v1/token":              laneAuth,
		"/mcp":                        laneDeploy,
		"/scim/v2/Users":              laneClient,
	}
	for path, want := range cases {
		if got := classifyAdmissionPath(path); got != want {
			t.Errorf("classifyAdmissionPath(%q)=%q want %q", path, got, want)
		}
	}
}

func TestMCPToolLane(t *testing.T) {
	if got := mcpToolLane([]byte(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"query"}}`)); got != laneClient {
		t.Fatalf("query tool lane=%q", got)
	}
	if got := mcpToolLane([]byte(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"org_validate"}}`)); got != laneDeploy {
		t.Fatalf("org_validate lane=%q", got)
	}
	if got := mcpToolLane([]byte(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"upsert_object"}}`)); got != laneMetadata {
		t.Fatalf("upsert_object lane=%q", got)
	}
	if got := mcpToolLane([]byte(`not-json`)); got != laneDeploy {
		t.Fatalf("parse fail should be deploy, got %q", got)
	}
}

func TestHandlerAdmissionLanesIsolateClientFromDeploy(t *testing.T) {
	s := New(Options{Config: &config.Config{
		RequestBodyLimit:        1 << 20,
		RateLimitPerMinute:      10,
		AdmissionClientRPMShare: 0.7,
		APIRevisionCurrent:      1,
		APIRevisionMin:          1,
	}})
	h := s.Handler()
	// 10 RPM, 0.7 share → client=7 remainder=3. Saturate remainder via Deploy.
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/deploy/v1/environment", nil))
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("deploy request %d unexpectedly rate-limited: %s", i, rec.Body.String())
		}
	}
	blocked := httptest.NewRecorder()
	h.ServeHTTP(blocked, httptest.NewRequest(http.MethodGet, "/deploy/v1/environment", nil))
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("expected deploy 429, got %d %s", blocked.Code, blocked.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(blocked.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "RATE_LIMITED" {
		t.Fatalf("error=%v", body["error"])
	}
	details, _ := body["details"].(map[string]any)
	if details["lane"] != laneDeploy {
		t.Fatalf("details.lane=%v", details["lane"])
	}

	client := httptest.NewRecorder()
	h.ServeHTTP(client, httptest.NewRequest(http.MethodGet, "/client/v1/me", nil))
	if client.Code == http.StatusTooManyRequests {
		t.Fatalf("client lane should still allow, got 429 %s", client.Body.String())
	}

	auth := httptest.NewRecorder()
	h.ServeHTTP(auth, httptest.NewRequest(http.MethodGet, "/auth/v1/login", nil))
	if auth.Code == http.StatusTooManyRequests {
		t.Fatalf("auth lane must not share remainder limiter, got 429")
	}
}

func TestHandlerSetsBaselineSecurityHeaders(t *testing.T) {
	s := New(Options{Config: &config.Config{RequestBodyLimit: 1 << 20}})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
		"X-Frame-Options":        "DENY",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s=%q, want %q", header, got, want)
		}
	}
}
