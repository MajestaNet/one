package gitremote_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MajestaNet/ide/internal/deploy/gitremote"
)

func TestMemoryRemoteSeedAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "one.yaml"), []byte("repoFormat: one/v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &gitremote.MemoryRemote{URL: "https://example.com/repo.git", CommitSHA: "abc123"}
	sha, err := m.SeedMain(context.Background(), dir, false)
	if err != nil || sha != "abc123" {
		t.Fatalf("seed: sha=%s err=%v", sha, err)
	}
	_, err = m.SeedMain(context.Background(), dir, false)
	if err != gitremote.ErrAlreadyInitialized {
		t.Fatalf("expected ErrAlreadyInitialized, got %v", err)
	}
	if _, err := m.SeedMain(context.Background(), dir, true); err != nil {
		t.Fatalf("force seed: %v", err)
	}
}

func TestMemoryRemoteRequiresURL(t *testing.T) {
	m := &gitremote.MemoryRemote{}
	_, err := m.SeedMain(context.Background(), t.TempDir(), false)
	if err != gitremote.ErrNotConfigured {
		t.Fatalf("got %v", err)
	}
}

func TestNewGoGitRemoteRequiresURL(t *testing.T) {
	_, err := gitremote.NewGoGitRemote("", nil)
	if err != gitremote.ErrNotConfigured {
		t.Fatalf("got %v", err)
	}
}
