package otel

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/logging"
)

func TestSetupNoopWhenUnset(t *testing.T) {
	p, err := Setup(context.Background(), Options{ServiceName: "one-test"})
	if err != nil {
		t.Fatal(err)
	}
	if p.active {
		t.Fatal("expected inactive provider when endpoint unset")
	}
	tr := Tracer("test")
	ctx, span := tr.Start(context.Background(), "noop")
	span.End()
	_ = ctx
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRedactURL(t *testing.T) {
	got := RedactURL("https://user:pass@api.example.com/v1/x?token=secret#frag")
	want := "https://api.example.com/v1/x"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if RedactURL("not a url") != "" {
		t.Fatal("expected empty for invalid")
	}
}

func TestTraceAttrsEmptyWithoutSpan(t *testing.T) {
	if attrs := TraceAttrs(context.Background()); len(attrs) != 0 {
		t.Fatalf("expected empty attrs, got %#v", attrs)
	}
}

func TestShutdownNilAndActiveNoop(t *testing.T) {
	var p *Provider
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := Setup(context.Background(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got.active {
		t.Fatal("noop setup must not mark the provider active")
	}
	_ = Meter("test")
	_ = SpanFromContext(context.Background())
}

func TestRedactURLStripsUserinfoOnlyWhenParseable(t *testing.T) {
	if RedactURL("") != "" {
		t.Fatal("empty")
	}
	got := RedactURL("http://example.com/path")
	if got != "http://example.com/path" {
		t.Fatalf("got %q", got)
	}
}

func TestLogsExporterNotStartedWhenNone(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	for _, logs := range []string{"", "none", "NONE"} {
		p, err := Setup(context.Background(), Options{
			Endpoint:        "http://127.0.0.1:4318",
			ServiceName:     "one-test",
			TracesExporter:  "none",
			MetricsExporter: "none",
			LogsExporter:    logs,
		})
		if err != nil {
			t.Fatalf("logs=%q: %v", logs, err)
		}
		if p.lp != nil {
			t.Fatalf("logs=%q: expected no log provider", logs)
		}
		if err := p.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLogsExporterStartsWhenOTLP(t *testing.T) {
	logging.Setup("info")
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	p, err := Setup(context.Background(), Options{
		Endpoint:        srv.URL,
		ServiceName:     "one-test",
		TracesExporter:  "none",
		MetricsExporter: "none",
		LogsExporter:    "otlp",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.lp == nil {
		t.Fatal("expected log provider when OTEL_LOGS_EXPORTER=otlp")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestLogsExporterIgnoredWhenEndpointUnset(t *testing.T) {
	p, err := Setup(context.Background(), Options{
		ServiceName:  "one-test",
		LogsExporter: "otlp",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.active || p.lp != nil {
		t.Fatal("endpoint unset must remain a no-op even when logs exporter is otlp")
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}
