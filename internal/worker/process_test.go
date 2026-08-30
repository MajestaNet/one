package worker_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/automation"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/worker"
)

func TestProcessJobsAutomationRun(t *testing.T) {
	ctx, pool := setupWorkerDB(t)

	var jobID string
	payload := map[string]any{"actions": []any{"log-hello"}}
	payloadJSON, _ := json.Marshal(payload)
	err := pool.QueryRow(ctx, `
INSERT INTO jobs (job_type, payload, status)
VALUES ('automation.run', $1::jsonb, 'pending')
RETURNING id::text`, string(payloadJSON)).Scan(&jobID)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE id=$1::uuid`, jobID) })

	n, err := worker.ProcessJobs(ctx, pool, &worker.ProcessOptions{
		JobLimit: 10,
	})
	if err != nil {
		t.Fatalf("ProcessJobs: %v", err)
	}
	if n == 0 {
		t.Fatal("expected at least one job processed")
	}

	var status string
	_ = pool.QueryRow(ctx, `SELECT status FROM jobs WHERE id=$1::uuid`, jobID).Scan(&status)
	if status != "completed" {
		t.Fatalf("expected completed, got %s", status)
	}
}

func TestProcessJobsRejectsUnknownJobType(t *testing.T) {
	ctx, pool := setupWorkerDB(t)
	var jobID string
	if err := pool.QueryRow(ctx, `
INSERT INTO jobs (job_type, payload, status, run_at)
VALUES ('test.unknown.job', '{}'::jsonb, 'pending', now() - interval '100 years')
RETURNING id::text`).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE id=$1::uuid`, jobID) })

	if _, err := worker.ProcessJobs(ctx, pool, &worker.ProcessOptions{JobLimit: 1, WorkerID: "unknown-job-test"}); err != nil {
		t.Fatal(err)
	}
	var status, lastErr string
	if err := pool.QueryRow(ctx, `SELECT status, COALESCE(last_error, '') FROM jobs WHERE id=$1::uuid`, jobID).Scan(&status, &lastErr); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || !strings.Contains(lastErr, "unknown job_type") {
		t.Fatalf("status=%s last_error=%q", status, lastErr)
	}
}

func TestProcessAgentRunFailsClosedWhenLivePlaybookUnavailable(t *testing.T) {
	ctx, pool := setupWorkerDB(t)
	var runID string
	if err := pool.QueryRow(ctx, `
INSERT INTO agent_runs (playbook_api_name, status, goal, input)
VALUES ('MissingSecurityReviewPlaybook', 'queued', 'must not run stale tools', '{}'::jsonb)
RETURNING id::text`).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{
		"runId": runID, "playbookApiName": "MissingSecurityReviewPlaybook",
		"allowedTools": []string{"create_record"}, "objectScopes": []string{"Account"},
	})
	var jobID string
	if err := pool.QueryRow(ctx, `
INSERT INTO jobs (job_type, payload, status)
VALUES ('agent.run', $1::jsonb, 'pending') RETURNING id::text`, string(payload)).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE id=$1::uuid`, jobID)
		_, _ = pool.Exec(ctx, `DELETE FROM agent_runs WHERE id=$1::uuid`, runID)
	})

	if _, err := worker.ProcessJobs(ctx, pool, &worker.ProcessOptions{JobID: jobID, WorkerID: "playbook-fail-closed"}); err != nil {
		t.Fatal(err)
	}
	var jobStatus, jobErr, runStatus string
	if err := pool.QueryRow(ctx, `SELECT status, COALESCE(last_error, '') FROM jobs WHERE id=$1::uuid`, jobID).Scan(&jobStatus, &jobErr); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM agent_runs WHERE id=$1::uuid`, runID).Scan(&runStatus); err != nil {
		t.Fatal(err)
	}
	if jobStatus != "failed" || runStatus != "failed" || !strings.Contains(jobErr, "active agent playbook") {
		t.Fatalf("job=%s run=%s err=%q", jobStatus, runStatus, jobErr)
	}
}

