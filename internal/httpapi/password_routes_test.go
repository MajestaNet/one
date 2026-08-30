package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestAdminSetPasswordAndSelfChange(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin", EnableJWT: true,
	})
	h := srv.Handler
	auth := func(method, path, bearer string, body any) *httptest.ResponseRecorder {
		return testutil.AuthRequest(h, method, path, bearer, body)
	}

	rr := auth(http.MethodPost, "/client/v1/principals", "admin-key", map[string]any{
		"email": "pwd-user@example.com", "displayName": "Pwd User", "principalType": "user",
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

	rr = auth(http.MethodPost, "/client/v1/principals/"+uid+"/password", "admin-key", map[string]any{
		"password": "initial-secret-ok",
	})
	if rr.Code != 200 {
		t.Fatalf("admin set password %d %s", rr.Code, rr.Body.String())
	}

	ctx := context.Background()
	var etype string
	if err := d.Pool.QueryRow(ctx, `
SELECT event_type FROM outbox_events
WHERE event_type = $1 AND record_id = $2::uuid
ORDER BY created_at DESC LIMIT 1`, db.EventPrincipalPasswordChanged, uid).Scan(&etype); err != nil {
		t.Fatalf("outbox password_changed: %v", err)
	}
	if err := d.Pool.QueryRow(ctx, `
SELECT event_type FROM outbox_events
WHERE event_type = $1 AND record_id = $2::uuid
ORDER BY created_at DESC LIMIT 1`, db.EventPrincipalCreated, uid).Scan(&etype); err != nil {
		t.Fatalf("outbox principal.created: %v", err)
	}

	form := url.Values{
		"grant_type": {"password"},
		"username":   {"pwd-user@example.com"},
		"password":   {"initial-secret-ok"},
	}
	tokRR := httptest.NewRecorder()
	tokReq := httptest.NewRequest(http.MethodPost, "/auth/v1/token", strings.NewReader(form.Encode()))
	tokReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(tokRR, tokReq)
	if tokRR.Code != 200 {
		t.Fatalf("password token %d %s", tokRR.Code, tokRR.Body.String())
	}
	var tok map[string]any
	_ = json.Unmarshal(tokRR.Body.Bytes(), &tok)
	access, _ := tok["access_token"].(string)
	if access == "" {
		t.Fatal("missing access_token")
	}

	rr = auth(http.MethodPost, "/client/v1/me/password", access, map[string]any{
		"currentPassword": "initial-secret-ok",
		"newPassword":     "rotated-secret-ok",
	})
	if rr.Code != 200 {
		t.Fatalf("self change password %d %s", rr.Code, rr.Body.String())
	}

	ok, err := db.NewCredentialStore(d.Pool).VerifyPassword(ctx, uid, "rotated-secret-ok")
	if err != nil || !ok {
		t.Fatalf("verify rotated password ok=%v err=%v", ok, err)
	}
}
