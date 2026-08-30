package httpapi_test

import (
	"encoding/json"
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
	"github.com/MajestaNet/ide/internal/identity"
	"github.com/MajestaNet/ide/internal/integration"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestSCIMUserCustomAndEmployeeNumberSplit(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:   "admin-key+admin",
		EnableJWT: true,
		Identity:  identity.NewMemoryBackend(),
	})
	h := srv.Handler
	auth := func(method, path, bearer string, body any) *httptest.ResponseRecorder {
		return testutil.AuthRequest(h, method, path, bearer, body)
	}

	stamp := time.Now().Format("150405.000")
	fieldName := "CostCenter" + strings.ReplaceAll(stamp, ".", "") + "__c"
	roleName := "SalesRep" + strings.ReplaceAll(stamp, ".", "")
	psName := "SalesUser" + strings.ReplaceAll(stamp, ".", "")

	rr := auth(http.MethodPost, "/metadata/v1/fields", "admin-key", map[string]any{
		"objectApiName": "User",
		"apiName":       fieldName,
		"label":         "Cost Center",
		"fieldType":     "text",
	})
	if rr.Code != 201 {
		t.Fatalf("create User field %d %s", rr.Code, rr.Body.String())
	}
	rr = auth(http.MethodPost, "/client/v1/data-roles", "admin-key", map[string]any{
		"apiName": roleName, "label": "Sales Rep",
	})
	if rr.Code != 201 {
		t.Fatalf("create data role %d %s", rr.Code, rr.Body.String())
	}
	rr = auth(http.MethodPost, "/metadata/v1/permissions/sets", "admin-key", map[string]any{
		"apiName": psName, "label": "Sales User",
	})
	if rr.Code != 201 {
		t.Fatalf("create PS %d %s", rr.Code, rr.Body.String())
	}

	rr = auth(http.MethodGet, "/scim/v2/Schemas", "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("schemas %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), fieldName) {
		t.Fatalf("Schemas missing %s: %s", fieldName, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom") {
		t.Fatalf("Schemas missing UserCustom URN: %s", rr.Body.String())
	}

	userName := "scim-ext-" + stamp
	email := userName + "@example.com"
	extID := "fed-" + stamp
	empNo := "E-" + stamp
	rr = auth(http.MethodPost, "/scim/v2/Users", "admin-key", map[string]any{
		"schemas": []string{
			"urn:ietf:params:scim:schemas:core:2.0:User",
			"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User",
			"urn:ietf:params:scim:schemas:extension:one:2.0:Principal",
			"urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom",
		},
		"userName":    userName,
		"externalId":  extID,
		"displayName": "SCIM Ext",
		"emails":      []map[string]any{{"value": email, "primary": true, "type": "work"}},
		"active":      true,
		"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User": map[string]any{
			"employeeNumber": empNo,
			"department":     "Sales",
		},
		"urn:ietf:params:scim:schemas:extension:one:2.0:Principal": map[string]any{
			"principalType":   "user",
			"roleApiNames":    []string{"StandardUser"},
			"dataRoleApiName": roleName,
		},
		"urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom": map[string]any{
			fieldName: "CC-1",
		},
	})
	if rr.Code != 201 {
		t.Fatalf("scim create %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("missing id")
	}
	if created["externalId"] != extID {
		t.Fatalf("externalId=%v", created["externalId"])
	}
	ent, _ := created["urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"].(map[string]any)
	if ent == nil || ent["employeeNumber"] != empNo {
		t.Fatalf("employeeNumber=%v", ent)
	}
	if created["externalId"] == ent["employeeNumber"] {
		t.Fatal("employeeNumber must not alias externalId")
	}
	custom, _ := created["urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom"].(map[string]any)
	if custom == nil || custom[fieldName] != "CC-1" {
		t.Fatalf("UserCustom=%v", created)
	}
	lat, _ := created["urn:ietf:params:scim:schemas:extension:one:2.0:Principal"].(map[string]any)
	if lat == nil || lat["dataRoleApiName"] != roleName {
		t.Fatalf("one=%v", lat)
	}

	rr = auth(http.MethodPatch, "/scim/v2/Users/"+id, "admin-key", map[string]any{
		"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": []map[string]any{
			{
				"op":    "add",
				"path":  "urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom:" + fieldName,
				"value": "CC-OKTA",
			},
		},
	})
	if rr.Code != 200 {
		t.Fatalf("okta patch %d %s", rr.Code, rr.Body.String())
	}
	var patched map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &patched)
	custom, _ = patched["urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom"].(map[string]any)
	if custom == nil || custom[fieldName] != "CC-OKTA" {
		t.Fatalf("patched custom=%v", patched)
	}

	rr = auth(http.MethodPut, "/metadata/v1/install/auth", "admin-key", map[string]any{
		"jitProvisionUsers": true,
		"provisioning": map[string]any{
			"scimDefaultRoleApiName":           "StandardUser",
			"scimDefaultPermissionSetApiNames": []string{psName},
		},
	})
	if rr.Code != 200 {
		t.Fatalf("put provisioning %d %s", rr.Code, rr.Body.String())
	}
	t.Cleanup(func() {
		testutil.AuthRequest(h, http.MethodPut, "/metadata/v1/install/auth", "admin-key", map[string]any{
			"provisioning": map[string]any{},
		})
	})

	plainName := "scim-plain-" + stamp
	rr = auth(http.MethodPost, "/scim/v2/Users", "admin-key", map[string]any{
		"schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		"userName":    plainName,
		"displayName": "Plain Human",
		"emails":      []map[string]any{{"value": plainName + "@example.com", "primary": true}},
		"active":      true,
	})
	if rr.Code != 201 {
		t.Fatalf("scim default grants %d %s", rr.Code, rr.Body.String())
	}
	var plain map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &plain)
	lat, _ = plain["urn:ietf:params:scim:schemas:extension:one:2.0:Principal"].(map[string]any)
	if lat == nil {
		t.Fatalf("missing one: %s", rr.Body.String())
	}
	roles, _ := lat["roleApiNames"].([]any)
	if len(roles) != 1 || roles[0] != "StandardUser" {
		t.Fatalf("roles=%v", lat["roleApiNames"])
	}
	ps, _ := lat["permissionSetApiNames"].([]any)
	foundPS := false
	for _, n := range ps {
		if n == psName {
			foundPS = true
		}
	}
	if !foundPS {
		t.Fatalf("expected default PS %s, got %v", psName, ps)
	}
}

