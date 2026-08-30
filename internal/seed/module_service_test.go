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

func TestServiceModuleRegistered(t *testing.T) {
	m, ok := packages.Get("service")
	if !ok || !m.Optional || m.AutoEnable {
		t.Fatalf("service registry=%+v ok=%v", m, ok)
	}
	if len(m.Objects) != 7 {
		t.Fatalf("service objects=%d want 7", len(m.Objects))
	}
	deps := map[string]bool{}
	for _, d := range m.DependsOn {
		deps[d] = true
	}
	if !deps["core"] || !deps["catalog"] {
		t.Fatalf("service DependsOn=%v", m.DependsOn)
	}
}

func TestEnableServicePackage(t *testing.T) {
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

	if _, err := seed.EnablePackage(ctx, meta, "service"); err == nil {
		t.Fatal("service enable should fail without catalog")
	}
	if _, err := seed.EnablePackage(ctx, meta, "catalog"); err != nil {
		t.Fatalf("enable catalog: %v", err)
	}

	st, err := seed.EnablePackage(ctx, meta, "service")
	if err != nil {
		t.Fatalf("enable service: %v", err)
	}
	if !st.Enabled || st.InstalledVersion != seed.ServicePackageVersion {
		t.Fatalf("service status=%+v", st)
	}
	// Sales not enabled → bridge must stay off.
	bridge, err := seed.GetPackageStatus(ctx, meta, "crm_bridge")
	if err != nil {
		t.Fatal(err)
	}
	if bridge.Enabled {
		t.Fatal("crm_bridge must not auto-enable without sales")
	}

	if _, err := meta.GetObject(ctx, "Case"); err != nil {
		t.Fatalf("Case: %v", err)
	}
	assetProduct, err := meta.GetField(ctx, "Asset", "ProductId")
	if err != nil || assetProduct.ReferenceTo == nil || *assetProduct.ReferenceTo != "Product" {
		t.Fatalf("Asset.ProductId=%+v err=%v", assetProduct, err)
	}
	ccParent, err := meta.GetField(ctx, "CaseComment", "ParentId")
	if err != nil || ccParent.FieldType != "master_detail" {
		t.Fatalf("CaseComment.ParentId=%+v err=%v", ccParent, err)
	}

	if _, err := seed.DisablePackage(ctx, meta, "service"); err != nil {
		t.Fatalf("disable service: %v", err)
	}
	if _, err := seed.DisablePackage(ctx, meta, "catalog"); err != nil {
		t.Fatalf("disable catalog: %v", err)
	}
}
