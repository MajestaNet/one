package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MajestaNet/ide/internal/config"
)

func TestDevCORSOrigin(t *testing.T) {
	dev := &config.Config{IsProduction: false}
	prod := &config.Config{IsProduction: true}

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/client/v1/me", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	if got := devCORSOrigin(req, dev); got != "http://127.0.0.1:5173" {
		t.Fatalf("dev loopback origin: got %q", got)
	}
	if got := devCORSOrigin(req, prod); got != "" {
		t.Fatalf("production must not reflect CORS origin, got %q", got)
	}

	req.Header.Set("Origin", "https://evil.example")
	if got := devCORSOrigin(req, dev); got != "" {
		t.Fatalf("non-loopback origin must be denied, got %q", got)
	}
}

func TestHandlerDevCORSPreflight(t *testing.T) {
	s := New(Options{Config: &config.Config{
		AppEnv:                      "development",
		IsProduction:                false,
		RequestBodyLimit:            1 << 20,
		RateLimitPerMinute:          600,
		AuthTokenRateLimitPerMinute: 30,
	}})
	req := httptest.NewRequest(http.MethodOptions, "/client/v1/me", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "authorization")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Allow-Origin=%q", got)
	}
}
