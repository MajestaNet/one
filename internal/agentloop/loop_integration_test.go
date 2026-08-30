package agentloop_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/agentharness"
	"github.com/MajestaNet/ide/internal/agentloop"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/inference"
	"github.com/MajestaNet/ide/internal/mcp"
	"github.com/MajestaNet/ide/internal/secretcrypt"
	"github.com/MajestaNet/ide/internal/testutil"
	"github.com/MajestaNet/ide/internal/worker"
)

type llmScript struct {
	queryCount  atomic.Int32
	createCount atomic.Int32
	tool        string
	args        map[string]any
	proseOnly   bool
	graphOnly   bool
	forbidden   string
}

func (s *llmScript) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var req inference.ChatRequest
		_ = json.Unmarshal(body, &req)
		lastRole := ""
		if n := len(req.Messages); n > 0 {
			lastRole = req.Messages[n-1].Role
		}
		var msg map[string]any
		finish := "stop"
		if s.proseOnly {
			msg = map[string]any{"role": "assistant", "content": "Hello with no tools."}
		} else if s.graphOnly {
			msg = map[string]any{"role": "assistant", "content": "Pinned.\n```oneEffects\n" + `{"graphCalls":[{"tool":"graph.pin","input":{"ref":{"objectApiName":"Account"}}}]}` + "\n```"}
		} else if lastRole == "tool" {
			msg = map[string]any{"role": "assistant", "content": "Finished after tool."}
		} else {
			name := s.tool
			args := s.args
			if s.forbidden != "" {
				name = s.forbidden
				args = map[string]any{}
			}
			if name == "query" {
				s.queryCount.Add(1)
			}
			if name == "create_record" {
				s.createCount.Add(1)
			}
			call := inference.NewToolCall("call_1", name, args)
			msg = map[string]any{
				"role":       "assistant",
				"content":    "",
				"tool_calls": []inference.ToolCall{call},
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
	})
}

