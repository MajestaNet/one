// Package otel wires optional OpenTelemetry OTLP export for Majesta One API/worker.
// When OTEL_EXPORTER_OTLP_ENDPOINT is unset, Setup is a no-op and Tracer is a noop tracer.
package otel

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Options configures OTLP export for one process (api or worker).
type Options struct {
	Endpoint       string
	ServiceName    string
	ServiceVersion string
	CustomerID     string
	InstallID      string
	InstallRole    string
	// TracesExporter: "otlp" (default when endpoint set) or "none".
	TracesExporter string
	// MetricsExporter: "otlp" (default when endpoint set) or "none".
	MetricsExporter string
	// LogsExporter: "none" (default even when endpoint set) or "otlp".
	LogsExporter string
}

// Provider holds shutdown hooks for the process tracer/meter/log providers.
type Provider struct {
	tp      *sdktrace.TracerProvider
	mp      *sdkmetric.MeterProvider
	lp      *sdklog.LoggerProvider
	prevLog *slog.Logger
	active  bool
}

var globalActive bool

// Active reports whether OTLP export was configured for this process.
func Active() bool { return globalActive }

// Setup installs global TracerProvider / MeterProvider, and an optional
// LoggerProvider when LogsExporter is "otlp". Safe when Endpoint is empty (noop).
func Setup(ctx context.Context, opt Options) (*Provider, error) {
	endpoint := strings.TrimSpace(opt.Endpoint)
	if endpoint == "" {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return &Provider{}, nil
	}

	serviceName := strings.TrimSpace(opt.ServiceName)
	if serviceName == "" {
		serviceName = "one"
	}
	traces := strings.ToLower(strings.TrimSpace(opt.TracesExporter))
	if traces == "" {
		traces = "otlp"
	}
	metricsExp := strings.ToLower(strings.TrimSpace(opt.MetricsExporter))
	if metricsExp == "" {
		metricsExp = "otlp"
	}
	logsExp := strings.ToLower(strings.TrimSpace(opt.LogsExporter))
	if logsExp == "" {
		logsExp = "none"
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(opt.ServiceVersion),
			attribute.String("one.customer_id", opt.CustomerID),
			attribute.String("one.install_id", opt.InstallID),
			attribute.String("one.install_role", opt.InstallRole),
		),
	)
	if err != nil {
		return nil, err
	}

	p := &Provider{active: true}
	globalActive = true

	endpointURL, insecure := normalizeOTLPEndpoint(endpoint)

	if traces != "none" {
		opts := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(endpointURL)}
		if insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		exp, err := otlptracehttp.New(ctx, opts...)
		if err != nil {
			return nil, err
		}
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(res),
		)
		otel.SetTracerProvider(tp)
		p.tp = tp
	} else {
		otel.SetTracerProvider(noop.NewTracerProvider())
	}

	if metricsExp != "none" {
		mopts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpointURL(endpointURL)}
		if insecure {
			mopts = append(mopts, otlpmetrichttp.WithInsecure())
		}
		mexp, err := otlpmetrichttp.New(ctx, mopts...)
		if err != nil {
			_ = p.Shutdown(ctx)
			return nil, err
		}
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(mexp, sdkmetric.WithInterval(30*time.Second))),
			sdkmetric.WithResource(res),
		)
		otel.SetMeterProvider(mp)
		p.mp = mp
	}

	if logsExp == "otlp" {
		if err := startLogs(ctx, p, res, serviceName, endpointURL, insecure); err != nil {
			_ = p.Shutdown(ctx)
			return nil, err
		}
	}

	slog.Info("otel configured", "endpoint", redactEndpoint(endpoint), "service", serviceName, "logs", logsExp)
	return p, nil
}

// Shutdown flushes and closes providers.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	var first error
	if p.tp != nil {
		if err := p.tp.Shutdown(ctx); err != nil && first == nil {
			first = err
		}
	}
	if p.mp != nil {
		if err := p.mp.Shutdown(ctx); err != nil && first == nil {
			first = err
		}
	}
	if p.lp != nil {
		if err := p.lp.Shutdown(ctx); err != nil && first == nil {
			first = err
		}
	}
	if p.prevLog != nil {
		slog.SetDefault(p.prevLog)
		p.prevLog = nil
	}
	return first
}

// Tracer returns a named tracer from the global provider.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// Meter returns a named meter from the global provider.
func Meter(name string) metric.Meter {
	return otel.Meter(name)
}

// SpanFromContext returns the current span.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// TraceAttrs returns slog attrs for correlation (empty when no span).
func TraceAttrs(ctx context.Context) []any {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return nil
	}
	return []any{
		"trace_id", sc.TraceID().String(),
		"span_id", sc.SpanID().String(),
	}
}

// RedactURL strips query/userinfo for safe span attributes.
func RedactURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func normalizeOTLPEndpoint(raw string) (endpointURL string, insecure bool) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw, strings.HasPrefix(raw, "http://")
	}
	insecure = u.Scheme == "http"
	// otlptracehttp.WithEndpointURL expects full URL including path if any.
	if u.Path == "" || u.Path == "/" {
		// Default OTLP/HTTP paths are appended by the exporter.
		u.Path = ""
	}
	return u.String(), insecure
}

func redactEndpoint(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "(set)"
	}
	u.User = nil
	u.RawQuery = ""
	return u.Host
}
