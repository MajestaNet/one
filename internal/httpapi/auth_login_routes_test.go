package httpapi_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authlogin"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/httpapi"
	"github.com/MajestaNet/ide/internal/identity"
	"github.com/MajestaNet/ide/internal/integration"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestSocialLoginPKCEHappyPath(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	cleanupSocialHumans(t, d)

	fake := &authlogin.FakeProvider{
		ProviderName: identity.ProviderGoogle,
		Claims: authlogin.SubjectClaims{
			Provider:      identity.ProviderGoogle,
			Issuer:        authlogin.IssuerGoogle,
			Subject:       "google-sub-happy",
			Email:         "happy@example.com",
			EmailVerified: true,
			Name:          "Happy User",
		},
	}
	broker := &authlogin.Broker{Providers: map[string]authlogin.Provider{
		identity.ProviderGoogle: fake,
	}}

	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:     "dev-admin-key+admin",
		EnableJWT:   true,
		LoginBroker: broker,
	})
	ts.Config.AuthLoginProviders = []string{"google"}
	ts.Config.AuthAutoProvisionUsers = true
	ts.Config.PlatformPublicURL = "http://one.test"

	integSvc := &integration.Service{Pool: d.Pool, Identity: identity.NopBackend{}, EncryptionKey: ts.Config.AuthJWTSigningKey}
	if err := integSvc.EnsureControlIDE(t.Context()); err != nil {
		t.Fatalf("EnsureControlIDE: %v", err)
	}

	verifier, err := authlogin.RandomURLToken(32)
	if err != nil {
		t.Fatal(err)
	}
	challenge := authlogin.PKCEChallengeS256(verifier)
	clientState := "client-state-1"

	authorizeURL := "/auth/v1/authorize?" + url.Values{
		"provider":              {"google"},
		"client_id":             {integration.APINameControlIDE},
		"redirect_uri":          {integration.DefaultControlIDERedirectURI},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {clientState},
	}.Encode()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, authorizeURL, nil)
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("authorize status %d body %s", rr.Code, rr.Body.String())
	}
	loc := rr.Header().Get("Location")
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	serverState := u.Query().Get("state")
	if serverState == "" {
		t.Fatalf("missing IdP state in %s", loc)
	}

	cb := "/auth/v1/callback/google?" + url.Values{
		"code":  {"idp-code"},
		"state": {serverState},
	}.Encode()
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, cb, nil)
	ts.Handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusFound {
		t.Fatalf("callback status %d body %s", rr2.Code, rr2.Body.String())
	}
	redir, err := url.Parse(rr2.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if redir.Query().Get("state") != clientState {
		t.Fatalf("client state mismatch: %s", redir.RawQuery)
	}
	code := redir.Query().Get("code")
	if code == "" {
		t.Fatal("missing one auth code")
	}

	body := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {integration.APINameControlIDE},
		"code":          {code},
		"redirect_uri":  {integration.DefaultControlIDERedirectURI},
		"code_verifier": {verifier},
		"scope":         {authz.ScopeOfflineAccess},
	}.Encode()
	rr3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/auth/v1/token", strings.NewReader(body))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ts.Handler.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusOK {
		t.Fatalf("token status %d body %s", rr3.Code, rr3.Body.String())
	}
	raw, _ := io.ReadAll(rr3.Body)
	if !strings.Contains(string(raw), "access_token") {
		t.Fatalf("token body: %s", raw)
	}
	if !strings.Contains(string(raw), "refresh_token") {
		t.Fatalf("PKCE token missing refresh_token: %s", raw)
	}

	rr4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodPost, "/auth/v1/token", strings.NewReader(body))
	req4.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ts.Handler.ServeHTTP(rr4, req4)
	if rr4.Code == http.StatusOK {
		t.Fatal("expected replay rejection")
	}

	ctx := t.Context()
	links := db.NewIdentityLinkStore(d.Pool)
	link, err := links.GetBySubject(ctx, identity.ProviderGoogle, authlogin.IssuerGoogle, "google-sub-happy")
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	urow, err := db.NewUserStore(d.Pool).GetByID(ctx, link.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if urow.Email != "happy@example.com" {
		t.Fatalf("expected email stored, got %q", urow.Email)
	}
	users := db.NewUserStore(d.Pool)
	scopes, isAdmin, roles, err := users.ListRoleGrants(ctx, link.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if !isAdmin {
		t.Fatal("first human should be admin after System Admin promotion")
	}
	hasSystemAdmin := false
	for _, r := range roles {
		if r == "SystemAdmin" {
			hasSystemAdmin = true
			break
		}
	}
	if !hasSystemAdmin {
		t.Fatalf("expected SystemAdmin role, got %v", roles)
	}
	wantScopes := map[string]bool{"client": true, "metadata": true, "deploy": true, "ops": true}
	for _, sc := range scopes {
		delete(wantScopes, string(sc))
	}
	if len(wantScopes) > 0 {
		t.Fatalf("missing scopes for all IDE tiles: still need %v; got %v", wantScopes, scopes)
	}
	psNames, err := users.ListPermissionSetAPINames(ctx, link.UserID)
	if err != nil {
		t.Fatal(err)
	}
	hasAdminPS := false
	for _, n := range psNames {
		if n == db.SystemAdminPermissionSetAPIName {
			hasAdminPS = true
			break
		}
	}
	if !hasAdminPS {
		t.Fatalf("expected System Admin permission set (%s), got %v", db.SystemAdminPermissionSetAPIName, psNames)
	}
	var label string
	if err := d.Pool.QueryRow(ctx, `SELECT label FROM permission_sets WHERE api_name = $1`, db.SystemAdminPermissionSetAPIName).Scan(&label); err != nil {
		t.Fatal(err)
	}
	if label != db.SystemAdminPermissionSetLabel {
		t.Fatalf("expected label %q, got %q", db.SystemAdminPermissionSetLabel, label)
	}
	var loginAudit, provisionAudit int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE action = 'auth.login'`).Scan(&loginAudit); err != nil || loginAudit < 1 {
		t.Fatalf("auth.login audit count=%d err=%v", loginAudit, err)
	}
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE action = 'auth.provision'`).Scan(&provisionAudit); err != nil || provisionAudit < 1 {
		t.Fatalf("auth.provision audit count=%d err=%v", provisionAudit, err)
	}
}

