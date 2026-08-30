package logging

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

type captureHandler struct {
	enabled bool
	records []slog.Record
	attrs   []slog.Attr
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return h.enabled }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	cp := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		cp.AddAttrs(a)
		return true
	})
	h.records = append(h.records, cp)
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := &captureHandler{enabled: h.enabled, records: h.records}
	next.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return next
}

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

func recordKeys(r slog.Record) map[string]bool {
	keys := map[string]bool{}
	r.Attrs(func(a slog.Attr) bool {
		collectKeys(a, keys)
		return true
	})
	return keys
}

func collectKeys(a slog.Attr, keys map[string]bool) {
	if a.Value.Kind() == slog.KindGroup {
		for _, c := range a.Value.Group() {
			collectKeys(c, keys)
		}
		return
	}
	keys[a.Key] = true
}

func TestRedactHandlerDropsSecretKeys(t *testing.T) {
	cap := &captureHandler{enabled: true}
	h := RedactHandler(cap)
	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "login", 0)
	rec.Add("user", "alice")
	rec.Add("authorization", "Bearer secret")
	rec.Add("Authorization", "also")
	rec.Add("access_token", "tok")
	rec.Add("ciphertext", "enc:v1:x")
	rec.Add("cookie", "sid=1")
	rec.Add("set-cookie", "sid=1")
	rec.Add("refresh_token", "r")
	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	if len(cap.records) != 1 {
		t.Fatalf("records=%d", len(cap.records))
	}
	keys := recordKeys(cap.records[0])
	if !keys["user"] {
		t.Fatal("expected user to be kept")
	}
	for _, drop := range []string{"authorization", "Authorization", "access_token", "ciphertext", "cookie", "set-cookie", "refresh_token"} {
		if keys[drop] {
			t.Fatalf("expected %q to be dropped, keys=%v", drop, keys)
		}
	}
}

func TestRedactHandlerDropsGroupedSecrets(t *testing.T) {
	cap := &captureHandler{enabled: true}
	h := RedactHandler(cap)
	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "outbound", 0)
	rec.AddAttrs(slog.Group("headers",
		slog.String("authorization", "Bearer x"),
		slog.String("content-type", "application/json"),
	))
	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	keys := recordKeys(cap.records[0])
	if !keys["content-type"] {
		t.Fatalf("expected content-type kept, keys=%v", keys)
	}
	if keys["authorization"] {
		t.Fatal("grouped authorization should be dropped")
	}
}

func TestRedactHandlerWithAttrs(t *testing.T) {
	cap := &captureHandler{enabled: true}
	h := RedactHandler(cap).WithAttrs([]slog.Attr{
		slog.String("token", "secret"),
		slog.String("job", "automation.run"),
	})
	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "run", 0)
	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	inner := h.(*redactHandler).inner.(*captureHandler)
	seen := map[string]bool{}
	for _, a := range inner.attrs {
		seen[a.Key] = true
	}
	if seen["token"] {
		t.Fatal("WithAttrs should drop token")
	}
	if !seen["job"] {
		t.Fatal("WithAttrs should keep job")
	}
}

func TestSensitiveAttrKey(t *testing.T) {
	drop := []string{"authorization", "Authorization", "id_token", "COOKIE", "cipher_text", "api_key", "client_secret"}
	keep := []string{"user", "trace_id", "path", "job_type", "tokenizer"}
	for _, k := range drop {
		if !SensitiveAttrKey(k) {
			t.Fatalf("expected drop %q", k)
		}
	}
	for _, k := range keep {
		if SensitiveAttrKey(k) {
			t.Fatalf("expected keep %q", k)
		}
	}
}

func TestFanOutWritesToAll(t *testing.T) {
	a := &captureHandler{enabled: true}
	b := &captureHandler{enabled: true}
	h := FanOut(a, b)
	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "hi", 0)
	rec.Add("k", "v")
	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	if len(a.records) != 1 || len(b.records) != 1 {
		t.Fatalf("a=%d b=%d", len(a.records), len(b.records))
	}
	if a.records[0].Message != "hi" || b.records[0].Message != "hi" {
		t.Fatal("expected both handlers to receive the message")
	}
}
