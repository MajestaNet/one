package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/testutil"
)

func TestSharingEnableRequiresConfirm(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{APIKeys: "admin-key+admin"})

	body, _ := json.Marshal(map[string]any{"confirm": false})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/metadata/v1/sharing/enable", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestSharingCreateRuleRejectsEmptyFilters(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{APIKeys: "admin-key+admin"})
	ctx := t.Context()
	_, _ = d.Pool.Exec(ctx, `DELETE FROM sharing_rules WHERE object_api_name='Account'`)
	_, _ = d.Pool.Exec(ctx, `DELETE FROM data_roles WHERE api_name='ShareRuleRole'`)

	roleName := "ShareRuleRole" + strconv.FormatInt(time.Now().UnixNano(), 10)
	ruleName := "EmptyFilters" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(ctx, `DELETE FROM sharing_rules WHERE object_api_name='Account'`)
		_, _ = d.Pool.Exec(ctx, `DELETE FROM data_roles WHERE api_name=$1`, roleName)
		_, _ = d.Pool.Exec(ctx, `UPDATE organization_settings SET record_sharing_enabled=false, record_sharing_enabled_at=NULL WHERE id=true`)
		_, _ = d.Pool.Exec(ctx, `UPDATE object_sharing_settings SET sharing_rules_enabled=false WHERE object_api_name='Account'`)
		_, _ = d.Pool.Exec(ctx, `DELETE FROM jobs WHERE job_type='sharing.recalc'`)
	})

	enable, _ := json.Marshal(map[string]any{"confirm": true})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/metadata/v1/sharing/enable", bytes.NewReader(enable))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusConflict {
		t.Fatalf("enable status=%d body=%s", rr.Code, rr.Body.String())
	}

	patchOWD, _ := json.Marshal(map[string]any{"defaultAccess": "private", "sharingRulesEnabled": true})
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/metadata/v1/sharing/objects/Account", bytes.NewReader(patchOWD))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("owd status=%d body=%s", rr.Code, rr.Body.String())
	}

	roleBody, _ := json.Marshal(map[string]any{"apiName": roleName, "label": "Share Rule Role"})
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/client/v1/data-roles", bytes.NewReader(roleBody))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("data-role status=%d body=%s", rr.Code, rr.Body.String())
	}

	rule, _ := json.Marshal(map[string]any{
		"apiName": ruleName, "label": "Bad", "active": true, "accessLevel": "read",
		"sharedToDataRoleApiName": roleName,
		"criteria":                map[string]any{"filters": []any{}},
	})
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/metadata/v1/sharing/objects/Account/rules", bytes.NewReader(rule))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code == http.StatusCreated {
		t.Fatal("empty filters must be rejected")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}
