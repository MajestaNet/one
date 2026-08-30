package seed_test

import (
	"context"
	"os"
	"testing"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/packages"
	"github.com/MajestaNet/ide/internal/seed"
)

func TestEnableNotesPackage(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	meta := metadata.NewService(pool)
	// Ensure notes starts disabled for this test's "before enable" assertion.
	_, _ = pool.Exec(ctx, `UPDATE package_installs SET enabled = false WHERE package_name = 'notes'`)
	_ = meta.DeleteObject(ctx, "Note") // may fail if fields exist; ignore

	if err := seed.Bootstrap(ctx, pool, meta, seed.Options{
		OwnerID:  "00000000-0000-4000-8000-000000000001",
		AutoSeed: true,
	}); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	// Notes must not be present until enabled (delete leftover managed metadata if prior run).
	if _, err := meta.GetObject(ctx, "Note"); err == nil {
		// Soft leftover from prior enable — remove via SQL for a clean enable path check.
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_fields WHERE object_api_name = 'Note'`)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name = 'Note'`)
		_, _ = pool.Exec(ctx, `DELETE FROM object_permissions WHERE object_api_name = 'Note'`)
		_, _ = pool.Exec(ctx, `DELETE FROM package_installs WHERE package_name = 'notes'`)
		meta.InvalidateCache()
	}
	if _, err := meta.GetObject(ctx, "Note"); err == nil {
		t.Fatal("Note should not exist before enable")
	}

	st, err := seed.EnablePackage(ctx, meta, "notes")
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !st.Enabled || st.InstalledVersion != seed.NotesPackageVersion {
		t.Fatalf("status=%+v", st)
	}

	// Idempotent enable
	st2, err := seed.EnablePackage(ctx, meta, "notes")
	if err != nil {
		t.Fatalf("enable second: %v", err)
	}
	if !st2.Enabled {
		t.Fatal("expected enabled after second enable")
	}

	obj, err := meta.GetObject(ctx, "Note")
	if err != nil {
		t.Fatal(err)
	}
	if obj.Ownership != "managed" || obj.PackageName == nil || *obj.PackageName != "notes" {
		t.Fatalf("Note ownership/package=%s %#v", obj.Ownership, obj.PackageName)
	}

	// Soft-disable
	st3, err := seed.DisablePackage(ctx, meta, "notes")
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if st3.Enabled {
		t.Fatal("expected soft-disabled")
	}
	// Metadata remains
	if _, err := meta.GetObject(ctx, "Note"); err != nil {
		t.Fatalf("Note metadata should remain after soft-disable: %v", err)
	}

	// MigrateEnabledModules should skip soft-disabled
	if err := seed.MigrateEnabledModules(ctx, meta); err != nil {
		t.Fatal(err)
	}

	// Re-enable and migrate, then soft-disable so shared DB does not keep notes enabled.
	if _, err := seed.EnablePackage(ctx, meta, "notes"); err != nil {
		t.Fatal(err)
	}
	if err := seed.MigrateEnabledModules(ctx, meta); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.DisablePackage(ctx, meta, "notes"); err != nil {
		t.Fatal(err)
	}

	// Cannot enable core via optional path
	if _, err := seed.EnablePackage(ctx, meta, "core"); err == nil {
		t.Fatal("expected error enabling core as optional")
	}

	// Customer apiName conflict check via managed package name registry.
	pkg := "notes"
	if !packages.IsManagedPackageName(&pkg) {
		t.Fatal("notes should be managed package name")
	}
}

func TestExportCustomerFieldOnManagedObject(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	meta := metadata.NewService(pool)
	if err := seed.Bootstrap(ctx, pool, meta, seed.Options{
		OwnerID:  "00000000-0000-4000-8000-000000000001",
		AutoSeed: true,
	}); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	fieldName := "Region__c"
	_, err = meta.InsertField(ctx, metadata.FieldDefinition{
		ObjectAPIName: "Account",
		APIName:       fieldName,
		Label:         "Region",
		FieldType:     "text",
		Filterable:    true,
		Sortable:      true,
	}, metadata.CreateOptions{})
	if err != nil {
		// Idempotent across test re-runs
		if _, getErr := meta.GetField(ctx, "Account", fieldName); getErr != nil {
			t.Fatalf("insert field: %v", err)
		}
	}

	snap, err := meta.ExportCustomerSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	switch fs := snap["fields"].(type) {
	case []map[string]any:
		for _, f := range fs {
			if f["apiName"] == fieldName && f["objectApiName"] == "Account" {
				found = true
			}
		}
	default:
		t.Fatalf("unexpected fields type %T", snap["fields"])
	}
	if !found {
		t.Fatalf("%s on Account missing from snapshot: %#v", fieldName, snap["fields"])
	}
}
