package packages_test

import (
	"testing"

	"github.com/MajestaNet/ide/internal/packages"
	_ "github.com/MajestaNet/ide/internal/seed" // register core + notes
)

func TestIsManagedPackageName(t *testing.T) {
	core := "core"
	notes := "notes"
	crm := "crm"
	customer := "customer.default"
	if !packages.IsManagedPackageName(&core) || !packages.IsManagedPackageName(&notes) || !packages.IsManagedPackageName(&crm) {
		t.Fatal("expected core/notes/crm managed")
	}
	if packages.IsManagedPackageName(&customer) || packages.IsManagedPackageName(nil) {
		t.Fatal("customer/nil should not be managed")
	}
	retired := "messages"
	if packages.IsManagedPackageName(&retired) {
		t.Fatal("retired messages package must not remain in the managed catalog")
	}
	m, ok := packages.Get("notes")
	if !ok || !m.Optional || len(m.DependsOn) == 0 || m.DependsOn[0] != "core" {
		t.Fatalf("notes module=%+v", m)
	}
}

func TestActionsByNameUnique(t *testing.T) {
	all, err := packages.ActionsByName()
	if err != nil {
		t.Fatal(err)
	}
	reg, ok := all["lead.convert"]
	if !ok || reg.Module != "lead_marketing" || !reg.Def.SyncSafe {
		t.Fatalf("lead.convert=%+v ok=%v", reg, ok)
	}
	qa, ok := all["quote.accept"]
	if !ok || qa.Module != "sales" || !qa.Def.SyncSafe {
		t.Fatalf("quote.accept=%+v ok=%v", qa, ok)
	}
}
