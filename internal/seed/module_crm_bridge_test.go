package seed_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/packages"
	"github.com/MajestaNet/ide/internal/seed"
)

func TestCrmBridgeModuleRegistered(t *testing.T) {
	m, ok := packages.Get("crm_bridge")
	if !ok || !m.Optional || !m.AutoEnable {
		t.Fatalf("crm_bridge should be optional AutoEnable; got %+v ok=%v", m, ok)
	}
	if len(m.Objects) != 0 {
		t.Fatal("crm_bridge should have no objects")
	}
	if len(m.FieldExtensions) != 1 || m.FieldExtensions[0].ObjectAPIName != "Case" {
		t.Fatalf("crm_bridge field extensions=%+v", m.FieldExtensions)
	}
}

func TestCrmBridgeAutoEnablesWhenSalesAndServiceEnabled(t *testing.T) {
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
	purgeAndInvalidate(t, ctx, pool, meta)

	if _, err := seed.EnablePackage(ctx, meta, "catalog"); err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if _, err := seed.EnablePackage(ctx, meta, "sales"); err != nil {
		t.Fatalf("sales: %v", err)
	}
	st, err := seed.GetPackageStatus(ctx, meta, "crm_bridge")
	if err != nil {
		t.Fatal(err)
	}
	if st.Enabled {
		t.Fatal("bridge should not enable with sales alone")
	}

	if _, err := seed.EnablePackage(ctx, meta, "service"); err != nil {
		t.Fatalf("service: %v", err)
	}
	st, err = seed.GetPackageStatus(ctx, meta, "crm_bridge")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Enabled || !st.AutoEnable || st.InstalledVersion != seed.CrmBridgePackageVersion {
		t.Fatalf("expected auto-enabled crm_bridge, got %+v", st)
	}

	caseOpp, err := meta.GetField(ctx, "Case", "OpportunityId")
	if err != nil {
		t.Fatalf("Case.OpportunityId: %v", err)
	}
	if caseOpp.PackageName == nil || *caseOpp.PackageName != "crm_bridge" {
		t.Fatalf("Case.OpportunityId package=%v", caseOpp.PackageName)
	}
	caseObj, err := meta.GetObject(ctx, "Case")
	if err != nil || caseObj.PackageName == nil || *caseObj.PackageName != "service" {
		t.Fatalf("Case package must remain service, got %+v err=%v", caseObj, err)
	}

	// Manual disable of auto-bridge is rejected.
	if _, err := seed.DisablePackage(ctx, meta, "crm_bridge"); err == nil {
		t.Fatal("expected error disabling auto-enable package")
	} else if !strings.Contains(err.Error(), "auto-enabled") {
		t.Fatalf("unexpected disable error: %v", err)
	}

	// Boot migrate keeps bridge enabled.
	if err := seed.MigrateEnabledModules(ctx, meta); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st, err = seed.GetPackageStatus(ctx, meta, "crm_bridge")
	if err != nil || !st.Enabled {
		t.Fatalf("bridge after migrate=%+v err=%v", st, err)
	}

	// Cleanup: disable parent clouds (bridge cannot be disabled directly).
	if _, err := seed.DisablePackage(ctx, meta, "sales"); err != nil {
		t.Fatalf("disable sales: %v", err)
	}
	if _, err := seed.DisablePackage(ctx, meta, "service"); err != nil {
		t.Fatalf("disable service: %v", err)
	}
	if _, err := seed.DisablePackage(ctx, meta, "catalog"); err != nil {
		t.Fatalf("disable catalog: %v", err)
	}
}
