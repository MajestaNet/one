package deploy_test

import (
	"testing"

	"github.com/MajestaNet/ide/internal/deploy"
)

func TestSignArtifact(t *testing.T) {
	artifact := map[string]any{
		"version": 1,
		"objects": []any{},
	}
	secret := "share-secret-for-tests"
	sig, err := deploy.SignArtifact(artifact, secret)
	if err != nil {
		t.Fatal(err)
	}
	if sig == "" {
		t.Fatal("empty signature")
	}
	sig2, err := deploy.SignArtifact(artifact, secret)
	if err != nil || sig2 != sig {
		t.Fatalf("expected stable signature, got %q vs %q err=%v", sig, sig2, err)
	}
}

func TestAssertChecksum(t *testing.T) {
	artifact := map[string]any{"a": 1}
	sum, err := deploy.AssertChecksum(artifact, "")
	if err != nil || sum == "" {
		t.Fatalf("sum=%q err=%v", sum, err)
	}
	if _, err := deploy.AssertChecksum(artifact, "wrong"); err == nil {
		t.Fatal("expected mismatch")
	}
	if got, err := deploy.AssertChecksum(artifact, sum); err != nil || got != sum {
		t.Fatalf("got=%q err=%v", got, err)
	}
}
