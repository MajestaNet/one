package db_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/db"
)

func TestDataAccessCatalogExpand(t *testing.T) {
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

	objName := "DataAccessCatObj__c"
	fieldName := "Amount__c"
	psName := "DataAccessBuilder"
	_, _ = pool.Exec(ctx, `DELETE FROM metadata_fields WHERE object_api_name=$1`, objName)
	_, _ = pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name=$1`, objName)
	_, _ = pool.Exec(ctx, `DELETE FROM permission_sets WHERE api_name=$1`, psName)

	// Ensure Admin exists for catalog grants.
	_, _ = pool.Exec(ctx, `
INSERT INTO permission_sets (api_name, label, description, is_system, system_permissions)
VALUES ('Admin', 'Admin', 'Full access', true, '[]'::jsonb)
ON CONFLICT (api_name) DO NOTHING`)

	var builderID string
	if err := pool.QueryRow(ctx, `
INSERT INTO permission_sets (api_name, label, description, is_system, system_permissions)
VALUES ($1, 'Builder', 'test', false, '[]'::jsonb)
RETURNING id::text`, psName).Scan(&builderID); err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, `
INSERT INTO metadata_objects (api_name, label, plural_label, storage_mode, package_name, ownership, features)
VALUES ($1, 'Obj', 'Objs', 'flexible', 'customer.default', 'custom', '{}'::jsonb)`, objName)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureObjectInDataAccessCatalog(ctx, pool, objName); err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, `
INSERT INTO metadata_fields (object_api_name, api_name, label, field_type, package_name, ownership)
VALUES ($1, $2, 'Amount', 'number', 'customer.default', 'custom')`, objName, fieldName)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureFieldInDataAccessCatalog(ctx, pool, objName, fieldName); err != nil {
		t.Fatal(err)
	}

	// Builder should have deny object stub.
	var canRead bool
	err = pool.QueryRow(ctx, `
SELECT can_read FROM object_permissions
WHERE permission_set_id=$1::uuid AND object_api_name=$2`, builderID, objName).Scan(&canRead)
	if err != nil {
		t.Fatalf("builder missing object stub: %v", err)
	}
	if canRead {
		t.Fatal("expected builder object stub can_read=false")
	}

	// Admin should have full object + field grant.
	var adminID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM permission_sets WHERE api_name='Admin'`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	var adminRead, adminCreate bool
	err = pool.QueryRow(ctx, `
SELECT can_read, can_create FROM object_permissions
WHERE permission_set_id=$1::uuid AND object_api_name=$2`, adminID, objName).Scan(&adminRead, &adminCreate)
	if err != nil || !adminRead || !adminCreate {
		t.Fatalf("admin object grant missing or incomplete: err=%v read=%v create=%v", err, adminRead, adminCreate)
	}
	var fieldCount int
	_ = pool.QueryRow(ctx, `
SELECT count(*) FROM field_permissions
WHERE permission_set_id=$1::uuid AND object_api_name=$2 AND field_api_name=$3`,
		adminID, objName, fieldName).Scan(&fieldCount)
	if fieldCount != 1 {
		t.Fatalf("expected Admin field row, got %d", fieldCount)
	}
	_ = pool.QueryRow(ctx, `
SELECT count(*) FROM field_permissions
WHERE permission_set_id=$1::uuid AND object_api_name=$2 AND field_api_name=$3`,
		builderID, objName, fieldName).Scan(&fieldCount)
	if fieldCount != 1 {
		t.Fatalf("non-Admin must get deny field stub, got %d", fieldCount)
	}
	var builderFieldRead, builderFieldEdit bool
	if err := pool.QueryRow(ctx, `
SELECT can_read, can_edit FROM field_permissions
WHERE permission_set_id=$1::uuid AND object_api_name=$2 AND field_api_name=$3`,
		builderID, objName, fieldName).Scan(&builderFieldRead, &builderFieldEdit); err != nil {
		t.Fatal(err)
	}
	if builderFieldRead || builderFieldEdit {
		t.Fatal("builder field stub must be deny")
	}

	if err := db.BackfillPermissionSetDataAccess(ctx, pool, builderID); err != nil {
		t.Fatal(err)
	}
	section, err := db.LoadDataAccessSection(ctx, pool, builderID)
	if err != nil {
		t.Fatal(err)
	}
	foundObj := false
	for _, op := range section.ObjectPermissions {
		if op.ObjectAPIName == objName {
			foundObj = true
			if op.CanRead || op.CanCreate {
				t.Fatal("expanded object stub should remain deny for builder")
			}
		}
	}
	if !foundObj {
		t.Fatal("dataAccess missing object row")
	}
	foundField := false
	for _, fp := range section.FieldPermissions {
		if fp.ObjectAPIName == objName && fp.FieldAPIName == fieldName {
			foundField = true
			if !fp.Configured {
				t.Fatal("stored field stub should be configured=true")
			}
			if fp.CanRead || fp.CanEdit {
				t.Fatal("deny stub should be canRead/canEdit false")
			}
		}
	}
	if !foundField {
		t.Fatal("dataAccess missing expanded field row")
	}

	if err := db.RemoveFieldFromDataAccessCatalog(ctx, pool, objName, fieldName); err != nil {
		t.Fatal(err)
	}
	if err := db.RemoveObjectFromDataAccessCatalog(ctx, pool, objName); err != nil {
		t.Fatal(err)
	}
}
