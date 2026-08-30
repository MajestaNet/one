package metadata_test

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
)

func TestAssertCustomerMutable(t *testing.T) {
	if err := metadata.AssertCustomerMutable("custom", "X", "object"); err != nil {
		t.Fatal(err)
	}
	err := metadata.AssertCustomerMutable("managed", "Account", "object")
	if !errors.Is(err, metadata.ErrForbidden) {
		t.Fatalf("got %v", err)
	}
}

func TestInsertObjectValidation(t *testing.T) {
	svc := metadata.NewService(nil)
	_, err := svc.InsertObject(t.Context(), metadata.ObjectDefinition{}, metadata.CreateOptions{})
	if !errors.Is(err, metadata.ErrValidation) {
		t.Fatalf("missing fields: %v", err)
	}
	_, err = svc.InsertObject(t.Context(), metadata.ObjectDefinition{
		APIName:     "X__c",
		Label:       "X",
		StorageMode: "row",
	}, metadata.CreateOptions{})
	if !errors.Is(err, metadata.ErrValidation) {
		t.Fatalf("bad storageMode: %v", err)
	}
	if !strings.Contains(err.Error(), "unsupported storageMode") {
		t.Fatalf("message=%v", err)
	}
	_, err = svc.InsertObject(t.Context(), metadata.ObjectDefinition{
		APIName:     "KernelUser__c",
		Label:       "Kernel User",
		StorageMode: db.StorageModeKernel,
	}, metadata.CreateOptions{})
	if !errors.Is(err, metadata.ErrValidation) {
		t.Fatalf("customer kernel: %v", err)
	}
	if !strings.Contains(err.Error(), "managed-only") {
		t.Fatalf("kernel message=%v", err)
	}
}

