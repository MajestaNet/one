package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestAwaitOrgWorkPollsUntilCompleted(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/deploy/v1/work/job-1" {
			http.NotFound(w, r)
			return
		}
		if n.Add(1) == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jobId": "job-1", "jobType": "deploy.validate", "status": "pending",
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jobId": "job-1", "jobType": "deploy.validate", "status": "completed",
			"result": map[string]any{"bundleId": "b1", "ok": true},
		})
	}))
	t.Cleanup(srv.Close)

	raw, err := awaitOrgWork(&resolvedOrg{BaseURL: srv.URL, APIKey: "k"}, []byte(`{"jobId":"job-1","poll":"/deploy/v1/work/job-1","accepted":true,"status":"queued"}`))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result["bundleId"] != "b1" || result["ok"] != true {
		t.Fatalf("result=%v", result)
	}
	if n.Load() < 2 {
		t.Fatalf("expected poll retry, calls=%d", n.Load())
	}
}

func TestAwaitOrgWorkFailedJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jobId": "job-2", "status": "failed", "lastError": "boom",
		})
	}))
	t.Cleanup(srv.Close)
	_, err := awaitOrgWork(&resolvedOrg{BaseURL: srv.URL, APIKey: "k"}, []byte(`{"jobId":"job-2"}`))
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err=%v", err)
	}
}
