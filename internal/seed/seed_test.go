package seed_test

import (
	"context"
	"os"
	"testing"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/integration"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/seed"
)

func TestBootstrapSeed(t *testing.T) {
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
	// Clear leftover optional module enablement / metadata from prior test runs on shared DB.
	_, _ = pool.Exec(ctx, `UPDATE package_installs SET enabled = false WHERE package_name <> 'core'`)
	purgeOptionalDomainMetadata(t, ctx, pool)

	meta := metadata.NewService(pool)
	if err := seed.Bootstrap(ctx, pool, meta, seed.Options{
		OwnerID:      "00000000-0000-4000-8000-000000000001",
		FeatureFlags: []string{"agents"},
		AutoSeed:     true,
	}); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	// Idempotent
	if err := seed.Bootstrap(ctx, pool, meta, seed.Options{
		OwnerID:      "00000000-0000-4000-8000-000000000001",
		FeatureFlags: []string{"agents"},
		AutoSeed:     true,
	}); err != nil {
		t.Fatalf("bootstrap second: %v", err)
	}

	objs, err := meta.ListObjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, o := range objs {
		names[o.APIName] = true
	}
	for _, want := range []string{"User", "Account", "Contact"} {
		if !names[want] {
			t.Fatalf("missing seeded object %s (have %v)", want, names)
		}
	}
	// Optional modules must not appear unless package_installs.enabled for that package.
	ver, enabled, err := meta.GetPackageInstall(ctx, "notes")
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatalf("notes should not be enabled after plain bootstrap (ver=%s); disable leftover state", ver)
	}
	for _, gone := range []string{"Product", "Invoice", "Lead", "Opportunity", "Activity", "PriceBook", "PriceList", "Order", "Payment", "Case", "Quote"} {
		if _, err := meta.GetObject(ctx, gone); err == nil {
			t.Fatalf("optional/legacy object %s should be absent after core-only bootstrap", gone)
		}
	}

	coreVer, err := meta.GetPackageInstallVersion(ctx, "core")
	if err != nil || coreVer != seed.CorePackageVersion {
		t.Fatalf("core package version=%q want=%s err=%v", coreVer, seed.CorePackageVersion, err)
	}

	agentsVer, agentsEnabled, err := meta.GetPackageInstall(ctx, "agents_starter")
	if err != nil || !agentsEnabled || agentsVer != seed.AgentsStarterPackageVersion {
		t.Fatalf("agents_starter should be always-on after bootstrap ver=%q enabled=%v err=%v", agentsVer, agentsEnabled, err)
	}

	var psCount int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM permission_sets WHERE api_name='Admin'`).Scan(&psCount)
	if psCount != 1 {
		t.Fatalf("Admin permission set count=%d", psCount)
	}

	var roleCount int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM roles WHERE api_name IN ('SystemAdmin','StandardUser')`).Scan(&roleCount)
	if roleCount != 2 {
		t.Fatalf("system roles count=%d want 2", roleCount)
	}
	var urCount int
	_ = pool.QueryRow(ctx, `
SELECT count(*) FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
WHERE ur.user_id = $1::uuid AND r.api_name = 'SystemAdmin'`, "00000000-0000-4000-8000-000000000001").Scan(&urCount)
	if urCount != 1 {
		t.Fatalf("bootstrap SystemAdmin assignment count=%d", urCount)
	}
	var pt string
	_ = pool.QueryRow(ctx, `SELECT principal_type FROM users WHERE id = $1::uuid`, "00000000-0000-4000-8000-000000000001").Scan(&pt)
	if pt != "service" {
		t.Fatalf("bootstrap principal_type=%q want service", pt)
	}

	acctID, err := meta.GetField(ctx, "Contact", "AccountId")
	if err != nil {
		t.Fatalf("Contact.AccountId: %v", err)
	}
	if acctID.Required {
		t.Fatal("Contact.AccountId must be optional")
	}

	name, err := meta.GetField(ctx, "Account", "Name")
	if err != nil {
		t.Fatalf("Account.Name: %v", err)
	}
	if !name.Searchable {
		t.Fatal("Account.Name must be searchable")
	}
	phone, err := meta.GetField(ctx, "Account", "Phone")
	if err != nil {
		t.Fatalf("Account.Phone: %v", err)
	}
	if !phone.Searchable || phone.Indexed {
		t.Fatalf("Account.Phone searchable=%v indexed=%v (searchable must not force btree indexed)", phone.Searchable, phone.Indexed)
	}

	// Core 2.0 additive managed fields.
	for _, check := range []struct{ obj, field string }{
		{"Account", "AccountNumber"},
		{"Account", "PrimaryContactId"},
		{"Account", "ParentAccountId"},
		{"Account", "BillingCity"},
		{"Contact", "JobTitle"},
		{"Contact", "MobilePhone"},
		{"Contact", "MailingCity"},
	} {
		if _, err := meta.GetField(ctx, check.obj, check.field); err != nil {
			t.Fatalf("core field %s.%s: %v", check.obj, check.field, err)
		}
	}
}

