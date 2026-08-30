package otel

import (
	"context"
	"log/slog"

	"github.com/MajestaNet/ide/internal/logging"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
)

func startLogs(ctx context.Context, p *Provider, res *resource.Resource, serviceName, endpointURL string, insecure bool) error {
	opts := []otlploghttp.Option{otlploghttp.WithEndpointURL(endpointURL)}
	if insecure {
		opts = append(opts, otlploghttp.WithInsecure())
	}
	exp, err := otlploghttp.New(ctx, opts...)
	if err != nil {
		return err
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exp)),
		sdklog.WithResource(res),
	)
	p.lp = lp

	stdout := slog.Default().Handler()
	otelH := logging.RedactHandler(&levelAlignHandler{
		src:   stdout,
		inner: otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(lp)),
	})
	p.prevLog = slog.Default()
	slog.SetDefault(slog.New(logging.FanOut(stdout, otelH)))
	return nil
}

// levelAlignHandler applies the stdout handler's Enabled check so OTEL logs
// honor LOG_LEVEL instead of exporting every debug record.
type levelAlignHandler struct {
	src   slog.Handler
	inner slog.Handler
}

func (h *levelAlignHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.src.Enabled(ctx, l)
}

func (h *levelAlignHandler) Handle(ctx context.Context, r slog.Record) error {
	if !h.src.Enabled(ctx, r.Level) {
		return nil
	}
	return h.inner.Handle(ctx, r)
}

func (h *levelAlignHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &levelAlignHandler{src: h.src, inner: h.inner.WithAttrs(attrs)}
}

func (h *levelAlignHandler) WithGroup(name string) slog.Handler {
	return &levelAlignHandler{src: h.src, inner: h.inner.WithGroup(name)}
}
