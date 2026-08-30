package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/testutil"
)

func TestUserKernelMetadataAndCustomFields(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:   "admin-key+admin",
		EnableJWT: true,
	})
	h := srv.Handler
	auth := func(method, path, bearer string, body any) *httptest.ResponseRecorder {
		return testutil.AuthRequest(h, method, path, bearer, body)
	}

	rr := auth(http.MethodGet, "/metadata/v1/objects/User", "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("metadata GET User %d %s", rr.Code, rr.Body.String())
	}
	var desc map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &desc)
	if desc["storageMode"] != "kernel" {
		t.Fatalf("storageMode=%v", desc["storageMode"])
	}
	if !describeHasField(desc, "Email") {
		t.Fatalf("User describe missing Email: %s", rr.Body.String())
	}

	suffix := time.Now().Format("150405.000")
	fieldName := "CostCenter" + strings.ReplaceAll(suffix, ".", "") + "__c"
	rr = auth(http.MethodPost, "/metadata/v1/fields", "admin-key", map[string]any{
		"objectApiName": "User",
		"apiName":       fieldName,
		"label":         "Cost Center",
		"fieldType":     "text",
	})
	if rr.Code != 201 {
		t.Fatalf("create User field %d %s", rr.Code, rr.Body.String())
	}

	rr = auth(http.MethodGet, "/metadata/v1/objects/User", "admin-key", nil)
	_ = json.Unmarshal(rr.Body.Bytes(), &desc)
	if !describeHasField(desc, "Email") || !describeHasField(desc, fieldName) {
		t.Fatalf("User describe missing Email or %s: %s", fieldName, rr.Body.String())
	}

	rr = auth(http.MethodPost, "/client/v1/sobjects/User", "admin-key", map[string]any{"Email": "nope@example.com"})
	if rr.Code < 400 || rr.Code >= 500 {
		t.Fatalf("User record create want 4xx got %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "not a flexible object") && rr.Code != http.StatusForbidden {
		t.Fatalf("User record create body=%s", rr.Body.String())
	}

	email := "user-ext-" + suffix + "@example.com"
	rr = auth(http.MethodPost, "/client/v1/principals", "admin-key", map[string]any{
		"email": email, "displayName": "Ext User",
		"roleApiNames": []string{"StandardUser"},
		fieldName:      "CC-100",
	})
	if rr.Code != 201 {
		t.Fatalf("create principal %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	pid, _ := created["id"].(string)
	if pid == "" || created[fieldName] != "CC-100" {
		t.Fatalf("create response missing custom field: %s", rr.Body.String())
	}

	rr = auth(http.MethodPatch, "/client/v1/principals/"+pid, "admin-key", map[string]any{
		fieldName:        "CC-200",
		"employeeNumber": "E-" + suffix,
	})
	if rr.Code != 200 {
		t.Fatalf("admin patch %d %s", rr.Code, rr.Body.String())
	}
	var patched map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &patched)
	if patched[fieldName] != "CC-200" {
		t.Fatalf("admin patch custom=%v", patched[fieldName])
	}
	if patched["employeeNumber"] != "E-"+suffix {
		t.Fatalf("employeeNumber=%v", patched["employeeNumber"])
	}

	rr = auth(http.MethodGet, "/client/v1/principals/"+pid, "admin-key", nil)
	_ = json.Unmarshal(rr.Body.Bytes(), &patched)
	if patched[fieldName] != "CC-200" {
		t.Fatalf("admin get custom=%v", patched[fieldName])
	}

	rr = auth(http.MethodGet, "/client/v1/principals", "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("list %d %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "CC-200") {
		t.Fatalf("list should omit custom data: %s", rr.Body.String())
	}

	rr = auth(http.MethodGet, "/metadata/v1/snapshot", "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("snapshot %d %s", rr.Code, rr.Body.String())
	}
	var snap map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &snap)
	if !snapshotHasField(snap, "User", fieldName) {
		t.Fatalf("snapshot missing User.%s: %s", fieldName, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "CC-200") {
		t.Fatalf("snapshot must not include user values")
	}

	rr = auth(http.MethodPatch, "/client/v1/principals/"+pid, "admin-key", map[string]any{
		"NotAUserField__c": "x",
	})
	if rr.Code != 400 {
		t.Fatalf("unknown field want 400 got %d %s", rr.Code, rr.Body.String())
	}

	dirEmail := "dir-" + suffix + "@example.com"
	rr = auth(http.MethodPost, "/client/v1/principals", "admin-key", map[string]any{
		"email": dirEmail, "displayName": "Directory Admin",
		"roleApiNames":          []string{"StandardUser"},
		"permissionSetApiNames": []string{"IdentityManage"},
	})
	if rr.Code != 201 {
		t.Fatalf("create identity manager %d %s", rr.Code, rr.Body.String())
	}
	var dir map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &dir)
	dirID, _ := dir["id"].(string)
	rr = auth(http.MethodPost, "/client/v1/principals/"+dirID+"/credentials", "admin-key", map[string]any{
		"label": "dir",
	})
	if rr.Code != 201 {
		t.Fatalf("issue credential %d %s", rr.Code, rr.Body.String())
	}
	var cred map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &cred)
	secret, _ := cred["clientSecret"].(string)
	tokBody, _ := json.Marshal(map[string]any{
		"grant_type": "client_credentials", "client_id": dirID, "client_secret": secret,
	})
	tokReq := httptest.NewRequest(http.MethodPost, "/auth/v1/token", bytes.NewReader(tokBody))
	tokReq.Header.Set("Content-Type", "application/json")
	tokRR := httptest.NewRecorder()
	h.ServeHTTP(tokRR, tokReq)
	if tokRR.Code != 200 {
		t.Fatalf("token %d %s", tokRR.Code, tokRR.Body.String())
	}
	var tok map[string]any
	_ = json.Unmarshal(tokRR.Body.Bytes(), &tok)
	access, _ := tok["access_token"].(string)
	if access == "" {
		t.Fatal("missing access token")
	}

	rr = auth(http.MethodGet, "/client/v1/principals/"+pid, access, nil)
	if rr.Code != 200 {
		t.Fatalf("identity get %d %s", rr.Code, rr.Body.String())
	}
	var viewed map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &viewed)
	if _, ok := viewed[fieldName]; ok {
		t.Fatalf("FLS should omit %s: %s", fieldName, rr.Body.String())
	}
	if viewed["email"] == "" {
		t.Fatalf("standard email should remain: %s", rr.Body.String())
	}

	rr = auth(http.MethodPatch, "/client/v1/principals/"+pid, access, map[string]any{
		fieldName: "CC-DENIED",
	})
	if rr.Code != 403 {
		t.Fatalf("FLS patch want 403 got %d %s", rr.Code, rr.Body.String())
	}

	rr = auth(http.MethodGet, "/client/v1/describe/User", access, nil)
	if rr.Code != 200 {
		t.Fatalf("describe User %d %s", rr.Code, rr.Body.String())
	}
	var clientDesc map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &clientDesc)
	if describeHasField(clientDesc, fieldName) {
		t.Fatalf("describe FLS should omit %s: %s", fieldName, rr.Body.String())
	}
}

func describeHasField(desc map[string]any, apiName string) bool {
	fields, _ := desc["fields"].([]any)
	for _, raw := range fields {
		f, _ := raw.(map[string]any)
		if f["apiName"] == apiName {
			return true
		}
	}
	return false
}

func snapshotHasField(snap map[string]any, objectAPIName, fieldAPIName string) bool {
	fields, _ := snap["fields"].([]any)
	for _, raw := range fields {
		f, _ := raw.(map[string]any)
		if f["objectApiName"] == objectAPIName && f["apiName"] == fieldAPIName {
			return true
		}
	}
	return false
}
