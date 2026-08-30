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

func TestBillingModuleRegistered(t *testing.T) {
	m, ok := packages.Get("billing")
	if !ok || !m.Optional || m.AutoEnable {
		t.Fatalf("billing registry=%+v ok=%v", m, ok)
	}
	if len(m.Objects) != 2 {
		t.Fatalf("billing objects=%d want 2", len(m.Objects))
	}
	deps := map[string]bool{}
	for _, d := range m.DependsOn {
		deps[d] = true
	}
	if !deps["catalog"] || !deps["sales"] {
		t.Fatalf("billing DependsOn=%v", m.DependsOn)
	}
	if len(m.FieldExtensions) != 1 || m.FieldExtensions[0].ObjectAPIName != "Quote" {
		t.Fatalf("billing FieldExtensions=%+v", m.FieldExtensions)
	}
	sales, ok := packages.Get("sales")
	if !ok {
		t.Fatal("sales missing")
	}
	for _, o := range sales.Objects {
		if o.APIName == "Order" {
			t.Fatal("sales must not include Order")
		}
	}
	foundAccept := false
	for _, a := range sales.Actions {
		if a.APIName == "quote.accept" && a.SyncSafe {
			foundAccept = true
		}
	}
	if !foundAccept {
		t.Fatalf("sales actions=%+v", sales.Actions)
	}
}

func TestEnableBillingPackage(t *testing.T) {
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

	if _, err := seed.EnablePackage(ctx, meta, "billing"); err == nil {
		t.Fatal("billing enable should fail without catalog and sales")
	}
	if _, err := seed.EnablePackage(ctx, meta, "catalog"); err != nil {
		t.Fatalf("enable catalog: %v", err)
	}
	if _, err := seed.EnablePackage(ctx, meta, "billing"); err == nil {
		t.Fatal("billing enable should fail without sales")
	}
	if _, err := seed.EnablePackage(ctx, meta, "sales"); err != nil {
		t.Fatalf("enable sales: %v", err)
	}

	st, err := seed.EnablePackage(ctx, meta, "billing")
	if err != nil {
		t.Fatalf("enable billing: %v", err)
	}
	if !st.Enabled || st.InstalledVersion != seed.BillingPackageVersion {
		t.Fatalf("billing status=%+v", st)
	}
	for _, api := range []string{"Order", "OrderLine"} {
		obj, err := meta.GetObject(ctx, api)
		if err != nil {
			t.Fatalf("get %s: %v", api, err)
		}
		if obj.Ownership != "managed" || obj.PackageName == nil || *obj.PackageName != "billing" {
			t.Fatalf("%s package=%v ownership=%s", api, obj.PackageName, obj.Ownership)
		}
	}
	on, err := meta.GetField(ctx, "Order", "OrderNumber")
	if err != nil || on.FieldType != "autonumber" {
		t.Fatalf("Order.OrderNumber=%+v err=%v", on, err)
	}
	if on.AutonumberFormat == nil || *on.AutonumberFormat != "ORD-{00000}" {
		t.Fatalf("OrderNumber format=%v", on.AutonumberFormat)
	}
	oid, err := meta.GetField(ctx, "Quote", "OrderId")
	if err != nil || oid.ReferenceTo == nil || *oid.ReferenceTo != "Order" {
		t.Fatalf("Quote.OrderId=%+v err=%v", oid, err)
	}
	if oid.PackageName == nil || *oid.PackageName != "billing" {
		t.Fatalf("Quote.OrderId package=%v", oid.PackageName)
	}
	if _, err := meta.GetField(ctx, "Quote", "BillingStreet"); err != nil {
		t.Fatalf("Quote.BillingStreet: %v", err)
	}
	if _, err := meta.GetField(ctx, "Product", "ProductType"); err != nil {
		t.Fatalf("Product.ProductType: %v", err)
	}

	if _, err := seed.EnablePackage(ctx, meta, "billing"); err != nil {
		t.Fatalf("idempotent enable: %v", err)
	}
	if _, err := seed.DisablePackage(ctx, meta, "billing"); err != nil {
		t.Fatalf("disable billing: %v", err)
	}
}