func TestProcessJobsAutomationRunCodeCreate(t *testing.T) {
	if _, err := automation.FindDeno(""); err != nil {
		t.Skip(err.Error())
	}
	ctx, pool := setupWorkerDB(t)

	ownerID := "00000000-0000-4000-8000-000000000001"
	store := db.NewUserStore(pool)
	if _, err := store.EnsureBootstrapAdmin(ctx, ownerID, "admin@one.local", "Admin"); err != nil {
		t.Fatal(err)
	}

	const parent = "AsyncCodeParent__c"
	const child = "AsyncCodeChild__c"
	const autoName = "AsyncCodeCreateChild"

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE payload->>'apiName'=$1`, autoName)
		_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name = ANY($1::text[])`, []string{parent, child})
		_, _ = pool.Exec(ctx, `DELETE FROM automation_permissions WHERE automation_api_name=$1`, autoName)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_fields WHERE object_api_name = ANY($1::text[])`, []string{parent, child})
		_, _ = pool.Exec(ctx, `DELETE FROM object_permissions WHERE object_api_name = ANY($1::text[])`, []string{parent, child})
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name = ANY($1::text[])`, []string{parent, child})
	}
	cleanup()
	t.Cleanup(cleanup)

	for _, obj := range []struct{ api, label, plural string }{
		{parent, "Async Code Parent", "Async Code Parents"},
		{child, "Async Code Child", "Async Code Children"},
	} {
		if _, err := pool.Exec(ctx, `
INSERT INTO metadata_objects (api_name, label, plural_label, storage_mode, ownership, features)
VALUES ($1,$2,$3,'flexible','custom','{}'::jsonb)`, obj.api, obj.label, obj.plural); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
INSERT INTO metadata_fields (object_api_name, api_name, label, field_type, required, ownership, filterable, sortable)
VALUES ($1,'Name','Name','text',true,'custom',true,true)`, obj.api); err != nil {
			t.Fatal(err)
		}
		_ = db.EnsureObjectInDataAccessCatalog(ctx, pool, obj.api)
	}

	src := `
export default async function run(ctx) {
  await ctx.createRecord({
    objectApiName: "` + child + `",
    data: { Name: String(ctx.trigger.data?.Name || "from-code") },
  });
  return { ok: true };
}
`
	var autoID string
	if err := pool.QueryRow(ctx, `
INSERT INTO metadata_automations (
  api_name, label, object_api_name, trigger_event, active, actions, ownership, package_name,
  runtime, execution, entry_file, source
) VALUES ($1,'ok',$2,'create',false,'[]'::jsonb,'custom','customer.default','code','async',
  'src/automations/AsyncCodeCreateChild.ts',$3)
RETURNING id::text`, autoName, parent, src).Scan(&autoID); err != nil {
		t.Fatal(err)
	}
	_ = db.EnsureAutomationInAccessCatalog(ctx, pool, autoName)

	meta := metadata.NewService(pool)
	data := dataengine.NewService(pool, meta)
	objectAz := &authz.ObjectAuthz{Store: &db.ObjectPermStore{Pool: pool}}
	automationAz := &authz.AutomationAuthz{Store: &db.AutomationPermStore{Pool: pool}}
	data.ObjectAz = objectAz
	data.AutomationAz = automationAz

	actor := &authz.Actor{ID: ownerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}
	parentRec, err := data.Create(ctx, parent, map[string]any{"Name": "ParentAsync"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	parentID, _ := parentRec["Id"].(string)
	_, _ = pool.Exec(ctx, `UPDATE metadata_automations SET active=true WHERE id=$1::uuid`, autoID)
	_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE payload->>'apiName'=$1`, autoName)

	var jobID string
	payload, _ := json.Marshal(map[string]any{
		"automationId":  autoID,
		"apiName":       autoName,
		"objectApiName": parent,
		"recordId":      parentID,
		"action":        "create",
		"runtime":       "code",
		"actorId":       ownerID,
	})
	if err := pool.QueryRow(ctx, `
INSERT INTO jobs (job_type, payload, status)
VALUES ('automation.run', $1::jsonb, 'pending')
RETURNING id::text`, string(payload)).Scan(&jobID); err != nil {
		t.Fatal(err)
	}

	n, err := worker.ProcessJobs(ctx, pool, &worker.ProcessOptions{
		JobLimit:     10,
		DataEngine:   data,
		ObjectAz:     objectAz,
		AutomationAz: automationAz,
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("expected job processed")
	}
	var status, lastErr string
	_ = pool.QueryRow(ctx, `SELECT status, COALESCE(last_error,'') FROM jobs WHERE id=$1::uuid`, jobID).Scan(&status, &lastErr)
	if status != "completed" {
		t.Fatalf("expected completed, got %s err=%s", status, lastErr)
	}
	var childCount int
	_ = pool.QueryRow(ctx, `
SELECT count(*) FROM records WHERE object_api_name=$1 AND data->>'Name'='ParentAsync'`,
		child).Scan(&childCount)
	if childCount != 1 {
		t.Fatalf("expected child from async code, got %d", childCount)
	}
}

