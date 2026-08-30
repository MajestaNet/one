package dataengine_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
)

func TestUpsertGetAndDeleteByExternalID(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := t.Context()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatal(err)
	}
	ownerID := "00000000-0000-4000-8000-000000000001"
	store := db.NewUserStore(pool)
	if _, err := store.EnsureBootstrapAdmin(ctx, ownerID, "admin@one.local", "Admin"); err != nil {
		t.Fatal(err)
	}
	obj := "UpsertDE" + time.Now().Format("150405")
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name=$1`, obj)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_fields WHERE object_api_name=$1`, obj)
		_, _ = pool.Exec(ctx, `DELETE FROM object_permissions WHERE object_api_name=$1`, obj)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name=$1`, obj)
	})
	if _, err := pool.Exec(ctx, `
INSERT INTO metadata_objects (api_name, label, plural_label, storage_mode, ownership, features)
VALUES ($1, $1, $1, 'flexible', 'custom', '{}'::jsonb)`, obj); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO metadata_fields (object_api_name, api_name, label, field_type, required, unique_field, ownership, filterable, sortable, indexed, external_id)
VALUES
  ($1, 'Name', 'Name', 'text', true, false, 'custom', true, true, true, false),
  ($1, 'ERP_Id__c', 'ERP Id', 'text', false, true, 'custom', true, true, true, true)`, obj); err != nil {
		t.Fatal(err)
	}
	_ = db.EnsureObjectInDataAccessCatalog(ctx, pool, obj)
	_, _ = pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)

	svc := dataengine.NewService(pool, metadata.NewService(pool))
	actor := &authz.Actor{ID: ownerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}
	az := permissiveUpsertAuthz()

	if _, err := svc.GetByExternalID(ctx, obj, "ERP_Id__c", "missing"); err == nil {
		t.Fatal("expected missing record")
	}

	created, err := svc.Upsert(ctx, obj, "ERP_Id__c", "ERP-1", map[string]any{"Name": "First"}, actor, az)
	if err != nil || !created.Created {
		t.Fatalf("create: %+v err=%v", created, err)
	}
	got, err := svc.GetByExternalID(ctx, obj, "ERP_Id__c", "ERP-1")
	if err != nil || got["Name"] != "First" {
		t.Fatalf("get: %v %v", got, err)
	}
	updated, err := svc.Upsert(ctx, obj, "ERP_Id__c", "ERP-1", map[string]any{"Name": "Second"}, actor, az)
	if err != nil || updated.Created {
		t.Fatalf("update: %+v err=%v", updated, err)
	}
	if updated.Record["Name"] != "Second" {
		t.Fatalf("name=%v", updated.Record["Name"])
	}
	failingFLS := permissiveUpsertAuthz()
	failingFLS.StripUnreadableFields = func(context.Context, *authz.Actor, string, map[string]any) (map[string]any, error) {
		return nil, errors.New("permission store unavailable")
	}
	if _, err := svc.Upsert(ctx, obj, "ERP_Id__c", "ERP-DENIED", map[string]any{"Name": "Must Roll Back"}, actor, failingFLS); err == nil {
		t.Fatal("field visibility failure must fail the upsert")
	}
	if _, err := svc.GetByExternalID(ctx, obj, "ERP_Id__c", "ERP-DENIED"); err == nil {
		t.Fatal("failed field visibility evaluation must roll back the mutation")
	}
	if _, err := svc.Upsert(ctx, obj, "Name", "x", map[string]any{"Name": "x"}, actor, az); err == nil {
		t.Fatal("expected non-externalId field rejected")
	}
	if err := svc.DeleteByExternalID(ctx, obj, "ERP_Id__c", "ERP-1", actor); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetByExternalID(ctx, obj, "ERP_Id__c", "ERP-1"); err == nil {
		t.Fatal("expected deleted")
	}
}
