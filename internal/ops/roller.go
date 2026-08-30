package ops

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MemoryRoller records intended rolls without calling AWS (Compose / unit tests).
type MemoryRoller struct {
	LastRoll   *RollRequest
	RolledBack bool
	FailRoll   bool
}

func (*MemoryRoller) Mode() string { return "local" }

func (m *MemoryRoller) CaptureCurrent(context.Context) (string, string, error) {
	return "local:api:previous", "local:worker:previous", nil
}

func (m *MemoryRoller) Roll(_ context.Context, req RollRequest) (string, string, error) {
	if m.FailRoll {
		return "", "", fmt.Errorf("simulated roll failure")
	}
	cp := req
	m.LastRoll = &cp
	m.RolledBack = false
	return "local:api:" + req.ProductVersion, "local:worker:" + req.ProductVersion, nil
}

func (m *MemoryRoller) Rollback(context.Context, string, string) error {
	m.RolledBack = true
	return nil
}

// LocalRoller is a package-level convenience alias for NewEngine default.
func LocalRoller() Roller { return &MemoryRoller{} }

// HTTPHealthChecker GETs /healthz and /readyz on BaseURL.
type HTTPHealthChecker struct {
	BaseURL    string
	HTTPClient *http.Client
}

func (h HTTPHealthChecker) Check(ctx context.Context) error {
	base := strings.TrimRight(h.BaseURL, "/")
	if base == "" {
		return nil
	}
	client := h.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	for _, path := range []string{"/healthz", "/readyz"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("%s returned %d", path, resp.StatusCode)
		}
	}
	return nil
}

// NopHealth always succeeds (unit tests).
type NopHealth struct{}

func (NopHealth) Check(context.Context) error { return nil }
