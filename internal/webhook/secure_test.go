package webhook_test

import (
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/webhook"
)

func TestValidateDeliveryURL(t *testing.T) {
	// Use a public literal IP so the test does not depend on external DNS.
	if err := webhook.ValidateDeliveryURL("https://93.184.216.34/one"); err != nil {
		t.Fatal(err)
	}
	cases := []string{
		"http://hooks.example.com/x",
		"https://127.0.0.1/x",
		"https://localhost/x",
		"https://169.254.169.254/latest",
		"https://10.0.0.1/hook",
		"https://user:pass@hooks.example.com/x",
	}
	for _, c := range cases {
		if err := webhook.ValidateDeliveryURL(c); err == nil {
			t.Fatalf("expected error for %q", c)
		}
	}
}

func TestEncryptDecryptSecret(t *testing.T) {
	key := "test-signing-key-material"
	enc, err := webhook.EncryptSecret("super-secret", key)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, "enc:v1:") {
		t.Fatalf("expected enc prefix, got %q", enc)
	}
	plain, err := webhook.DecryptSecret(enc, key)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "super-secret" {
		t.Fatalf("got %q", plain)
	}
	// Legacy plaintext passthrough.
	plain, err = webhook.DecryptSecret("legacy-clear", key)
	if err != nil || plain != "legacy-clear" {
		t.Fatalf("legacy=%q err=%v", plain, err)
	}
}
