package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/testutil"
	"github.com/MajestaNet/ide/internal/worker"
)

func TestClientIngestInsertAndAbort(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin,client-key:client",
	})
	obj := "IngestItem" + time.Now().Format("150405")
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM ingest_jobs WHERE object_api_name=$1`, obj)
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM jobs WHERE job_type='ingest.process' AND payload->>'ingestJobId' IS NOT NULL`)
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM records WHERE object_api_name=$1`, obj)
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM metadata_fields WHERE object_api_name=$1`, obj)
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM object_permissions WHERE object_api_name=$1`, obj)
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM metadata_objects WHERE api_name=$1`, obj)
	})

	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/objects", "admin-key", map[string]any{
		"apiName": obj, "label": obj, "pluralLabel": obj + "s",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create object: %d %s", rr.Code, rr.Body.String())
	}
	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/fields", "admin-key", map[string]any{
		"objectApiName": obj, "apiName": "Name", "label": "Name", "fieldType": "text", "required": true,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create Name: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/jobs/ingest", "admin-key", map[string]any{
		"object": obj, "operation": "insert",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create job: %d %s", rr.Code, rr.Body.String())
	}
	var job map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &job)
	jobID, _ := job["id"].(string)
	if jobID == "" || job["state"] != dataengine.IngestStateOpen {
		t.Fatalf("job=%v", job)
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/jobs/ingest", "admin-key", map[string]any{
		"object": obj, "operation": "explode",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad op: %d %s", rr.Code, rr.Body.String())
	}

	ndjson := []byte(`{"Name":"Alpha"}` + "\n" + `{"Name":"Beta"}` + "\n")
	rr = authRaw(t, srv.Handler, http.MethodPut, "/client/v1/jobs/ingest/"+jobID+"/batches", "admin-key", "application/x-ndjson", ndjson)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("batch: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/jobs/ingest/"+jobID, "client-key", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("non-owner get: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPatch, "/client/v1/jobs/ingest/"+jobID, "admin-key", map[string]any{
		"state": dataengine.IngestStateUploadComplete,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("close: %d %s", rr.Code, rr.Body.String())
	}

	var workerJobID string
	if err := d.Pool.QueryRow(t.Context(), `
SELECT id::text FROM jobs WHERE job_type='ingest.process' AND payload->>'ingestJobId'=$1
ORDER BY created_at DESC LIMIT 1`, jobID).Scan(&workerJobID); err != nil {
		t.Fatalf("lookup ingest.process job: %v", err)
	}
	objectAz := &authz.ObjectAuthz{Store: &db.ObjectPermStore{Pool: d.Pool}}
	fieldAz := &authz.FieldAuthz{Store: &db.FieldPermStore{Pool: d.Pool}}
	if _, err := worker.ProcessJobs(t.Context(), d.Pool, &worker.ProcessOptions{
		DataEngine: srv.Data,
		ObjectAz:   objectAz,
		FieldAz:    fieldAz,
		JobID:      workerJobID,
	}); err != nil {
		t.Fatalf("process ingest: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var state string
	for time.Now().Before(deadline) {
		rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/jobs/ingest/"+jobID, "admin-key", nil)
		_ = json.Unmarshal(rr.Body.Bytes(), &job)
		state, _ = job["state"].(string)
		if state == dataengine.IngestStateJobComplete {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if state != dataengine.IngestStateJobComplete {
		t.Fatalf("state=%s job=%v", state, job)
	}
	if intVal(job["successCount"]) != 2 {
		t.Fatalf("successCount=%v", job["successCount"])
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/jobs/ingest/"+jobID+"/successfulResults", "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("success results: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"created":true`) {
		t.Fatalf("results=%s", rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/query", "admin-key", map[string]any{
		"object": obj, "limit": 10,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("query: %d %s", rr.Code, rr.Body.String())
	}
	var qres map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &qres)
	if len(asSlice(qres["records"])) != 2 {
		t.Fatalf("records=%v", qres)
	}

	abortRR := testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/jobs/ingest", "admin-key", map[string]any{
		"object": obj, "operation": "insert",
	})
	var abortJob map[string]any
	_ = json.Unmarshal(abortRR.Body.Bytes(), &abortJob)
	abortID, _ := abortJob["id"].(string)
	rr = testutil.AuthRequest(srv.Handler, http.MethodDelete, "/client/v1/jobs/ingest/"+abortID, "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("abort: %d %s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &abortJob)
	if abortJob["state"] != dataengine.IngestStateAborted {
		t.Fatalf("aborted=%v", abortJob)
	}
}

func TestClientUpsertByExternalID(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin,client-key:client",
	})
	obj := "UpsertItem" + time.Now().Format("150405")
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM records WHERE object_api_name=$1`, obj)
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM metadata_fields WHERE object_api_name=$1`, obj)
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM object_permissions WHERE object_api_name=$1`, obj)
		_, _ = d.Pool.Exec(t.Context(), `DELETE FROM metadata_objects WHERE api_name=$1`, obj)
	})

	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/objects", "admin-key", map[string]any{
		"apiName": obj, "label": obj, "pluralLabel": obj + "s",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create object: %d %s", rr.Code, rr.Body.String())
	}
	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/fields", "admin-key", map[string]any{
		"objectApiName": obj, "apiName": "Name", "label": "Name", "fieldType": "text", "required": true,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create Name: %d %s", rr.Code, rr.Body.String())
	}
	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/metadata/v1/fields", "admin-key", map[string]any{
		"objectApiName": obj, "apiName": "ERP_Id__c", "label": "ERP Id", "fieldType": "text", "externalId": true,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create external id: %d %s", rr.Code, rr.Body.String())
	}

	ext := "ERP-" + time.Now().Format("150405.000")
	rr = testutil.AuthRequest(srv.Handler, http.MethodPatch, "/client/v1/sobjects/"+obj+"/ERP_Id__c/"+ext, "admin-key", map[string]any{
		"Name": "First",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("upsert create: %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	if created["created"] != true || created["Name"] != "First" {
		t.Fatalf("created=%v", created)
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/sobjects/"+obj+"/ERP_Id__c/"+ext, "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get by ext: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/sobjects/"+obj+"/upsert", "admin-key", map[string]any{
		"externalIdField": "ERP_Id__c",
		"externalId":      ext,
		"Name":            "Updated",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("upsert update: %d %s", rr.Code, rr.Body.String())
	}
	var updated map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &updated)
	if updated["created"] != false || updated["Name"] != "Updated" {
		t.Fatalf("updated=%v", updated)
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/sobjects/"+obj+"/upsert", "admin-key", map[string]any{
		"Name": "NoKey",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing externalIdField: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodDelete, "/client/v1/sobjects/"+obj+"/ERP_Id__c/"+ext, "admin-key", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete by ext: %d %s", rr.Code, rr.Body.String())
	}
	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/sobjects/"+obj+"/ERP_Id__c/"+ext, "admin-key", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("get after delete: %d %s", rr.Code, rr.Body.String())
	}
}

func authRaw(t *testing.T, h http.Handler, method, path, bearer, contentType string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+bearer)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func intVal(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}
