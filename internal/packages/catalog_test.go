package packages_test

import (
	"testing"

	"github.com/MajestaNet/ide/internal/packages"
	_ "github.com/MajestaNet/ide/internal/seed" // register core + optional modules
)

func TestCatalogObjectsSalesLookups(t *testing.T) {
	m, ok := packages.Get("sales")
	if !ok {
		t.Fatal("sales module missing")
	}
	objs := packages.CatalogObjects(m)
	byName := map[string]packages.CatalogObject{}
	for _, o := range objs {
		byName[o.APIName] = o
	}
	opp, ok := byName["Opportunity"]
	if !ok {
		t.Fatalf("Opportunity missing: %#v", objs)
	}
	if opp.Label != "Opportunity" || opp.FieldCount < 10 {
		t.Fatalf("Opportunity shape=%+v", opp)
	}
	foundAccount := false
	for _, f := range opp.Fields {
		if f.APIName == "AccountId" && f.ReferenceTo != nil && *f.ReferenceTo == "Account" {
			foundAccount = true
		}
	}
	if !foundAccount {
		t.Fatalf("Opportunity.AccountId lookup missing: %+v", opp.Fields)
	}
}

func TestCatalogObjectsCrmBridgeFieldExtension(t *testing.T) {
	m, ok := packages.Get("crm_bridge")
	if !ok {
		t.Fatal("crm_bridge module missing")
	}
	objs := packages.CatalogObjects(m)
	if len(objs) != 1 || objs[0].APIName != "Case" {
		t.Fatalf("crm_bridge catalog=%+v", objs)
	}
	found := false
	for _, f := range objs[0].Fields {
		if f.APIName == "OpportunityId" && f.ReferenceTo != nil && *f.ReferenceTo == "Opportunity" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Case.OpportunityId missing: %+v", objs[0].Fields)
	}
}

func TestCatalogObjectsLeadMarketingEvenWhenNotEnabled(t *testing.T) {
	m, ok := packages.Get("lead_marketing")
	if !ok {
		t.Fatal("lead_marketing module missing")
	}
	objs := packages.CatalogObjects(m)
	var lead *packages.CatalogObject
	for i := range objs {
		if objs[i].APIName == "Lead" {
			lead = &objs[i]
			break
		}
	}
	if lead == nil {
		t.Fatalf("Lead missing: %#v", objs)
	}
	if len(lead.Fields) == 0 {
		t.Fatal("Lead should declare lookup fields for Explorer")
	}
}
