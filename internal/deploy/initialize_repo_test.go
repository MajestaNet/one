package deploy_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/deploy/gitremote"
)

func TestGetEnvironmentInitializeRepoCapability(t *testing.T) {
	eng := deploy.NewDeployEngine(nil, nil, nil, deploy.Options{
		CustomerRepoURL: "https://example.com/customer.git",
		CustomerID:      "acme",
	})
	env := eng.GetEnvironment()
	if !env.Capabilities["packageInitializeRepo"] {
		t.Fatal("expected packageInitializeRepo when URL configured")
	}
	eng2 := deploy.NewDeployEngine(nil, nil, nil, deploy.Options{CustomerID: "acme"})
	if eng2.GetEnvironment().Capabilities["packageInitializeRepo"] {
		t.Fatal("expected packageInitializeRepo false without URL")
	}
}

func TestSeedCustomerRepoDirMemory(t *testing.T) {
	eng := deploy.NewDeployEngine(nil, nil, nil, deploy.Options{
		CustomerRepoURL:      "https://example.com/customer.git",
		CustomerRepoProvider: "codecommit",
		CustomerID:           "acme",
		ProductVersion:       "0.1.0",
	})
	mem := &gitremote.MemoryRemote{URL: "https://example.com/customer.git", CommitSHA: "cafebabe"}
	eng.SetGitRemote(mem)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "one.yaml"), []byte("repoFormat: one/v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sha, err := eng.SeedCustomerRepoDir(t.Context(), dir, false)
	if err != nil || sha != "cafebabe" {
		t.Fatalf("seed: sha=%s err=%v", sha, err)
	}
	_, err = eng.SeedCustomerRepoDir(t.Context(), dir, false)
	if !errors.Is(err, deploy.ErrRepoAlreadyInitialized) {
		t.Fatalf("expected already initialized, got %v", err)
	}
	if _, err := eng.SeedCustomerRepoDir(t.Context(), dir, true); err != nil {
		t.Fatalf("force: %v", err)
	}

	art := &deploy.BundleArtifact{
		Objects: []deploy.SnapshotObject{{APIName: "X"}},
		Baseline: &deploy.ManagedBaseline{
			ProductVersion: "0.1.0",
			Objects:        []deploy.SnapshotObject{{APIName: "Account"}},
			Fields:         []deploy.SnapshotField{{APIName: "Name"}},
		},
	}
	res := eng.BuildInitializeRepoResult(art, sha, true)
	if res.BaselineProductVersion != "0.1.0" || res.ArtifactCounts["baselineObjects"] != 1 {
		t.Fatalf("result=%+v", res)
	}
}

func TestSeedCustomerRepoNotConfigured(t *testing.T) {
	eng := deploy.NewDeployEngine(nil, nil, nil, deploy.Options{CustomerID: "acme"})
	_, err := eng.SeedCustomerRepoDir(t.Context(), t.TempDir(), false)
	if !errors.Is(err, deploy.ErrCustomerRepoNotConfigured) {
		t.Fatalf("got %v", err)
	}
}
