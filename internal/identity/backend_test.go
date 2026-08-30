package identity_test

import (
	"testing"

	"github.com/MajestaNet/ide/internal/identity"
)

func TestMemoryBackendProvision(t *testing.T) {
	b := identity.NewMemoryBackend()
	sub, err := b.ProvisionUser(t.Context(), "a@example.com", "A")
	if err != nil || sub == "" {
		t.Fatalf("ProvisionUser: %v %s", err, sub)
	}
	sub2, err := b.ProvisionUser(t.Context(), "a@example.com", "A")
	if err != nil || sub2 != sub {
		t.Fatalf("idempotent: %v %s vs %s", err, sub, sub2)
	}
	id, secret, err := b.CreateAppClient(t.Context(), identity.DefaultM2MAppClientSpec("bot", "service"))
	if err != nil || id == "" || secret == "" {
		t.Fatalf("CreateAppClient: %v %s %s", err, id, secret)
	}
	other := identity.NewMemoryBackend()
	id2, _, err := other.CreateAppClient(t.Context(), identity.DefaultM2MAppClientSpec("bot2", "service"))
	if err != nil || id2 == "" || id2 == id {
		t.Fatalf("client ids must be unique across backends: %v %s %s", err, id, id2)
	}
	if err := b.UpdateAppClient(t.Context(), id, identity.AppClientSpec{
		Name: "bot", PrincipalType: "service", Confidential: true,
		OAuthFlows: []string{identity.FlowClientCredentials}, GenerateSecret: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetUserActive(t.Context(), "a@example.com", false); err != nil {
		t.Fatal(err)
	}
}

func TestProviderSlackExactString(t *testing.T) {
	if identity.ProviderSlack != "slack" {
		t.Fatalf("ProviderSlack=%q want slack", identity.ProviderSlack)
	}
}

func TestNewBackendFromConfig(t *testing.T) {
	if identity.NewBackendFromConfig("off").Enabled() {
		t.Fatal("off should be disabled")
	}
	if !identity.NewBackendFromConfig("memory").Enabled() {
		t.Fatal("memory should be enabled")
	}
	if identity.NewBackendFromConfig("cognito").Enabled() {
		t.Fatal("cognito mode is not wired in product binary")
	}
}