func TestSocialLoginSecondHumanKeepsStandardUser(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	cleanupSocialHumans(t, d)

	users := db.NewUserStore(d.Pool)
	first, err := users.CreateSocialUser(t.Context(), "first@example.com", "First", "StandardUser")
	if err != nil {
		t.Fatal(err)
	}
	if err := users.EnsureInitialHumanSystemAdmin(t.Context(), first.ID); err != nil {
		t.Fatal(err)
	}

	fake := &authlogin.FakeProvider{
		ProviderName: identity.ProviderGoogle,
		Claims: authlogin.SubjectClaims{
			Provider:      identity.ProviderGoogle,
			Issuer:        authlogin.IssuerGoogle,
			Subject:       "google-sub-second",
			Email:         "second@example.com",
			EmailVerified: true,
			Name:          "Second User",
		},
	}
	broker := &authlogin.Broker{Providers: map[string]authlogin.Provider{
		identity.ProviderGoogle: fake,
	}}
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "dev-admin-key+admin", EnableJWT: true, LoginBroker: broker,
	})
	ts.Config.AuthLoginProviders = []string{"google"}
	ts.Config.AuthAutoProvisionUsers = true
	ts.Config.AuthAutoProvisionRole = "StandardUser"
	ts.Config.PlatformPublicURL = "http://one.test"

	integSvc := &integration.Service{Pool: d.Pool, Identity: identity.NopBackend{}, EncryptionKey: ts.Config.AuthJWTSigningKey}
	if err := integSvc.EnsureControlIDE(t.Context()); err != nil {
		t.Fatal(err)
	}

	verifier, _ := authlogin.RandomURLToken(32)
	challenge := authlogin.PKCEChallengeS256(verifier)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/v1/authorize?"+url.Values{
		"provider": {"google"}, "client_id": {integration.APINameControlIDE},
		"redirect_uri":          {integration.DefaultControlIDERedirectURI},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode(), nil)
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("authorize %d %s", rr.Code, rr.Body.String())
	}
	state, _ := url.Parse(rr.Header().Get("Location"))
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/auth/v1/callback/google?"+url.Values{
		"code": {"c"}, "state": {state.Query().Get("state")},
	}.Encode(), nil)
	ts.Handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusFound {
		t.Fatalf("callback %d %s", rr2.Code, rr2.Body.String())
	}

	ctx := t.Context()
	link, err := db.NewIdentityLinkStore(d.Pool).GetBySubject(ctx, identity.ProviderGoogle, authlogin.IssuerGoogle, "google-sub-second")
	if err != nil {
		t.Fatal(err)
	}
	_, isAdmin, roles, err := users.ListRoleGrants(ctx, link.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if isAdmin {
		t.Fatal("second human must not become install System Admin")
	}
	for _, r := range roles {
		if r == "SystemAdmin" {
			t.Fatalf("second human should not have SystemAdmin, roles=%v", roles)
		}
	}
}

