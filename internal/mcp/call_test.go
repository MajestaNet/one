package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/MajestaNet/ide/internal/actions"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/mcp"
	"github.com/MajestaNet/ide/internal/testutil"
)

type stubActions struct {
	out map[string]any
	err error
}

func (s stubActions) Invoke(context.Context, *authz.Actor, string, map[string]any) (map[string]any, error) {
	return s.out, s.err
}

func TestCallToolUnknownAndMissingArgs(t *testing.T) {
	actor := &authz.Actor{ID: "u1", Scopes: []authz.Scope{authz.ScopeClient}}
	_, err := mcp.CallTool(context.Background(), mcp.Deps{}, actor, "no_such_tool", nil)
	if !errors.Is(err, mcp.ErrNotFound) {
		t.Fatalf("unknown tool: %v", err)
	}
	_, err = mcp.CallTool(context.Background(), mcp.Deps{}, actor, "describe_object", map[string]any{})
	if err == nil {
		t.Fatal("expected object required")
	}
	_, err = mcp.CallTool(context.Background(), mcp.Deps{}, actor, "invoke_action", map[string]any{})
	if err == nil {
		t.Fatal("expected apiName required")
	}
	_, err = mcp.CallTool(context.Background(), mcp.Deps{}, actor, "invoke_action", map[string]any{"apiName": "lead.convert"})
	if err == nil {
		t.Fatal("expected actions unavailable")
	}
}

func TestInvokeActionStubAndForbiddenMapping(t *testing.T) {
	actor := &authz.Actor{ID: "u1", Scopes: []authz.Scope{authz.ScopeClient}}
	out, err := mcp.CallTool(context.Background(), mcp.Deps{
		Actions: stubActions{out: map[string]any{"ok": true}},
	}, actor, "invoke_action", map[string]any{"apiName": "lead.convert"})
	if err != nil {
		t.Fatal(err)
	}
	m, _ := out.(map[string]any)
	if m["ok"] != true {
		t.Fatalf("out=%v", out)
	}
	_, err = mcp.CallTool(context.Background(), mcp.Deps{
		Actions: stubActions{err: &actions.Error{Status: 403, Code: "FORBIDDEN", Message: "nope"}},
	}, actor, "invoke_action", map[string]any{"apiName": "lead.convert"})
	if !errors.Is(err, mcp.ErrForbidden) {
		t.Fatalf("mapped forbidden: %v", err)
	}
	_, err = mcp.CallTool(context.Background(), mcp.Deps{
		Actions: stubActions{err: &actions.Error{Status: 404, Code: "ACTION_NOT_FOUND", Message: "missing"}},
	}, actor, "invoke_action", map[string]any{"apiName": "nope"})
	if !errors.Is(err, mcp.ErrNotFound) {
		t.Fatalf("mapped not found: %v", err)
	}
}

func TestCallToolDescribeAndCreateRecord(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{APIKeys: "admin-key+admin"})
	actor := &authz.Actor{
		ID: testutil.DefaultOwnerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient, authz.ScopeMetadata},
	}
	deps := mcp.Deps{
		Meta:     d.Meta,
		Data:     srv.Data,
		Pool:     d.Pool,
		ObjectAz: &authz.ObjectAuthz{Store: &db.ObjectPermStore{Pool: d.Pool}},
		FieldAz:  &authz.FieldAuthz{Store: &db.FieldPermStore{Pool: d.Pool}},
		Actions:  stubActions{out: map[string]any{"invoked": true}},
	}
	if _, err := mcp.CallTool(t.Context(), deps, actor, "describe_global", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := mcp.CallTool(t.Context(), deps, actor, "describe_object", map[string]any{"object": "Account"}); err != nil {
		t.Fatal(err)
	}
	created, err := mcp.CallTool(t.Context(), deps, actor, "create_record", map[string]any{
		"object": "Account",
		"data":   map[string]any{"Name": "MCP Cover Co"},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(created)
	var rec map[string]any
	_ = json.Unmarshal(raw, &rec)
	id, _ := rec["Id"].(string)
	if id == "" {
		t.Fatalf("created=%s", raw)
	}
	if _, err := mcp.CallTool(t.Context(), deps, actor, "get_record", map[string]any{"object": "Account", "id": id}); err != nil {
		t.Fatal(err)
	}
	if _, err := mcp.CallTool(t.Context(), deps, actor, "invoke_action", map[string]any{"apiName": "lead.convert"}); err != nil {
		t.Fatal(err)
	}
	if _, err := mcp.CallTool(t.Context(), deps, actor, "get_object_metadata", map[string]any{"apiName": "Account"}); err != nil {
		t.Fatal(err)
	}

	updated, err := mcp.CallTool(t.Context(), deps, actor, "update_record", map[string]any{
		"object": "Account", "id": id, "data": map[string]any{"Name": "MCP Cover Co 2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	updRaw, _ := json.Marshal(updated)
	var upd map[string]any
	_ = json.Unmarshal(updRaw, &upd)
	if upd["Name"] != "MCP Cover Co 2" {
		t.Fatalf("updated=%s", updRaw)
	}

	queried, err := mcp.CallTool(t.Context(), deps, actor, "query", map[string]any{
		"object": "Account", "limit": 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	qmap, _ := queried.(map[string]any)
	if qmap["totalSize"] == nil {
		t.Fatalf("query=%v", queried)
	}

	if _, err := mcp.CallTool(t.Context(), deps, actor, "search", map[string]any{
		"q": "MCP Cover", "objects": []any{"Account"}, "limit": 10,
	}); err != nil {
		t.Fatal(err)
	}

	specs, err := mcp.CallTool(t.Context(), deps, actor, "list_agent_specs", nil)
	if err != nil {
		t.Fatal(err)
	}
	specMap, _ := specs.(map[string]any)
	if specMap["playbooks"] == nil {
		t.Fatalf("specs=%v", specs)
	}

	run, err := mcp.CallTool(t.Context(), deps, actor, "create_agent_run", map[string]any{
		"goal": "cover MCP agent run", "dryRun": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	runMap, _ := run.(map[string]any)
	runID, _ := runMap["id"].(string)
	if runID == "" {
		t.Fatalf("run=%v", run)
	}
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM jobs WHERE payload->>'runId'=$1`, runID)
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM agent_runs WHERE id=$1::uuid`, runID)
	})
	if _, err := mcp.CallTool(t.Context(), deps, actor, "get_agent_run", map[string]any{"id": runID}); err != nil {
		t.Fatal(err)
	}
}

func TestListToolsCatalog(t *testing.T) {
	tools := mcp.ListTools()
	want := map[string]bool{
		"describe_global": false, "query": false, "search": false, "update_record": false,
		"list_agent_specs": false, "create_agent_run": false, "get_agent_run": false,
	}
	for _, tool := range tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Fatalf("missing tool %s", name)
		}
	}
}
