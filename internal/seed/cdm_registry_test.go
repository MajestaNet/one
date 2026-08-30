package seed_test

import (
	"testing"

	"github.com/MajestaNet/ide/internal/packages"
	"github.com/MajestaNet/ide/internal/seed"
)

func TestManagedPackageRegistry(t *testing.T) {
	_ = seed.CorePackageVersion // ensure seed init runs
	want := map[string]struct {
		optional bool
		version  string
	}{
		"core":               {false, seed.CorePackageVersion},
		"address":            {true, seed.AddressPackageVersion},
		"activities":         {true, seed.ActivitiesPackageVersion},
		"lead_marketing":     {true, seed.LeadMarketingPackageVersion},
		"catalog":            {true, seed.CatalogPackageVersion},
		"sales":              {true, seed.SalesPackageVersion},
		"service":            {true, seed.ServicePackageVersion},
		"billing":            {true, seed.BillingPackageVersion},
		"healthcare":         {true, seed.HealthcarePackageVersion},
		"financial_services": {true, seed.FinancialServicesPackageVersion},
		"retail":             {true, seed.RetailPackageVersion},
		"sustainability":     {true, seed.SustainabilityPackageVersion},
		"education":          {true, seed.EducationPackageVersion},
		"automotive":         {true, seed.AutomotivePackageVersion},
		"nonprofit":          {true, seed.NonprofitPackageVersion},
		"marketing_events":   {true, seed.MarketingEventsPackageVersion},
		"portals":            {true, seed.PortalsPackageVersion},
		"project_service":    {true, seed.ProjectServicePackageVersion},
	}
	for name, exp := range want {
		m, ok := packages.Get(name)
		if !ok {
			t.Fatalf("missing package %s", name)
		}
		if m.Optional != exp.optional {
			t.Fatalf("%s Optional=%v want %v", name, m.Optional, exp.optional)
		}
		if m.Version != exp.version {
			t.Fatalf("%s Version=%s want %s", name, m.Version, exp.version)
		}
	}
	if seed.CorePackageVersion != "2.2.0" {
		t.Fatalf("core version=%s", seed.CorePackageVersion)
	}
}