func TestSocialLoginEmailRequired(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})

	fake := &authlogin.FakeProvider{
		ProviderName: identity.ProviderGoogle,
		Claims: authlogin.SubjectClaims{
			Provider: identity.ProviderGoogle,
			Issuer:   authlogin.IssuerGoogle,
			Subject:  "google-sub-no-email",
			Email:    "",
			Name:     "No Email User",
		},
	}
	broker := &authlogin.Broker{Providers: map[string]authlogin.Provider{
		identity.ProviderGoogle: fake,
	}}
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "dev-admin-key+admin", EnableJWT: true, LoginBroker: broker,
	})
	ts.Config.AuthLoginProviders = []string{"google"}
	ts.Config.AuthAutoProvisionUsers = true
	ts.Config.PlatformPublicURL = "http://one.test"

	integSvc := &integration.Service{Pool: d.Pool, Identity: identity.NopBackend{}, EncryptionKey: ts.Config.AuthJWTSigningKey}
	if err := integSvc.EnsureControlIDE(t.Context()); err != nil {
		t.Fatal(err)
	}

	verifier, _ := authlogin.RandomURLToken(32)
	challenge := authlogin.PKCEChallengeS256(verifier)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/v1/authorize?"+url.Values{
		"provider": {"google"}, "client_id": {integration.APINameControlIDE},
		"redirect_uri":          {integration.DefaultControlIDERedirectURI},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode(), nil)
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("authorize %d %s", rr.Code, rr.Body.String())
	}
	state, _ := url.Parse(rr.Header().Get("Location"))
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/auth/v1/callback/google?"+url.Values{
		"code": {"c"}, "state": {state.Query().Get("state")},
	}.Encode(), nil)
	ts.Handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 LOGIN_EMAIL_REQUIRED, got %d %s", rr2.Code, rr2.Body.String())
	}
	if !strings.Contains(rr2.Body.String(), "LOGIN_EMAIL_REQUIRED") {
		t.Fatalf("body=%s", rr2.Body.String())
	}
}

func TestSocialLoginNotProvisioned(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})

	fake := &authlogin.FakeProvider{
		ProviderName: identity.ProviderGoogle,
		Claims: authlogin.SubjectClaims{
			Provider: identity.ProviderGoogle,
			Issuer:   authlogin.IssuerGoogle,
			Subject:  "unknown-sub",
			Email:    "x@example.com",
			Name:     "X",
		},
	}
	broker := &authlogin.Broker{Providers: map[string]authlogin.Provider{identity.ProviderGoogle: fake}}
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "dev-admin-key+admin", EnableJWT: true, LoginBroker: broker,
	})
	ts.Config.AuthLoginProviders = []string{"google"}
	ts.Config.AuthAutoProvisionUsers = false
	ts.Config.PlatformPublicURL = "http://one.test"

	integSvc := &integration.Service{Pool: d.Pool, Identity: identity.NopBackend{}, EncryptionKey: ts.Config.AuthJWTSigningKey}
	if err := integSvc.EnsureControlIDE(t.Context()); err != nil {
		t.Fatal(err)
	}

	verifier, _ := authlogin.RandomURLToken(32)
	challenge := authlogin.PKCEChallengeS256(verifier)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/v1/authorize?"+url.Values{
		"provider": {"google"}, "client_id": {integration.APINameControlIDE},
		"redirect_uri":          {integration.DefaultControlIDERedirectURI},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode(), nil)
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("authorize %d %s", rr.Code, rr.Body.String())
	}
	state, _ := url.Parse(rr.Header().Get("Location"))
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/auth/v1/callback/google?"+url.Values{
		"code": {"c"}, "state": {state.Query().Get("state")},
	}.Encode(), nil)
	ts.Handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rr2.Code, rr2.Body.String())
	}
}

