package httpapi_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/testutil"
)

func TestDirectoryTagsAndSCIMGroups(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:   "admin-key+admin,meta-key:metadata",
		EnableJWT: true,
	})
	h := srv.Handler
	stamp := time.Now().Format("150405.000")
	auth := func(method, path, bearer string, body any) *httptest.ResponseRecorder {
		return testutil.AuthRequest(h, method, path, bearer, body)
	}

	rr := auth(http.MethodGet, "/scim/v2/ResourceTypes", "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("resource types %d %s", rr.Code, rr.Body.String())
	}
	var types map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &types)
	if int(types["totalResults"].(float64)) != 2 {
		t.Fatalf("expected 2 resource types: %s", rr.Body.String())
	}

	rr = auth(http.MethodGet, "/scim/v2/ServiceProviderConfig", "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("spc %d %s", rr.Code, rr.Body.String())
	}
	var spc map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &spc)
	bulk, _ := spc["bulk"].(map[string]any)
	if bulk["supported"] != false {
		t.Fatalf("bulk.supported=%v", bulk["supported"])
	}

	rr = auth(http.MethodPost, "/scim/v2/Groups", "meta-key", map[string]any{
		"schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:Group"},
		"displayName": "Nope",
	})
	if rr.Code != 403 {
		t.Fatalf("metadata Groups %d %s", rr.Code, rr.Body.String())
	}

	humanEmail := "dir-human-" + stamp + "@example.com"
	rr = auth(http.MethodPost, "/client/v1/principals", "admin-key", map[string]any{
		"email": humanEmail, "displayName": "Directory Human", "principalType": "user",
		"roleApiNames": []string{"StandardUser"},
	})
	if rr.Code != 201 {
		t.Fatalf("create human %d %s", rr.Code, rr.Body.String())
	}
	var human map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &human)
	humanID, _ := human["id"].(string)

	svcEmail := "dir-svc-" + stamp + "@example.com"
	rr = auth(http.MethodPost, "/client/v1/principals", "admin-key", map[string]any{
		"email": svcEmail, "displayName": "Directory Service", "principalType": "service",
		"roleApiNames": []string{"StandardUser"},
	})
	if rr.Code != 201 {
		t.Fatalf("create service %d %s", rr.Code, rr.Body.String())
	}
	var svc map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &svc)
	svcID, _ := svc["id"].(string)

	rr = auth(http.MethodPost, "/client/v1/principals/"+svcID+"/credentials", "admin-key", map[string]any{"label": "dir-svc"})
	if rr.Code != 201 {
		t.Fatalf("svc cred %d %s", rr.Code, rr.Body.String())
	}
	var svcCred map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &svcCred)
	svcSecret, _ := svcCred["clientSecret"].(string)

	preTok := mintClientCredentials(t, h, svcID, svcSecret)
	preScopes := jwtScopes(t, preTok)

	rr = auth(http.MethodPost, "/client/v1/sobjects/Account", preTok, map[string]any{"Name": "Denied " + stamp})
	if rr.Code != 403 {
		t.Fatalf("expected Account 403 before tag, got %d %s", rr.Code, rr.Body.String())
	}

	display := "Sales " + stamp
	rr = auth(http.MethodPost, "/scim/v2/Groups", "admin-key", map[string]any{
		"schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:Group"},
		"displayName": display,
		"externalId":  "okta-grp-" + stamp,
		"members":     []map[string]any{{"value": humanID, "type": "User"}},
	})
	if rr.Code != 201 {
		t.Fatalf("scim group create %d %s", rr.Code, rr.Body.String())
	}
	var createdGroup map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &createdGroup)
	groupID, _ := createdGroup["id"].(string)
	if groupID == "" {
		t.Fatal("missing group id")
	}

	rr = auth(http.MethodPost, "/scim/v2/Groups", "admin-key", map[string]any{
		"schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:Group"},
		"displayName": display,
	})
	if rr.Code != 409 {
		t.Fatalf("duplicate displayName %d %s", rr.Code, rr.Body.String())
	}

	rr = auth(http.MethodPatch, "/scim/v2/Groups/"+groupID, "admin-key", map[string]any{
		"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": []map[string]any{
			{"op": "add", "path": "members", "value": map[string]any{"value": svcID, "type": "User"}},
		},
	})
	if rr.Code != 200 {
		t.Fatalf("patch add member %d %s", rr.Code, rr.Body.String())
	}

	rr = auth(http.MethodGet, "/client/v1/principals/"+svcID, "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("get principal %d %s", rr.Code, rr.Body.String())
	}
	var tagged map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &tagged)
	tagNames, _ := tagged["directoryTagApiNames"].([]any)
	if len(tagNames) != 1 {
		t.Fatalf("directoryTagApiNames=%v body=%s", tagged["directoryTagApiNames"], rr.Body.String())
	}

	postTok := mintClientCredentials(t, h, svcID, svcSecret)
	postScopes := jwtScopes(t, postTok)
	if strings.Join(preScopes, ",") != strings.Join(postScopes, ",") {
		t.Fatalf("scopes changed by membership pre=%v post=%v", preScopes, postScopes)
	}
	rr = auth(http.MethodPost, "/client/v1/sobjects/Account", postTok, map[string]any{"Name": "Still Denied " + stamp})
	if rr.Code != 403 {
		t.Fatalf("expected Account 403 after tag, got %d %s", rr.Code, rr.Body.String())
	}

	rr = auth(http.MethodPatch, "/scim/v2/Users/"+humanID, "admin-key", map[string]any{
		"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": []map[string]any{
			{"op": "add", "path": "groups", "value": []any{}},
		},
	})
	if rr.Code != 400 {
		t.Fatalf("patch user groups %d %s", rr.Code, rr.Body.String())
	}
	var scimErr map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &scimErr)
	if scimErr["scimType"] != "invalidValue" {
		t.Fatalf("expected invalidValue: %s", rr.Body.String())
	}

	rr = auth(http.MethodGet, "/scim/v2/Users/"+svcID, "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("scim get user %d %s", rr.Code, rr.Body.String())
	}
	var scimUser map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &scimUser)
	groups, _ := scimUser["groups"].([]any)
	if len(groups) != 1 {
		t.Fatalf("user.groups=%v", scimUser["groups"])
	}

	rr = auth(http.MethodPatch, "/scim/v2/Groups/"+groupID, "admin-key", map[string]any{
		"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": []map[string]any{
			{"op": "remove", "path": `members[value eq "` + svcID + `"]`},
		},
	})
	if rr.Code != 200 {
		t.Fatalf("patch remove member %d %s", rr.Code, rr.Body.String())
	}

	psName := "DirUsers" + strings.ReplaceAll(stamp, ".", "")
	rr = auth(http.MethodPost, "/metadata/v1/permissions/sets", "admin-key", map[string]any{
		"apiName": psName, "label": "Directory Users Only",
		"systemPermissions": []string{"identity.users"},
	})
	if rr.Code != 201 {
		t.Fatalf("create PS %d %s", rr.Code, rr.Body.String())
	}
	viewerEmail := "dir-viewer-" + stamp + "@example.com"
	rr = auth(http.MethodPost, "/client/v1/principals", "admin-key", map[string]any{
		"email": viewerEmail, "displayName": "Dir Viewer", "principalType": "service",
		"roleApiNames": []string{"StandardUser"}, "permissionSetApiNames": []string{psName},
	})
	if rr.Code != 201 {
		t.Fatalf("create viewer %d %s", rr.Code, rr.Body.String())
	}
	var viewer map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &viewer)
	viewerID, _ := viewer["id"].(string)
	rr = auth(http.MethodPost, "/client/v1/principals/"+viewerID+"/credentials", "admin-key", map[string]any{"label": "viewer"})
	if rr.Code != 201 {
		t.Fatalf("viewer cred %d %s", rr.Code, rr.Body.String())
	}
	var viewerCred map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &viewerCred)
	viewerTok := mintClientCredentials(t, h, viewerID, viewerCred["clientSecret"].(string))

	rr = auth(http.MethodPatch, "/scim/v2/Groups/"+groupID, "admin-key", map[string]any{
		"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": []map[string]any{
			{"op": "add", "path": "members", "value": map[string]any{"value": svcID, "type": "User"}},
		},
	})
	if rr.Code != 200 {
		t.Fatalf("re-add service member %d %s", rr.Code, rr.Body.String())
	}

	rr = auth(http.MethodGet, "/scim/v2/Groups/"+groupID, viewerTok, nil)
	if rr.Code != 200 {
		t.Fatalf("viewer get group %d %s", rr.Code, rr.Body.String())
	}
	var viewed map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &viewed)
	members, _ := viewed["members"].([]any)
	for _, raw := range members {
		m, _ := raw.(map[string]any)
		if m["value"] == svcID {
			t.Fatalf("identity.users viewer must not see service member: %s", rr.Body.String())
		}
	}
	sawHuman := false
	for _, raw := range members {
		m, _ := raw.(map[string]any)
		if m["value"] == humanID {
			sawHuman = true
		}
	}
	if !sawHuman {
		t.Fatalf("expected human member visible: %s", rr.Body.String())
	}

	rr = auth(http.MethodPost, "/client/v1/directory-tags", "admin-key", map[string]any{
		"label": "Client Tag " + stamp, "apiName": "ClientTag" + strings.ReplaceAll(stamp, ".", ""),
	})
	if rr.Code != 201 {
		t.Fatalf("client tag create %d %s", rr.Code, rr.Body.String())
	}
	var clientTag map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &clientTag)
	apiName, _ := clientTag["apiName"].(string)
	rr = auth(http.MethodPost, "/client/v1/directory-tags/assign", "admin-key", map[string]any{
		"principalId": humanID, "tagApiName": apiName,
	})
	if rr.Code != 204 {
		t.Fatalf("assign %d %s", rr.Code, rr.Body.String())
	}
	rr = auth(http.MethodGet, "/client/v1/principals/"+humanID, "admin-key", nil)
	if rr.Code != 200 {
		t.Fatalf("get tagged human %d %s", rr.Code, rr.Body.String())
	}

	// Group representations are capped at 200 members, but a metadata-only
	// PATCH must preserve the complete backing membership set.
	bulkDisplay := "Bulk Group " + stamp
	rr = auth(http.MethodPost, "/scim/v2/Groups", "admin-key", map[string]any{
		"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:Group"}, "displayName": bulkDisplay,
	})
	if rr.Code != 201 {
		t.Fatalf("bulk group create %d %s", rr.Code, rr.Body.String())
	}
	var bulkGroup map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &bulkGroup)
	bulkGroupID, _ := bulkGroup["id"].(string)
	bulkPrefix := "dir-bulk-" + strings.ReplaceAll(stamp, ".", "")
	if _, err := d.Pool.Exec(t.Context(), `
WITH inserted AS (
  INSERT INTO users (email, display_name, principal_type)
  SELECT $1 || '-' || n::text || '@example.com', 'Bulk Member ' || n::text, 'user'
  FROM generate_series(1, 205) AS n
  RETURNING id
)
INSERT INTO user_directory_tags (user_id, tag_id)
SELECT id, $2::uuid FROM inserted`, bulkPrefix, bulkGroupID); err != nil {
		t.Fatal(err)
	}
	rr = auth(http.MethodPatch, "/scim/v2/Groups/"+bulkGroupID, "admin-key", map[string]any{
		"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": []map[string]any{
			{"op": "replace", "path": "displayName", "value": bulkDisplay + " Updated"},
		},
	})
	if rr.Code != 200 {
		t.Fatalf("bulk group patch %d %s", rr.Code, rr.Body.String())
	}
	var memberCount int
	if err := d.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM user_directory_tags WHERE tag_id=$1::uuid`, bulkGroupID).Scan(&memberCount); err != nil {
		t.Fatal(err)
	}
	if memberCount != 205 {
		t.Fatalf("metadata-only SCIM patch retained %d members, want 205", memberCount)
	}
}

func mintClientCredentials(t *testing.T, h http.Handler, clientID, secret string) string {
	t.Helper()
	rr := testutil.AuthRequest(h, http.MethodPost, "/auth/v1/token", "", map[string]any{
		"grant_type": "client_credentials", "client_id": clientID, "client_secret": secret,
	})
	if rr.Code != 200 {
		t.Fatalf("token %d %s", rr.Code, rr.Body.String())
	}
	var tok map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &tok)
	access, _ := tok["access_token"].(string)
	if access == "" {
		t.Fatal("missing access_token")
	}
	return access
}

func jwtScopes(t *testing.T, token string) []string {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		t.Fatalf("not a jwt: %s", token)
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims struct {
		Scopes []string `json:"scopes"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatal(err)
	}
	return claims.Scopes
}
