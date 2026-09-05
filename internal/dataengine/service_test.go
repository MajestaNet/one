package dataengine_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
)

func TestCRUDAndQuery(t *testing.T) {
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

	const obj = "GoContact"
	_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name = $1`, obj)
	_, _ = pool.Exec(ctx, `DELETE FROM metadata_fields WHERE object_api_name = $1`, obj)
	_, _ = pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name = $1`, obj)
	_, err = pool.Exec(ctx, `
INSERT INTO metadata_objects (api_name, label, plural_label, storage_mode, ownership, features)
VALUES ($1, 'Contact', 'Contacts', 'flexible', 'custom', '{}'::jsonb)`, obj)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
INSERT INTO metadata_fields (
  object_api_name, api_name, label, field_type, required, unique_field, ownership, filterable, sortable, indexed
) VALUES
 ($1, 'Name', 'Name', 'text', true, false, 'custom', true, true, true),
 ($1, 'Score', 'Score', 'number', false, false, 'custom', true, true, false)`, obj)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)

	meta := metadata.NewService(pool)
	svc := dataengine.NewService(pool, meta)
	actor := &authz.Actor{ID: ownerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}

	created, err := svc.Create(ctx, obj, map[string]any{"Name": "Ada", "Score": 10}, actor)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := created["Id"].(string)
	if id == "" || created["Name"] != "Ada" {
		t.Fatalf("created=%v", created)
	}
	if created["OwnerId"] != ownerID {
		t.Fatalf("OwnerId should default to the creating actor, got %v", created["OwnerId"])
	}
	if created["CreatedById"] != ownerID {
		t.Fatalf("CreatedById=%v want %s", created["CreatedById"], ownerID)
	}
	if created["LastModifiedById"] != ownerID {
		t.Fatalf("LastModifiedById=%v want %s", created["LastModifiedById"], ownerID)
	}

	if _, err := svc.Create(ctx, obj, map[string]any{"Name": "Bad", "CreatedById": ownerID}, actor); err == nil {
		t.Fatal("expected reject CreatedById on create")
	}

	owned, err := svc.Create(ctx, obj, map[string]any{"Name": "Owned", "OwnerId": ownerID, "Score": 1}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if owned["OwnerId"] != ownerID {
		t.Fatalf("OwnerId not set: %v", owned["OwnerId"])
	}

	got, err := svc.Get(ctx, obj, id)
	if err != nil || got["Name"] != "Ada" {
		t.Fatalf("get=%v err=%v", got, err)
	}

	otherActor := &authz.Actor{ID: "00000000-0000-4000-8000-000000000099", IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}
	if _, err := store.EnsureBootstrapAdmin(ctx, otherActor.ID, "other@one.local", "Other"); err != nil {
		t.Fatal(err)
	}
	updated, err := svc.Update(ctx, obj, id, map[string]any{"Score": 42}, otherActor)
	if err != nil {
		t.Fatal(err)
	}
	if updated["Score"].(float64) != 42 {
		t.Fatalf("updated=%v", updated)
	}
	if updated["CreatedById"] != ownerID {
		t.Fatalf("CreatedById should be stable, got %v", updated["CreatedById"])
	}
	if updated["LastModifiedById"] != otherActor.ID {
		t.Fatalf("LastModifiedById=%v want %s", updated["LastModifiedById"], otherActor.ID)
	}

	// second record for query/filter
	_, err = svc.Create(ctx, obj, map[string]any{"Name": "Bob", "Score": 5}, actor)
	if err != nil {
		t.Fatal(err)
	}

	qraw, _ := json.Marshal(map[string]any{
		"object": obj,
		"filters": []map[string]any{
			{"field": "Name", "op": "eq", "value": "Ada"},
		},
		"limit": 10,
	})
	qr, err := svc.Query(ctx, qraw, dataengine.QueryVisibility{})
	if err != nil {
		t.Fatal(err)
	}
	if len(qr.Records) != 1 || qr.Records[0]["Name"] != "Ada" {
		t.Fatalf("query=%+v", qr)
	}

	likeRaw, _ := json.Marshal(map[string]any{
		"object":  obj,
		"filters": []map[string]any{{"field": "Name", "op": "like", "value": "a"}},
		"limit":   10,
	})
	likeRes, err := svc.Query(ctx, likeRaw, dataengine.QueryVisibility{})
	if err != nil || len(likeRes.Records) < 1 {
		t.Fatalf("like=%+v err=%v", likeRes, err)
	}

	if err := svc.Delete(ctx, obj, id, actor); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Get(ctx, obj, id)
	if !errors.Is(err, dataengine.ErrNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}

	var auditCount int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE object_api_name = $1 AND record_id = $2::uuid`, obj, id).Scan(&auditCount)
	if auditCount < 2 {
		t.Fatalf("expected audit rows, got %d", auditCount)
	}
	var outboxCount int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE object_api_name = $1 AND record_id = $2::uuid`, obj, id).Scan(&outboxCount)
	if outboxCount < 2 {
		t.Fatalf("expected outbox rows, got %d", outboxCount)
	}
}
