package logging

import (
	"context"
	"log/slog"
	"strings"
)

// RedactHandler wraps h and drops slog attributes whose keys look like secrets
// (authorization, token, ciphertext, cookie, and common variants). Intended for
// the OTEL logs exporter so stdout JSON is unchanged.
func RedactHandler(h slog.Handler) slog.Handler {
	if h == nil {
		return nil
	}
	return &redactHandler{inner: h}
}

type redactHandler struct {
	inner slog.Handler
}

func (h *redactHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h *redactHandler) Handle(ctx context.Context, r slog.Record) error {
	nr := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		if fa, ok := filterAttr(a); ok {
			nr.AddAttrs(fa)
		}
		return true
	})
	return h.inner.Handle(ctx, nr)
}

func (h *redactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &redactHandler{inner: h.inner.WithAttrs(filterAttrs(attrs))}
}

func (h *redactHandler) WithGroup(name string) slog.Handler {
	return &redactHandler{inner: h.inner.WithGroup(name)}
}

func filterAttrs(attrs []slog.Attr) []slog.Attr {
	out := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		if fa, ok := filterAttr(a); ok {
			out = append(out, fa)
		}
	}
	return out
}

func filterAttr(a slog.Attr) (slog.Attr, bool) {
	a.Value = a.Value.Resolve()
	if a.Value.Kind() == slog.KindGroup {
		children := filterAttrs(a.Value.Group())
		if len(children) == 0 {
			return slog.Attr{}, false
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(children...)}, true
	}
	if SensitiveAttrKey(a.Key) {
		return slog.Attr{}, false
	}
	return a, true
}

// SensitiveAttrKey reports whether a slog attribute key must not be exported
// on the OTEL logs path.
func SensitiveAttrKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	k = strings.ReplaceAll(k, "-", "_")
	switch k {
	case "authorization", "proxy_authorization", "token", "access_token",
		"refresh_token", "id_token", "bearer", "ciphertext", "cipher_text",
		"cookie", "cookies", "set_cookie", "secret", "password", "passwd",
		"api_key", "apikey", "private_key":
		return true
	}
	if strings.Contains(k, "authorization") || strings.Contains(k, "ciphertext") || strings.Contains(k, "cookie") {
		return true
	}
	if strings.HasSuffix(k, "_token") || strings.HasSuffix(k, "_secret") || strings.HasSuffix(k, "_password") || strings.HasSuffix(k, "_ciphertext") {
		return true
	}
	return false
}
