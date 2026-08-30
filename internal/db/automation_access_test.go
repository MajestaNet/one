package db_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
)

func TestAutomationAccessCatalog(t *testing.T) {
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

	autoName := "AutoAccessCat__c"
	psName := "AutoAccessBuilder"
	_, _ = pool.Exec(ctx, `DELETE FROM automation_permissions WHERE automation_api_name=$1`, autoName)
	_, _ = pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
	_, _ = pool.Exec(ctx, `DELETE FROM permission_sets WHERE api_name=$1`, psName)

	_, _ = pool.Exec(ctx, `
INSERT INTO permission_sets (api_name, label, description, is_system, system_permissions, all_automations)
VALUES ('Admin', 'Admin', 'Full access', true, '[]'::jsonb, true)
ON CONFLICT (api_name) DO UPDATE SET all_automations = true`)

	var builderID string
	if err := pool.QueryRow(ctx, `
INSERT INTO permission_sets (api_name, label, description, is_system, system_permissions, all_automations)
VALUES ($1, 'Builder', 'test', false, '[]'::jsonb, false)
RETURNING id::text`, psName).Scan(&builderID); err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, `
INSERT INTO metadata_automations (api_name, label, object_api_name, trigger_event, active, actions, package_name, ownership)
VALUES ($1, 'Auto', 'Account', 'create', true, '[]'::jsonb, 'customer.default', 'custom')`, autoName)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureAutomationInAccessCatalog(ctx, pool, autoName); err != nil {
		t.Fatal(err)
	}

	var canRun bool
	err = pool.QueryRow(ctx, `
SELECT can_run FROM automation_permissions
WHERE permission_set_id=$1::uuid AND automation_api_name=$2`, builderID, autoName).Scan(&canRun)
	if err != nil {
		t.Fatalf("builder missing automation stub: %v", err)
	}
	if canRun {
		t.Fatal("expected builder automation stub can_run=false")
	}

	var adminID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM permission_sets WHERE api_name='Admin'`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `
SELECT can_run FROM automation_permissions
WHERE permission_set_id=$1::uuid AND automation_api_name=$2`, adminID, autoName).Scan(&canRun)
	if err != nil || !canRun {
		t.Fatalf("admin automation grant missing: err=%v canRun=%v", err, canRun)
	}

	if err := db.BackfillPermissionSetAutomationAccess(ctx, pool, builderID); err != nil {
		t.Fatal(err)
	}
	section, err := db.LoadAutomationAccessSection(ctx, pool, builderID)
	if err != nil {
		t.Fatal(err)
	}
	if section.AllAutomations {
		t.Fatal("builder should not have allAutomations")
	}
	found := false
	for _, e := range section.Automations {
		if e.APIName == autoName {
			found = true
			if e.CanRun {
				t.Fatal("expected canRun=false on builder")
			}
		}
	}
	if !found {
		t.Fatal("expected automation in LoadAutomationAccessSection")
	}

	if err := db.UpsertAutomationAccessEntries(ctx, pool, builderID, []db.AutomationAccessEntry{
		{APIName: autoName, CanRun: true},
	}); err != nil {
		t.Fatal(err)
	}

	store := &db.AutomationPermStore{Pool: pool}
	az := &authz.AutomationAuthz{Store: store}
	ok, err := az.ActorCanRunAutomation(ctx, &authz.Actor{
		ID: "u", PermissionSetIDs: []string{builderID},
	}, autoName)
	if err != nil || !ok {
		t.Fatalf("expected can run after grant, ok=%v err=%v", ok, err)
	}

	if err := db.SetPermissionSetAllAutomations(ctx, pool, builderID, true); err != nil {
		t.Fatal(err)
	}
	ok, err = az.ActorCanRunAutomation(ctx, &authz.Actor{
		ID: "u", PermissionSetIDs: []string{builderID},
	}, "MissingAuto")
	if err != nil || !ok {
		t.Fatalf("expected allAutomations grant, ok=%v err=%v", ok, err)
	}

	if err := db.RemoveAutomationFromAccessCatalog(ctx, pool, autoName); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM automation_permissions WHERE automation_api_name=$1`, autoName).Scan(&n)
	if n != 0 {
		t.Fatalf("expected catalog rows removed, got %d", n)
	}

	_, _ = pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
	_, _ = pool.Exec(ctx, `DELETE FROM permission_sets WHERE api_name=$1`, psName)
}
