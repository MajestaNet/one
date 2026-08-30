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

func TestAddressModuleRegistered(t *testing.T) {
	m, ok := packages.Get("address")
	if !ok || !m.Optional {
		t.Fatalf("address registry=%+v ok=%v", m, ok)
	}
	if len(m.Objects) != 1 || m.Objects[0].APIName != "Address" {
		t.Fatalf("address objects=%+v", m.Objects)
	}
}

func TestEnableAddressPackage(t *testing.T) {
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

	st, err := seed.EnablePackage(ctx, meta, "address")
	if err != nil {
		t.Fatalf("enable address: %v", err)
	}
	if !st.Enabled || st.InstalledVersion != seed.AddressPackageVersion {
		t.Fatalf("address status=%+v", st)
	}
	obj, err := meta.GetObject(ctx, "Address")
	if err != nil {
		t.Fatal(err)
	}
	if obj.PackageName == nil || *obj.PackageName != "address" {
		t.Fatalf("Address package=%v", obj.PackageName)
	}
	for _, f := range []string{"AccountId", "ContactId", "AddressType", "IsPrimary"} {
		if _, err := meta.GetField(ctx, "Address", f); err != nil {
			t.Fatalf("Address.%s: %v", f, err)
		}
	}
	if _, err := seed.DisablePackage(ctx, meta, "address"); err != nil {
		t.Fatalf("disable: %v", err)
	}
}
