package edge_test

import (
	"testing"

	"github.com/MajestaNet/ide/internal/edge"
)

func TestValidatePolicy(t *testing.T) {
	p := edge.DefaultPolicy()
	if err := edge.Validate(p); err != nil {
		t.Fatal(err)
	}
	if p.Metadata.Mode != edge.ModeBlocked || p.Deploy.Mode != edge.ModeBlocked || p.Ops.Mode != edge.ModeBlocked {
		t.Fatalf("expected control-plane families blocked by default, got %+v", p)
	}

	p.Auth.Mode = edge.ModeBlocked
	p.Client.Mode = edge.ModePublic
	if err := edge.Validate(p); err == nil {
		t.Fatal("expected error for public client + blocked auth")
	}

	p = edge.DefaultPolicy()
	p.Metadata.Mode = edge.ModeAllowlist
	p.Metadata.CIDRs = nil
	if err := edge.Validate(p); err == nil {
		t.Fatal("expected error for empty allowlist")
	}

	p.Metadata.CIDRs = []string{"10.0.0.0/8"}
	if err := edge.Validate(p); err != nil {
		t.Fatal(err)
	}

	p.Deploy.CIDRs = []string{"not-a-cidr"}
	p.Deploy.Mode = edge.ModeAllowlist
	if err := edge.Validate(p); err == nil {
		t.Fatal("expected invalid cidr error")
	}

	p = edge.DefaultPolicy()
	p.Metadata.Mode = edge.ModePublic
	if err := edge.Validate(p); err == nil {
		t.Fatal("expected error for public metadata")
	}
}

func TestMemoryRollerApply(t *testing.T) {
	m := &edge.MemoryRoller{}
	p := edge.DefaultPolicy()
	p.Metadata.Mode = edge.ModeAllowlist
	p.Metadata.CIDRs = []string{"10.0.0.0/8"}
	if err := m.Apply(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	if m.Last.Metadata.Mode != edge.ModeAllowlist {
		t.Fatalf("got %+v", m.Last.Metadata)
	}
	if m.Mode() != "local" {
		t.Fatalf("mode=%s", m.Mode())
	}
}

func TestMergeCIDRsAndAccessMode(t *testing.T) {
	got := edge.MergeCIDRs([]string{"10.0.0.0/8"}, []string{"10.0.0.0/8", "192.168.0.0/16"})
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
	p := edge.DefaultPolicy()
	if p.EffectiveClientAccessMode() != edge.ClientAccessOpen {
		t.Fatalf("default access mode %q", p.EffectiveClientAccessMode())
	}
	p.ClientAccessMode = "nope"
	if err := edge.Validate(p); err == nil {
		t.Fatal("expected invalid clientAccessMode")
	}
	p.ClientAccessMode = edge.ClientAccessIDEUsers
	if err := edge.Validate(p); err == nil {
		t.Fatal("expected ide_users to be rejected")
	}
	stored := edge.Policy{ClientAccessMode: edge.ClientAccessIDEUsers}
	if stored.EffectiveClientAccessMode() != edge.ClientAccessOpen {
		t.Fatalf("stored ide_users should map to open, got %q", stored.EffectiveClientAccessMode())
	}
	p.ClientAccessMode = edge.ClientAccessRegistered
	if err := edge.Validate(p); err != nil {
		t.Fatal(err)
	}
	// VPN lockdown recipe: Client+Auth allowlist
	p.Client.Mode = edge.ModeAllowlist
	p.Client.CIDRs = []string{"203.0.113.0/24"}
	p.Auth.Mode = edge.ModeAllowlist
	p.Auth.CIDRs = []string{"203.0.113.0/24"}
	if err := edge.Validate(p); err != nil {
		t.Fatal(err)
	}
}