func TestSlackSocialLoginPKCEWritesIdentityLink(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	cleanupSocialHumans(t, d)

	fake := &authlogin.FakeProvider{
		ProviderName: identity.ProviderSlack,
		Claims: authlogin.SubjectClaims{
			Provider:      identity.ProviderSlack,
			Issuer:        authlogin.IssuerSlack,
			Subject:       "U123SLACKPKCE",
			Email:         "slackuser-pkce@example.com",
			EmailVerified: true,
			Name:          "Slack User",
		},
	}
	broker := &authlogin.Broker{Providers: map[string]authlogin.Provider{
		identity.ProviderSlack: fake,
	}}
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "dev-admin-key+admin", EnableJWT: true, LoginBroker: broker,
	})
	ts.Config.AuthLoginProviders = []string{"slack"}
	ts.Config.AuthAutoProvisionUsers = true
	ts.Config.PlatformPublicURL = "http://one.test"

	integSvc := &integration.Service{Pool: d.Pool, Identity: identity.NopBackend{}, EncryptionKey: ts.Config.AuthJWTSigningKey}
	if err := integSvc.EnsureControlIDE(t.Context()); err != nil {
		t.Fatal(err)
	}

	verifier, _ := authlogin.RandomURLToken(32)
	challenge := authlogin.PKCEChallengeS256(verifier)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/v1/authorize?"+url.Values{
		"provider": {"slack"}, "client_id": {integration.APINameControlIDE},
		"redirect_uri":          {integration.DefaultControlIDERedirectURI},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode(), nil)
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("authorize %d %s", rr.Code, rr.Body.String())
	}
	state, _ := url.Parse(rr.Header().Get("Location"))
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/auth/v1/callback/slack?"+url.Values{
		"code": {"c"}, "state": {state.Query().Get("state")},
	}.Encode(), nil)
	ts.Handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusFound {
		t.Fatalf("callback %d %s", rr2.Code, rr2.Body.String())
	}

	link, err := db.NewIdentityLinkStore(d.Pool).GetBySubject(t.Context(), identity.ProviderSlack, authlogin.IssuerSlack, "U123SLACKPKCE")
	if err != nil {
		t.Fatal(err)
	}
	if link.Provider != identity.ProviderSlack {
		t.Fatalf("provider=%q want slack", link.Provider)
	}
}