func setupLoopHarness(t *testing.T, script *llmScript) (*testutil.Database, *testutil.TestServer, agentloop.Config) {
	t.Helper()
	d := testutil.RequireDatabase(t)
	testutil.LockInferenceConfig(t, d.Pool)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{APIKeys: "loop-admin+admin"})
	mock := httptest.NewServer(script.handler())
	t.Cleanup(mock.Close)

	ctx := t.Context()
	prov := "loop_" + strings.ReplaceAll(t.Name(), "/", "_")
	if len(prov) > 60 {
		prov = prov[:60]
	}
	ref := "inference." + prov
	ct, err := secretcrypt.Encrypt("sk-test", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertInstallSecret(ctx, d.Pool, ref, "t", ct); err != nil {
		t.Fatal(err)
	}
	base := mock.URL + "/v1"
	if err := inference.UpsertProvider(ctx, d.Pool, inference.Provider{
		APIName: prov, Label: "loop", BaseURL: base, SecretRef: &ref, DefaultModel: "test", Active: true,
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

	applied := agentharness.Apply(agentharness.Spec{
		JobClass:        "operate",
		PrimarySection:  "run",
		AllowedTools:    []string{"query", "sobjects.write"},
		RequireApproval: true,
	})
	cfg := agentloop.Config{
		Pool: d.Pool,
		MCP: mcp.Deps{
			Meta:         d.Meta,
			Data:         ts.Data,
			Pool:         d.Pool,
			ObjectAz:     &authz.ObjectAuthz{Store: &db.ObjectPermStore{Pool: d.Pool}},
			FieldAz:      &authz.FieldAuthz{Store: &db.FieldPermStore{Pool: d.Pool}},
			RecordAccess: db.NewRecordAccessEvaluator(d.Pool),
			SystemAz:     &authz.SystemAuthz{Store: &db.AuthzSystemPerms{Store: db.NewSystemPermStore(d.Pool)}},
			AutomationAz: &authz.AutomationAuthz{Store: &db.AutomationPermStore{Pool: d.Pool}},
		},
		Applied:     applied,
		ResolveOpts: inference.ResolveOptions{AllowDevLocal: true},
	}
	return d, ts, cfg
}

func insertRun(t *testing.T, pool *db.Pool, actorID, goal string, dryRun bool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(t.Context(), `
INSERT INTO agent_runs (playbook_api_name, status, goal, input, actor_id, dry_run)
VALUES (NULL, 'queued', $1, '{}'::jsonb, $2::uuid, $3)
RETURNING id::text`, goal, actorID, dryRun).Scan(&id)
	if err != nil {
		t.Fatalf("insert run: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM agent_run_events WHERE run_id=$1::uuid`, id)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agent_runs WHERE id=$1::uuid`, id)
	})
	return id
}

func runActorID(t *testing.T, ts *testutil.TestServer) string {
	t.Helper()
	actor, err := ts.Resolver.ResolveAPIKey("loop-admin")
	if err != nil {
		t.Fatal(err)
	}
	return actor.ID
}

func TestHostedLoopExecutesReadQuery(t *testing.T) {
	script := &llmScript{tool: "query", args: map[string]any{"object": "Account", "limit": 1}}
	d, ts, cfg := setupLoopHarness(t, script)
	actorID := runActorID(t, ts)
	runID := insertRun(t, d.Pool, actorID, "list accounts", false)
	res, err := agentloop.Execute(t.Context(), cfg, agentloop.Input{RunID: runID, Goal: "list accounts"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != agentloop.StatusCompleted {
		t.Fatalf("status=%s err=%s", res.Status, res.Error)
	}
	if script.queryCount.Load() < 1 {
		t.Fatal("model did not request query")
	}
	var status string
	_ = d.Pool.QueryRow(t.Context(), `SELECT status FROM agent_runs WHERE id=$1::uuid`, runID).Scan(&status)
	if status != "completed" {
		t.Fatalf("db status=%s", status)
	}
}

func TestHostedLoopParksWriteThenResumeExecutes(t *testing.T) {
	name := "LoopWrite " + time.Now().Format("150405.000")
	script := &llmScript{tool: "create_record", args: map[string]any{
		"object": "Account", "data": map[string]any{"Name": name},
	}}
	d, ts, cfg := setupLoopHarness(t, script)
	actorID := runActorID(t, ts)
	runID := insertRun(t, d.Pool, actorID, "create account", false)

	res, err := agentloop.Execute(t.Context(), cfg, agentloop.Input{RunID: runID, Goal: "create account"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != agentloop.StatusAwaitingToolApproval {
		t.Fatalf("want parked, got %s %s", res.Status, res.Error)
	}
	if script.createCount.Load() != 1 {
		t.Fatalf("model should request create once, got %d", script.createCount.Load())
	}
	var n int
	_ = d.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM records WHERE object_api_name='Account' AND data->>'Name'=$1`, name).Scan(&n)
	if n != 0 {
		t.Fatal("parked write must not execute")
	}

	res, err = agentloop.Execute(t.Context(), cfg, agentloop.Input{RunID: runID, Goal: "create account", Resume: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != agentloop.StatusCompleted {
		t.Fatalf("resume status=%s %s", res.Status, res.Error)
	}
	_ = d.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM records WHERE object_api_name='Account' AND data->>'Name'=$1`, name).Scan(&n)
	if n != 1 {
		t.Fatalf("expected created account, count=%d", n)
	}
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(context.Background(), `DELETE FROM records WHERE object_api_name='Account' AND data->>'Name'=$1`, name)
	})
}

func TestHostedLoopDryRunExecutesNothing(t *testing.T) {
	name := "DryRunMustNotExist"
	script := &llmScript{tool: "create_record", args: map[string]any{
		"object": "Account", "data": map[string]any{"Name": name},
	}}
	d, ts, cfg := setupLoopHarness(t, script)
	actorID := runActorID(t, ts)
	runID := insertRun(t, d.Pool, actorID, "dry create", true)
	res, err := agentloop.Execute(t.Context(), cfg, agentloop.Input{RunID: runID, Goal: "dry create", DryRun: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != agentloop.StatusDryRunComplete {
		t.Fatalf("status=%s", res.Status)
	}
	var n int
	_ = d.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM records WHERE object_api_name='Account' AND data->>'Name'=$1`, name).Scan(&n)
	if n != 0 {
		t.Fatal("dry-run executed a write")
	}
}

func TestHostedLoopMissingActorFails(t *testing.T) {
	script := &llmScript{proseOnly: true}
	d, _, cfg := setupLoopHarness(t, script)
	var id string
	err := d.Pool.QueryRow(t.Context(), `
INSERT INTO agent_runs (status, goal, input, dry_run)
VALUES ('queued', 'no actor', '{}'::jsonb, false)
RETURNING id::text`).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(context.Background(), `DELETE FROM agent_run_events WHERE run_id=$1::uuid`, id)
		_, _ = d.Pool.Exec(context.Background(), `DELETE FROM agent_runs WHERE id=$1::uuid`, id)
	})
	res, err := agentloop.Execute(t.Context(), cfg, agentloop.Input{RunID: id, Goal: "no actor"}, nil)
	if err == nil {
		t.Fatal("expected missing actor error")
	}
	if res.Status != agentloop.StatusFailed {
		t.Fatalf("status=%s", res.Status)
	}
}

func TestHostedLoopIgnoresGraphCalls(t *testing.T) {
	script := &llmScript{graphOnly: true}
	d, ts, cfg := setupLoopHarness(t, script)
	actorID := runActorID(t, ts)
	runID := insertRun(t, d.Pool, actorID, "pin graph", false)
	res, err := agentloop.Execute(t.Context(), cfg, agentloop.Input{RunID: runID, Goal: "pin graph"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != agentloop.StatusCompleted {
		t.Fatalf("status=%s %s", res.Status, res.Error)
	}
	if res.Output["graphCalls"] == nil {
		t.Fatalf("graphCalls should persist on output: %+v", res.Output)
	}
	if script.queryCount.Load() != 0 || script.createCount.Load() != 0 {
		t.Fatal("graphCalls must not execute MCP tools")
	}
}

func TestHostedLoopStopsOnAllowlistDeny(t *testing.T) {
	script := &llmScript{forbidden: "org_deploy"}
	d, ts, cfg := setupLoopHarness(t, script)
	actorID := runActorID(t, ts)
	runID := insertRun(t, d.Pool, actorID, "deploy", false)
	res, err := agentloop.Execute(t.Context(), cfg, agentloop.Input{RunID: runID, Goal: "deploy"}, nil)
	if err == nil {
		t.Fatal("expected allowlist deny")
	}
	if res.Status != agentloop.StatusFailed {
		t.Fatalf("status=%s", res.Status)
	}
}

func TestHostedLoopDeniesInvokeSkillNotInAllowlist(t *testing.T) {
	script := &llmScript{tool: "invoke_skill", args: map[string]any{"apiName": "NotGrantedSkill"}}
	d, ts, cfg := setupLoopHarness(t, script)
	pb := "LoopSkillDeny_" + strings.ReplaceAll(t.Name(), "/", "_")
	if len(pb) > 80 {
		pb = pb[:80]
	}
	_, _ = d.Pool.Exec(t.Context(), `DELETE FROM agent_playbooks WHERE api_name=$1`, pb)
	_, err := d.Pool.Exec(t.Context(), `
INSERT INTO agent_playbooks (api_name, label, goal_template, instructions, require_approval, active, ownership, package_name,
  allowed_tools, allowed_skills, primary_section, job_class, harness_id, harness_version)
VALUES ($1,'Skill deny','Invoke','Invoke',false,true,'custom','customer.default',
  '["skills.invoke"]'::jsonb, '[]'::jsonb, 'run', 'operate', 'harness.operate.mutate', '1')`, pb)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = d.Pool.Exec(context.Background(), `DELETE FROM agent_playbooks WHERE api_name=$1`, pb) })
	applied := agentharness.Apply(agentharness.Spec{
		JobClass:        "operate",
		PrimarySection:  "run",
		AllowedTools:    []string{"skills.invoke"},
		RequireApproval: false,
	})
	applied.RequireApproval = false
	cfg.Applied = applied
	cfg.PlaybookAPIName = pb
	actorID := runActorID(t, ts)
	runID := insertRun(t, d.Pool, actorID, "invoke skill", false)
	res, err := agentloop.Execute(t.Context(), cfg, agentloop.Input{RunID: runID, Goal: "invoke skill"}, nil)
	if err == nil {
		t.Fatal("expected invoke_skill allowlist deny")
	}
	if res.Status != agentloop.StatusFailed {
		t.Fatalf("status=%s err=%s", res.Status, res.Error)
	}
}

func TestWorkerResumeDoesNotStartBlankGeneration(t *testing.T) {
	name := "WorkerResume " + time.Now().Format("150405.000")
	script := &llmScript{tool: "create_record", args: map[string]any{
		"object": "Account", "data": map[string]any{"Name": name},
	}}
	d, ts, cfg := setupLoopHarness(t, script)
	actorID := runActorID(t, ts)
	runID := insertRun(t, d.Pool, actorID, "worker write", false)
	res, err := agentloop.Execute(t.Context(), cfg, agentloop.Input{RunID: runID, Goal: "worker write"}, nil)
	if err != nil || res.Status != agentloop.StatusAwaitingToolApproval {
		t.Fatalf("park: %+v %v", res, err)
	}
	payload, _ := json.Marshal(map[string]any{
		"runId": runID, "goal": "worker write", "dryRun": false,
		"allowedTools": cfg.Applied.AllowedTools, "resume": true,
		"jobClass": "operate", "primarySection": "run",
	})
	var jobID string
	if err := d.Pool.QueryRow(t.Context(), `
INSERT INTO jobs (job_type, payload, status) VALUES ('agent.run', $1::jsonb, 'pending')
RETURNING id::text`, string(payload)).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(context.Background(), `DELETE FROM jobs WHERE id=$1::uuid`, jobID)
		_, _ = d.Pool.Exec(context.Background(), `DELETE FROM records WHERE object_api_name='Account' AND data->>'Name'=$1`, name)
	})
	nProcessed := 0
	for i := 0; i < 8; i++ {
		n, err := worker.ProcessJobs(t.Context(), d.Pool, &worker.ProcessOptions{
			JobLimit:               1,
			JobID:                  jobID,
			WorkerID:               "loop-resume-test",
			DataEngine:             ts.Data,
			Metadata:               d.Meta,
			ObjectAz:               &authz.ObjectAuthz{Store: &db.ObjectPermStore{Pool: d.Pool}},
			FieldAz:                &authz.FieldAuthz{Store: &db.FieldPermStore{Pool: d.Pool}},
			RecordAccess:           db.NewRecordAccessEvaluator(d.Pool),
			AllowDevLocalInference: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		nProcessed += n
		var jobStatus string
		_ = d.Pool.QueryRow(t.Context(), `SELECT status FROM jobs WHERE id=$1::uuid`, jobID).Scan(&jobStatus)
		if jobStatus == "completed" || jobStatus == "failed" {
			break
		}
	}
	if nProcessed == 0 {
		t.Fatal("expected job processed")
	}
	var status, jobStatus, lastErr string
	_ = d.Pool.QueryRow(t.Context(), `SELECT status FROM agent_runs WHERE id=$1::uuid`, runID).Scan(&status)
	_ = d.Pool.QueryRow(t.Context(), `SELECT status, COALESCE(last_error, '') FROM jobs WHERE id=$1::uuid`, jobID).Scan(&jobStatus, &lastErr)
	if status != "completed" {
		t.Fatalf("run status=%s job=%s last_error=%s", status, jobStatus, lastErr)
	}
	if jobStatus != "completed" {
		t.Fatalf("job status=%s last_error=%q (lease must be released on park/resume)", jobStatus, lastErr)
	}
	var count int
	_ = d.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM records WHERE object_api_name='Account' AND data->>'Name'=$1`, name).Scan(&count)
	if count != 1 {
		t.Fatalf("resume should create the record, count=%d", count)
	}
}

func TestHostedLoopInvokeSkillEnqueuesAutomationRun(t *testing.T) {
	const autoName = "LoopSkillHappy__c"
	script := &llmScript{tool: "invoke_skill", args: map[string]any{"apiName": autoName}}
	d, ts, cfg := setupLoopHarness(t, script)
	pb := "LoopSkillHappy_" + strings.ReplaceAll(t.Name(), "/", "_")
	if len(pb) > 80 {
		pb = pb[:80]
	}
	cleanup := func() {
		_, _ = d.Pool.Exec(context.Background(), `DELETE FROM jobs WHERE payload->>'apiName'=$1`, autoName)
		_, _ = d.Pool.Exec(context.Background(), `DELETE FROM agent_playbooks WHERE api_name=$1`, pb)
		_, _ = d.Pool.Exec(context.Background(), `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
	}
	cleanup()
	t.Cleanup(cleanup)

	rr := testutil.AuthRequest(ts.Handler, http.MethodPost, "/metadata/v1/automations", "loop-admin", map[string]any{
		"apiName": autoName, "label": "Loop skill", "objectApiName": "Account",
		"triggerEvent": "manual", "active": true, "runtime": "actions", "execution": "async",
		"actions": []any{},
	})
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("create automation: %d %s", rr.Code, rr.Body.String())
	}

	skillsJSON, _ := json.Marshal([]string{autoName})
	_, err := d.Pool.Exec(t.Context(), `
INSERT INTO agent_playbooks (api_name, label, goal_template, instructions, require_approval, active, ownership, package_name,
  allowed_tools, allowed_skills, primary_section, job_class, harness_id, harness_version)
VALUES ($1,'Skill happy','Invoke','Invoke',false,true,'custom','customer.default',
  '["skills.invoke"]'::jsonb, $2::jsonb, NULL, 'skill', 'harness.skill.invoke', '1')`, pb, string(skillsJSON))
	if err != nil {
		t.Fatal(err)
	}

	applied := agentharness.Apply(agentharness.Spec{
		JobClass:        "skill",
		AllowedTools:    []string{"skills.invoke"},
		RequireApproval: false,
	})
	applied.RequireApproval = false
	cfg.Applied = applied
	cfg.PlaybookAPIName = pb

	actorID := runActorID(t, ts)
	runID := insertRun(t, d.Pool, actorID, "invoke granted skill", false)
	res, err := agentloop.Execute(t.Context(), cfg, agentloop.Input{RunID: runID, Goal: "invoke granted skill"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != agentloop.StatusCompleted {
		t.Fatalf("status=%s err=%s", res.Status, res.Error)
	}

	var jobType, apiName, jobActor string
	err = d.Pool.QueryRow(t.Context(), `
SELECT job_type, payload->>'apiName', payload->>'actorId'
FROM jobs WHERE job_type='automation.run' AND payload->>'apiName'=$1
ORDER BY created_at DESC LIMIT 1`, autoName).Scan(&jobType, &apiName, &jobActor)
	if err != nil {
		t.Fatalf("expected automation.run job: %v", err)
	}
	if jobType != "automation.run" || apiName != autoName {
		t.Fatalf("job_type=%s apiName=%s", jobType, apiName)
	}
	if jobActor != actorID {
		t.Fatalf("actorId=%s want %s", jobActor, actorID)
	}
}
