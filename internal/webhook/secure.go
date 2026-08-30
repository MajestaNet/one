// Package webhook provides SSRF-safe delivery URL validation and secret encryption.
package webhook

import (
	"github.com/MajestaNet/ide/internal/egress"
	"github.com/MajestaNet/ide/internal/secretcrypt"
)

// ValidateDeliveryURL enforces HTTPS-only webhook targets and blocks loopback,
// link-local, metadata, and private/shared address ranges (SSRF hardening).
func ValidateDeliveryURL(raw string) error {
	return egress.ValidateURL(raw)
}

// EncryptSecret encrypts a webhook shared secret for at-rest storage.
// keyMaterial is typically AUTH_JWT_SIGNING_KEY or WEBHOOK_ENCRYPTION_KEY.
func EncryptSecret(plaintext, keyMaterial string) (string, error) {
	return secretcrypt.Encrypt(plaintext, keyMaterial)
}

// DecryptSecret reverses EncryptSecret. Legacy plaintext (no prefix) is returned as-is.
func DecryptSecret(stored, keyMaterial string) (string, error) {
	return secretcrypt.Decrypt(stored, keyMaterial)
}
