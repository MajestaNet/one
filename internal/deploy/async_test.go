package deploy_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/deploy"
)

func TestArtifactFileCountAndSyncGate(t *testing.T) {
	art := &deploy.BundleArtifact{
		ManifestVersion: 1,
		Objects:         make([]deploy.SnapshotObject, 3),
		Sources:         map[string]string{"a.ts": "x", "b.ts": "y"},
	}
	if n := deploy.ArtifactFileCount(art); n != 5 {
		t.Fatalf("file count=%d", n)
	}
	eng := deploy.NewDeployEngine(nil, nil, nil, deploy.Options{SyncMaxFiles: 4, SyncMaxBytes: 100})
	if !eng.ExceedsSyncGate(5, 10) {
		t.Fatal("expected files over gate")
	}
	if eng.ExceedsSyncGate(2, 10) {
		t.Fatal("tiny pack should stay sync")
	}
	if !eng.ExceedsSyncGate(1, 101) {
		t.Fatal("expected bytes over gate")
	}
}

func TestDeployQueueBusy(t *testing.T) {
	ctx, pool, meta, data, _ := setupDeployTest(t)
	eng := deploy.NewDeployEngine(pool, meta, data, deploy.Options{
		InstallID:      "test-install",
		CustomerID:     "test-customer",
		ProductVersion: "0.1.0",
		QueueMax:       1,
		SyncMaxFiles:   1,
	})
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE job_type = ANY($1)`, deploy.DeployQueueJobTypes)
	})

	if _, err := pool.Exec(ctx, `
INSERT INTO jobs (job_type, payload, status) VALUES ('deploy.validate', '{}'::jsonb, 'pending')`); err != nil {
		t.Fatal(err)
	}

	pkg := deploy.DefaultCustomerPackage
	art := &deploy.BundleArtifact{
		ManifestVersion:    1,
		Ownership:          "custom",
		DefaultPackageName: pkg,
		Objects: []deploy.SnapshotObject{
			{APIName: "A__c", Label: "A", PluralLabel: "As", StorageMode: "flexible", Ownership: "custom", PackageName: &pkg, Features: map[string]bool{}},
			{APIName: "B__c", Label: "B", PluralLabel: "Bs", StorageMode: "flexible", Ownership: "custom", PackageName: &pkg, Features: map[string]bool{}},
		},
	}
	_, queued, err := eng.EnqueueValidate(ctx, struct {
		Artifact  any
		BundleID  string
		Label     *string
		CreatedBy *string
	}{Artifact: art}, 0)
	if queued != nil {
		t.Fatal("expected no queued work when busy")
	}
	if !errors.Is(err, deploy.ErrBusy) {
		t.Fatalf("expected ErrBusy, got %v", err)
	}
}

func TestEnqueueValidateUsesExpandedArtifactSize(t *testing.T) {
	ctx, pool, meta, data, _ := setupDeployTest(t)
	eng := deploy.NewDeployEngine(pool, meta, data, deploy.Options{
		InstallID:      "test-install",
		CustomerID:     "test-customer",
		ProductVersion: "0.1.0",
		QueueMax:       8,
		SyncMaxFiles:   50,
		SyncMaxBytes:   100,
	})
	art := &deploy.BundleArtifact{
		ManifestVersion:    1,
		Ownership:          "custom",
		DefaultPackageName: deploy.DefaultCustomerPackage,
		Sources:            map[string]string{"large.ts": strings.Repeat("x", 256)},
	}
	_, queued, err := eng.EnqueueValidate(ctx, struct {
		Artifact  any
		BundleID  string
		Label     *string
		CreatedBy *string
	}{Artifact: art}, 10) // compressed/upload bytes are below the gate
	if err != nil {
		t.Fatal(err)
	}
	if queued == nil || queued.JobID == "" {
		t.Fatal("expanded artifact over the byte gate must be queued")
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE id=$1::uuid`, queued.JobID)
		_, _ = pool.Exec(ctx, `DELETE FROM deploy_bundles WHERE id=$1::uuid`, queued.BundleID)
	})
}
