package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/config"
	"github.com/MajestaNet/ide/internal/httpapi"
)

type memSysStore struct {
	byPS map[string][]string
}

func (m *memSysStore) ListSystemPermissions(_ context.Context, ids []string) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	for _, id := range ids {
		for _, c := range m.byPS[id] {
			if _, ok := seen[c]; ok {
				continue
			}
			seen[c] = struct{}{}
			out = append(out, c)
		}
	}
	return out, nil
}

func TestMetadataCustomizeRequiresCapability(t *testing.T) {
	entries, err := authz.ParseAPIKeyEntries("builder:metadata+client,admin+admin")
	if err != nil {
		t.Fatal(err)
	}
	store := &memSysStore{byPS: map[string][]string{}}
	sys := &authz.SystemAuthz{Store: store}
	cfg := &config.Config{
		DefaultOwnerID: "00000000-0000-4000-8000-000000000001",
		APIKeyEntries:  entries,
	}
	srv := httpapi.New(httpapi.Options{
		Config:   cfg,
		Resolver: &authz.Resolver{Entries: entries, DefaultOwnerID: cfg.DefaultOwnerID},
		SystemAz: sys,
	})

	// builder key has metadata scope but no capability / no Users → admin false → CAPABILITY_REQUIRED
	req := httptest.NewRequest(http.MethodPost, "/metadata/v1/objects", nil)
	req.Header.Set("Authorization", "Bearer builder")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}

	// admin+admin implies all capabilities
	req2 := httptest.NewRequest(http.MethodPost, "/metadata/v1/objects", nil)
	req2.Header.Set("Authorization", "Bearer admin")
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req2)
	// Admin passes capability; may fail later for missing pool (503) or validation — not 403 CAPABILITY
	if rec2.Code == http.StatusForbidden && rec2.Body.String() != "" {
		body := rec2.Body.String()
		if contains(body, "CAPABILITY_REQUIRED") {
			t.Fatalf("admin should not get CAPABILITY_REQUIRED: %s", body)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}
