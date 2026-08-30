package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/httpapi"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestAuthenticationPoliciesFailClosedWhenDatabaseUnavailable(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := db.Connect(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()

	cfg := testutil.NewTestConfig(t, testutil.ServerOptions{APIKeys: "policy-key:client"})
	resolver := &authz.Resolver{Entries: cfg.APIKeyEntries, DefaultOwnerID: cfg.DefaultOwnerID}
	h := httpapi.New(httpapi.Options{Config: cfg, Resolver: resolver, DB: pool, Pool: pool}).Handler()

	login := httptest.NewRecorder()
	h.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/auth/v1/login/providers", nil))
	if login.Code != http.StatusServiceUnavailable || !strings.Contains(login.Body.String(), "AUTH_POLICY_UNAVAILABLE") {
		t.Fatalf("install auth lookup failure: status=%d body=%s", login.Code, login.Body.String())
	}

	client := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/client/v1/me", nil)
	req.Header.Set("Authorization", "Bearer policy-key")
	h.ServeHTTP(client, req)
	if client.Code != http.StatusServiceUnavailable || !strings.Contains(client.Body.String(), "POLICY_UNAVAILABLE") {
		t.Fatalf("exposure policy lookup failure: status=%d body=%s", client.Code, client.Body.String())
	}
}
