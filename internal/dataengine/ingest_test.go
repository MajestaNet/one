package dataengine_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
)

func TestCreateIngestJobValidation(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatal(err)
	}
	meta := metadata.NewService(pool)
	svc := dataengine.NewService(pool, meta)
	ownerID := "00000000-0000-4000-8000-000000000001"
	actor := &authz.Actor{ID: ownerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}

	if _, err := svc.CreateIngestJob(ctx, nil, dataengine.CreateIngestJobInput{ObjectAPIName: "Account", Operation: "insert"}); err == nil {
		t.Fatal("expected missing actor")
	}
	if _, err := svc.CreateIngestJob(ctx, actor, dataengine.CreateIngestJobInput{ObjectAPIName: "Account", Operation: "nope"}); err == nil {
		t.Fatal("expected bad operation")
	}
	if _, err := svc.CreateIngestJob(ctx, actor, dataengine.CreateIngestJobInput{Operation: "insert"}); err == nil {
		t.Fatal("expected missing object")
	}
	if _, err := svc.CreateIngestJob(ctx, actor, dataengine.CreateIngestJobInput{
		ObjectAPIName: "Account", Operation: "upsert",
	}); err == nil {
		t.Fatal("expected missing externalIdField for upsert")
	}
	if _, err := svc.CreateIngestJob(ctx, actor, dataengine.CreateIngestJobInput{
		ObjectAPIName: "Account", Operation: "insert", ContentType: "text/csv",
	}); err == nil || !strings.Contains(err.Error(), "contentType") {
		t.Fatalf("expected contentType error, got %v", err)
	}
}