func TestInstallAuthProvisioningValidation(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:   "admin-key+admin",
		EnableJWT: true,
	})
	rr := testutil.AuthRequest(srv.Handler, http.MethodPut, "/metadata/v1/install/auth", "admin-key", map[string]any{
		"provisioning": map[string]any{
			"claimMappings": []map[string]any{
				{"claim": "cost_center", "fieldApiName": "NoSuchField__c"},
			},
		},
	})
	if rr.Code != 400 {
		t.Fatalf("invalid mapping want 400 got %d %s", rr.Code, rr.Body.String())
	}
	rr = testutil.AuthRequest(srv.Handler, http.MethodPut, "/metadata/v1/install/auth", "admin-key", map[string]any{
		"provisioning": map[string]any{
			"scimDefaultPermissionSetApiNames": []string{"DoesNotExistPS"},
		},
	})
	if rr.Code != 400 {
		t.Fatalf("invalid PS want 400 got %d %s", rr.Code, rr.Body.String())
	}
}

func TestJITClaimMappingAndDefaultPermissionSet(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	cleanupSocialHumans(t, d)

	users := db.NewUserStore(d.Pool)
	first, err := users.CreateSocialUser(t.Context(), "first-jit@example.com", "First", "StandardUser")
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
			Subject:       "google-sub-jit-map",
			Email:         "mapped@example.com",
			EmailVerified: true,
			Name:          "Mapped User",
			Extra:         map[string]string{"cost_center": "CC-JIT", "given_name": "Mapped"},
		},
	}
	broker := &authlogin.Broker{Providers: map[string]authlogin.Provider{
		identity.ProviderGoogle: fake,
	}}
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:     "admin-key+admin",
		EnableJWT:   true,
		LoginBroker: broker,
	})
	ts.Config.AuthLoginProviders = []string{"google"}
	ts.Config.AuthAutoProvisionUsers = true
	ts.Config.AuthAutoProvisionRole = "StandardUser"
	ts.Config.PlatformPublicURL = "http://one.test"

	integSvc := &integration.Service{Pool: d.Pool, Identity: identity.NopBackend{}, EncryptionKey: ts.Config.AuthJWTSigningKey}
	if err := integSvc.EnsureControlIDE(t.Context()); err != nil {
		t.Fatal(err)
	}

	h := ts.Handler
	stamp := time.Now().Format("150405.000")
	fieldName := "CostCenter" + strings.ReplaceAll(stamp, ".", "") + "__c"
	psName := "SalesUser" + strings.ReplaceAll(stamp, ".", "")

	rr := testutil.AuthRequest(h, http.MethodPost, "/metadata/v1/fields", "admin-key", map[string]any{
		"objectApiName": "User", "apiName": fieldName, "label": "Cost Center", "fieldType": "text",
	})
	if rr.Code != 201 {
		t.Fatalf("field %d %s", rr.Code, rr.Body.String())
	}
	rr = testutil.AuthRequest(h, http.MethodPost, "/metadata/v1/permissions/sets", "admin-key", map[string]any{
		"apiName": psName, "label": "Sales User",
	})
	if rr.Code != 201 {
		t.Fatalf("ps %d %s", rr.Code, rr.Body.String())
	}
	rr = testutil.AuthRequest(h, http.MethodPut, "/metadata/v1/install/auth", "admin-key", map[string]any{
		"jitProvisionUsers":   true,
		"jitDefaultRole":      "StandardUser",
		"allowedEmailDomains": []string{"example.com"},
		"provisioning": map[string]any{
			"jitDefaultPermissionSetApiNames": []string{psName},
			"claimMappings": []map[string]any{
				{"claim": "cost_center", "fieldApiName": fieldName},
				{"claim": "given_name", "fieldApiName": "GivenName"},
			},
		},
	})
	if rr.Code != 200 {
		t.Fatalf("put auth %d %s", rr.Code, rr.Body.String())
	}
	t.Cleanup(func() {
		testutil.AuthRequest(h, http.MethodPut, "/metadata/v1/install/auth", "admin-key", map[string]any{
			"provisioning": map[string]any{},
		})
	})

	token := socialLoginAccessToken(t, ts, "google")
	urow, err := users.GetByEmail(t.Context(), "mapped@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if urow.Data[fieldName] != "CC-JIT" {
		t.Fatalf("custom data=%v", urow.Data)
	}
	if urow.GivenName == nil || *urow.GivenName != "Mapped" {
		t.Fatalf("givenName=%v", urow.GivenName)
	}
	psNames, err := users.ListPermissionSetAPINames(t.Context(), urow.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundPS := false
	for _, n := range psNames {
		if n == psName {
			foundPS = true
		}
	}
	if !foundPS {
		t.Fatalf("expected PS %s, got %v", psName, psNames)
	}

	req := httptest.NewRequest(http.MethodGet, "/client/v1/principals", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	denied := httptest.NewRecorder()
	h.ServeHTTP(denied, req)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("JIT user principals admin want 403 got %d %s", denied.Code, denied.Body.String())
	}
}

