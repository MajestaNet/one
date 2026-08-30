package deploy

import (
	"testing"

	_ "github.com/MajestaNet/ide/internal/seed"
)

func TestIsManagedPackageNameIncludesNotes(t *testing.T) {
	pkg := "notes"
	if !isManagedPackageName(&pkg) {
		t.Fatal("notes should be treated as managed for Deploy reject list")
	}
	customer := "customer.default"
	if isManagedPackageName(&customer) {
		t.Fatal("customer.default must not be managed")
	}
}
