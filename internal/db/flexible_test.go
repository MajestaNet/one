package db_test

import (
	"os"
	"testing"

	"github.com/MajestaNet/ide/internal/db"
)

func TestEnsureFlexiblePartitionAttachesLeaf(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := t.Context()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatal(err)
	}

	obj := "CustomerScaleObj__c"
	_, _ = pool.Exec(ctx, `DELETE FROM flexible_objects WHERE object_api_name=$1`, obj)
	_, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS records_o_customerscaleobj__c`)

	if err := db.EnsureFlexiblePartition(ctx, pool, obj); err != nil {
		t.Fatalf("EnsureFlexiblePartition: %v", err)
	}
	var part string
	if err := pool.QueryRow(ctx, `
SELECT partition_name FROM flexible_objects WHERE object_api_name=$1`, obj).Scan(&part); err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if part == "" {
		t.Fatal("expected partition name")
	}
	// Idempotent
	if err := db.EnsureFlexiblePartition(ctx, pool, obj); err != nil {
		t.Fatalf("second EnsureFlexiblePartition: %v", err)
	}
}
