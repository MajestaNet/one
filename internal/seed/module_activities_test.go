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

func TestActivitiesModuleRegistered(t *testing.T) {
	m, ok := packages.Get("activities")
	if !ok || !m.Optional {
		t.Fatalf("activities registry=%+v ok=%v", m, ok)
	}
	if len(m.Objects) != 4 {
		t.Fatalf("activities objects=%d want 4", len(m.Objects))
	}
}

func TestEnableActivitiesPackage(t *testing.T) {
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

	st, err := seed.EnablePackage(ctx, meta, "activities")
	if err != nil {
		t.Fatalf("enable activities: %v", err)
	}
	if !st.Enabled || st.InstalledVersion != seed.ActivitiesPackageVersion {
		t.Fatalf("activities status=%+v", st)
	}
	for _, api := range []string{"Task", "Appointment", "PhoneCall", "Email"} {
		obj, err := meta.GetObject(ctx, api)
		if err != nil {
			t.Fatalf("get %s: %v", api, err)
		}
		if obj.PackageName == nil || *obj.PackageName != "activities" {
			t.Fatalf("%s package=%v", api, obj.PackageName)
		}
		if _, err := meta.GetField(ctx, api, "RegardingAccountId"); err != nil {
			t.Fatalf("%s.RegardingAccountId: %v", api, err)
		}
		if _, err := meta.GetField(ctx, api, "RegardingContactId"); err != nil {
			t.Fatalf("%s.RegardingContactId: %v", api, err)
		}
	}
	if _, err := seed.DisablePackage(ctx, meta, "activities"); err != nil {
		t.Fatalf("disable: %v", err)
	}
}
