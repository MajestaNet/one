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

func TestIndustryPackagesRegistered(t *testing.T) {
	want := map[string]string{
		"healthcare":         seed.HealthcarePackageVersion,
		"financial_services": seed.FinancialServicesPackageVersion,
		"retail":             seed.RetailPackageVersion,
		"sustainability":     seed.SustainabilityPackageVersion,
		"education":          seed.EducationPackageVersion,
		"automotive":         seed.AutomotivePackageVersion,
		"nonprofit":          seed.NonprofitPackageVersion,
		"marketing_events":   seed.MarketingEventsPackageVersion,
		"portals":            seed.PortalsPackageVersion,
		"project_service":    seed.ProjectServicePackageVersion,
	}
	for name, ver := range want {
		m, ok := packages.Get(name)
		if !ok || !m.Optional {
			t.Fatalf("%s registry ok=%v optional=%v", name, ok, m.Optional)
		}
		if m.Version != ver {
			t.Fatalf("%s version=%s want %s", name, m.Version, ver)
		}
		if len(m.DependsOn) != 1 || m.DependsOn[0] != "core" {
			t.Fatalf("%s DependsOn=%v", name, m.DependsOn)
		}
		if len(m.Objects) == 0 {
			t.Fatalf("%s has no objects", name)
		}
		// Must not collide with spine apiNames.
		for _, o := range m.Objects {
			switch o.APIName {
			case "Account", "Contact", "Product", "Case", "Opportunity", "Campaign", "Asset", "Lead", "Quote":
				t.Fatalf("%s must not define spine object %s", name, o.APIName)
			}
		}
	}
}

func TestEnableIndustryPackages(t *testing.T) {
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

	samples := []struct {
		pkg    string
		object string
	}{
		{"healthcare", "Patient"},
		{"financial_services", "Bank"},
		{"retail", "LoyaltyProgram"},
		{"sustainability", "Facility"},
		{"education", "Program"},
		{"automotive", "Device"},
		{"nonprofit", "DonorCommitment"},
		{"marketing_events", "MarketingEvent"},
		{"portals", "Website"},
		{"project_service", "Project"},
	}
	for _, s := range samples {
		st, err := seed.EnablePackage(ctx, meta, s.pkg)
		if err != nil {
			t.Fatalf("enable %s: %v", s.pkg, err)
		}
		if !st.Enabled {
			t.Fatalf("%s not enabled", s.pkg)
		}
		obj, err := meta.GetObject(ctx, s.object)
		if err != nil {
			t.Fatalf("get %s after %s enable: %v", s.object, s.pkg, err)
		}
		if obj.PackageName == nil || *obj.PackageName != s.pkg {
			t.Fatalf("%s package=%v want %s", s.object, obj.PackageName, s.pkg)
		}
		if _, err := seed.DisablePackage(ctx, meta, s.pkg); err != nil {
			t.Fatalf("disable %s: %v", s.pkg, err)
		}
	}
}
