package secretcrypt_test

import (
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/secretcrypt"
)

func TestEncryptDecrypt(t *testing.T) {
	key := "test-signing-key-material"
	enc, err := secretcrypt.Encrypt("super-secret", key)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, secretcrypt.EncPrefix) {
		t.Fatalf("expected enc prefix, got %q", enc)
	}
	plain, err := secretcrypt.Decrypt(enc, key)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "super-secret" {
		t.Fatalf("got %q", plain)
	}
	plain, err = secretcrypt.Decrypt("legacy-clear", key)
	if err != nil || plain != "legacy-clear" {
		t.Fatalf("legacy=%q err=%v", plain, err)
	}
}

func TestEncryptEmptyKeyPassthrough(t *testing.T) {
	enc, err := secretcrypt.Encrypt("plain", "")
	if err != nil || enc != "plain" {
		t.Fatalf("got %q err=%v", enc, err)
	}
}
