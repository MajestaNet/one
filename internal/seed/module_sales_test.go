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

func TestSalesModuleRegistered(t *testing.T) {
	m, ok := packages.Get("sales")
	if !ok || !m.Optional || m.AutoEnable {
		t.Fatalf("sales registry=%+v ok=%v", m, ok)
	}
	if len(m.Objects) != 5 {
		t.Fatalf("sales objects=%d want 5", len(m.Objects))
	}
	for _, o := range m.Objects {
		switch o.APIName {
		case "Lead", "OpportunityLineItem", "Order", "Contract":
			t.Fatalf("sales must not include %s", o.APIName)
		}
	}
	deps := map[string]bool{}
	for _, d := range m.DependsOn {
		deps[d] = true
	}
	if !deps["core"] || !deps["catalog"] {
		t.Fatalf("sales DependsOn=%v", m.DependsOn)
	}
	if len(m.Actions) != 1 || m.Actions[0].APIName != "quote.accept" {
		t.Fatalf("sales actions=%+v", m.Actions)
	}
}

func TestEnableSalesPackage(t *testing.T) {
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

	if _, err := seed.EnablePackage(ctx, meta, "sales"); err == nil {
		t.Fatal("sales enable should fail without catalog")
	}
	if _, err := seed.EnablePackage(ctx, meta, "catalog"); err != nil {
		t.Fatalf("enable catalog: %v", err)
	}

	st, err := seed.EnablePackage(ctx, meta, "sales")
	if err != nil {
		t.Fatalf("enable sales: %v", err)
	}
	if !st.Enabled || st.InstalledVersion != seed.SalesPackageVersion {
		t.Fatalf("sales status=%+v", st)
	}
	// Service not enabled → bridge must stay off.
	bridge, err := seed.GetPackageStatus(ctx, meta, "crm_bridge")
	if err != nil {
		t.Fatal(err)
	}
	if bridge.Enabled {
		t.Fatal("crm_bridge must not auto-enable without service")
	}

	opp, err := meta.GetObject(ctx, "Opportunity")
	if err != nil || opp.PackageName == nil || *opp.PackageName != "sales" {
		t.Fatalf("Opportunity=%+v err=%v", opp, err)
	}
	pq, err := meta.GetField(ctx, "Opportunity", "PrimaryQuoteId")
	if err != nil || pq.ReferenceTo == nil || *pq.ReferenceTo != "Quote" {
		t.Fatalf("PrimaryQuoteId=%+v err=%v", pq, err)
	}
	ps, err := meta.GetField(ctx, "QuoteLine", "PriceSource")
	if err != nil || len(ps.PicklistValues) == 0 {
		t.Fatalf("PriceSource=%+v err=%v", ps, err)
	}
	if _, err := meta.GetField(ctx, "Quote", "TaxAmount"); err != nil {
		t.Fatalf("Quote.TaxAmount: %v", err)
	}
	if _, err := meta.GetField(ctx, "QuoteLine", "UnitId"); err != nil {
		t.Fatalf("QuoteLine.UnitId: %v", err)
	}
	comp, err := meta.GetObject(ctx, "Competitor")
	if err != nil || comp.PackageName == nil || *comp.PackageName != "sales" {
		t.Fatalf("Competitor=%+v err=%v", comp, err)
	}
	if _, err := meta.GetObject(ctx, "Lead"); err == nil {
		t.Fatal("Lead must not be seeded by sales")
	}

	if _, err := seed.DisablePackage(ctx, meta, "sales"); err != nil {
		t.Fatalf("disable sales: %v", err)
	}
	if _, err := seed.DisablePackage(ctx, meta, "catalog"); err != nil {
		t.Fatalf("disable catalog: %v", err)
	}
}