func TestProcessJobsAutomationRunCodeAuthzDenied(t *testing.T) {
	if _, err := automation.FindDeno(""); err != nil {
		t.Skip(err.Error())
	}
	ctx, pool := setupWorkerDB(t)

	adminID := "00000000-0000-4000-8000-000000000001"
	userID := "00000000-0000-4000-8000-0000000000a1"
	store := db.NewUserStore(pool)
	if _, err := store.EnsureBootstrapAdmin(ctx, adminID, "admin@one.local", "Admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO users (id, email, display_name, is_admin, principal_type, is_active)
VALUES ($1::uuid, 'limited@one.local', 'Limited', false, 'user', true)
ON CONFLICT (id) DO UPDATE SET is_admin=false, is_active=true`, userID); err != nil {
		t.Fatal(err)
	}

	const parent = "DeniedCodeParent__c"
	const child = "DeniedCodeChild__c"
	const autoName = "DeniedCodeCreateChild"

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE payload->>'apiName'=$1`, autoName)
		_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name = ANY($1::text[])`, []string{parent, child})
		_, _ = pool.Exec(ctx, `DELETE FROM automation_permissions WHERE automation_api_name=$1`, autoName)
		_, _ = pool.Exec(ctx, `DELETE FROM user_permission_sets WHERE user_id=$1::uuid`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM object_permissions WHERE object_api_name = ANY($1::text[])`, []string{parent, child})
		_, _ = pool.Exec(ctx, `DELETE FROM permission_sets WHERE api_name='DeniedCodePS'`)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_fields WHERE object_api_name = ANY($1::text[])`, []string{parent, child})
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name = ANY($1::text[])`, []string{parent, child})
	}
	cleanup()
	t.Cleanup(cleanup)

	for _, obj := range []struct{ api, label, plural string }{
		{parent, "Denied Parent", "Denied Parents"},
		{child, "Denied Child", "Denied Children"},
	} {
		if _, err := pool.Exec(ctx, `
INSERT INTO metadata_objects (api_name, label, plural_label, storage_mode, ownership, features)
VALUES ($1,$2,$3,'flexible','custom','{}'::jsonb)`, obj.api, obj.label, obj.plural); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
INSERT INTO metadata_fields (object_api_name, api_name, label, field_type, required, ownership, filterable, sortable)
VALUES ($1,'Name','Name','text',true,'custom',true,true)`, obj.api); err != nil {
			t.Fatal(err)
		}
		_ = db.EnsureObjectInDataAccessCatalog(ctx, pool, obj.api)
	}

	// PS: can run automation + create parent, but NOT create child.
	var psID string
	if err := pool.QueryRow(ctx, `
INSERT INTO permission_sets (api_name, label) VALUES ('DeniedCodePS','Denied') RETURNING id::text`).Scan(&psID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO object_permissions (permission_set_id, object_api_name, can_create, can_read, can_update, can_delete, view_all, modify_all)
VALUES ($1::uuid, $2, true, true, true, true, true, true)`, psID, parent); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO object_permissions (permission_set_id, object_api_name, can_create, can_read, can_update, can_delete, view_all, modify_all)
VALUES ($1::uuid, $2, false, true, false, false, false, false)`, psID, child); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO user_permission_sets (user_id, permission_set_id) VALUES ($1::uuid, $2::uuid)`, userID, psID); err != nil {
		t.Fatal(err)
	}

	src := `
export default async function run(ctx) {
  await ctx.createRecord({ objectApiName: "` + child + `", data: { Name: "nope" } });
  return { ok: true };
}
`
	var autoID string
	if err := pool.QueryRow(ctx, `
INSERT INTO metadata_automations (
  api_name, label, object_api_name, trigger_event, active, actions, ownership, package_name,
  runtime, execution, entry_file, source
) VALUES ($1,'x',$2,'create',false,'[]'::jsonb,'custom','customer.default','code','async',
  'src/automations/Denied.ts',$3) RETURNING id::text`, autoName, parent, src).Scan(&autoID); err != nil {
		t.Fatal(err)
	}
	_ = db.EnsureAutomationInAccessCatalog(ctx, pool, autoName)
	_, _ = pool.Exec(ctx, `
UPDATE automation_permissions SET can_run=true
WHERE permission_set_id=$1::uuid AND automation_api_name=$2`, psID, autoName)

	meta := metadata.NewService(pool)
	data := dataengine.NewService(pool, meta)
	objectAz := &authz.ObjectAuthz{Store: &db.ObjectPermStore{Pool: pool}}
	automationAz := &authz.AutomationAuthz{Store: &db.AutomationPermStore{Pool: pool}}

	// Seed parent as admin so the job has a trigger record.
	admin := &authz.Actor{ID: adminID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}
	parentRec, err := data.Create(ctx, parent, map[string]any{"Name": "Seed"}, admin)
	if err != nil {
		t.Fatal(err)
	}
	parentID, _ := parentRec["Id"].(string)
	_, _ = pool.Exec(ctx, `UPDATE metadata_automations SET active=true WHERE id=$1::uuid`, autoID)
	_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE payload->>'apiName'=$1`, autoName)

	var jobID string
	payload, _ := json.Marshal(map[string]any{
		"automationId":  autoID,
		"apiName":       autoName,
		"objectApiName": parent,
		"recordId":      parentID,
		"action":        "create",
		"runtime":       "code",
		"actorId":       userID,
	})
	if err := pool.QueryRow(ctx, `
