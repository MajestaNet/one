package authz_test

import (
	"errors"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/edge"
)

func TestAllowClientAccess(t *testing.T) {
	if err := authz.AllowClientAccess(edge.ClientAccessOpen, "", "client_credentials", false); err != nil {
		t.Fatal(err)
	}
	if err := authz.AllowClientAccess(edge.ClientAccessOpen, authz.InstallAzp, "password", false); err != nil {
		t.Fatal(err)
	}
	if err := authz.AllowBearerAzp(edge.ClientAccessRegistered, "", "one_jwt", false); !errors.Is(err, authz.ErrClientAccessDenied) {
		t.Fatalf("expected deny empty azp, got %v", err)
	}
	if err := authz.AllowBearerAzp(edge.ClientAccessRegistered, "my.app", "one_jwt", false); err != nil {
		t.Fatal(err)
	}
	if err := authz.AllowBearerAzp(edge.ClientAccessRegistered, authz.InstallAzp, "one_jwt", false); err != nil {
		t.Fatal(err)
	}
	if err := authz.AllowClientAccess(edge.ClientAccessIDEUsers, authz.ControlIDEAzp, "password", false); !errors.Is(err, authz.ErrClientAccessDenied) {
		t.Fatalf("expected unknown-mode deny for leftover ide_users, got %v", err)
	}
}

func TestInstallAzpConstant(t *testing.T) {
	if authz.InstallAzp != "one.install" {
		t.Fatalf("InstallAzp=%q", authz.InstallAzp)
	}
	if authz.ControlIDEAzp != "one.controlIde" {
		t.Fatalf("ControlIDEAzp=%q", authz.ControlIDEAzp)
	}
}
