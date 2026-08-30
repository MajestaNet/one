package metadata_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
)

func TestDescribeAndListObjects(t *testing.T) {
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

	_, _ = pool.Exec(ctx, `DELETE FROM metadata_fields WHERE object_api_name = 'GoAccount'`)
	_, _ = pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name = 'GoAccount'`)
	_, err = pool.Exec(ctx, `
INSERT INTO metadata_objects (api_name, label, plural_label, storage_mode, ownership, features)
VALUES ('GoAccount', 'Account', 'Accounts', 'flexible', 'custom', '{}'::jsonb)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
INSERT INTO metadata_fields (
  object_api_name, api_name, label, field_type, required, unique_field, ownership
) VALUES ('GoAccount', 'Name', 'Name', 'text', true, false, 'custom')`)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1, updated_at = now() WHERE id = 1`)

	svc := metadata.NewService(pool)
	global, err := svc.DescribeGlobal(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, sobj := range global.SObjects {
		if sobj.Name == "GoAccount" {
			found = true
			if !sobj.Custom || sobj.LabelPlural != "Accounts" {
				t.Fatalf("sobject=%+v", sobj)
			}
		}
	}
	if !found {
		t.Fatal("GoAccount missing from global describe")
	}

	desc, err := svc.Describe(ctx, "GoAccount")
	if err != nil {
		t.Fatal(err)
	}
	if len(desc.Fields) != 1 || desc.Fields[0].APIName != "Name" {
		t.Fatalf("fields=%+v", desc.Fields)
	}
	if desc.Limits.MaxFieldsPerObject != 2000 {
		t.Fatalf("limits=%+v", desc.Limits)
	}

	// Cached second describe
	desc2, err := svc.Describe(ctx, "GoAccount")
	if err != nil || desc2.APIName != "GoAccount" {
		t.Fatalf("cached describe: %v %+v", err, desc2)
	}

	objs, err := svc.ListObjects(ctx)
	if err != nil || len(objs) == 0 {
		t.Fatalf("list objects: %v len=%d", err, len(objs))
	}

	_, err = svc.Describe(ctx, "DoesNotExist__c")
	if !errors.Is(err, metadata.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCacheEpochInvalidation(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_ = pool.EnsureKernel(ctx)

	svc := metadata.NewService(pool)
	_, _ = svc.ListObjects(ctx)
	svc.CacheForTest().ExpireEpochCheck()
	_, _ = pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)
	// Next list should re-check epoch and succeed
	if _, err := svc.ListObjects(ctx); err != nil {
		t.Fatal(err)
	}
}
