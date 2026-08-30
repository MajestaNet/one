package logging

import (
	"context"
	"errors"
	"log/slog"
	"slices"
)

// FanOut returns a slog.Handler that writes to every non-nil handler.
// Enabled is true if any child is enabled.
func FanOut(handlers ...slog.Handler) slog.Handler {
	var hs []slog.Handler
	for _, h := range handlers {
		if h != nil {
			hs = append(hs, h)
		}
	}
	if len(hs) == 0 {
		return discardHandler{}
	}
	if len(hs) == 1 {
		return hs[0]
	}
	return &fanOutHandler{handlers: hs}
}

type fanOutHandler struct {
	handlers []slog.Handler
}

func (h *fanOutHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, c := range h.handlers {
		if c.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (h *fanOutHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for i, c := range h.handlers {
		if !c.Enabled(ctx, r.Level) {
			continue
		}
		rec := r.Clone()
		if i == len(h.handlers)-1 {
			rec = r
		}
		if err := c.Handle(ctx, rec); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (h *fanOutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(h.handlers))
	for i, c := range h.handlers {
		next[i] = c.WithAttrs(slices.Clone(attrs))
	}
	return &fanOutHandler{handlers: next}
}

func (h *fanOutHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	next := make([]slog.Handler, len(h.handlers))
	for i, c := range h.handlers {
		next[i] = c.WithGroup(name)
	}
	return &fanOutHandler{handlers: next}
}

type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (h discardHandler) WithAttrs([]slog.Attr) slog.Handler      { return h }
func (h discardHandler) WithGroup(string) slog.Handler           { return h }