func TestIngestInsertProcessAndPartialFailure(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatal(err)
	}
	ownerID := "00000000-0000-4000-8000-000000000001"
	store := db.NewUserStore(pool)
	if _, err := store.EnsureBootstrapAdmin(ctx, ownerID, "admin@one.local", "Admin"); err != nil {
		t.Fatal(err)
	}
	obj := "IngestDE" + time.Now().Format("150405")
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM ingest_jobs WHERE object_api_name=$1`, obj)
		_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name=$1`, obj)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_fields WHERE object_api_name=$1`, obj)
		_, _ = pool.Exec(ctx, `DELETE FROM object_permissions WHERE object_api_name=$1`, obj)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name=$1`, obj)
	})
	if _, err := pool.Exec(ctx, `
INSERT INTO metadata_objects (api_name, label, plural_label, storage_mode, ownership, features)
VALUES ($1, $1, $1, 'flexible', 'custom', '{}'::jsonb)`, obj); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO metadata_fields (object_api_name, api_name, label, field_type, required, unique_field, ownership, filterable, sortable, indexed)
VALUES ($1, 'Name', 'Name', 'text', true, false, 'custom', true, true, true)`, obj); err != nil {
		t.Fatal(err)
	}
	_ = db.EnsureObjectInDataAccessCatalog(ctx, pool, obj)
	_, _ = pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)

	meta := metadata.NewService(pool)
	svc := dataengine.NewService(pool, meta)
	actor := &authz.Actor{ID: ownerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}
	job, err := svc.CreateIngestJob(ctx, actor, dataengine.CreateIngestJobInput{
		ObjectAPIName: obj, Operation: dataengine.IngestOpInsert,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendIngestBatch(ctx, job.ID, actor.ID, []byte(`{"Name":"One"}`+"\n"+`{}`+"\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CloseIngestJob(ctx, job.ID, actor.ID); err != nil {
		t.Fatal(err)
	}

	az := permissiveUpsertAuthz()
	if err := svc.ProcessIngestJob(ctx, job.ID, az, func(context.Context, string) (*authz.Actor, error) {
		return actor, nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetIngestJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != dataengine.IngestStateJobComplete {
		t.Fatalf("state=%s err=%v", got.State, got.ErrorMessage)
	}
	if got.SuccessCount != 1 || got.FailureCount != 1 {
		t.Fatalf("success=%d failure=%d", got.SuccessCount, got.FailureCount)
	}
	failed, err := svc.IngestJobResults(ctx, job.ID, actor.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(failed), `"line":2`) {
		t.Fatalf("failed results=%s", failed)
	}
	okBytes, err := svc.IngestJobResults(ctx, job.ID, actor.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(okBytes), `"created":true`) {
		t.Fatalf("success results=%s", okBytes)
	}

	// Simulate a worker that committed its first chunk and exited before marking
	// the job complete. A retry must resume at line 2, not insert line 1 again.
	retryJob, err := svc.CreateIngestJob(ctx, actor, dataengine.CreateIngestJobInput{
		ObjectAPIName: obj, Operation: dataengine.IngestOpInsert,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendIngestBatch(ctx, retryJob.ID, actor.ID, []byte(
		`{"Name":"Retry One"}`+"\n"+`{"Name":"Retry Two"}`+"\n",
	)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CloseIngestJob(ctx, retryJob.ID, actor.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, obj, map[string]any{"Name": "Retry One"}, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
UPDATE ingest_jobs
SET state='InProgress', success_count=1,
    result_success=$2::bytea
WHERE id=$1::uuid`, retryJob.ID, []byte("{\"created\":true,\"line\":1}\n")); err != nil {
		t.Fatal(err)
	}
	if err := svc.ProcessIngestJob(ctx, retryJob.ID, az, func(context.Context, string) (*authz.Actor, error) {
		return actor, nil
	}); err != nil {
		t.Fatal(err)
	}
	var retryRecords int
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM records
WHERE object_api_name=$1 AND data->>'Name' IN ('Retry One','Retry Two')`, obj).Scan(&retryRecords); err != nil {
		t.Fatal(err)
	}
	if retryRecords != 2 {
		t.Fatalf("retry replayed a committed insert: got %d records, want 2", retryRecords)
	}
	retried, err := svc.GetIngestJob(ctx, retryJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retried.State != dataengine.IngestStateJobComplete || retried.SuccessCount != 2 {
		t.Fatalf("resumed ingest = %+v", retried)
	}
	retryResults, err := svc.IngestJobResults(ctx, retryJob.ID, actor.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(retryResults), `"line":1`) || !strings.Contains(string(retryResults), `"line":2`) {
		t.Fatalf("resumed results=%s", retryResults)
	}
	// A duplicate delivery after completion is harmless and cannot overwrite a
	// successful terminal state with Failed.
	if err := svc.ProcessIngestJob(ctx, retryJob.ID, az, func(context.Context, string) (*authz.Actor, error) {
		return actor, nil
	}); err != nil {
		t.Fatalf("duplicate completion: %v", err)
	}
	retried, err = svc.GetIngestJob(ctx, retryJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retried.State != dataengine.IngestStateJobComplete || retried.SuccessCount != 2 {
		t.Fatalf("duplicate completion changed ingest = %+v", retried)
	}

	if _, err := svc.GetIngestJob(ctx, "00000000-0000-4000-8000-000000000099"); err == nil {
		t.Fatal("expected missing job")
	}
}

func TestAbortIngestJobAndUpsertUpdateDelete(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatal(err)
	}
	ownerID := "00000000-0000-4000-8000-000000000001"
	if _, err := db.NewUserStore(pool).EnsureBootstrapAdmin(ctx, ownerID, "admin@one.local", "Admin"); err != nil {
		t.Fatal(err)
	}
	obj := "IngestMut" + time.Now().Format("150405")
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM ingest_jobs WHERE object_api_name=$1`, obj)
		_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name=$1`, obj)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_fields WHERE object_api_name=$1`, obj)
		_, _ = pool.Exec(ctx, `DELETE FROM object_permissions WHERE object_api_name=$1`, obj)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name=$1`, obj)
	})
	if _, err := pool.Exec(ctx, `
