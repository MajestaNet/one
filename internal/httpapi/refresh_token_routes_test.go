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

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/edge"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestRefreshTokenGrantLifecycle(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin", EnableJWT: true,
	})
	h := srv.Handler
	email := fmt.Sprintf("rt-http-%d@example.com", time.Now().UnixNano())

	rr := testutil.AuthRequest(h, http.MethodPost, "/client/v1/principals", "admin-key", map[string]any{
		"email": email, "displayName": "RT User", "principalType": "user",
		"roleApiNames": []string{"StandardUser"},
	})
	if rr.Code != 201 {
		t.Fatalf("create principal %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	uid, _ := created["id"].(string)
	if uid == "" {
		t.Fatal("missing user id")
	}
	rr = testutil.AuthRequest(h, http.MethodPost, "/client/v1/principals/"+uid+"/password", "admin-key", map[string]any{
		"password": "initial-secret-ok",
	})
	if rr.Code != 200 {
		t.Fatalf("admin set password %d %s", rr.Code, rr.Body.String())
	}

	tok := passwordGrant(t, h, email, "initial-secret-ok")
	access, _ := tok["access_token"].(string)
	rt1, _ := tok["refresh_token"].(string)
	if access == "" || rt1 == "" {
		t.Fatalf("password grant missing tokens: %v", tok)
	}
	if tok["expires_in"] != float64(3600) {
		t.Fatalf("access TTL want 3600 got %v", tok["expires_in"])
	}
	if _, ok := tok["refresh_expires_in"].(float64); !ok {
		t.Fatalf("missing refresh_expires_in: %v", tok)
	}

	exposure := db.NewExposureStore(d.Pool)
	originalExposure, err := exposure.Get(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	invalidExposure := originalExposure.Policy
	invalidExposure.ClientAccessMode = edge.ClientAccessMode("corrupt")
	if err := exposure.Put(t.Context(), invalidExposure, edge.StatusApplied, nil); err != nil {
		t.Fatal(err)
	}
	policyUnavailable := refreshGrant(t, h, rt1, authz.InstallAzp)
	if policyUnavailable.Code != http.StatusServiceUnavailable || !strings.Contains(policyUnavailable.Body.String(), "POLICY_UNAVAILABLE") {
		t.Fatalf("invalid policy refresh %d %s", policyUnavailable.Code, policyUnavailable.Body.String())
	}
	if err := exposure.Put(t.Context(), originalExposure.Policy, originalExposure.Status, originalExposure.LastError); err != nil {
		t.Fatal(err)
	}

	me := testutil.AuthRequest(h, http.MethodGet, "/client/v1/me", access, nil)
	if me.Code != 200 {
		t.Fatalf("me %d %s", me.Code, me.Body.String())
	}

	refreshed := refreshGrant(t, h, rt1, authz.InstallAzp)
	if refreshed.Code != 200 {
		t.Fatalf("refresh %d %s", refreshed.Code, refreshed.Body.String())
	}
	var round2 map[string]any
	_ = json.Unmarshal(refreshed.Body.Bytes(), &round2)
	rt2, _ := round2["refresh_token"].(string)
	access2, _ := round2["access_token"].(string)
	if rt2 == "" || rt2 == rt1 {
		t.Fatalf("expected rotated refresh token: %v", round2)
	}
	if access2 == "" {
		t.Fatalf("missing access token after refresh: %v", round2)
	}
	me2 := testutil.AuthRequest(h, http.MethodGet, "/client/v1/me", access2, nil)
	if me2.Code != 200 {
		t.Fatalf("me after refresh %d %s", me2.Code, me2.Body.String())
	}

	old := refreshGrant(t, h, rt1, authz.InstallAzp)
	if old.Code != http.StatusUnauthorized {
		t.Fatalf("old RT want 401 got %d %s", old.Code, old.Body.String())
	}
	reuse := refreshGrant(t, h, rt2, authz.InstallAzp)
	if reuse.Code != http.StatusUnauthorized {
		t.Fatalf("reuse should kill family; got %d %s", reuse.Code, reuse.Body.String())
	}
	var reuseAudit int
	if err := d.Pool.QueryRow(t.Context(), `SELECT count(*) FROM audit_log WHERE action = 'token.refresh.reuse'`).Scan(&reuseAudit); err != nil || reuseAudit < 1 {
		t.Fatalf("reuse audit count=%d err=%v", reuseAudit, err)
	}

	tok = passwordGrant(t, h, email, "initial-secret-ok")
	rtLive, _ := tok["refresh_token"].(string)
	accessLive, _ := tok["access_token"].(string)

	rev := httptest.NewRecorder()
	revReq := httptest.NewRequest(http.MethodPost, "/auth/v1/revoke", bytes.NewBufferString(
		fmt.Sprintf(`{"token":%q,"token_type_hint":"refresh_token"}`, rtLive)))
	revReq.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rev, revReq)
	if rev.Code != 200 {
		t.Fatalf("revoke %d %s", rev.Code, rev.Body.String())
	}
	afterRevoke := refreshGrant(t, h, rtLive, authz.InstallAzp)
	if afterRevoke.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after revoke want 401 got %d %s", afterRevoke.Code, afterRevoke.Body.String())
	}

	_ = accessLive
	tok = passwordGrant(t, h, email, "initial-secret-ok")
	rtPwd, _ := tok["refresh_token"].(string)
	accessPwd, _ := tok["access_token"].(string)
	chg := testutil.AuthRequest(h, http.MethodPost, "/client/v1/me/password", accessPwd, map[string]any{
		"currentPassword": "initial-secret-ok",
		"newPassword":     "rotated-secret-ok",
	})
	if chg.Code != 200 {
		t.Fatalf("self password %d %s", chg.Code, chg.Body.String())
	}
	if rr := refreshGrant(t, h, rtPwd, authz.InstallAzp); rr.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after password change want 401 got %d %s", rr.Code, rr.Body.String())
	}

	tok = passwordGrant(t, h, email, "rotated-secret-ok")
	rtFreeze, _ := tok["refresh_token"].(string)
	fr := testutil.AuthRequest(h, http.MethodPost, "/client/v1/principals/"+uid+"/freeze", "admin-key", map[string]any{
		"reason": "test",
	})
	if fr.Code != 200 {
		t.Fatalf("freeze %d %s", fr.Code, fr.Body.String())
	}
	if rr := refreshGrant(t, h, rtFreeze, authz.InstallAzp); rr.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after freeze want 401 got %d %s", rr.Code, rr.Body.String())
	}

	cc := httptest.NewRecorder()
	ccReq := httptest.NewRequest(http.MethodPost, "/auth/v1/token", strings.NewReader(
		`{"grant_type":"client_credentials","client_secret":"admin-key"}`))
	ccReq.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(cc, ccReq)
	if cc.Code != 200 {
		t.Fatalf("client_credentials %d %s", cc.Code, cc.Body.String())
	}
	var ccBody map[string]any
	_ = json.Unmarshal(cc.Body.Bytes(), &ccBody)
	if ccBody["refresh_token"] != nil && ccBody["refresh_token"] != "" {
		t.Fatalf("client_credentials issued refresh: %v", ccBody)
	}

	unknown := httptest.NewRecorder()
	unkReq := httptest.NewRequest(http.MethodPost, "/auth/v1/revoke", strings.NewReader(`{"token":"not-a-token"}`))
	unkReq.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(unknown, unkReq)
	if unknown.Code != 200 {
		t.Fatalf("unknown revoke want 200 got %d", unknown.Code)
	}
}

func passwordGrant(t *testing.T, h http.Handler, email, password string) map[string]any {
	t.Helper()
	form := url.Values{
		"grant_type": {"password"},
		"username":   {email},
		"password":   {password},
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
	return tok
}

func refreshGrant(t *testing.T, h http.Handler, refreshToken, clientID string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     clientID,
	})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/v1/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)
	return rr
}
