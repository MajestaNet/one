package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/testutil"
)

func TestToolAccessPermissionSetAndClientFilter(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:   "admin-key+admin",
		EnableJWT: true,
	})
	ctx := t.Context()
	suffix := time.Now().Format("150405000")
	allowTool := "ClientToolAllow" + suffix + "__c"
	denyTool := "ClientToolDeny" + suffix + "__c"
	psName := "ClientToolPS" + suffix
	email := "tool-client-" + suffix + "@example.com"

	cleanup := func() {
		_, _ = d.Pool.Exec(ctx, `DELETE FROM tool_permissions WHERE tool_api_name = ANY($1::text[])`, []string{allowTool, denyTool})
		_, _ = d.Pool.Exec(ctx, `DELETE FROM metadata_canvases WHERE api_name = ANY($1::text[])`, []string{allowTool, denyTool})
		_, _ = d.Pool.Exec(ctx, `DELETE FROM users WHERE email=$1`, email)
		_, _ = d.Pool.Exec(ctx, `DELETE FROM permission_sets WHERE api_name=$1`, psName)
	}
	cleanup()
	t.Cleanup(cleanup)

	createTool := func(apiName, label string) {
		rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/tools", "admin-key", map[string]any{
			"apiName":     apiName,
			"label":       label,
			"description": label + " description",
			"icon":        "table",
			"sortOrder":   10,
			"layout": map[string]any{
				"mode":     "sections",
				"sections": []map[string]any{{"id": "main", "nodeIds": []string{"stat-1"}}},
			},
			"nodes": []map[string]any{
				{"id": "stat-1", "kind": "stat", "props": map[string]any{"value": 1, "label": label}},
			},
			"dataBindings": []any{},
		})
		if rr.Code != http.StatusCreated {
			t.Fatalf("create tool %s: %d %s", apiName, rr.Code, rr.Body.String())
		}
	}
	createTool(allowTool, "Allowed Tool")
	createTool(denyTool, "Denied Tool")

	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/permissions/sets", "admin-key", map[string]any{
		"apiName": psName,
		"label":   "Client Tool PS",
		"toolAccess": map[string]any{
			"allTools": false,
			"tools": []map[string]any{
				{"apiName": allowTool, "canOpen": true, "canInteract": true, "canPublish": true},
				{"apiName": denyTool, "canOpen": false},
			},
		},
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create PS: %d %s", rr.Code, rr.Body.String())
	}
	var ps map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &ps)
	if ps["toolAccess"] == nil {
		t.Fatalf("missing toolAccess in create response: %v", ps)
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/principals", "admin-key", map[string]any{
		"email": email, "displayName": "Tool Client", "principalType": "service",
		"roleApiNames": []string{"StandardUser"}, "permissionSetApiNames": []string{psName},
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create principal: %d %s", rr.Code, rr.Body.String())
	}
	var principal map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &principal)
	pid, _ := principal["id"].(string)
	if pid == "" {
		t.Fatal("missing principal id")
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/principals/"+pid+"/credentials", "admin-key", map[string]any{
		"label": "tool-client",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("issue credential: %d %s", rr.Code, rr.Body.String())
	}
	var cred map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &cred)
	secret, _ := cred["clientSecret"].(string)
	if secret == "" {
		t.Fatal("missing clientSecret")
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/auth/v1/token", "", map[string]any{
		"grant_type": "client_credentials", "client_id": pid, "client_secret": secret,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("token: %d %s", rr.Code, rr.Body.String())
	}
	var tok map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &tok)
	access, _ := tok["access_token"].(string)
	if access == "" {
		t.Fatal("missing access token")
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/tools", access, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list tools: %d %s", rr.Code, rr.Body.String())
	}
	var list map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &list)
	tools, _ := list["tools"].([]any)
	seenAllow := false
	seenDeny := false
	for _, raw := range tools {
		m, _ := raw.(map[string]any)
		switch m["apiName"] {
		case allowTool:
			seenAllow = true
			if _, ok := m["document"]; ok {
				t.Fatalf("list should return chrome only: %v", m)
			}
		case denyTool:
			seenDeny = true
		}
	}
	if !seenAllow || seenDeny {
		t.Fatalf("client tool filter mismatch allow=%v deny=%v body=%s", seenAllow, seenDeny, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/tools/"+allowTool, access, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get allowed tool: %d %s", rr.Code, rr.Body.String())
	}
	var full map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &full)
	if full["document"] == nil {
		t.Fatalf("expected full ToolSpec document: %v", full)
	}
	permissions, _ := full["permissions"].(map[string]any)
	if permissions["canInteract"] != true || permissions["canPublish"] != true || permissions["canModify"] != false {
		t.Fatalf("unexpected client Tool permission matrix: %v", permissions)
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/tools/"+denyTool, access, nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("denied tool should be 404, got %d %s", rr.Code, rr.Body.String())
	}
}