func TestBootstrapCoreAlwaysWhenAutoSeed(t *testing.T) {
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
	// No package feature flag — core model still installs.
	if err := seed.Bootstrap(ctx, pool, meta, seed.Options{
		OwnerID:      "00000000-0000-4000-8000-000000000001",
		FeatureFlags: []string{"agents"},
		AutoSeed:     true,
	}); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	for _, want := range []string{"User", "Account", "Contact"} {
		obj, err := meta.GetObject(ctx, want)
		if err != nil || obj.Ownership != "managed" {
			t.Fatalf("standard object %s: %+v err=%v", want, obj, err)
		}
		if obj.PackageName == nil || *obj.PackageName != "core" {
			t.Fatalf("object %s package=%v want core", want, obj.PackageName)
		}
		if want == "User" && obj.StorageMode != "kernel" {
			t.Fatalf("User storageMode=%s want kernel", obj.StorageMode)
		}
	}
	email, err := meta.GetField(ctx, "User", "Email")
	if err != nil || email.KernelColumn == nil || *email.KernelColumn != "email" {
		t.Fatalf("User.Email kernelColumn: %+v err=%v", email, err)
	}
}

func TestPackageMigrateAddsNewManagedField(t *testing.T) {
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
	if err := seed.InstallCore(ctx, meta); err != nil {
		t.Fatal(err)
	}

	pkg := "core"
	if err := meta.SyncFieldManaged(ctx, metadata.FieldDefinition{
		ObjectAPIName: "Account", APIName: "MigrateProbe", Label: "Migrate Probe",
		FieldType: "text", Length: intPtr(100), PackageName: &pkg, Ownership: "managed",
	}); err != nil {
		t.Fatal(err)
	}
	// Re-run InstallCore (simulates product restart) then ensure additive field remains
	// and product-defined fields stay synced.
	if err := seed.InstallCore(ctx, meta); err != nil {
		t.Fatal(err)
	}
	f, err := meta.GetField(ctx, "Account", "MigrateProbe")
	if err != nil || f.Ownership != "managed" {
		t.Fatalf("additive managed field missing: %+v err=%v", f, err)
	}
	name, err := meta.GetField(ctx, "Account", "Name")
	if err != nil || name.Label != "Name" {
		t.Fatalf("core managed field: %+v err=%v", name, err)
	}
}

func TestBootstrapSkipControlIDE(t *testing.T) {
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
	store := db.NewIntegrationStore(pool)
	if err := store.Delete(ctx, integration.APINameControlIDE); err != nil && err != db.ErrNotFound {
		t.Fatalf("delete leftover Control IDE app: %v", err)
	}
	meta := metadata.NewService(pool)
	if err := seed.Bootstrap(ctx, pool, meta, seed.Options{
		OwnerID:        "00000000-0000-4000-8000-000000000001",
		FeatureFlags:   []string{"agents"},
		AutoSeed:       true,
		SkipControlIDE: true,
	}); err != nil {
		t.Fatalf("bootstrap skip: %v", err)
	}
	if _, err := store.GetByAPIName(ctx, integration.APINameControlIDE); err == nil {
		t.Fatal("expected Control IDE app skipped")
	} else if err != db.ErrNotFound {
		t.Fatalf("GetByAPIName: %v", err)
	}
}

func intPtr(n int) *int { return &n }

func TestLegacyManagedObjectsDroppedByMigration(t *testing.T) {
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

	// Simulate a pre-core install that still has Lead (migration 0012 already ran on
	// EnsureKernel; re-apply the purge SQL to prove cleanup is idempotent and correct).
	_, err = pool.Exec(ctx, `
INSERT INTO metadata_objects (api_name, label, plural_label, storage_mode, package_name, ownership)
VALUES ('Lead', 'Lead', 'Leads', 'flexible', 'crm', 'managed')
ON CONFLICT (api_name) DO UPDATE SET package_name='crm', ownership='managed'`)
	if err != nil {
		t.Fatalf("insert legacy Lead: %v", err)
	}
	_, err = pool.Exec(ctx, `
INSERT INTO metadata_fields (object_api_name, api_name, label, field_type, ownership, package_name)
VALUES ('Lead', 'LastName', 'Last Name', 'text', 'managed', 'crm')
ON CONFLICT (object_api_name, api_name) DO NOTHING`)
	if err != nil {
		t.Fatalf("insert legacy Lead field: %v", err)
	}

	_, err = pool.Exec(ctx, `
DO $$
DECLARE
  legacy text[] := ARRAY[
    'Lead', 'Opportunity', 'Activity',
    'Product', 'PriceBook', 'Order', 'Invoice', 'Payment'
  ];
BEGIN
  DELETE FROM field_projections WHERE object_api_name = ANY (legacy);
  DELETE FROM field_permissions WHERE object_api_name = ANY (legacy);
  DELETE FROM object_permissions WHERE object_api_name = ANY (legacy);
  DELETE FROM metadata_validation_rules WHERE object_api_name = ANY (legacy);
  DELETE FROM metadata_automations WHERE object_api_name = ANY (legacy);
  DELETE FROM metadata_relationships
  WHERE from_object = ANY (legacy) OR to_object = ANY (legacy);
  DELETE FROM metadata_fields WHERE object_api_name = ANY (legacy);
  DELETE FROM metadata_objects WHERE api_name = ANY (legacy);
  DELETE FROM records WHERE object_api_name = ANY (legacy);
END $$`)
	if err != nil {
		t.Fatalf("purge legacy: %v", err)
	}

	meta := metadata.NewService(pool)
	if _, err := meta.GetObject(ctx, "Lead"); err == nil {
		t.Fatal("Lead should be deleted by legacy purge")
	}
}
