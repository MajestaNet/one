package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/agentloop"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/inference"
	"github.com/MajestaNet/ide/internal/secretcrypt"
	"github.com/MajestaNet/ide/internal/testutil"
)

func setupLoopHTTP(t *testing.T, toolName string, args map[string]any) (*testutil.Database, *testutil.TestServer, *httptest.Server) {
	t.Helper()
	d := testutil.RequireDatabase(t)
	testutil.LockInferenceConfig(t, d.Pool)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{APIKeys: "admin+admin"})
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var req inference.ChatRequest
		_ = json.Unmarshal(raw, &req)
		last := ""
		if n := len(req.Messages); n > 0 {
			last = req.Messages[n-1].Role
		}
		var msg map[string]any
		finish := "stop"
		if last == "tool" {
			msg = map[string]any{"role": "assistant", "content": "ok"}
		} else {
			msg = map[string]any{
				"role": "assistant", "content": "",
				"tool_calls": []inference.ToolCall{inference.NewToolCall("call_1", toolName, args)},
			}
			finish = "tool_calls"
		}
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			payload, _ := json.Marshal(map[string]any{
				"choices": []any{map[string]any{"delta": msg, "finish_reason": finish}},
			})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
			_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": msg, "finish_reason": finish}},
		})
	}))
	t.Cleanup(mock.Close)

	ctx := t.Context()
	prov := "http_" + strings.ReplaceAll(t.Name(), "/", "_")
	if len(prov) > 60 {
		prov = prov[:60]
	}
	ref := "inference." + prov
	ct, _ := secretcrypt.Encrypt("sk-test", "")
	if err := db.UpsertInstallSecret(ctx, d.Pool, ref, "t", ct); err != nil {
		t.Fatal(err)
	}
	if err := inference.UpsertProvider(ctx, d.Pool, inference.Provider{
		APIName: prov, Label: "t", BaseURL: mock.URL + "/v1", SecretRef: &ref, DefaultModel: "test", Active: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := inference.PatchBYOConfig(ctx, d.Pool, true, &prov); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(context.Background(), `UPDATE install_inference_config SET active_source='none', default_provider_api_name=NULL WHERE id=1`)
		_ = inference.DeleteProvider(context.Background(), d.Pool, prov)
		_ = db.DeleteInstallSecret(context.Background(), d.Pool, ref)
	})
	return d, ts, mock
}

func insertWritePlaybook(t *testing.T, pool *db.Pool) string {
	t.Helper()
	name := "LoopWritePB_" + strings.ReplaceAll(t.Name(), "/", "_")
	if len(name) > 80 {
		name = name[:80]
	}
	_, _ = pool.Exec(t.Context(), `DELETE FROM agent_playbooks WHERE api_name=$1`, name)
	_, err := pool.Exec(t.Context(), `
INSERT INTO agent_playbooks (api_name, label, goal_template, instructions, require_approval, active, ownership, package_name,
  allowed_tools, primary_section, job_class, harness_id, harness_version)
VALUES ($1,'Loop write','Create a record','Create when asked',true,true,'custom','customer.default',
  '["sobjects.write","query"]'::jsonb, 'run', 'operate', 'harness.operate.mutate', '1')`, name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM agent_playbooks WHERE api_name=$1`, name) })
	return name
}

func TestSSEApproveParkedWriteDoesNotEnqueue(t *testing.T) {
	acct := "SSEPark " + time.Now().Format("150405.000")
	d, ts, _ := setupLoopHTTP(t, "create_record", map[string]any{
		"object": "Account", "data": map[string]any{"Name": acct},
	})
	pb := insertWritePlaybook(t, d.Pool)
	rec := createAgentRunHTTP(t, ts.Handler, map[string]any{
		"goal":            "create it",
		"playbookApiName": pb,
		"approved":        false,
		"stream":          true,
	}, "text/event-stream")
	if rec.Code != http.StatusOK {
		t.Fatalf("create stream: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "approval_required") {
		t.Fatalf("expected approval_required SSE: %s", body)
	}
	runID := sseRunID(t, body)
	t.Cleanup(func() {
		cleanupAgentRun(t, d.Pool, runID)
		_, _ = d.Pool.Exec(context.Background(), `DELETE FROM records WHERE object_api_name='Account' AND data->>'Name'=$1`, acct)
	})
	var status string
	_ = d.Pool.QueryRow(t.Context(), `SELECT status FROM agent_runs WHERE id=$1::uuid`, runID).Scan(&status)
	if status != agentloop.StatusAwaitingToolApproval {
		t.Fatalf("status=%s", status)
	}
	if n := countAgentRunJobs(t, d.Pool, runID); n != 0 {
		t.Fatalf("stream create must not enqueue, got %d", n)
	}

	req := httptest.NewRequest(http.MethodPost, "/client/v1/agents/runs/"+runID+"/approve", bytes.NewReader([]byte(`{"stream":true}`)))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	approve := httptest.NewRecorder()
	ts.Handler.ServeHTTP(approve, req)
	if approve.Code != http.StatusOK || !strings.Contains(approve.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("approve SSE: %d %s", approve.Code, approve.Body.String())
	}
	if n := countAgentRunJobs(t, d.Pool, runID); n != 0 {
		t.Fatalf("SSE approve must not enqueue, got %d", n)
	}
	_ = d.Pool.QueryRow(t.Context(), `SELECT status FROM agent_runs WHERE id=$1::uuid`, runID).Scan(&status)
	if status != "completed" {
		t.Fatalf("after SSE approve status=%s body=%s", status, approve.Body.String())
	}
	var n int
	_ = d.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM records WHERE object_api_name='Account' AND data->>'Name'=$1`, acct).Scan(&n)
	if n != 1 {
		t.Fatalf("expected create after SSE approve, count=%d", n)
	}
}

