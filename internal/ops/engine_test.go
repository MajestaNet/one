package ops_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/ops"
	"github.com/MajestaNet/ide/internal/seed"
)

func setupOpsTest(t *testing.T) (context.Context, *db.Pool, *deploy.DeployEngine, *ops.Engine, *ops.MemoryRoller) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	meta := metadata.NewService(pool)
	if err := seed.Bootstrap(ctx, pool, meta, seed.Options{
		OwnerID:  "00000000-0000-4000-8000-000000000001",
		AutoSeed: true,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	data := dataengine.NewService(pool, meta)
	deployEng := deploy.NewDeployEngine(pool, meta, data, deploy.Options{
		InstallID: "ops-test", InstallRole: "test", ProductVersion: "0.1.0", CustomerID: "ops-customer",
	})
	roller := &ops.MemoryRoller{}
	eng := ops.NewEngine(pool, deployEng, ops.Options{
		ProductVersion: "0.1.0",
		PublicURL:      "",
		Roller:         roller,
		Health:         ops.NopHealth{},
	})
	return ctx, pool, deployEng, eng, roller
}

func TestOpsConfirmGateAndRollbackOnRollFailure(t *testing.T) {
	ctx, pool, _, eng, roller := setupOpsTest(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM platform_upgrades`)
	})

	actor := &authz.Actor{
		ID:      "00000000-0000-4000-8000-000000000001",
		Scopes:  []authz.Scope{authz.ScopeOps, authz.ScopeDeploy, authz.ScopeClient, authz.ScopeMetadata},
		IsAdmin: true,
	}

	row, err := eng.Confirm(ctx, ops.ConfirmInput{
		APIImage: "one-api:0.2.0", WorkerImage: "one-worker:0.2.0",
		ProductVersion: "0.2.0", Actor: actor,
	})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if row.Status != ops.StatusSucceeded {
		t.Fatalf("status=%s error=%v gate=%s", row.Status, row.Error, string(row.GateResult))
	}
	if roller.LastRoll == nil || roller.LastRoll.ProductVersion != "0.2.0" {
		t.Fatalf("expected roller to record roll: %+v", roller.LastRoll)
	}

	// Force roll failure → rollback
	roller.FailRoll = true
	row2, err := eng.Confirm(ctx, ops.ConfirmInput{
		APIImage: "one-api:0.3.0", WorkerImage: "one-worker:0.3.0",
		ProductVersion: "0.3.0", Actor: actor,
	})
	if err == nil {
		t.Fatal("expected error on roll failure")
	}
	if row2 == nil || row2.Status != ops.StatusRolledBack {
		t.Fatalf("expected rolled_back, got %+v", row2)
	}
	if !roller.RolledBack {
		t.Fatal("expected roller rollback")
	}
}

func TestOpsAvailableAndList(t *testing.T) {
	ctx, _, _, eng, _ := setupOpsTest(t)
	av := eng.GetAvailable()
	if av.CurrentVersion != "0.1.0" || av.PlatformSmoke != ops.PlatformSmokeSuite {
		t.Fatalf("unexpected available: %+v", av)
	}
	rows, err := eng.List(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	_ = rows
}

func TestPlatformSmokeSeeded(t *testing.T) {
	ctx, _, deployEng, _, _ := setupOpsTest(t)
	suite, err := deployEng.GetTestSuite(ctx, seed.PlatformSmokeAPIName)
	if err != nil {
		t.Fatal(err)
	}
	if suite.Ownership != "managed" {
		t.Fatalf("ownership=%s", suite.Ownership)
	}
	_, err = deployEng.UpsertTestSuite(ctx, &deploy.TestSuiteInput{
		APIName: seed.PlatformSmokeAPIName,
		Label:   "hijack",
		Steps:   []any{map[string]any{"type": "objectExists", "objectApiName": "Account"}},
	})
	if err == nil {
		t.Fatal("expected forbid overwrite of managed suite")
	}
}