func TestInsertObjectAndField(t *testing.T) {
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

	svc := metadata.NewService(pool)
	name := "WriteObj" + time.Now().Format("150405")
	obj, err := svc.InsertObject(ctx, metadata.ObjectDefinition{
		APIName: name,
		Label:   name,
	}, metadata.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if obj.APIName != name || obj.Ownership != "custom" {
		t.Fatalf("obj=%+v", obj)
	}

	field, err := svc.InsertField(ctx, metadata.FieldDefinition{
		ObjectAPIName: name,
		APIName:       "Title",
		Label:         "Title",
		FieldType:     "text",
	}, metadata.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if field.APIName != "Title" {
		t.Fatalf("field=%+v", field)
	}

	// Ensure Admin exists so catalog sync has a target; create a non-Admin PS beforehand.
	_, _ = pool.Exec(ctx, `
INSERT INTO permission_sets (api_name, label, is_system, system_permissions)
VALUES ('Admin', 'Admin', true, '[]'::jsonb) ON CONFLICT (api_name) DO NOTHING`)
	psName := "WritePS" + time.Now().Format("150405")
	var psID string
	if err := pool.QueryRow(ctx, `
INSERT INTO permission_sets (api_name, label, is_system, system_permissions)
VALUES ($1, $1, false, '[]'::jsonb) RETURNING id::text`, psName).Scan(&psID); err != nil {
		t.Fatal(err)
	}
	// Re-run catalog ensure as if the PS existed before object create (heal path).
	if err := db.EnsureObjectInDataAccessCatalog(ctx, pool, name); err != nil {
		t.Fatal(err)
	}
	var canRead bool
	if err := pool.QueryRow(ctx, `
SELECT can_read FROM object_permissions WHERE permission_set_id=$1::uuid AND object_api_name=$2`,
		psID, name).Scan(&canRead); err != nil {
		t.Fatalf("expected deny stub on non-Admin PS: %v", err)
	}
	if canRead {
		t.Fatal("non-Admin stub must be deny")
	}

	_, err = svc.InsertObject(ctx, metadata.ObjectDefinition{
		APIName: "Account",
		Label:   "Account",
	}, metadata.CreateOptions{Role: "managed"})
	if !errors.Is(err, metadata.ErrForbidden) {
		t.Fatalf("managed without package: %v", err)
	}
}

func TestSyncManagedAdditiveAndConflict(t *testing.T) {
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
	svc := metadata.NewService(pool)
	pkg := "core"
	objName := "SyncObj" + time.Now().Format("150405000")

	if err := svc.SyncObjectManaged(ctx, metadata.ObjectDefinition{
		APIName: objName, Label: "Sync Obj", PluralLabel: "Sync Objs",
		StorageMode: "flexible", PackageName: &pkg, Ownership: "managed",
		Features: map[string]bool{"history": true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SyncFieldManaged(ctx, metadata.FieldDefinition{
		ObjectAPIName: objName, APIName: "Name", Label: "Name", FieldType: "text",
		Required: true, PackageName: &pkg, Ownership: "managed",
	}); err != nil {
		t.Fatal(err)
	}

	// Additive: new field appears on second sync.
	if err := svc.SyncFieldManaged(ctx, metadata.FieldDefinition{
		ObjectAPIName: objName, APIName: "Code", Label: "Code", FieldType: "text",
		PackageName: &pkg, Ownership: "managed",
	}); err != nil {
		t.Fatal(err)
	}
	f, err := svc.GetField(ctx, objName, "Code")
	if err != nil || f.Ownership != "managed" {
		t.Fatalf("expected managed Code field, got %+v err=%v", f, err)
	}

	// Attribute sync on existing managed field.
	if err := svc.SyncFieldManaged(ctx, metadata.FieldDefinition{
		ObjectAPIName: objName, APIName: "Name", Label: "Display Name", FieldType: "text",
		Required: true, PackageName: &pkg, Ownership: "managed",
	}); err != nil {
		t.Fatal(err)
	}
	nameField, err := svc.GetField(ctx, objName, "Name")
	if err != nil || nameField.Label != "Display Name" {
		t.Fatalf("expected label update, got %+v err=%v", nameField, err)
	}

	// Object label sync.
	if err := svc.SyncObjectManaged(ctx, metadata.ObjectDefinition{
		APIName: objName, Label: "Synced Object", PluralLabel: "Synced Objects",
		StorageMode: "flexible", PackageName: &pkg, Ownership: "managed",
		Features: map[string]bool{"history": true},
	}); err != nil {
		t.Fatal(err)
	}
	obj, err := svc.GetObject(ctx, objName)
	if err != nil || obj.Label != "Synced Object" {
		t.Fatalf("expected object label sync, got %+v err=%v", obj, err)
	}

	if err := svc.RecordPackageInstall(ctx, "core", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	ver, err := svc.GetPackageInstallVersion(ctx, "core")
	if err != nil || ver != "1.0.0" {
		t.Fatalf("package version=%q err=%v", ver, err)
	}

	// Customer collision blocks managed sync.
	customerName := "CustomerClash" + time.Now().Format("150405000")
	if _, err := svc.InsertObject(ctx, metadata.ObjectDefinition{
		APIName: customerName, Label: "Customer Clash",
	}, metadata.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	err = svc.SyncObjectManaged(ctx, metadata.ObjectDefinition{
		APIName: customerName, Label: "Managed Clash", PluralLabel: "Managed Clashes",
		StorageMode: "flexible", PackageName: &pkg, Ownership: "managed",
	})
	if !errors.Is(err, metadata.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestCustomerMutateManagedForbidden(t *testing.T) {
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
	svc := metadata.NewService(pool)
	pkg := "core"
	objName := "ManagedMut" + time.Now().Format("150405000")
	if err := svc.SyncObjectManaged(ctx, metadata.ObjectDefinition{
		APIName: objName, Label: "Managed", PluralLabel: "Manageds",
		StorageMode: "flexible", PackageName: &pkg, Ownership: "managed",
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SyncFieldManaged(ctx, metadata.FieldDefinition{
		ObjectAPIName: objName, APIName: "Name", Label: "Name", FieldType: "text",
		PackageName: &pkg, Ownership: "managed",
	}); err != nil {
		t.Fatal(err)
	}

	_, err = svc.UpdateObject(ctx, objName, metadata.ObjectDefinition{Label: "Nope"})
	if !errors.Is(err, metadata.ErrForbidden) {
		t.Fatalf("update managed object: %v", err)
	}
	if err := svc.DeleteObject(ctx, objName); !errors.Is(err, metadata.ErrForbidden) {
		t.Fatalf("delete managed object: %v", err)
	}
	label := "Nope"
	_, err = svc.UpdateField(ctx, objName, "Name", metadata.FieldPatch{Label: &label})
	if !errors.Is(err, metadata.ErrForbidden) {
		t.Fatalf("update managed field: %v", err)
	}
	if err := svc.DeleteField(ctx, objName, "Name"); !errors.Is(err, metadata.ErrForbidden) {
		t.Fatalf("delete managed field: %v", err)
	}

	// Customer object/field mutations succeed.
	customer := "CustomerMut" + time.Now().Format("150405000")
	if _, err := svc.InsertObject(ctx, metadata.ObjectDefinition{
		APIName: customer, Label: "Customer", PluralLabel: "Customers",
	}, metadata.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	updated, err := svc.UpdateObject(ctx, customer, metadata.ObjectDefinition{Label: "Customer Updated"})
	if err != nil || updated.Label != "Customer Updated" {
		t.Fatalf("customer update: %+v err=%v", updated, err)
	}
	if _, err := svc.InsertField(ctx, metadata.FieldDefinition{
		ObjectAPIName: customer, APIName: "Title", Label: "Title", FieldType: "text",
	}, metadata.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	newLabel := "Title Updated"
	field, err := svc.UpdateField(ctx, customer, "Title", metadata.FieldPatch{Label: &newLabel})
	if err != nil || field.Label != "Title Updated" {
		t.Fatalf("customer field update: %+v err=%v", field, err)
	}
	if err := svc.DeleteField(ctx, customer, "Title"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteObject(ctx, customer); err != nil {
		t.Fatal(err)
	}
}
