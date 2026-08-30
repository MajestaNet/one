package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/MajestaNet/ide/internal/testutil"
)

func TestBundlePushRouteRemoved(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{APIKeys: "admin-key+admin"})
	rr := testutil.AuthRequest(ts.Handler, http.MethodPost, "/deploy/v1/bundles/00000000-0000-4000-8000-000000000099/push", "admin-key", map[string]any{
		"targetUrl":    "https://peer.example",
		"targetApiKey": "x",
	})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for removed peer push route, got %d %s", rr.Code, rr.Body.String())
	}
}