INSERT INTO metadata_objects (api_name, label, plural_label, storage_mode, ownership, features)
VALUES ($1, $1, $1, 'flexible', 'custom', '{}'::jsonb)`, obj); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO metadata_fields (object_api_name, api_name, label, field_type, required, unique_field, ownership, filterable, sortable, indexed, external_id)
VALUES
  ($1, 'Name', 'Name', 'text', true, false, 'custom', true, true, true, false),
  ($1, 'ERP_Id__c', 'ERP Id', 'text', false, true, 'custom', true, true, true, true)`, obj); err != nil {
		t.Fatal(err)
	}
	_ = db.EnsureObjectInDataAccessCatalog(ctx, pool, obj)
	_, _ = pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)

	meta := metadata.NewService(pool)
	svc := dataengine.NewService(pool, meta)
	actor := &authz.Actor{ID: ownerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}
	az := permissiveUpsertAuthz()
	resolve := func(context.Context, string) (*authz.Actor, error) { return actor, nil }

	open, err := svc.CreateIngestJob(ctx, actor, dataengine.CreateIngestJobInput{ObjectAPIName: obj, Operation: dataengine.IngestOpInsert})
	if err != nil {
		t.Fatal(err)
	}
	aborted, err := svc.AbortIngestJob(ctx, open.ID, actor.ID)
	if err != nil || aborted.State != dataengine.IngestStateAborted {
		t.Fatalf("abort: %+v err=%v", aborted, err)
	}
	if _, err := svc.AbortIngestJob(ctx, open.ID, actor.ID); err == nil {
		t.Fatal("second abort should fail")
	}

	seed, err := svc.Create(ctx, obj, map[string]any{"Name": "Seed", "ERP_Id__c": "ERP-SEED"}, actor)
	if err != nil {
		t.Fatal(err)
	}

	upsertJob, err := svc.CreateIngestJob(ctx, actor, dataengine.CreateIngestJobInput{
		ObjectAPIName: obj, Operation: dataengine.IngestOpUpsert, ExternalIDField: "ERP_Id__c",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendIngestBatch(ctx, upsertJob.ID, actor.ID, []byte(
		`{"Name":"Seed2","ERP_Id__c":"ERP-SEED"}`+"\n"+`{"Name":"New","ERP_Id__c":"ERP-NEW"}`+"\n",
	)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CloseIngestJob(ctx, upsertJob.ID, actor.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.ProcessIngestJob(ctx, upsertJob.ID, az, resolve); err != nil {
		t.Fatal(err)
	}

	updateJob, err := svc.CreateIngestJob(ctx, actor, dataengine.CreateIngestJobInput{
		ObjectAPIName: obj, Operation: dataengine.IngestOpUpdate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendIngestBatch(ctx, updateJob.ID, actor.ID, []byte(`{"Id":"`+seed["Id"].(string)+`","Name":"Patched"}`+"\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CloseIngestJob(ctx, updateJob.ID, actor.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.ProcessIngestJob(ctx, updateJob.ID, az, resolve); err != nil {
		t.Fatal(err)
	}

	deleteJob, err := svc.CreateIngestJob(ctx, actor, dataengine.CreateIngestJobInput{
		ObjectAPIName: obj, Operation: dataengine.IngestOpDelete, ExternalIDField: "ERP_Id__c",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendIngestBatch(ctx, deleteJob.ID, actor.ID, []byte(`{"ERP_Id__c":"ERP-NEW"}`+"\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CloseIngestJob(ctx, deleteJob.ID, actor.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.ProcessIngestJob(ctx, deleteJob.ID, az, resolve); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetByExternalID(ctx, obj, "ERP_Id__c", "ERP-NEW"); err == nil {
		t.Fatal("expected deleted upserted row")
	}
}

func permissiveUpsertAuthz() *dataengine.UpsertAuthz {
	return &dataengine.UpsertAuthz{
		AssertObjectAccess: func(context.Context, *authz.Actor, string, authz.CrudAction) error { return nil },
		CanModifyRecord: func(context.Context, *authz.Actor, string, string, string, string, map[string]struct{}) (bool, error) {
			return true, nil
		},
		GetModifyAllObjects: func(context.Context, *authz.Actor) (map[string]struct{}, error) {
			return map[string]struct{}{}, nil
		},
		AssertEditableFields: func(context.Context, *authz.Actor, string, map[string]any) error { return nil },
		StripUnreadableFields: func(_ context.Context, _ *authz.Actor, _ string, rec map[string]any) (map[string]any, error) {
			return rec, nil
		},
	}
}
