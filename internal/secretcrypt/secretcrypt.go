// Package secretcrypt provides AES-GCM enc:v1: helpers for secrets at rest
// (webhooks, integration client secrets, etc.).
package secretcrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const EncPrefix = "enc:v1:"

// Encrypt encrypts plaintext for at-rest storage.
// keyMaterial is typically AUTH_JWT_SIGNING_KEY or WEBHOOK_ENCRYPTION_KEY.
// When keyMaterial is empty, plaintext is returned unchanged (dev convenience).
func Encrypt(plaintext, keyMaterial string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if keyMaterial == "" {
		return plaintext, nil
	}
	block, err := aes.NewCipher(deriveKey(keyMaterial))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return EncPrefix + base64.RawStdEncoding.EncodeToString(out), nil
}

// Decrypt reverses Encrypt. Legacy plaintext (no prefix) is returned as-is.
func Decrypt(stored, keyMaterial string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, EncPrefix) {
		return stored, nil
	}
	if keyMaterial == "" {
		return "", fmt.Errorf("encrypted secret requires decryption key")
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(stored, EncPrefix))
	if err != nil {
		return "", fmt.Errorf("decode encrypted secret: %w", err)
	}
	block, err := aes.NewCipher(deriveKey(keyMaterial))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("encrypted secret too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plain), nil
}

func deriveKey(material string) []byte {
	sum := sha256.Sum256([]byte(material))
	return sum[:]
}
