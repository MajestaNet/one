package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/testutil"
)

func lockUnsetInference(t *testing.T, pool *db.Pool) {
	t.Helper()
	testutil.LockInferenceConfig(t, pool)
	testutil.ResetInferenceConfig(t, pool)
}

func countAgentRunJobs(t *testing.T, pool *db.Pool, runID string) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM jobs WHERE job_type='agent.run' AND payload->>'runId'=$1`, runID).Scan(&n)
	if err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	return n
}

func cleanupAgentRun(t *testing.T, pool *db.Pool, runID string) {
	t.Helper()
	if runID == "" {
		return
	}
	_, _ = pool.Exec(context.Background(), `DELETE FROM agent_run_events WHERE run_id=$1::uuid`, runID)
	_, _ = pool.Exec(context.Background(), `DELETE FROM jobs WHERE job_type='agent.run' AND payload->>'runId'=$1`, runID)
	_, _ = pool.Exec(context.Background(), `DELETE FROM agent_runs WHERE id=$1::uuid`, runID)
}

func createAgentRunHTTP(t *testing.T, srv http.Handler, body map[string]any, accept string) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/client/v1/agents/runs", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestStreamCreateSkipsAwaitingApproval(t *testing.T) {
	_, pool, srv := setupPlaybookTest(t)
	lockUnsetInference(t, pool)
	rec := createAgentRunHTTP(t, srv.Handler(), map[string]any{
		"goal":            "say hello",
		"playbookApiName": "RunCoach",
		"approved":        false,
		"stream":          true,
	}, "text/event-stream")
	ct := rec.Header().Get("Content-Type")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 SSE, got %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("expected event-stream, got %q body %s", ct, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, `"status":"awaiting_approval"`) {
		t.Fatalf("stream create parked on approval: %s", body)
	}
	if !strings.Contains(body, "INFERENCE_NOT_CONFIGURED") {
		t.Fatalf("expected inference error SSE, got %s", body)
	}
	runID := sseRunID(t, body)
	t.Cleanup(func() { cleanupAgentRun(t, pool, runID) })
	if n := countAgentRunJobs(t, pool, runID); n != 0 {
		t.Fatalf("stream create must not enqueue worker job, got %d", n)
	}
}

func TestNonStreamCreateParksForApproval(t *testing.T) {
	_, pool, srv := setupPlaybookTest(t)
	rec := createAgentRunHTTP(t, srv.Handler(), map[string]any{
		"goal":            "change data",
		"playbookApiName": "RunCoach",
		"approved":        false,
	}, "application/json")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["status"] != "awaiting_approval" {
		t.Fatalf("expected awaiting_approval, got %v", out["status"])
	}
	runID, _ := out["id"].(string)
	if runID == "" {
		t.Fatal("missing run id")
	}
	t.Cleanup(func() { cleanupAgentRun(t, pool, runID) })
	if n := countAgentRunJobs(t, pool, runID); n != 0 {
		t.Fatalf("parked run must not enqueue job, got %d", n)
	}
	detail := "/client/v1/agents/runs/" + runID
	stream := detail + "/stream"
	allowed := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, detail, nil)
	req.Header.Set("Authorization", "Bearer admin")
	srv.Handler().ServeHTTP(allowed, req)
	if allowed.Code != http.StatusOK {
		t.Fatalf("owner/admin read %s: got %d %s", detail, allowed.Code, allowed.Body.String())
	}
	for _, path := range []string{detail, stream} {
		denied := httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer clientonly")
		srv.Handler().ServeHTTP(denied, req)
		if denied.Code != http.StatusNotFound {
			t.Fatalf("cross-principal read %s: got %d %s", path, denied.Code, denied.Body.String())
		}
	}
}

func TestApproveJSONEnqueuesJob(t *testing.T) {
	_, pool, srv := setupPlaybookTest(t)
	created := createAgentRunHTTP(t, srv.Handler(), map[string]any{
		"goal":            "queued after approve",
		"playbookApiName": "RunCoach",
		"approved":        false,
	}, "application/json")
	var parked map[string]any
	_ = json.Unmarshal(created.Body.Bytes(), &parked)
	runID, _ := parked["id"].(string)
	if runID == "" {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	t.Cleanup(func() { cleanupAgentRun(t, pool, runID) })

	req := httptest.NewRequest(http.MethodPost, "/client/v1/agents/runs/"+runID+"/approve", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["status"] != "queued" {
		t.Fatalf("expected queued, got %v %s", out["status"], rec.Body.String())
	}
	if n := countAgentRunJobs(t, pool, runID); n != 1 {
		t.Fatalf("JSON approve should enqueue one job, got %d", n)
	}
}

func TestApproveSSEDoesNotEnqueueJob(t *testing.T) {
	_, pool, srv := setupPlaybookTest(t)
	lockUnsetInference(t, pool)
	created := createAgentRunHTTP(t, srv.Handler(), map[string]any{
		"goal":            "stream after approve",
		"playbookApiName": "RunCoach",
		"approved":        false,
	}, "application/json")
	var parked map[string]any
	_ = json.Unmarshal(created.Body.Bytes(), &parked)
	runID, _ := parked["id"].(string)
	if runID == "" {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	t.Cleanup(func() { cleanupAgentRun(t, pool, runID) })

	req := httptest.NewRequest(http.MethodPost, "/client/v1/agents/runs/"+runID+"/approve", bytes.NewReader([]byte(`{"stream":true}`)))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	ct := rec.Header().Get("Content-Type")
	if rec.Code != http.StatusOK || !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("expected SSE approve, got %d %s %s", rec.Code, ct, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "INFERENCE_NOT_CONFIGURED") {
		t.Fatalf("expected inference error SSE, got %s", rec.Body.String())
	}
	if n := countAgentRunJobs(t, pool, runID); n != 0 {
		t.Fatalf("SSE approve must not enqueue worker job, got %d", n)
	}
}

func TestApproveNotAwaitingReturnsNotFound(t *testing.T) {
	_, pool, srv := setupPlaybookTest(t)
	created := createAgentRunHTTP(t, srv.Handler(), map[string]any{
		"goal":            "already approved",
		"playbookApiName": "RunCoach",
		"approved":        true,
	}, "application/json")
	var out map[string]any
	_ = json.Unmarshal(created.Body.Bytes(), &out)
	runID, _ := out["id"].(string)
	if runID == "" {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	t.Cleanup(func() { cleanupAgentRun(t, pool, runID) })

	req := httptest.NewRequest(http.MethodPost, "/client/v1/agents/runs/"+runID+"/approve", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d %s", rec.Code, rec.Body.String())
	}
}

func sseRunID(t *testing.T, body string) string {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var payload map[string]any
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			continue
		}
		if id, ok := payload["id"].(string); ok && id != "" {
			return id
		}
	}
	t.Fatalf("no run id in SSE: %s", body)
	return ""
}