INSERT INTO jobs (job_type, payload, status)
VALUES ('automation.run', $1::jsonb, 'pending') RETURNING id::text`, string(payload)).Scan(&jobID); err != nil {
		t.Fatal(err)
	}

	_, err = worker.ProcessJobs(ctx, pool, &worker.ProcessOptions{
		JobLimit:     10,
		DataEngine:   data,
		ObjectAz:     objectAz,
		AutomationAz: automationAz,
	})
	if err != nil {
		t.Fatal(err)
	}
	var status, lastErr string
	_ = pool.QueryRow(ctx, `SELECT status, COALESCE(last_error,'') FROM jobs WHERE id=$1::uuid`, jobID).Scan(&status, &lastErr)
	if status != "failed" {
		t.Fatalf("expected failed job, got %s err=%s", status, lastErr)
	}
	var childCount int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM records WHERE object_api_name=$1`, child).Scan(&childCount)
	if childCount != 0 {
		t.Fatalf("expected no child, got %d", childCount)
	}
}

func TestProcessOutboxWebhookDelivery(t *testing.T) {
	ctx, pool := setupWorkerDB(t)

	// Spin up a test HTTP server to receive the webhook.
	var received []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		received = append(received, body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, _ = pool.Exec(ctx, `DELETE FROM webhook_deliveries WHERE webhook_id IN (SELECT id FROM webhooks WHERE api_name='test_hook_process')`)
	_, _ = pool.Exec(ctx, `DELETE FROM webhooks WHERE api_name='test_hook_process'`)

	// Create a webhook pointing at the test server.
	var hookID string
	err := pool.QueryRow(ctx, `
INSERT INTO webhooks (api_name, url, event_types, active)
VALUES ('test_hook_process', $1, '["test.delivery"]'::jsonb, true)
RETURNING id::text`, server.URL).Scan(&hookID)
	if err != nil {
		t.Fatalf("insert webhook: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM webhook_deliveries WHERE webhook_id=$1::uuid`, hookID)
		_, _ = pool.Exec(ctx, `DELETE FROM webhooks WHERE id=$1::uuid`, hookID)
	})

	_, _ = pool.Exec(ctx, `
UPDATE outbox_events
SET published_at = COALESCE(published_at, now()), locked_at = NULL, locked_by = NULL
WHERE published_at IS NULL`)

	// Insert an outbox event.
	var evID string
	err = pool.QueryRow(ctx, `
INSERT INTO outbox_events (event_type, payload)
VALUES ('test.delivery', '{"hello":"world"}')
RETURNING id::text`).Scan(&evID)
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE id=$1::uuid`, evID) })

	fetchFn := func(req *http.Request) (*http.Response, error) {
		return http.DefaultClient.Do(req)
	}

	n, err := worker.ProcessOutbox(ctx, pool, &worker.ProcessOptions{
		OutboxLimit:             10,
		WebhookTimeoutMs:        5000,
		FetchFunc:               fetchFn,
		AllowPrivateWebhookURLs: true,
	})
	if err != nil {
		t.Fatalf("ProcessOutbox: %v", err)
	}
	if n == 0 {
		t.Fatal("expected outbox events processed")
	}

	// Give the webhook a moment.
	time.Sleep(100 * time.Millisecond)

	// Verify event is published.
	var publishedAt *time.Time
	_ = pool.QueryRow(ctx, `SELECT published_at FROM outbox_events WHERE id=$1::uuid`, evID).Scan(&publishedAt)
	if publishedAt == nil {
		t.Fatal("expected published_at to be set after delivery")
	}
	if len(received) == 0 {
		t.Fatal("test webhook server received no requests")
	}
	if received[0]["type"] != "test.delivery" {
		t.Fatalf("unexpected event type: %v", received[0]["type"])
	}
}

func TestProcessOutboxIdempotency(t *testing.T) {
	ctx, pool := setupWorkerDB(t)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var hookID string
	err := pool.QueryRow(ctx, `
INSERT INTO webhooks (api_name, url, event_types, active)
VALUES ('test_hook_idempotent', $1, '["test.idempotent"]'::jsonb, true)
RETURNING id::text`, server.URL).Scan(&hookID)
	if err != nil {
		t.Fatalf("insert webhook: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM webhook_deliveries WHERE webhook_id=$1::uuid`, hookID)
		_, _ = pool.Exec(ctx, `DELETE FROM webhooks WHERE id=$1::uuid`, hookID)
	})

	var evID string
	err = pool.QueryRow(ctx, `
INSERT INTO outbox_events (event_type, payload)
VALUES ('test.idempotent', '{}')
RETURNING id::text`).Scan(&evID)
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE id=$1::uuid`, evID) })

	// Pre-insert a delivery row to simulate already-delivered scenario.
	_, err = pool.Exec(ctx,
		`INSERT INTO webhook_deliveries (event_id, webhook_id) VALUES ($1::uuid, $2::uuid)`,
		evID, hookID)
	if err != nil {
		t.Fatalf("pre-insert delivery: %v", err)
	}

	_, err = worker.ProcessOutbox(ctx, pool, &worker.ProcessOptions{
		OutboxLimit:             10,
		WebhookTimeoutMs:        5000,
		AllowPrivateWebhookURLs: true,
		FetchFunc:               func(req *http.Request) (*http.Response, error) { return http.DefaultClient.Do(req) },
	})
	if err != nil {
		t.Fatalf("ProcessOutbox: %v", err)
	}

	if callCount > 0 {
		t.Fatalf("expected webhook not to be called again (idempotent), got %d calls", callCount)
	}
}

func TestProcessJobsSearchReindex(t *testing.T) {
	ctx, pool := setupWorkerDB(t)

	ownerID := "00000000-0000-4000-8000-000000000001"
	store := db.NewUserStore(pool)
	if _, err := store.EnsureBootstrapAdmin(ctx, ownerID, "admin@one.local", "Admin"); err != nil {
		t.Fatal(err)
	}

	obj := "SearchReidx" + time.Now().Format("150405000")
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE payload->>'objectApiName' = $1`, obj)
		_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name = $1`, obj)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_fields WHERE object_api_name = $1`, obj)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name = $1`, obj)
	})

	meta := metadata.NewService(pool)
	if _, err := meta.InsertObject(ctx, metadata.ObjectDefinition{
		APIName: obj, Label: "Search Reindex", PluralLabel: "Search Reindex", StorageMode: "flexible",
	}, metadata.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := meta.InsertField(ctx, metadata.FieldDefinition{
		ObjectAPIName: obj, APIName: "Name", Label: "Name", FieldType: "text", Searchable: true, Filterable: true,
	}, metadata.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	svc := dataengine.NewService(pool, meta)
	actor := &authz.Actor{ID: ownerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}
	rec, err := svc.Create(ctx, obj, map[string]any{"Name": "Backfill NeedleCo"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := rec["Id"].(string)
	if _, err := pool.Exec(ctx, `
