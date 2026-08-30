package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authlogin"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/testutil"
)

func resetInstallClaim(t *testing.T, pool *db.Pool) {
	t.Helper()
	_, err := pool.Exec(t.Context(), `
		UPDATE organization_settings
		SET claimed_at = NULL, claim_token_hash = NULL, password_login_enabled = true
		WHERE id = true`)
	if err != nil {
		t.Fatalf("reset claim: %v", err)
	}
}

func TestAuthLoginPageUnclaimedShowsClaim(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	resetInstallClaim(t, d.Pool)
	_, err := db.SyncInstallClaimToken(t.Context(), d.Pool, "test-claim-token-xyz", false)
	if err != nil {
		t.Fatal(err)
	}

	broker := authlogin.NewBroker(authlogin.GoogleConfig{}, authlogin.AppleConfig{}, true)
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "dev-admin-key+admin", EnableJWT: true, LoginBroker: broker,
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/v1/login", nil)
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Claim install") {
		t.Fatalf("expected claim form: %s", body)
	}
	if strings.Contains(body, "Continue with Google") {
		t.Fatalf("unclaimed install should not default to Google: %s", body)
	}
}

func TestInstallClaimAndPasswordGrant(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	resetInstallClaim(t, d.Pool)
	const claimToken = "test-claim-token-abc12345"
	claimEmail := fmt.Sprintf("claim-%d@example.com", time.Now().UnixNano())
	if _, err := db.SyncInstallClaimToken(t.Context(), d.Pool, claimToken, false); err != nil {
		t.Fatal(err)
	}

	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "dev-admin-key+admin", EnableJWT: true,
	})

	statusRR := httptest.NewRecorder()
	ts.Handler.ServeHTTP(statusRR, httptest.NewRequest(http.MethodGet, "/auth/v1/install/status", nil))
	if statusRR.Code != http.StatusOK {
		t.Fatalf("status %d", statusRR.Code)
	}
	var status db.InstallAuthStatus
	if err := json.Unmarshal(statusRR.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Claimed {
		t.Fatal("expected unclaimed")
	}

	claimBody, _ := json.Marshal(map[string]any{
		"token": claimToken, "email": claimEmail, "password": "supersecret1", "displayName": "Admin",
	})
	claimRR := httptest.NewRecorder()
	claimReq := httptest.NewRequest(http.MethodPost, "/auth/v1/install/claim", bytes.NewReader(claimBody))
	claimReq.Header.Set("Content-Type", "application/json")
	ts.Handler.ServeHTTP(claimRR, claimReq)
	if claimRR.Code != http.StatusOK {
		t.Fatalf("claim %d %s", claimRR.Code, claimRR.Body.String())
	}
	var claimEvt string
	if err := d.Pool.QueryRow(t.Context(), `
SELECT event_type FROM outbox_events WHERE event_type = $1
ORDER BY created_at DESC LIMIT 1`, db.EventInstallClaimed).Scan(&claimEvt); err != nil {
		t.Fatalf("outbox install.claimed: %v", err)
	}
	var claimResp map[string]any
	if err := json.Unmarshal(claimRR.Body.Bytes(), &claimResp); err != nil {
		t.Fatal(err)
	}
	if claimResp["access_token"] == nil || claimResp["access_token"] == "" {
		t.Fatalf("missing token: %v", claimResp)
	}
	if claimResp["refresh_token"] == nil || claimResp["refresh_token"] == "" {
		t.Fatalf("claim missing refresh_token: %v", claimResp)
	}
	if got := jwtAzp(t, claimResp["access_token"].(string)); got != "one.install" {
		t.Fatalf("claim azp=%q want one.install", got)
	}

	// Replay rejected.
	replayRR := httptest.NewRecorder()
	replayReq := httptest.NewRequest(http.MethodPost, "/auth/v1/install/claim", bytes.NewReader(claimBody))
	replayReq.Header.Set("Content-Type", "application/json")
	ts.Handler.ServeHTTP(replayRR, replayReq)
	if replayRR.Code != http.StatusConflict {
		t.Fatalf("replay want 409 got %d %s", replayRR.Code, replayRR.Body.String())
	}

	form := url.Values{
		"grant_type": {"password"},
		"username":   {claimEmail},
		"password":   {"supersecret1"},
	}
	pwRR := httptest.NewRecorder()
	pwReq := httptest.NewRequest(http.MethodPost, "/auth/v1/token", strings.NewReader(form.Encode()))
	pwReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ts.Handler.ServeHTTP(pwRR, pwReq)
	if pwRR.Code != http.StatusOK {
		t.Fatalf("password grant %d %s", pwRR.Code, pwRR.Body.String())
	}
	var pwTok map[string]any
	if err := json.Unmarshal(pwRR.Body.Bytes(), &pwTok); err != nil {
		t.Fatal(err)
	}
	if pwTok["refresh_token"] == nil || pwTok["refresh_token"] == "" {
		t.Fatalf("password without client_id missing refresh: %v", pwTok)
	}
	if got := jwtAzp(t, pwTok["access_token"].(string)); got != "one.install" {
		t.Fatalf("password empty client_id azp=%q want one.install", got)
	}

	// Configure SSO + JIT via Metadata.
	token := claimResp["access_token"].(string)
	authBody, _ := json.Marshal(map[string]any{
		"oidcIssuer":           "https://idp.example.com",
		"oidcAudience":         "one-app",
		"oidcDisplayName":      "Example SSO",
		"oidcClientId":         "one-app",
		"oidcClientSecret":     "s3cret",
		"jitProvisionUsers":    true,
		"jitDefaultRole":       "StandardUser",
		"allowedEmailDomains":  []string{"example.com"},
		"socialProviders":      []string{},
		"passwordLoginEnabled": true,
	})
	putRR := httptest.NewRecorder()
	putReq := httptest.NewRequest(http.MethodPut, "/metadata/v1/install/auth", bytes.NewReader(authBody))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("Authorization", "Bearer "+token)
	ts.Handler.ServeHTTP(putRR, putReq)
	if putRR.Code != http.StatusOK {
		t.Fatalf("put install auth %d %s", putRR.Code, putRR.Body.String())
	}
}

func TestAuthLoginPageClaimedDevProvider(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	store := db.NewInstallAuthStore(d.Pool)
	if err := store.SetClaimTokenHash(t.Context(), "x"); err != nil && err != db.ErrConflict {
		t.Fatalf("SetClaimTokenHash: %v", err)
	}
	_ = store.MarkClaimed(t.Context())

	broker := authlogin.NewBroker(authlogin.GoogleConfig{}, authlogin.AppleConfig{}, true)
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "dev-admin-key+admin", EnableJWT: true, LoginBroker: broker,
	})
	ts.Config.AuthLoginProviders = []string{"dev"}

	q := url.Values{
		"client_id":             {"one.controlIde"},
		"redirect_uri":          {"one-control://oauth/callback"},
		"code_challenge":        {"challenge"},
		"code_challenge_method": {"S256"},
		"state":                 {"st"},
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/v1/login?"+q.Encode(), nil)
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "provider=dev") {
		t.Fatalf("expected provider=dev href: %s", body)
	}
	if strings.Contains(body, "Continue with Google") {
		t.Fatalf("dev should not be labeled Google by default: %s", body)
	}
}
