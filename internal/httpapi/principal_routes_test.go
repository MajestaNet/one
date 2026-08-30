package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/config"
	"github.com/MajestaNet/ide/internal/httpapi"
)

func TestPrincipalAdminRequiresIdentityManage(t *testing.T) {
	entries, err := authz.ParseAPIKeyEntries("clientonly:client,admin+admin")
	if err != nil {
		t.Fatal(err)
	}
	store := &memSysStore{byPS: map[string][]string{}}
	sys := &authz.SystemAuthz{Store: store}
	cfg := &config.Config{
		DefaultOwnerID:     "00000000-0000-4000-8000-000000000001",
		APIKeyEntries:      entries,
		RequestBodyLimit:   1 << 20,
		RateLimitPerMinute: 0,
	}
	srv := httpapi.New(httpapi.Options{
		Config:   cfg,
		Resolver: &authz.Resolver{Entries: entries, DefaultOwnerID: cfg.DefaultOwnerID},
		SystemAz: sys,
	})

	req := httptest.NewRequest(http.MethodPost, "/client/v1/principals", nil)
	req.Header.Set("Authorization", "Bearer clientonly")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}

	reqAssign := httptest.NewRequest(http.MethodPost, "/client/v1/roles/assign", nil)
	reqAssign.Header.Set("Authorization", "Bearer clientonly")
	recAssign := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recAssign, reqAssign)
	if recAssign.Code != http.StatusForbidden {
		t.Fatalf("assign expected 403, got %d body=%s", recAssign.Code, recAssign.Body.String())
	}
}

func TestExposureRequiresNetworkCapability(t *testing.T) {
	entries, err := authz.ParseAPIKeyEntries("net:metadata+client,admin+admin")
	if err != nil {
		t.Fatal(err)
	}
	store := &memSysStore{byPS: map[string][]string{
		"ps-net": {authz.CapGovernNetwork},
	}}
	// Without Users, actor has no PermissionSetIDs → still forbidden for non-admin
	sys := &authz.SystemAuthz{Store: store}
	cfg := &config.Config{
		DefaultOwnerID:     "00000000-0000-4000-8000-000000000001",
		APIKeyEntries:      entries,
		RequestBodyLimit:   1 << 20,
		RateLimitPerMinute: 0,
	}
	srv := httpapi.New(httpapi.Options{
		Config:   cfg,
		Resolver: &authz.Resolver{Entries: entries, DefaultOwnerID: cfg.DefaultOwnerID},
		SystemAz: sys,
	})
	req := httptest.NewRequest(http.MethodGet, "/metadata/v1/install/exposure", nil)
	req.Header.Set("Authorization", "Bearer net")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without PS, got %d", rec.Code)
	}

	// admin bypasses capability
	req2 := httptest.NewRequest(http.MethodGet, "/metadata/v1/install/exposure", nil)
	req2.Header.Set("Authorization", "Bearer admin")
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req2)
	if rec2.Code == http.StatusForbidden && contains(rec2.Body.String(), "CAPABILITY_REQUIRED") {
		t.Fatalf("admin should not get CAPABILITY_REQUIRED: %s", rec2.Body.String())
	}
}
