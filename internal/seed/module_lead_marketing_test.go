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

func TestLeadMarketingModuleRegistered(t *testing.T) {
	m, ok := packages.Get("lead_marketing")
	if !ok || !m.Optional {
		t.Fatalf("lead_marketing registry=%+v ok=%v", m, ok)
	}
	if len(m.Objects) != 4 {
		t.Fatalf("lead_marketing objects=%d want 4", len(m.Objects))
	}
	if len(m.Actions) != 1 || m.Actions[0].APIName != "lead.convert" || !m.Actions[0].SyncSafe {
		t.Fatalf("lead_marketing actions=%+v", m.Actions)
	}
}

func TestEnableLeadMarketingPackage(t *testing.T) {
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

	// Lead must not exist until lead_marketing is enabled.
	if _, err := meta.GetObject(ctx, "Lead"); err == nil {
		t.Fatal("Lead should be absent before lead_marketing enable")
	}

	st, err := seed.EnablePackage(ctx, meta, "lead_marketing")
	if err != nil {
		t.Fatalf("enable lead_marketing: %v", err)
	}
	if !st.Enabled || st.InstalledVersion != seed.LeadMarketingPackageVersion {
		t.Fatalf("lead_marketing status=%+v", st)
	}
	for _, api := range []string{"Lead", "Campaign", "MarketingList", "MarketingListMember"} {
		obj, err := meta.GetObject(ctx, api)
		if err != nil {
			t.Fatalf("get %s: %v", api, err)
		}
		if obj.PackageName == nil || *obj.PackageName != "lead_marketing" {
			t.Fatalf("%s package=%v", api, obj.PackageName)
		}
	}
	if _, err := seed.DisablePackage(ctx, meta, "lead_marketing"); err != nil {
		t.Fatalf("disable: %v", err)
	}
}
