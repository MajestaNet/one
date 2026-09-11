package httpapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/MajestaNet/ide/internal/seed"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestManagedAutomationRunPackageDisabled409(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin,client-key:client",
	})
	ctx := context.Background()
	if _, err := seed.EnablePackage(ctx, d.Meta, "lead_marketing"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = seed.EnablePackage(bg, d.Meta, "lead_marketing")
	})

	const autoName = "Lead_ConvertOnConvertedStatus"
	rr := testutil.AuthRequest(srv.Handler, http.MethodGet, "/metadata/v1/automations/"+autoName, "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET while enabled: %d %s", rr.Code, rr.Body.String())
	}

	if _, err := seed.DisablePackage(ctx, d.Meta, "lead_marketing"); err != nil {
		t.Fatal(err)
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/metadata/v1/automations/"+autoName, "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET after pack disable: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/automations/"+autoName+"/runs", "admin-key", map[string]any{
		"input": map[string]any{},
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("run pack-disabled: expected 409, got %d %s", rr.Code, rr.Body.String())
	}
	assertErrorCode(t, rr.Body.Bytes(), "PACKAGE_NOT_ENABLED")
}
