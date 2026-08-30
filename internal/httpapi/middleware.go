package httpapi

import (
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/MajestaNet/ide/internal/config"
	oneotel "github.com/MajestaNet/ide/internal/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

type rateLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	hits        map[string][]time.Time
	lastCleanup time.Time
}

func newRateLimiter(perMinute int) *rateLimiter {
	if perMinute <= 0 {
		return nil
	}
	return &rateLimiter{
		limit:  perMinute,
		window: time.Minute,
		hits:   map[string][]time.Time{},
	}
}

func (rl *rateLimiter) allow(key string) bool {
	if rl == nil {
		return true
	}
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := now.Add(-rl.window)
	if rl.lastCleanup.IsZero() || now.Sub(rl.lastCleanup) >= rl.window {
		for client, timestamps := range rl.hits {
			if len(timestamps) == 0 || !timestamps[len(timestamps)-1].After(cutoff) {
				delete(rl.hits, client)
			}
		}
		rl.lastCleanup = now
	}
	arr := rl.hits[key]
	kept := arr[:0]
	for _, t := range arr {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if rl.limit <= 0 || len(kept) >= rl.limit {
		rl.hits[key] = kept
		return false
	}
	rl.hits[key] = append(kept, now)
	return true
}

// clientKey uses forwarding headers only when the direct peer is a loopback or
// private-network reverse proxy. Directly exposed installs must not let callers
// select a fresh rate-limit bucket by spoofing X-Forwarded-For or X-Real-IP.
func clientKey(r *http.Request) string {
	peer := remoteIP(r.RemoteAddr)
	if peer != nil && (peer.IsLoopback() || peer.IsPrivate()) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := net.ParseIP(strings.TrimSpace(parts[len(parts)-1])); ip != nil {
				return ip.String()
			}
		}
		if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
			return ip.String()
		}
	}
	if peer != nil {
		return peer.String()
	}
	return r.RemoteAddr
}

func remoteIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return net.ParseIP(strings.TrimSpace(host))
}

func bootProbePath(path string) bool {
	switch path {
	case "/healthz", "/readyz", "/version":
		return true
	default:
		return false
	}
}

// Handler returns the root handler with request-id, body limit, rate limit, and access logging.
func (s *Server) Handler() http.Handler {
	bodyLimit := s.cfg.RequestBodyLimit
	if bodyLimit <= 0 {
		bodyLimit = 1 << 20
	}
	lanes := newAdmissionLimiters(s.cfg)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := r.Header.Get("x-request-id")
		if reqID == "" || len(reqID) > 128 {
			reqID = randomRequestID()
		}
		w.Header().Set("x-request-id", reqID)
		w.Header().Set("content-type", "application/json")
		w.Header().Set("x-content-type-options", "nosniff")
		w.Header().Set("referrer-policy", "no-referrer")
		w.Header().Set("x-frame-options", "DENY")

		tr := oneotel.Tracer("one.httpapi")
		ctx, span := tr.Start(r.Context(), r.Method)
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("one.request_id", reqID),
		)
		r = r.WithContext(ctx)
		defer span.End()

		// Development CORS for Control IDE browser preview (`npm run dev:web` on loopback).
		// Electron loadFile does not need this; production stays closed by default.
		if corsOrigin := devCORSOrigin(r, s.cfg); corsOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", corsOrigin)
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-Id, X-One-Device-Id, One-API-Revision")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Vary", "Origin")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		if !s.ready.Load() && !bootProbePath(r.URL.Path) {
			writeErr(w, http.StatusServiceUnavailable, "STARTING", "API is still applying bootstrap/seed; retry shortly")
			span.SetStatus(codes.Error, "starting")
			return
		}

		var ok bool
		r, ok = s.applyAPIRevision(w, r)
		if !ok {
			span.SetStatus(codes.Error, "api revision rejected")
			return
		}

		if !bootProbePath(r.URL.Path) {
			lane := classifyAdmissionPath(r.URL.Path)
			if lane == laneDeploy && (r.URL.Path == "/mcp" || strings.HasPrefix(r.URL.Path, "/mcp/")) {
				lane, r = peekMCPLane(r)
			}
			if lane != laneAuth && lane != "" && !lanes.allow(lane, clientKey(r)) {
				writeErrDetails(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests", map[string]any{"lane": lane})
				span.SetStatus(codes.Error, "rate limited")
				return
			}
		}

		if r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodHead {
			r.Body = http.MaxBytesReader(w, r.Body, bodyLimit)
		}

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		s.mux.ServeHTTP(rec, r)

		routePattern := r.Pattern
		if routePattern == "" {
			routePattern = "unmatched"
		}
		span.SetName(r.Method + " " + routePattern)
		span.SetAttributes(
			attribute.String("http.route", routePattern),
			attribute.Int("http.status_code", rec.status),
		)
		if rec.status >= 500 {
			span.SetStatus(codes.Error, "server error")
		}

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"bytes", rec.bytes,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", reqID,
		}
		attrs = append(attrs, oneotel.TraceAttrs(ctx)...)
		slog.Info("request", attrs...)
	})
}

// devCORSOrigin returns an allowed Origin for non-production loopback IDE previews.
func devCORSOrigin(r *http.Request, cfg *config.Config) string {
	if cfg == nil || cfg.IsProduction {
		return ""
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return ""
	}
	u, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return origin
	default:
		return ""
	}
}
