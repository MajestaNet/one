package db_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
)

func TestToolAccessCatalog(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatal(err)
	}

	toolName := "ToolAccessCat__c"
	psName := "ToolAccessBuilder"
	_, _ = pool.Exec(ctx, `DELETE FROM tool_permissions WHERE tool_api_name=$1`, toolName)
	_, _ = pool.Exec(ctx, `DELETE FROM metadata_canvases WHERE api_name=$1`, toolName)
	_, _ = pool.Exec(ctx, `DELETE FROM permission_sets WHERE api_name=$1`, psName)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM tool_permissions WHERE tool_api_name=$1`, toolName)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_canvases WHERE api_name=$1`, toolName)
		_, _ = pool.Exec(ctx, `DELETE FROM permission_sets WHERE api_name=$1`, psName)
	})

	_, _ = pool.Exec(ctx, `
INSERT INTO permission_sets (api_name, label, description, is_system, system_permissions, all_automations, all_tools)
VALUES ('Admin', 'Admin', 'Full access', true, '[]'::jsonb, true, true)
ON CONFLICT (api_name) DO UPDATE SET all_tools = true`)
	_, _ = pool.Exec(ctx, `
INSERT INTO permission_sets (api_name, label, description, is_system, system_permissions)
VALUES ('Operate', 'Operate', 'Operate', true, '[]'::jsonb)
ON CONFLICT (api_name) DO NOTHING`)

	var builderID string
	if err := pool.QueryRow(ctx, `
INSERT INTO permission_sets (api_name, label, description, is_system, system_permissions, all_automations, all_tools)
VALUES ($1, 'Builder', 'test', false, '[]'::jsonb, false, false)
RETURNING id::text`, psName).Scan(&builderID); err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, `
INSERT INTO metadata_canvases (api_name, label, description, layout, nodes, data_bindings, active, ownership, package_name)
VALUES ($1, 'Tool', 'Tool', '{}'::jsonb, '[]'::jsonb, '[]'::jsonb, true, 'managed', 'sales')`, toolName)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureToolInAccessCatalog(ctx, pool, toolName); err != nil {
		t.Fatal(err)
	}

	var canOpen bool
	err = pool.QueryRow(ctx, `
SELECT can_open FROM tool_permissions
WHERE permission_set_id=$1::uuid AND tool_api_name=$2`, builderID, toolName).Scan(&canOpen)
	if err != nil {
		t.Fatalf("builder missing tool stub: %v", err)
	}
	if canOpen {
		t.Fatal("expected builder tool stub can_open=false")
	}

	var operateCanOpen bool
	err = pool.QueryRow(ctx, `
SELECT tp.can_open FROM tool_permissions tp
JOIN permission_sets ps ON ps.id = tp.permission_set_id
WHERE ps.api_name='Operate' AND tp.tool_api_name=$1`, toolName).Scan(&operateCanOpen)
	if err != nil || !operateCanOpen {
		t.Fatalf("Operate sales/service grant missing: err=%v canOpen=%v", err, operateCanOpen)
	}

	var adminID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM permission_sets WHERE api_name='Admin'`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `
SELECT can_open FROM tool_permissions
WHERE permission_set_id=$1::uuid AND tool_api_name=$2`, adminID, toolName).Scan(&canOpen)
	if err != nil || !canOpen {
		t.Fatalf("admin tool grant missing: err=%v canOpen=%v", err, canOpen)
	}

	if err := db.BackfillPermissionSetToolAccess(ctx, pool, builderID); err != nil {
		t.Fatal(err)
	}
	section, err := db.LoadToolAccessSection(ctx, pool, builderID)
	if err != nil {
		t.Fatal(err)
	}
	if section.AllTools {
		t.Fatal("builder should not have allTools")
	}
	found := false
	for _, e := range section.Tools {
		if e.APIName == toolName {
			found = true
			if e.CanOpen {
				t.Fatal("expected canOpen=false on builder")
			}
		}
	}
	if !found {
		t.Fatal("expected tool in LoadToolAccessSection")
	}

	if err := db.UpsertToolAccessEntries(ctx, pool, builderID, []db.ToolAccessEntry{
		{APIName: toolName, CanOpen: true, CanInteract: true, CanPublish: true},
	}); err != nil {
		t.Fatal(err)
	}

	store := &db.ToolPermStore{Pool: pool}
	az := &authz.ToolAuthz{Store: store}
	ok, err := az.ActorCanOpenTool(ctx, &authz.Actor{
		ID: "u", PermissionSetIDs: []string{builderID},
	}, toolName)
	if err != nil || !ok {
		t.Fatalf("expected can open after grant, ok=%v err=%v", ok, err)
	}
	access, err := az.ActorToolAccess(ctx, &authz.Actor{ID: "u", PermissionSetIDs: []string{builderID}}, toolName)
	if err != nil || !access.CanInteract || !access.CanPublish || access.CanModify {
		t.Fatalf("unexpected fine-grained access: %+v err=%v", access, err)
	}

	if err := db.SetPermissionSetAllTools(ctx, pool, builderID, true); err != nil {
		t.Fatal(err)
	}
	ok, err = az.ActorCanOpenTool(ctx, &authz.Actor{
		ID: "u", PermissionSetIDs: []string{builderID},
	}, "MissingTool")
	if err != nil || !ok {
		t.Fatalf("expected allTools grant, ok=%v err=%v", ok, err)
	}

	if err := db.RemoveToolFromAccessCatalog(ctx, pool, toolName); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM tool_permissions WHERE tool_api_name=$1`, toolName).Scan(&n)
	if n != 0 {
		t.Fatalf("expected catalog rows removed, got %d", n)
	}
}
