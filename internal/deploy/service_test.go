package deploy_test

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
)

func setupDeployTest(t *testing.T) (context.Context, *db.Pool, *metadata.Service, *dataengine.Service, *deploy.DeployEngine) {
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
	data := dataengine.NewService(pool, meta)
	engine := deploy.NewDeployEngine(pool, meta, data, deploy.Options{
		InstallID:      "test-install",
		InstallRole:    "test",
		ProductVersion: "0.1.0",
		CustomerID:     "test-customer",
		PeerMode:       deploy.PeerModeCustomer,
	})
	return ctx, pool, meta, data, engine
}

func cleanupObject(t *testing.T, ctx context.Context, pool *db.Pool, apiName string) {
	t.Helper()
	_, _ = pool.Exec(ctx, `DELETE FROM metadata_fields WHERE object_api_name=$1`, apiName)
	_, _ = pool.Exec(ctx, `DELETE FROM metadata_validation_rules WHERE object_api_name=$1`, apiName)
	_, _ = pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name=$1`, apiName)
}

func TestDeploySnapshotRoundTrip(t *testing.T) {
	ctx, pool, meta, _, engine := setupDeployTest(t)

	const objName = "DeployTestObj__c"
	cleanupObject(t, ctx, pool, objName)
	// Delete test suite if it exists.
	_, _ = pool.Exec(ctx, `DELETE FROM customer_tests WHERE api_name='deploy_test_suite'`)
	t.Cleanup(func() {
		cleanupObject(t, ctx, pool, objName)
		_, _ = pool.Exec(ctx, `DELETE FROM customer_tests WHERE api_name='deploy_test_suite'`)
	})

	// Create a customer object.
	if err := meta.CreateObject(ctx, metadata.ObjectDefinition{
		APIName:     objName,
		Label:       "Deploy Test Object",
		PluralLabel: "Deploy Test Objects",
		StorageMode: "flexible",
		Ownership:   "custom",
		Features:    map[string]bool{},
	}); err != nil {
		t.Fatalf("CreateObject: %v", err)
	}
	if err := meta.CreateField(ctx, metadata.FieldDefinition{
		ObjectAPIName: objName,
		APIName:       "Name",
		Label:         "Name",
		FieldType:     "text",
		Required:      true,
		Ownership:     "custom",
	}); err != nil {
		t.Fatalf("CreateField: %v", err)
	}
	if err := meta.BumpEpoch(ctx); err != nil {
		t.Fatalf("BumpEpoch: %v", err)
	}

	// CreateBundleFromSnapshot.
	bundle, err := engine.CreateBundleFromSnapshot(ctx, struct {
		Label               *string
		CreatedBy           *string
		ProductVersionRange string
	}{})
	if err != nil {
		t.Fatalf("CreateBundleFromSnapshot: %v", err)
	}
	if bundle.ID == "" {
		t.Fatal("expected non-empty bundle ID")
	}
	t.Logf("bundle id=%s checksum=%s", bundle.ID, bundle.Checksum)

	// ValidateBundle.
	vr, err := engine.ValidateBundle(ctx, bundle.ID)
	if err != nil {
		t.Fatalf("ValidateBundle: %v", err)
	}
	if !vr.Report.OK {
		t.Fatalf("expected validation OK, issues: %+v", vr.Report.Issues)
	}

	// Delete the object so apply has something to do.
	cleanupObject(t, ctx, pool, objName)
	if err := meta.BumpEpoch(ctx); err != nil {
		t.Fatalf("BumpEpoch after delete: %v", err)
	}

	// PromoteBundle dryRun.
	dryResult, err := engine.PromoteBundle(ctx, struct {
		BundleID  string
		DryRun    bool
		CreatedBy *string
	}{BundleID: bundle.ID, DryRun: true})
	if err != nil {
		t.Fatalf("PromoteBundle dryRun: %v", err)
	}
	if dryResult.Promotion.Status != "validated" {
		t.Fatalf("expected status=validated, got %s", dryResult.Promotion.Status)
	}

	// PromoteBundle for real.
	result, err := engine.PromoteBundle(ctx, struct {
		BundleID  string
		DryRun    bool
		CreatedBy *string
	}{BundleID: bundle.ID, DryRun: false})
	if err != nil {
		t.Fatalf("PromoteBundle: %v", err)
	}
	if result.Promotion.Status != "applied" {
		t.Fatalf("expected status=applied, got %s", result.Promotion.Status)
	}
	if result.Apply == nil || result.Apply.Created == 0 {
		t.Fatalf("expected created>0, got: %+v", result.Apply)
	}

	// Verify object re-applied.
	obj, err := meta.GetObject(ctx, objName)
	if err != nil {
		t.Fatalf("GetObject after apply: %v", err)
	}
	if obj.Label != "Deploy Test Object" {
		t.Fatalf("unexpected label: %s", obj.Label)
	}

	// GetPromotion.
	prom, err := engine.GetPromotion(ctx, result.Promotion.ID)
	if err != nil {
		t.Fatalf("GetPromotion: %v", err)
	}
	if prom.Status != "applied" {
		t.Fatalf("promotion status: %s", prom.Status)
	}

	// ListBundles.
	bundles, err := engine.ListBundles(ctx, 10)
	if err != nil {
		t.Fatalf("ListBundles: %v", err)
	}
	if len(bundles) == 0 {
		t.Fatal("expected at least one bundle")
	}
}

func TestDeployTestSuiteRunSync(t *testing.T) {
	ctx, pool, meta, _, engine := setupDeployTest(t)

	const objName = "TSTestObj__c"
	cleanupObject(t, ctx, pool, objName)
	_, _ = pool.Exec(ctx, `DELETE FROM customer_tests WHERE api_name='ts_sync_suite'`)
	t.Cleanup(func() {
		cleanupObject(t, ctx, pool, objName)
		_, _ = pool.Exec(ctx, `DELETE FROM customer_tests WHERE api_name='ts_sync_suite'`)
	})

	// Create object for objectExists step.
	if err := meta.CreateObject(ctx, metadata.ObjectDefinition{
		APIName:     objName,
		Label:       "TS Object",
		PluralLabel: "TS Objects",
		StorageMode: "flexible",
		Ownership:   "custom",
		Features:    map[string]bool{},
	}); err != nil {
		t.Fatalf("CreateObject: %v", err)
	}
	if err := meta.BumpEpoch(ctx); err != nil {
		t.Fatalf("BumpEpoch: %v", err)
	}

	boolTrue := true
	suite, err := engine.UpsertTestSuite(ctx, &deploy.TestSuiteInput{
		APIName: "ts_sync_suite",
		Label:   "TS Sync Suite",
		Active:  &boolTrue,
		Steps: []any{
			map[string]any{"type": "objectExists", "objectApiName": objName},
		},
	})
	if err != nil {
		t.Fatalf("UpsertTestSuite: %v", err)
	}
	if suite.APIName != "ts_sync_suite" {
		t.Fatalf("unexpected apiName: %s", suite.APIName)
	}

	actor := &authz.Actor{
		ID:               "00000000-0000-4000-8000-000000000001",
		Scopes:           []authz.Scope{authz.ScopeClient, authz.ScopeMetadata, authz.ScopeDeploy},
		PermissionSetIDs: []string{},
		IsAdmin:          true,
	}

	runResult, err := engine.StartTestRun(ctx, struct {
		SuiteAPIName string
		Actor        *authz.Actor
		Async        bool
		Trigger      string
	}{SuiteAPIName: "ts_sync_suite", Actor: actor, Async: false})
	if err != nil {
		t.Fatalf("StartTestRun: %v", err)
	}
	if runResult.Run.Status != "passed" {
		t.Fatalf("expected passed, got %s", runResult.Run.Status)
	}
	if runResult.Mode != "sync" {
		t.Fatalf("expected sync, got %s", runResult.Mode)
	}

	// GetTestRun.
	run, err := engine.GetTestRun(ctx, runResult.Run.ID)
	if err != nil {
		t.Fatalf("GetTestRun: %v", err)
	}
	if run.Status != "passed" {
		t.Fatalf("expected passed, got %s", run.Status)
	}

	// ListTestRuns.
	runs, err := engine.ListTestRuns(ctx, 10)
	if err != nil {
		t.Fatalf("ListTestRuns: %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("expected at least one run")
	}

	// ListTestSuites.
	suites, err := engine.ListTestSuites(ctx)
	if err != nil {
		t.Fatalf("ListTestSuites: %v", err)
	}
	if len(suites) == 0 {
		t.Fatal("expected at least one suite")
	}
}
