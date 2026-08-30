package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	fixtureAPIDigest    = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	fixtureWorkerDigest = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	staleAPIDigest      = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	staleWorkerDigest   = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestApplyDoAppDigestsRewritesPinnedSpec(t *testing.T) {
	dir := scriptsDir(t)
	src := filepath.Join(dir, "testdata", "app-already-pinned.yaml")
	digests := filepath.Join(dir, "testdata", "image-digests-1.2.3.txt")
	script := filepath.Join(dir, "apply-do-app-digests.sh")

	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(specPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", script, digests, specPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("apply: %v\n%s", err, out)
	}

	updated, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(updated)
	if strings.Contains(got, staleAPIDigest) || strings.Contains(got, staleWorkerDigest) {
		t.Fatal("stale digests still present after apply")
	}
	if !strings.Contains(got, "digest: "+fixtureAPIDigest) {
		t.Fatalf("api digest not rewritten:\n%s", got)
	}
	if !strings.Contains(got, "digest: "+fixtureWorkerDigest) {
		t.Fatalf("worker digest not rewritten:\n%s", got)
	}
	if strings.Count(got, `tag: "1.2.3"`) != 2 {
		t.Fatalf("expected api+worker tag 1.2.3, got:\n%s", got)
	}
	if strings.Contains(got, `tag: "0.9.0"`) || strings.Contains(got, `value: "0.9.0"`) {
		t.Fatal("0.9.0 still present; PRODUCT_VERSION/tag must follow digests filename")
	}
	if strings.Count(got, `value: "1.2.3"`) < 2 {
		t.Fatalf("expected PRODUCT_VERSION 1.2.3 on api and worker:\n%s", got)
	}
	if err := validateBytes(updated, validateOptions{strictDigest: true}); err != nil {
		t.Fatalf("applied spec failed -strict-digest: %v", err)
	}
}

func TestApplyDoAppDigestsRewritesCommentedPlaceholders(t *testing.T) {
	example := filepath.Join(repoRoot(t), "deploy", "digitalocean", "app.yaml")
	raw, err := os.ReadFile(example)
	if err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(specPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	digests := filepath.Join(scriptsDir(t), "testdata", "image-digests-1.2.3.txt")
	script := filepath.Join(scriptsDir(t), "apply-do-app-digests.sh")
	cmd := exec.Command("bash", script, digests, specPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("apply: %v\n%s", err, out)
	}
	updated, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(updated)
	if strings.Contains(got, "REPLACE_WITH_API_DIGEST") || strings.Contains(got, "REPLACE_WITH_WORKER_DIGEST") {
		t.Fatal("placeholder digest comments not rewritten")
	}
	if strings.Contains(got, "# digest:") {
		t.Fatal("commented digest lines remain")
	}
	if !strings.Contains(got, "digest: "+fixtureAPIDigest) || !strings.Contains(got, "digest: "+fixtureWorkerDigest) {
		t.Fatalf("expected fixture digests:\n%s", got)
	}
	if strings.Contains(got, `value: "0.1.0"`) {
		t.Fatal("PRODUCT_VERSION still 0.1.0")
	}
	if err := validateBytes(updated, validateOptions{}); err != nil {
		t.Fatalf("applied example failed validate: %v", err)
	}
	if err := validateBytes(updated, validateOptions{strictDigest: true}); err != nil {
		t.Fatalf("applied example failed -strict-digest: %v", err)
	}
}