func TestJSONApproveParkedWriteEnqueuesResume(t *testing.T) {
	acct := "JSONPark " + time.Now().Format("150405.000")
	d, ts, _ := setupLoopHTTP(t, "create_record", map[string]any{
		"object": "Account", "data": map[string]any{"Name": acct},
	})
	pb := insertWritePlaybook(t, d.Pool)
	rec := createAgentRunHTTP(t, ts.Handler, map[string]any{
		"goal":            "create it",
		"playbookApiName": pb,
		"stream":          true,
	}, "text/event-stream")
	runID := sseRunID(t, rec.Body.String())
	t.Cleanup(func() {
		cleanupAgentRun(t, d.Pool, runID)
		_, _ = d.Pool.Exec(context.Background(), `DELETE FROM records WHERE object_api_name='Account' AND data->>'Name'=$1`, acct)
	})
	req := httptest.NewRequest(http.MethodPost, "/client/v1/agents/runs/"+runID+"/approve", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	approve := httptest.NewRecorder()
	ts.Handler.ServeHTTP(approve, req)
	if approve.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", approve.Code, approve.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(approve.Body.Bytes(), &out)
	if out["resume"] != true {
		t.Fatalf("expected resume true: %s", approve.Body.String())
	}
	if n := countAgentRunJobs(t, d.Pool, runID); n != 1 {
		t.Fatalf("JSON approve should enqueue one resume job, got %d", n)
	}
	var resume any
	_ = d.Pool.QueryRow(t.Context(), `SELECT payload->'resume' FROM jobs WHERE job_type='agent.run' AND payload->>'runId'=$1`, runID).Scan(&resume)
	if fmt.Sprint(resume) != "true" {
		t.Fatalf("job resume=%v", resume)
	}
}

func TestStreamDryRunDoesNotExecuteTools(t *testing.T) {
	acct := "DryHTTPMustNotExist"
	d, ts, _ := setupLoopHTTP(t, "create_record", map[string]any{
		"object": "Account", "data": map[string]any{"Name": acct},
	})
	pb := insertWritePlaybook(t, d.Pool)
	rec := createAgentRunHTTP(t, ts.Handler, map[string]any{
		"goal":            "create it",
		"playbookApiName": pb,
		"dryRun":          true,
		"stream":          true,
	}, "text/event-stream")
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	runID := sseRunID(t, rec.Body.String())
	t.Cleanup(func() { cleanupAgentRun(t, d.Pool, runID) })
	var status string
	_ = d.Pool.QueryRow(t.Context(), `SELECT status FROM agent_runs WHERE id=$1::uuid`, runID).Scan(&status)
	if status != "dry_run_complete" {
		t.Fatalf("status=%s body=%s", status, rec.Body.String())
	}
	var n int
	_ = d.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM records WHERE object_api_name='Account' AND data->>'Name'=$1`, acct).Scan(&n)
	if n != 0 {
		t.Fatal("dry-run executed create_record")
	}
}
