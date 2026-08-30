package httpapi

import (
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/identity"
)

func TestInstallAuthClientSecretEncryptedAndLegacyReadable(t *testing.T) {
	const (
		secret = "customer-oidc-client-secret"
		key    = "install-encryption-key-0123456789abcdef"
	)
	stored, err := protectInstallAuthClientSecret(secret, key)
	if err != nil {
		t.Fatal(err)
	}
	if stored == secret || !strings.HasPrefix(stored, "enc:v1:") {
		t.Fatalf("secret was not encrypted: %q", stored)
	}
	got, err := revealInstallAuthClientSecret(stored, key)
	if err != nil || got != secret {
		t.Fatalf("decrypt = %q, %v", got, err)
	}
	got, err = revealInstallAuthClientSecret("plain:"+secret, key)
	if err != nil || got != secret {
		t.Fatalf("legacy decrypt = %q, %v", got, err)
	}
}

func TestSocialJITRequiresVerifiedEmailForFirstPartyProviders(t *testing.T) {
	for _, provider := range []string{identity.ProviderGoogle, identity.ProviderApple, identity.ProviderSlack} {
		if !requiresVerifiedJITEmail(provider) {
			t.Fatalf("provider %q must require verified email", provider)
		}
	}
	if requiresVerifiedJITEmail(identity.ProviderOIDC) {
		t.Fatal("customer OIDC verification policy must remain provider-configurable")
	}
}
