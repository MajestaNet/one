package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestPackageValidateLocal(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{APIKeys: "admin-key+admin"})

	const obj = "DxValidateObj__c"
	_, _ = d.Pool.Exec(t.Context(), `DELETE FROM metadata_fields WHERE object_api_name=$1`, obj)
	_, _ = d.Pool.Exec(t.Context(), `DELETE FROM metadata_objects WHERE api_name=$1`, obj)
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM metadata_fields WHERE object_api_name=$1`, obj)
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM metadata_objects WHERE api_name=$1`, obj)
	})

	if err := d.Meta.CreateObject(t.Context(), metadata.ObjectDefinition{
		APIName: obj, Label: "DX Obj", PluralLabel: "DX Objs", StorageMode: "flexible", Ownership: "custom", Features: map[string]bool{},
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.Meta.BumpEpoch(t.Context()); err != nil {
		t.Fatal(err)
	}

	pkg := deploy.DefaultCustomerPackage
	art := &deploy.BundleArtifact{
		ManifestVersion:    1,
		Ownership:          "custom",
		DefaultPackageName: pkg,
		Objects: []deploy.SnapshotObject{
			{APIName: obj, Label: "DX Obj Changed", PluralLabel: "DX Objs", StorageMode: "flexible", Ownership: "custom", PackageName: &pkg, Features: map[string]bool{}},
			{APIName: "OnlyLocal__c", Label: "Only Local", PluralLabel: "Only Locals", StorageMode: "flexible", Ownership: "custom", PackageName: &pkg, Features: map[string]bool{}},
		},
		Fields:          []deploy.SnapshotField{},
		ValidationRules: []deploy.SnapshotRule{},
		Automations:     []deploy.SnapshotAutomation{},
		AgentPlaybooks:  []deploy.SnapshotAgentPlaybook{},
		PermissionSets:  []deploy.SnapshotPermissionSet{},
		Webhooks:        []deploy.SnapshotWebhook{},
		Tests:           []deploy.SnapshotTestSuite{},
		Sources:         map[string]string{},
	}

	body, _ := json.Marshal(map[string]any{"artifact": art, "label": "dx-test"})
	req := httptest.NewRequest(http.MethodPost, "/deploy/v1/packages/validate-local", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ts.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var result deploy.ValidateLocalResult
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.BundleID == "" || result.Checksum == "" {
		t.Fatalf("missing ids: %+v", result)
	}
	if result.Diff == nil || result.Diff.Counts.Add < 1 || result.Diff.Counts.Change < 1 {
		t.Fatalf("expected add+change diff, got %+v", result.Diff)
	}
	if result.Validation == nil || !result.OK {
		t.Fatalf("expected ok validation, got %+v", result.Validation)
	}

	// Re-validate by bundleId.
	rr2 := testutil.AuthRequest(ts.Handler, http.MethodPost, "/deploy/v1/packages/validate-local", "admin-key", map[string]any{
		"bundleId": result.BundleID,
	})
	if rr2.Code != http.StatusOK {
		t.Fatalf("bundleId validate: %d %s", rr2.Code, rr2.Body.String())
	}
}

func TestPackageValidateLocalAsyncAndBusy(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:            "admin-key+admin",
		DeploySyncMaxFiles: 1,
		DeployQueueMax:     8,
	})

	pkg := deploy.DefaultCustomerPackage
	art := &deploy.BundleArtifact{
		ManifestVersion:    1,
		Ownership:          "custom",
		DefaultPackageName: pkg,
		Objects: []deploy.SnapshotObject{
			{APIName: "AsyncA__c", Label: "A", PluralLabel: "As", StorageMode: "flexible", Ownership: "custom", PackageName: &pkg, Features: map[string]bool{}},
			{APIName: "AsyncB__c", Label: "B", PluralLabel: "Bs", StorageMode: "flexible", Ownership: "custom", PackageName: &pkg, Features: map[string]bool{}},
		},
		Fields:          []deploy.SnapshotField{},
		ValidationRules: []deploy.SnapshotRule{},
		Automations:     []deploy.SnapshotAutomation{},
		AgentPlaybooks:  []deploy.SnapshotAgentPlaybook{},
		PermissionSets:  []deploy.SnapshotPermissionSet{},
		Webhooks:        []deploy.SnapshotWebhook{},
		Tests:           []deploy.SnapshotTestSuite{},
		Sources:         map[string]string{},
	}
	rr := testutil.AuthRequest(ts.Handler, http.MethodPost, "/deploy/v1/packages/validate-local", "admin-key", map[string]any{
		"artifact": art, "label": "async-dx",
	})
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d %s", rr.Code, rr.Body.String())
	}
	var queued deploy.QueuedWork
	if err := json.Unmarshal(rr.Body.Bytes(), &queued); err != nil {
		t.Fatal(err)
	}
	if !queued.Accepted || queued.JobID == "" || queued.Poll == "" || queued.BundleID == "" {
		t.Fatalf("queued=%+v", queued)
	}
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM jobs WHERE id=$1::uuid`, queued.JobID)
	})

	work := testutil.AuthRequest(ts.Handler, http.MethodGet, queued.Poll, "admin-key", nil)
	if work.Code != http.StatusOK {
		t.Fatalf("poll %d %s", work.Code, work.Body.String())
	}
	var status deploy.WorkStatus
	if err := json.Unmarshal(work.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.JobType != deploy.JobTypeValidate || status.Status != "pending" {
		t.Fatalf("work=%+v", status)
	}

	busyTS := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:            "admin-key+admin",
		DeploySyncMaxFiles: 1,
		DeployQueueMax:     1,
	})
	busy := testutil.AuthRequest(busyTS.Handler, http.MethodPost, "/deploy/v1/packages/validate-local", "admin-key", map[string]any{
		"artifact": art, "label": "busy-dx",
	})
	if busy.Code != http.StatusTooManyRequests {
		t.Fatalf("expected DEPLOY_BUSY 429, got %d %s", busy.Code, busy.Body.String())
	}
	var errBody map[string]any
	if err := json.Unmarshal(busy.Body.Bytes(), &errBody); err != nil {
		t.Fatal(err)
	}
	if errBody["error"] != "DEPLOY_BUSY" {
		t.Fatalf("error=%v", errBody["error"])
	}
}
