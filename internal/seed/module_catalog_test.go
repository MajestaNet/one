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

func TestCatalogModuleRegistered(t *testing.T) {
	m, ok := packages.Get("catalog")
	if !ok || !m.Optional || m.AutoEnable {
		t.Fatalf("catalog registry=%+v ok=%v", m, ok)
	}
	if len(m.Objects) != 5 {
		t.Fatalf("catalog objects=%d want 5", len(m.Objects))
	}
	name := "catalog"
	if !packages.IsManagedPackageName(&name) {
		t.Fatal("catalog should be managed")
	}
}

func TestEnableCatalogPackage(t *testing.T) {
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

	st, err := seed.EnablePackage(ctx, meta, "catalog")
	if err != nil {
		t.Fatalf("enable catalog: %v", err)
	}
	if !st.Enabled || st.InstalledVersion != seed.CatalogPackageVersion {
		t.Fatalf("catalog status=%+v", st)
	}
	for _, api := range []string{"Product", "PriceList", "PriceListEntry", "Unit", "UnitGroup"} {
		obj, err := meta.GetObject(ctx, api)
		if err != nil {
			t.Fatalf("get %s: %v", api, err)
		}
		if obj.Ownership != "managed" || obj.PackageName == nil || *obj.PackageName != "catalog" {
			t.Fatalf("%s package=%v ownership=%s", api, obj.PackageName, obj.Ownership)
		}
	}
	if _, err := meta.GetField(ctx, "Product", "StockKeepingUnit"); err != nil {
		t.Fatalf("Product.StockKeepingUnit: %v", err)
	}
	if _, err := meta.GetField(ctx, "Product", "ProductType"); err != nil {
		t.Fatalf("Product.ProductType: %v", err)
	}
	if _, err := meta.GetField(ctx, "PriceListEntry", "UnitId"); err != nil {
		t.Fatalf("PriceListEntry.UnitId: %v", err)
	}
	ple, err := meta.GetField(ctx, "PriceListEntry", "PriceListId")
	if err != nil || ple.FieldType != "master_detail" {
		t.Fatalf("PriceListEntry.PriceListId=%+v err=%v", ple, err)
	}

	if _, err := seed.EnablePackage(ctx, meta, "catalog"); err != nil {
		t.Fatalf("idempotent enable: %v", err)
	}
	if _, err := seed.DisablePackage(ctx, meta, "catalog"); err != nil {
		t.Fatalf("disable catalog: %v", err)
	}
}
