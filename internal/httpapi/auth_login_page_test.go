package httpapi_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/authlogin"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/identity"
	"github.com/MajestaNet/ide/internal/testutil"
)

func forceLoginPageClaimed(t *testing.T, d *testutil.Database) {
	t.Helper()
	ctx := t.Context()
	_, _ = d.Pool.Exec(ctx, `UPDATE organization_settings SET social_providers = '{}'::text[] WHERE id = true`)
	if err := db.NewInstallAuthStore(d.Pool).MarkClaimed(ctx); err != nil && !errors.Is(err, db.ErrConflict) {
		t.Fatalf("MarkClaimed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(ctx, `
			UPDATE organization_settings
			SET claimed_at = NULL, claim_token_hash = NULL, password_login_enabled = true, social_providers = '{}'::text[]
			WHERE id = true`)
	})
}

func TestAuthLoginPageSlackButtonWhenEnabled(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	forceLoginPageClaimed(t, d)

	fake := &authlogin.FakeProvider{ProviderName: identity.ProviderSlack}
	broker := &authlogin.Broker{Providers: map[string]authlogin.Provider{
		identity.ProviderSlack: fake,
	}}
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "dev-admin-key+admin", EnableJWT: true, LoginBroker: broker,
	})
	ts.Config.AuthLoginProviders = []string{"slack"}

	q := url.Values{
		"client_id":             {"one.controlIde"},
		"redirect_uri":          {"one-control://oauth/callback"},
		"code_challenge":        {"challenge"},
		"code_challenge_method": {"S256"},
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/v1/login?"+q.Encode(), nil)
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Continue with Slack") || !strings.Contains(body, "provider=slack") {
		t.Fatalf("expected Slack button: %s", body)
	}
}

func TestAuthLoginPageHidesSlackWhenNotEnabled(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	forceLoginPageClaimed(t, d)

	fake := &authlogin.FakeProvider{ProviderName: identity.ProviderSlack}
	broker := &authlogin.Broker{Providers: map[string]authlogin.Provider{
		identity.ProviderSlack: fake,
	}}
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "dev-admin-key+admin", EnableJWT: true, LoginBroker: broker,
	})
	ts.Config.AuthLoginProviders = []string{"dev"}

	q := url.Values{
		"client_id":             {"one.controlIde"},
		"redirect_uri":          {"one-control://oauth/callback"},
		"code_challenge":        {"challenge"},
		"code_challenge_method": {"S256"},
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/v1/login?"+q.Encode(), nil)
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "Continue with Slack") {
		t.Fatalf("Slack must be hidden when not enabled: %s", rr.Body.String())
	}
}

func TestAuthLoginPageUnclaimedHidesSlack(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	if _, err := db.SyncInstallClaimToken(t.Context(), d.Pool, "test-claim-token-slack", false); err != nil {
		t.Fatal(err)
	}
	resetInstallClaim(t, d.Pool)
	_, _ = d.Pool.Exec(t.Context(), `UPDATE organization_settings SET social_providers = '{}'::text[] WHERE id = true`)
	fake := &authlogin.FakeProvider{ProviderName: identity.ProviderSlack}
	broker := &authlogin.Broker{Providers: map[string]authlogin.Provider{
		identity.ProviderSlack: fake,
	}}
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "dev-admin-key+admin", EnableJWT: true, LoginBroker: broker,
	})
	ts.Config.AuthLoginProviders = []string{"slack"}
	rr := httptest.NewRecorder()
	ts.Handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/v1/login", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "Continue with Slack") {
		t.Fatalf("unclaimed install must not show Slack: %s", rr.Body.String())
	}
}