UPDATE records SET search_document = '', search_title = '', search_subtitle = ''
WHERE id = $1::uuid AND object_api_name = $2`, id, obj); err != nil {
		t.Fatal(err)
	}

	scopes := []dataengine.SearchScope{{ObjectAPIName: obj, StorageMode: "flexible"}}
	before, err := svc.Search(ctx, dataengine.SearchRequest{Query: "backfill needle"}, scopes)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Hits) != 0 {
		t.Fatalf("expected no hits before reindex, got %+v", before.Hits)
	}

	payload, _ := json.Marshal(map[string]any{"objectApiName": obj})
	var jobID string
	if err := pool.QueryRow(ctx, `
INSERT INTO jobs (job_type, payload, status)
VALUES ('search.reindex', $1::jsonb, 'pending')
RETURNING id::text`, string(payload)).Scan(&jobID); err != nil {
		t.Fatal(err)
	}

	n, err := worker.ProcessJobs(ctx, pool, &worker.ProcessOptions{
		JobLimit:   5,
		WorkerID:   "search-reindex-test",
		DataEngine: svc,
		Metadata:   meta,
	})
	if err != nil {
		t.Fatalf("ProcessJobs: %v", err)
	}
	if n < 1 {
		t.Fatal("expected at least one job processed")
	}

	after, err := svc.Search(ctx, dataengine.SearchRequest{Query: "backfill needle"}, scopes)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Hits) == 0 {
		t.Fatal("expected hit after search.reindex")
	}
	if after.Hits[0].ID != id {
		t.Fatalf("hit id=%s want=%s", after.Hits[0].ID, id)
	}
}

func TestProcessJobsDeployValidate(t *testing.T) {
	ctx, pool := setupWorkerDB(t)
	meta := metadata.NewService(pool)
	data := dataengine.NewService(pool, meta)
	eng := deploy.NewDeployEngine(pool, meta, data, deploy.Options{
		InstallID: "w1", CustomerID: "c1", ProductVersion: "0.1.0",
	})
	pkg := deploy.DefaultCustomerPackage
	row, err := eng.CreateBundleFromArtifact(ctx, struct {
		Artifact  any
		Label     *string
		CreatedBy *string
		Origin    string
		Signature *string
	}{
		Artifact: &deploy.BundleArtifact{
			ManifestVersion: 1, Ownership: "custom", DefaultPackageName: pkg,
			Objects: []deploy.SnapshotObject{},
			Fields:  []deploy.SnapshotField{},
		},
		Origin: "test",
	})
	if err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	payload, _ := json.Marshal(map[string]any{"bundleId": row.ID})
	var jobID string
	if err := pool.QueryRow(ctx, `
INSERT INTO jobs (job_type, payload, status)
VALUES ('deploy.validate', $1::jsonb, 'pending')
RETURNING id::text`, string(payload)).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE id=$1::uuid`, jobID) })

	n, err := worker.ProcessJobs(ctx, pool, &worker.ProcessOptions{
		JobLimit:     1,
		WorkerID:     "deploy-validate-test",
		DeployEngine: eng,
		JobID:        jobID,
	})
	if err != nil {
		t.Fatalf("ProcessJobs: %v", err)
	}
	if n != 1 {
		t.Fatalf("processed=%d", n)
	}
	var status string
	var payloadOut []byte
	if err := pool.QueryRow(ctx, `SELECT status, payload FROM jobs WHERE id=$1::uuid`, jobID).Scan(&status, &payloadOut); err != nil {
		t.Fatal(err)
	}
	if status != "completed" {
		t.Fatalf("status=%s", status)
	}
	var wrap struct {
		Result deploy.ValidateLocalResult `json:"result"`
	}
	if err := json.Unmarshal(payloadOut, &wrap); err != nil {
		t.Fatal(err)
	}
	if wrap.Result.BundleID != row.ID {
		t.Fatalf("result=%+v", wrap.Result)
	}
}