func socialLoginAccessToken(t *testing.T, ts *testutil.TestServer, provider string) string {
	t.Helper()
	verifier, err := authlogin.RandomURLToken(32)
	if err != nil {
		t.Fatal(err)
	}
	challenge := authlogin.PKCEChallengeS256(verifier)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/v1/authorize?"+url.Values{
		"provider":              {provider},
		"client_id":             {integration.APINameControlIDE},
		"redirect_uri":          {integration.DefaultControlIDERedirectURI},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {"st"},
	}.Encode(), nil)
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("authorize %d %s", rr.Code, rr.Body.String())
	}
	loc, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/auth/v1/callback/"+provider+"?"+url.Values{
		"code": {"idp-code"}, "state": {loc.Query().Get("state")},
	}.Encode(), nil)
	ts.Handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusFound {
		t.Fatalf("callback %d %s", rr2.Code, rr2.Body.String())
	}
	redir, err := url.Parse(rr2.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
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
		t.Fatalf("token %d %s", rr3.Code, rr3.Body.String())
	}
	raw, _ := io.ReadAll(rr3.Body)
	var tok map[string]any
	_ = json.Unmarshal(raw, &tok)
	access, _ := tok["access_token"].(string)
	if access == "" {
		t.Fatalf("token body %s", raw)
	}
	return access
}
