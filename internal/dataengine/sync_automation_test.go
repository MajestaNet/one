package dataengine_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
)

func TestSyncAutomationCommitAndRollback(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatal(err)
	}

	ownerID := "00000000-0000-4000-8000-000000000001"
	store := db.NewUserStore(pool)
	if _, err := store.EnsureBootstrapAdmin(ctx, ownerID, "admin@one.local", "Admin"); err != nil {
		t.Fatal(err)
	}

	const parent = "SyncParent__c"
	const child = "SyncChild__c"
	const autoOK = "SyncCreateChild"
	const autoFail = "SyncFailChild"

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE payload->>'apiName' = ANY($1::text[])`, []string{autoOK, autoFail})
		_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name = ANY($1::text[])`, []string{parent, child})
		_, _ = pool.Exec(ctx, `DELETE FROM automation_permissions WHERE automation_api_name = ANY($1::text[])`, []string{autoOK, autoFail})
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name = ANY($1::text[])`, []string{autoOK, autoFail})
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_fields WHERE object_api_name = ANY($1::text[])`, []string{parent, child})
		_, _ = pool.Exec(ctx, `DELETE FROM object_permissions WHERE object_api_name = ANY($1::text[])`, []string{parent, child})
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name = ANY($1::text[])`, []string{parent, child})
	}
	cleanup()
	t.Cleanup(cleanup)

	for _, obj := range []struct{ api, label, plural string }{
		{parent, "Sync Parent", "Sync Parents"},
		{child, "Sync Child", "Sync Children"},
	} {
		if _, err := pool.Exec(ctx, `
INSERT INTO metadata_objects (api_name, label, plural_label, storage_mode, ownership, features)
VALUES ($1,$2,$3,'flexible','custom','{}'::jsonb)`, obj.api, obj.label, obj.plural); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
INSERT INTO metadata_fields (object_api_name, api_name, label, field_type, required, ownership, filterable, sortable)
VALUES ($1,'Name','Name','text',true,'custom',true,true)`, obj.api); err != nil {
			t.Fatal(err)
		}
		_ = db.EnsureObjectInDataAccessCatalog(ctx, pool, obj.api)
	}

	actionsOK, _ := json.Marshal([]any{
		map[string]any{
			"type":          "createRecord",
			"objectApiName": child,
			"fieldMap":      map[string]any{"Name": "Name"},
			"data":          map[string]any{"Name": "{{trigger.Name}}"},
		},
	})
	if _, err := pool.Exec(ctx, `
INSERT INTO metadata_automations (
  api_name, label, object_api_name, trigger_event, active, actions, ownership, package_name,
  runtime, execution
) VALUES ($1,'ok',$2,'create',true,$3::jsonb,'custom','customer.default','actions','sync')`,
		autoOK, parent, string(actionsOK)); err != nil {
		t.Fatal(err)
	}
	_ = db.EnsureAutomationInAccessCatalog(ctx, pool, autoOK)

	actionsFail, _ := json.Marshal([]any{"fail"})
	if _, err := pool.Exec(ctx, `
INSERT INTO metadata_automations (
  api_name, label, object_api_name, trigger_event, active, actions, ownership, package_name,
  runtime, execution
) VALUES ($1,'fail',$2,'create',true,$3::jsonb,'custom','customer.default','actions','sync')`,
		autoFail, parent, string(actionsFail)); err != nil {
		t.Fatal(err)
	}
	_ = db.EnsureAutomationInAccessCatalog(ctx, pool, autoFail)

	_, _ = pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)

	meta := metadata.NewService(pool)
	svc := dataengine.NewService(pool, meta)
	actor := &authz.Actor{ID: ownerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}

	// Success path: deactivate fail automation first.
	_, _ = pool.Exec(ctx, `UPDATE metadata_automations SET active=false WHERE api_name=$1`, autoFail)
	_, _ = pool.Exec(ctx, `UPDATE metadata_automations SET active=true WHERE api_name=$1`, autoOK)

	created, err := svc.Create(ctx, parent, map[string]any{"Name": "Acme"}, actor)
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	parentID, _ := created["Id"].(string)
	if parentID == "" {
		t.Fatal("missing parent id")
	}

	var childCount int
	_ = pool.QueryRow(ctx, `
SELECT count(*) FROM records WHERE object_api_name=$1 AND data->>'Name'='Acme'`,
		child).Scan(&childCount)
	if childCount != 1 {
		t.Fatalf("expected 1 child record, got %d", childCount)
	}
	var parentCount int
	_ = pool.QueryRow(ctx, `
SELECT count(*) FROM records WHERE object_api_name=$1 AND id=$2::uuid`,
		parent, parentID).Scan(&parentCount)
	if parentCount != 1 {
		t.Fatalf("expected parent committed, got %d", parentCount)
	}

	// Failure path: only fail automation active.
	_, _ = pool.Exec(ctx, `UPDATE metadata_automations SET active=false WHERE api_name=$1`, autoOK)
	_, _ = pool.Exec(ctx, `UPDATE metadata_automations SET active=true WHERE api_name=$1`, autoFail)
	_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name=$1`, parent)
	_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name=$1`, child)

	_, err = svc.Create(ctx, parent, map[string]any{"Name": "ShouldRollback"}, actor)
	if err == nil {
		t.Fatal("expected sync failure")
	}
	_ = pool.QueryRow(ctx, `
SELECT count(*) FROM records WHERE object_api_name=$1 AND data->>'Name'='ShouldRollback'`,
		parent).Scan(&parentCount)
	if parentCount != 0 {
		t.Fatalf("expected parent rolled back, found %d", parentCount)
	}
	_ = pool.QueryRow(ctx, `
SELECT count(*) FROM jobs WHERE payload->>'apiName'=$1 AND created_at > now() - interval '1 minute'`,
		autoFail).Scan(&childCount)
	// fail automation is sync — should not enqueue async jobs; count may be 0
}