func TestAuthorizeDisabledSlackProvider(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	fake := &authlogin.FakeProvider{ProviderName: identity.ProviderSlack}
	broker := &authlogin.Broker{Providers: map[string]authlogin.Provider{identity.ProviderSlack: fake}}
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "dev-admin-key+admin", EnableJWT: true, LoginBroker: broker,
	})
	ts.Config.AuthLoginProviders = []string{"google"}
	ts.Config.PlatformPublicURL = "http://one.test"
	integSvc := &integration.Service{Pool: d.Pool, Identity: identity.NopBackend{}, EncryptionKey: ts.Config.AuthJWTSigningKey}
	if err := integSvc.EnsureControlIDE(t.Context()); err != nil {
		t.Fatal(err)
	}
	verifier, _ := authlogin.RandomURLToken(32)
	challenge := authlogin.PKCEChallengeS256(verifier)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/v1/authorize?"+url.Values{
		"provider": {"slack"}, "client_id": {integration.APINameControlIDE},
		"redirect_uri":          {integration.DefaultControlIDERedirectURI},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode(), nil)
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected PROVIDER_DISABLED, got %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "PROVIDER_DISABLED") {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestAuthorizeAndCallbackRateLimited(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	fake := &authlogin.FakeProvider{
		ProviderName: identity.ProviderGoogle,
		Claims: authlogin.SubjectClaims{
			Provider: identity.ProviderGoogle, Issuer: authlogin.IssuerGoogle, Subject: "g1",
			Email: "r@example.com", EmailVerified: true, Name: "R",
		},
	}
	broker := &authlogin.Broker{Providers: map[string]authlogin.Provider{identity.ProviderGoogle: fake}}
	cfg := testutil.NewTestConfig(t, testutil.ServerOptions{
		APIKeys: "dev-admin-key+admin", EnableJWT: true, LoginBroker: broker,
	})
	cfg.AuthTokenRateLimitPerMinute = 1
	cfg.AuthLoginProviders = []string{"google"}
	cfg.PlatformPublicURL = "http://one.test"
	userStore := db.NewUserStore(d.Pool)
	resolver := &authz.Resolver{
		Entries:        cfg.APIKeyEntries,
		DefaultOwnerID: cfg.DefaultOwnerID,
		Users:          &db.AuthzUsers{Store: userStore},
		One: &authz.OneSigner{
			SigningKey: []byte(cfg.AuthJWTSigningKey),
			Issuer:     cfg.AuthJWTIssuer,
			TTL:        time.Hour,
		},
	}
	h := httpapi.New(httpapi.Options{
		Config: cfg, Resolver: resolver, Pool: d.Pool, DB: d.Pool, Metadata: d.Meta, LoginBroker: broker,
	}).Handler()
	integSvc := &integration.Service{Pool: d.Pool, Identity: identity.NopBackend{}, EncryptionKey: cfg.AuthJWTSigningKey}
	if err := integSvc.EnsureControlIDE(t.Context()); err != nil {
		t.Fatal(err)
	}
	verifier, _ := authlogin.RandomURLToken(32)
	challenge := authlogin.PKCEChallengeS256(verifier)
	authorizeURL := "/auth/v1/authorize?" + url.Values{
		"provider": {"google"}, "client_id": {integration.APINameControlIDE},
		"redirect_uri":          {integration.DefaultControlIDERedirectURI},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode()
	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest(http.MethodGet, authorizeURL, nil))
	if first.Code != http.StatusFound {
		t.Fatalf("first authorize %d %s", first.Code, first.Body.String())
	}
	second := httptest.NewRecorder()
	h.ServeHTTP(second, httptest.NewRequest(http.MethodGet, authorizeURL, nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second authorize want 429 got %d %s", second.Code, second.Body.String())
	}

	cb1 := httptest.NewRecorder()
	h.ServeHTTP(cb1, httptest.NewRequest(http.MethodGet, "/auth/v1/callback/google?code=c&state=missing", nil))
	if cb1.Code == http.StatusTooManyRequests {
		t.Fatal("first callback should not already be rate-limited")
	}
	cb2 := httptest.NewRecorder()
	h.ServeHTTP(cb2, httptest.NewRequest(http.MethodGet, "/auth/v1/callback/google?code=c&state=missing", nil))
	if cb2.Code != http.StatusTooManyRequests {
		t.Fatalf("second callback want 429 got %d %s", cb2.Code, cb2.Body.String())
	}
}

func cleanupSocialHumans(t *testing.T, d *testutil.Database) {
	t.Helper()
	_, err := d.Pool.Exec(t.Context(), `
DELETE FROM record_access_grants WHERE user_id IN (SELECT id FROM users WHERE principal_type = 'user');
DELETE FROM records WHERE owner_id IN (SELECT id FROM users WHERE principal_type = 'user')
   OR created_by_id IN (SELECT id FROM users WHERE principal_type = 'user')
   OR last_modified_by_id IN (SELECT id FROM users WHERE principal_type = 'user');
DELETE FROM identity_links WHERE user_id IN (SELECT id FROM users WHERE principal_type = 'user');
DELETE FROM user_permission_sets WHERE user_id IN (SELECT id FROM users WHERE principal_type = 'user');
DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE principal_type = 'user');
DELETE FROM users WHERE principal_type = 'user'`)
	if err != nil {
		t.Fatalf("cleanup humans: %v", err)
	}
}
