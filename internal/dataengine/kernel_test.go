package dataengine

import (
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/db"
)

func TestRejectKernelStorage(t *testing.T) {
	err := rejectKernelStorage("User", db.StorageModeKernel)
	if err == nil || !strings.Contains(err.Error(), "not a flexible object") {
		t.Fatalf("kernel: %v", err)
	}
	if err := rejectKernelStorage("Account", db.StorageModeFlexible); err != nil {
		t.Fatalf("flexible: %v", err)
	}
	if err := rejectKernelStorage("HvEvent", db.StorageModeHighVolume); err != nil {
		t.Fatalf("high_volume: %v", err)
	}
}
