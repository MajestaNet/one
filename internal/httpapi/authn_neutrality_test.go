package httpapi_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/integration"
	"github.com/MajestaNet/ide/internal/testutil"
)

func jwtAzp(t *testing.T, access string) string {
	t.Helper()
	parts := strings.Split(access, ".")
	if len(parts) < 2 {
		t.Fatalf("not a jwt: %s", access)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("jwt payload: %v", err)
	}
	var claims struct {
		Azp string `json:"azp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("jwt claims: %v", err)
	}
	return claims.Azp
}

func TestPasswordGrantAzpAndRefresh(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	if _, err := d.Pool.Exec(t.Context(), `UPDATE organization_settings SET password_login_enabled = true WHERE id = true`); err != nil {
		t.Fatalf("enable password login: %v", err)
	}
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin", EnableJWT: true,
	})
	h := srv.Handler
	email := fmt.Sprintf("azp-pw-%d@example.com", time.Now().UnixNano())
	rr := testutil.AuthRequest(h, http.MethodPost, "/client/v1/principals", "admin-key", map[string]any{
		"email": email, "displayName": "Azp User", "principalType": "user",
		"roleApiNames": []string{"StandardUser"},
	})
	if rr.Code != 201 {
		t.Fatalf("create principal %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	uid, _ := created["id"].(string)
	rr = testutil.AuthRequest(h, http.MethodPost, "/client/v1/principals/"+uid+"/password", "admin-key", map[string]any{
		"password": "initial-secret-ok",
	})
	if rr.Code != 200 {
		t.Fatalf("set password %d %s", rr.Code, rr.Body.String())
	}

	empty := passwordGrantForm(t, h, email, "initial-secret-ok", "", "")
	if jwtAzp(t, empty["access_token"].(string)) != authz.InstallAzp {
		t.Fatalf("empty client_id azp=%v", empty)
	}
	if empty["refresh_token"] == nil || empty["refresh_token"] == "" {
		t.Fatalf("install password should issue refresh: %v", empty)
	}

	noOffline := passwordGrantForm(t, h, email, "initial-secret-ok", authz.ControlIDEAzp, "")
	if jwtAzp(t, noOffline["access_token"].(string)) != authz.ControlIDEAzp {
		t.Fatalf("control ide password azp=%v", noOffline)
	}
	if noOffline["refresh_token"] != nil && noOffline["refresh_token"] != "" {
		t.Fatalf("control ide password without offline_access must not issue refresh: %v", noOffline)
	}

	withOffline := passwordGrantForm(t, h, email, "initial-secret-ok", authz.ControlIDEAzp, authz.ScopeOfflineAccess)
	if jwtAzp(t, withOffline["access_token"].(string)) != authz.ControlIDEAzp {
		t.Fatalf("control ide + offline_access azp=%v", withOffline)
	}
	if withOffline["refresh_token"] == nil || withOffline["refresh_token"] == "" {
		t.Fatalf("control ide password with offline_access should issue refresh: %v", withOffline)
	}
}

func passwordGrantForm(t *testing.T, h http.Handler, email, password, clientID, scope string) map[string]any {
	t.Helper()
	form := url.Values{
		"grant_type": {"password"},
		"username":   {email},
		"password":   {password},
	}
	if clientID != "" {
		form.Set("client_id", clientID)
	}
	if scope != "" {
		form.Set("scope", scope)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/v1/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("password grant %d %s", rr.Code, rr.Body.String())
	}
	var tok map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &tok)
	if tok["access_token"] == nil || tok["access_token"] == "" {
		t.Fatalf("missing access_token: %v", tok)
	}
	return tok
}

func TestInstallClaimWithoutControlIDEApp(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{SkipControlIDE: true})
	if err := db.NewIntegrationStore(d.Pool).Delete(t.Context(), integration.APINameControlIDE); err != nil && err != db.ErrNotFound {
		t.Fatalf("delete control ide app: %v", err)
	}
	resetInstallClaim(t, d.Pool)

	const claimToken = "test-claim-no-ide-app"
	if _, err := db.SyncInstallClaimToken(t.Context(), d.Pool, claimToken, false); err != nil {
		t.Fatal(err)
	}
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "dev-admin-key+admin", EnableJWT: true,
	})
	email := fmt.Sprintf("noide-%d@example.com", time.Now().UnixNano())
	claimBody, _ := json.Marshal(map[string]any{
		"token": claimToken, "email": email, "password": "supersecret1", "displayName": "Admin",
	})
	claimRR := httptest.NewRecorder()
	claimReq := httptest.NewRequest(http.MethodPost, "/auth/v1/install/claim", bytes.NewReader(claimBody))
	claimReq.Header.Set("Content-Type", "application/json")
	ts.Handler.ServeHTTP(claimRR, claimReq)
	if claimRR.Code != http.StatusOK {
		t.Fatalf("claim %d %s", claimRR.Code, claimRR.Body.String())
	}
	var claimResp map[string]any
	if err := json.Unmarshal(claimRR.Body.Bytes(), &claimResp); err != nil {
		t.Fatal(err)
	}
	if claimResp["refresh_token"] == nil || claimResp["refresh_token"] == "" {
		t.Fatalf("claim missing refresh: %v", claimResp)
	}
	if got := jwtAzp(t, claimResp["access_token"].(string)); got != authz.InstallAzp {
		t.Fatalf("claim azp=%q want %s", got, authz.InstallAzp)
	}
	if _, err := db.NewIntegrationStore(d.Pool).GetByAPIName(t.Context(), integration.APINameControlIDE); err == nil {
		t.Fatal("expected Control IDE app to remain absent")
	}
}

func TestPutExposureRejectsIDEUsers(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin", EnableJWT: true,
	})
	rr := testutil.AuthRequest(srv.Handler, http.MethodPut, "/metadata/v1/install/exposure", "admin-key", map[string]any{
		"clientAccessMode": "ide_users",
		"client":           map[string]any{"mode": "public", "cidrs": []string{}},
		"auth":             map[string]any{"mode": "public", "cidrs": []string{}},
		"metadata":         map[string]any{"mode": "blocked", "cidrs": []string{}},
		"deploy":           map[string]any{"mode": "blocked", "cidrs": []string{}},
		"ops":              map[string]any{"mode": "blocked", "cidrs": []string{}},
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("ide_users want 400 got %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "no longer supported") && !strings.Contains(rr.Body.String(), "ide_users") {
		t.Fatalf("expected ide_users rejection message: %s", rr.Body.String())
	}
}
