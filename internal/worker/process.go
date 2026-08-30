package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/MajestaNet/ide/internal/agentharness"
	"github.com/MajestaNet/ide/internal/agentloop"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/egress"
	"github.com/MajestaNet/ide/internal/inference"
	"github.com/MajestaNet/ide/internal/mcp"
	"github.com/MajestaNet/ide/internal/metadata"
	oneotel "github.com/MajestaNet/ide/internal/otel"
	"github.com/MajestaNet/ide/internal/webhook"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// webhookHTTPClient disables redirects to prevent SSRF via Location headers.
var webhookHTTPClient = egress.NewSafeClient(30*time.Second, func(_ *http.Request, _ []*http.Request) error {
	return fmt.Errorf("redirects disabled for webhook delivery")
})

// ProcessOptions configures a processing round.
type ProcessOptions struct {
	WorkerID         string
	LeaseMs          int64
	JobLimit         int
	OutboxLimit      int
	WebhookTimeoutMs int
	// WebhookEncryptionKey decrypts enc:v1: webhook secrets (AUTH_JWT_SIGNING_KEY / WEBHOOK_ENCRYPTION_KEY).
	WebhookEncryptionKey string
	// DigitalOceanAPIToken is used for Native DO Inference routing (BP-052).
	DigitalOceanAPIToken string
	// AllowDevLocalInference permits http://localhost BYO providers (non-production).
	AllowDevLocalInference bool
	// AllowPrivateWebhookURLs skips SSRF URL checks (unit/integration tests only).
	AllowPrivateWebhookURLs bool
	// FetchFunc is injected for tests; defaults to a redirect-disabled client.
	FetchFunc func(req *http.Request) (*http.Response, error)
	// DeployEngine is required for customer.test.run jobs.
	DeployEngine *deploy.DeployEngine
	// DataEngine is required for projection.build, automation.run, and search.reindex jobs.
	DataEngine *dataengine.Service
	// Metadata is used for sharing.recalc jobs; defaults to metadata.NewService(pool) when nil.
	Metadata *metadata.Service
	// ObjectAz enforces CRUD AuthZ for automation.run-as mutations.
	ObjectAz *authz.ObjectAuthz
	// FieldAz enforces FLS for ingest / mutation paths that use UpsertAuthz.
	FieldAz *authz.FieldAuthz
	// AutomationAz enforces canRun for automation.run.
	AutomationAz *authz.AutomationAuthz
	// SystemAz enforces Metadata/Deploy capabilities for mcp.CallTool.
	SystemAz *authz.SystemAuthz
	// RecordAccess is sharing evaluation for mcp.CallTool record reads/writes.
	RecordAccess *authz.RecordAccessEvaluator
	// Actions is the platform-action invoker for invoke_action.
	Actions mcp.ActionInvoker
	// Retention configures periodic soft-delete / jobs / outbox / audit purges.
	Retention *RetentionOptions
	// JobID when set claims and processes only that pending job (integration tests).
	JobID string
}

func (o *ProcessOptions) workerID() string {
	if o.WorkerID != "" {
		return o.WorkerID
	}
	return CreateWorkerID("worker")
}

func (o *ProcessOptions) leaseMs() int64 {
	if o.LeaseMs > 0 {
		return o.LeaseMs
	}
	return DefaultLeaseMS
}

func (o *ProcessOptions) fetch(req *http.Request) (*http.Response, error) {
	if o.FetchFunc != nil {
		return o.FetchFunc(req)
	}
	return webhookHTTPClient.Do(req)
}

// completeJob marks a job as completed.
func completeJob(ctx context.Context, pool *db.Pool, jobID, workerID string) error {
	tag, err := pool.Exec(ctx, `
UPDATE jobs SET status='completed', completed_at=$2, locked_at=NULL, locked_by=NULL
WHERE id=$1::uuid AND status='running' AND locked_by=$3`, jobID, time.Now(), workerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("job %s lease lost before completion", jobID)
	}
	return nil
}

func storeJobResult(ctx context.Context, pool *db.Pool, jobID string, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
UPDATE jobs SET payload = COALESCE(payload, '{}'::jsonb) || jsonb_build_object('result', $2::jsonb)
WHERE id=$1::uuid`, jobID, string(raw))
	return err
}

// failJob marks a job as failed with an error message.
func failJob(ctx context.Context, pool *db.Pool, jobID, workerID string, jobErr error) error {
	msg := ""
	if jobErr != nil {
		msg = jobErr.Error()
	}
	tag, err := pool.Exec(ctx, `
UPDATE jobs SET status='failed', last_error=$2, completed_at=$3, locked_at=NULL, locked_by=NULL
WHERE id=$1::uuid AND status='running' AND locked_by=$4`, jobID, msg, time.Now(), workerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("job %s lease lost before failure recording", jobID)
	}
	return nil
}

func renewJobLease(ctx context.Context, pool *db.Pool, jobID, workerID string) error {
	tag, err := pool.Exec(ctx, `
UPDATE jobs SET locked_at=now()
WHERE id=$1::uuid AND status='running' AND locked_by=$2`, jobID, workerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("job %s lease lost", jobID)
	}
	return nil
}

func runClaimedJobWithLease(ctx context.Context, pool *db.Pool, job *ClaimedJob, opts *ProcessOptions, workerID string, leaseMs int64) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	interval := time.Duration(leaseMs) * time.Millisecond / 3
	if interval < time.Millisecond {
		interval = time.Millisecond
	}
	heartbeatErr := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				heartbeatErr <- nil
				return
			case <-ticker.C:
				if runCtx.Err() != nil {
					heartbeatErr <- nil
					return
				}
				if err := renewJobLease(runCtx, pool, job.ID, workerID); err != nil {
					heartbeatErr <- err
					cancel()
					return
				}
			}
		}
	}()
	err := runClaimedJob(runCtx, pool, job, opts)
	cancel()
	if heartbeat := <-heartbeatErr; heartbeat != nil {
		return heartbeat
	}
	return err
}

// runClaimedJob dispatches a single job.
func runClaimedJob(ctx context.Context, pool *db.Pool, job *ClaimedJob, opts *ProcessOptions) error {
	tr := oneotel.Tracer("one.worker")
	ctx, span := tr.Start(ctx, "job."+job.JobType)
	defer span.End()
	span.SetAttributes(attribute.String("one.job_type", job.JobType), attribute.String("one.job_id", job.ID))

	var payload map[string]any
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("invalid job payload: %w", err)
		}
	}
	if payload == nil {
		payload = map[string]any{}
	}

	err := dispatchClaimedJob(ctx, pool, job, payload, opts)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

func dispatchClaimedJob(ctx context.Context, pool *db.Pool, job *ClaimedJob, payload map[string]any, opts *ProcessOptions) error {
	switch job.JobType {
	case "automation.run":
		if err := processAutomationRun(ctx, pool, payload, opts); err != nil {
			return err
		}

	case "agent.run":
		runID, _ := payload["runId"].(string)
		dryRun, _ := payload["dryRun"].(bool)
		goal, _ := payload["goal"].(string)
		allowedTools := stringSliceFromAny(payload["allowedTools"])
		objectScopes := stringSliceFromAny(payload["objectScopes"])
		allowedSkills := stringSliceFromAny(payload["allowedSkills"])
		inputMap, _ := payload["input"].(map[string]any)
		if inputMap == nil {
			inputMap = map[string]any{}
		}
		primarySection, _ := payload["primarySection"].(string)
		harnessID, _ := payload["harnessId"].(string)
		harnessVersion, _ := payload["harnessVersion"].(string)
		jobClass, _ := payload["jobClass"].(string)
		playbookAPIName, _ := payload["playbookApiName"].(string)
		customerInstructions, _ := inputMap["instructions"].(string)
		requireApproval := false
		// Prefer live playbook binding so catalog floors stay authoritative.
		if playbookAPIName != "" {
			var toolsJSON, scopesJSON, skillsJSON []byte
			var instr, section, hid, hver, jc string
			var reqAppr bool
			err := pool.QueryRow(ctx, `
SELECT allowed_tools, object_scopes, COALESCE(allowed_skills, '[]'::jsonb),
       COALESCE(instructions, ''), require_approval,
       COALESCE(primary_section, ''), COALESCE(harness_id, ''), COALESCE(harness_version, ''),
       COALESCE(job_class, '')
FROM agent_playbooks WHERE api_name=$1 AND active=true`, playbookAPIName).Scan(
				&toolsJSON, &scopesJSON, &skillsJSON, &instr, &reqAppr, &section, &hid, &hver, &jc)
			if err != nil {
				err = fmt.Errorf("active agent playbook %q unavailable: %w", playbookAPIName, err)
				_, _ = pool.Exec(ctx, `
UPDATE agent_runs SET status='failed', error=$2, completed_at=$3
WHERE id=$1::uuid`, runID, err.Error(), time.Now())
				return err
			}
			for _, item := range []struct {
				raw  []byte
				dest *[]string
				name string
			}{
				{toolsJSON, &allowedTools, "allowed_tools"},
				{scopesJSON, &objectScopes, "object_scopes"},
				{skillsJSON, &allowedSkills, "allowed_skills"},
			} {
				if err := json.Unmarshal(item.raw, item.dest); err != nil {
					err = fmt.Errorf("agent playbook %q has invalid %s: %w", playbookAPIName, item.name, err)
					_, _ = pool.Exec(ctx, `
UPDATE agent_runs SET status='failed', error=$2, completed_at=$3
WHERE id=$1::uuid`, runID, err.Error(), time.Now())
					return err
				}
			}
			if customerInstructions == "" {
				customerInstructions = instr
			}
			requireApproval = reqAppr
			if section != "" {
				primarySection = section
			}
			if hid != "" {
				harnessID = hid
			}
			if hver != "" {
				harnessVersion = hver
			}
			if jc != "" {
				jobClass = jc
			}
		}
		applied := agentharness.Apply(agentharness.Spec{
			PrimarySection:  primarySection,
			JobClass:        jobClass,
			HarnessID:       harnessID,
			HarnessVersion:  harnessVersion,
			Instructions:    customerInstructions,
			AllowedTools:    allowedTools,
			RequireApproval: requireApproval,
		})
		allowedTools = applied.AllowedTools

		if err := validateAgentAllowlists(allowedTools, objectScopes); err != nil {
			_, _ = pool.Exec(ctx, `
UPDATE agent_runs SET status='failed', error=$2, completed_at=$3
WHERE id=$1::uuid`, runID, err.Error(), time.Now())
			return err
		}
		if err := validateAgentSkills(ctx, pool, allowedSkills); err != nil {
			_, _ = pool.Exec(ctx, `
UPDATE agent_runs SET status='failed', error=$2, completed_at=$3
WHERE id=$1::uuid`, runID, err.Error(), time.Now())
			return err
		}
		resume, _ := payload["resume"].(bool)
		var actorID *string
		_ = pool.QueryRow(ctx, `SELECT actor_id::text FROM agent_runs WHERE id=$1::uuid`, runID).Scan(&actorID)

		_, err := agentloop.Execute(ctx, agentloop.Config{
			Pool:            pool,
			MCP:             mcpDeps(pool, opts),
			Applied:         applied,
			AllowedSkills:   allowedSkills,
			ObjectScopes:    objectScopes,
			PlaybookAPIName: playbookAPIName,
			ResolveOpts: inference.ResolveOptions{
				DOAPIToken:    opts.DigitalOceanAPIToken,
				EncKey:        opts.WebhookEncryptionKey,
				AllowDevLocal: opts.AllowDevLocalInference,
			},
			SoftSkipInference: true,
		}, agentloop.Input{
			RunID:  runID,
			Goal:   goal,
			Input:  inputMap,
			DryRun: dryRun,
			Resume: resume,
		}, func(event string, payload any) {
			_, _ = inference.AppendRunEvent(ctx, pool, runID, event, payload)
		})
		if err != nil {
			return err
		}
		if actorID != nil {
			details, _ := json.Marshal(map[string]any{
				"runId": runID, "tools": allowedTools, "objectScopes": objectScopes,
				"skills": allowedSkills, "dryRun": dryRun, "resume": resume, "harness": applied.Meta(),
			})
			_, _ = pool.Exec(ctx, `
INSERT INTO audit_log (actor_id, action, details)
VALUES ($1::uuid, 'agent.run', $2::jsonb)`, *actorID, string(details))
		}
		return nil

	case "projection.build":
		objectAPIName, _ := payload["objectApiName"].(string)
		if objectAPIName == "" {
			return fmt.Errorf("projection.build missing objectApiName")
		}
		if opts.DataEngine == nil {
			return fmt.Errorf("DataEngine not configured for projection.build")
		}
		result, err := opts.DataEngine.BuildFieldProjections(ctx, objectAPIName)
		if err != nil {
			return err
		}
		slog.Info("projection.build", "object", objectAPIName, "built", len(result.Built), "errors", len(result.Errors))
		return nil

	case "hv.partition.roll":
		created, err := db.EnsureHighVolumeRangePartitions(ctx, pool, time.Now().UTC())
		if err != nil {
			return err
		}
		log.Printf("[worker] hv.partition.roll created=%v", created)
		return nil

	case "retention.purge":
		ret := RetentionOptions{}
		if opts.Retention != nil {
			ret = *opts.Retention
		}
		// Payload can override days (0 keeps config / disables that path when explicitly 0 in payload — skip).
		deleted, err := RunRetentionPurges(ctx, pool, ret)
		if err != nil {
			return err
		}
		slog.Info("retention.purge", "deleted", deleted)
		return nil

	case "sharing.recalc":
		meta := opts.Metadata
		if meta == nil {
			meta = metadata.NewService(pool)
		}
		if err := processSharingRecalc(ctx, pool, meta, payload); err != nil {
			return err
		}

	case "customer.test.run":
		if opts.DeployEngine == nil {
			return fmt.Errorf("DeployEngine not configured for customer.test.run")
		}
		runID, _ := payload["runId"].(string)
		if runID == "" {
			return fmt.Errorf("customer.test.run: missing runId in payload")
		}
		run, err := opts.DeployEngine.ExecuteTestRun(ctx, runID, nil)
		if err != nil {
			return fmt.Errorf("executeTestRun %s: %w", runID, err)
		}
		log.Printf("[worker] customer.test.run run=%s status=%s", runID, run.Status)
		if err := storeJobResult(ctx, pool, job.ID, run); err != nil {
			return err
		}

	case "ingest.process":
		if opts.DataEngine == nil {
			return fmt.Errorf("DataEngine not configured for ingest.process")
		}
		ingestJobID, _ := payload["ingestJobId"].(string)
		if ingestJobID == "" {
			return fmt.Errorf("ingest.process missing ingestJobId")
		}
		az := &dataengine.UpsertAuthz{}
		if opts.ObjectAz != nil {
			az.AssertObjectAccess = opts.ObjectAz.AssertObjectAccess
			az.GetModifyAllObjects = opts.ObjectAz.GetModifyAllObjects
			recordAccess := db.NewRecordAccessEvaluator(pool)
			az.CanModifyRecord = func(
				ctx context.Context,
				actor *authz.Actor,
				recordID, ownerID, createdByID, objectAPIName string,
				modifyAll map[string]struct{},
			) (bool, error) {
				return recordAccess.CanModifyRecordFull(
					ctx, actor, recordID, ownerID, createdByID, objectAPIName, modifyAll, true,
				)
			}
		}
		if opts.FieldAz != nil {
			az.AssertEditableFields = opts.FieldAz.AssertEditableFields
			az.StripUnreadableFields = opts.FieldAz.StripUnreadableFields
		}
		if err := opts.DataEngine.ProcessIngestJob(ctx, ingestJobID, az, func(ctx context.Context, actorID string) (*authz.Actor, error) {
			return resolveAutomationActor(ctx, pool, actorID)
		}); err != nil {
			return err
		}

	case "search.reindex":
		if opts.DataEngine == nil {
			return fmt.Errorf("DataEngine not configured for search.reindex")
		}
		objectAPIName, _ := payload["objectApiName"].(string)
		if err := opts.DataEngine.ReindexSearch(ctx, objectAPIName); err != nil {
			return err
		}
		slog.Info("search.reindex", "object", objectAPIName)

	case "deploy.validate":
		if opts.DeployEngine == nil {
			return fmt.Errorf("DeployEngine not configured for deploy.validate")
		}
		result, err := opts.DeployEngine.RunValidateJob(ctx, payload)
		if err != nil {
			return err
		}
		if err := storeJobResult(ctx, pool, job.ID, result); err != nil {
			return err
		}

	case "deploy.apply":
		if opts.DeployEngine == nil {
			return fmt.Errorf("DeployEngine not configured for deploy.apply")
		}
		result, err := opts.DeployEngine.RunApplyJob(ctx, payload)
		if err != nil {
			return err
		}
		if err := storeJobResult(ctx, pool, job.ID, result); err != nil {
			return err
		}

	default:
		return fmt.Errorf("unknown job_type %q", job.JobType)
	}
	return nil
}

// ProcessJobs claims and runs a batch of pending jobs.
// Returns the number of jobs processed.
func ProcessJobs(ctx context.Context, pool *db.Pool, opts *ProcessOptions) (int, error) {
	if opts == nil {
		opts = &ProcessOptions{}
	}
	workerID := opts.workerID()
	leaseMs := opts.leaseMs()

	if _, err := ReclaimExpiredJobLeases(ctx, pool, leaseMs); err != nil {
		log.Printf("[worker] reclaim jobs: %v", err)
	}

	if opts.JobID != "" {
		claimed, err := ClaimJobByID(ctx, pool, workerID, opts.JobID)
		if err != nil {
			return 0, err
		}
		if claimed == nil {
			return 0, nil
		}
		if err := runClaimedJobWithLease(ctx, pool, claimed, opts, workerID, leaseMs); err != nil {
			log.Printf("[worker] job %s (%s) failed: %v", claimed.ID, claimed.JobType, err)
			if markErr := failJob(ctx, pool, claimed.ID, workerID, err); markErr != nil {
				return 0, markErr
			}
			return 1, nil
		}
		if err := completeJob(ctx, pool, claimed.ID, workerID); err != nil {
			return 0, err
		}
		return 1, nil
	}

	limit := opts.JobLimit
	if limit <= 0 {
		limit = 20
	}
	processed := 0
	for processed < limit {
		claimed, err := ClaimJobs(ctx, pool, workerID, 1)
		if err != nil {
			return processed, err
		}
		if len(claimed) == 0 {
			break
		}
		job := claimed[0]
		if err := runClaimedJobWithLease(ctx, pool, &job, opts, workerID, leaseMs); err != nil {
			log.Printf("[worker] job %s (%s) failed: %v", job.ID, job.JobType, err)
			if markErr := failJob(ctx, pool, job.ID, workerID, err); markErr != nil {
				return processed, markErr
			}
		} else if err := completeJob(ctx, pool, job.ID, workerID); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

// WebhookRow holds the webhook columns needed for delivery.
type WebhookRow struct {
	ID         string
	URL        string
	Secret     *string
	EventTypes []byte // JSONB array
	Active     bool
}

func loadActiveWebhooks(ctx context.Context, pool *db.Pool) ([]WebhookRow, error) {
	rows, err := pool.Query(ctx, `
SELECT id::text, url, secret, event_types FROM webhooks WHERE active=true`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hooks []WebhookRow
	for rows.Next() {
		var h WebhookRow
		h.Active = true
		if err := rows.Scan(&h.ID, &h.URL, &h.Secret, &h.EventTypes); err != nil {
			return nil, err
		}
		hooks = append(hooks, h)
	}
	return hooks, rows.Err()
}

func webhookEventTypes(hook *WebhookRow) []string {
	if len(hook.EventTypes) == 0 {
		return []string{"*"}
	}
	var types []string
	_ = json.Unmarshal(hook.EventTypes, &types)
	if len(types) == 0 {
		return []string{"*"}
	}
	return types
}

func matchesEventType(types []string, eventType string) bool {
	for _, t := range types {
		if t == "*" || t == eventType {
			return true
		}
	}
	return false
}

// tryRecordDelivery inserts a webhook_deliveries row to prevent duplicate delivery.
// Returns true if a new row was inserted (should deliver), false if already delivered.
func tryRecordDelivery(ctx context.Context, pool *db.Pool, eventID, webhookID string) (bool, error) {
	_, err := pool.Exec(ctx,
		`INSERT INTO webhook_deliveries (event_id, webhook_id) VALUES ($1::uuid, $2::uuid)`,
		eventID, webhookID)
	if err != nil {
		msg := err.Error()
		if containsAny(msg, "webhook_deliveries_uniq", "duplicate key", "UNIQUE") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && containsStr(s, sub) {
			return true
		}
	}
	return false
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func deliverEvent(
	ctx context.Context,
	pool *db.Pool,
	event *ClaimedOutboxEvent,
	hooks []WebhookRow,
	opts *ProcessOptions,
	timeoutMs int,
	fetchFn func(*http.Request) (*http.Response, error),
) (delivered bool, lastErr error) {
	if len(hooks) == 0 {
		return true, nil
	}
	if opts == nil {
		opts = &ProcessOptions{}
	}

	deliveredAny := false
	for _, hook := range hooks {
		types := webhookEventTypes(&hook)
		if !matchesEventType(types, event.EventType) {
			continue
		}

		shouldSend, err := tryRecordDelivery(ctx, pool, event.ID, hook.ID)
		if err != nil {
			return false, err
		}
		if !shouldSend {
			deliveredAny = true
			continue
		}

		if !opts.AllowPrivateWebhookURLs {
			if err := webhook.ValidateDeliveryURL(hook.URL); err != nil {
				_, _ = pool.Exec(ctx,
					`DELETE FROM webhook_deliveries WHERE event_id=$1::uuid AND webhook_id=$2::uuid`,
					event.ID, hook.ID)
				return false, fmt.Errorf("webhook url blocked: %w", err)
			}
		}

		var secret *string
		if hook.Secret != nil && *hook.Secret != "" {
			plain, err := webhook.DecryptSecret(*hook.Secret, opts.WebhookEncryptionKey)
			if err != nil {
				_, _ = pool.Exec(ctx,
					`DELETE FROM webhook_deliveries WHERE event_id=$1::uuid AND webhook_id=$2::uuid`,
					event.ID, hook.ID)
				return false, fmt.Errorf("webhook secret: %w", err)
			}
			secret = &plain
		}

		body := buildEventBody(event)
		req, err := buildWebhookRequest(ctx, hook.URL, body, secret, event.EventType, timeoutMs)
		if err != nil {
			return false, err
		}
		resp, err := fetchFn(req)
		if err != nil {
			// Remove optimistic delivery row so retry is possible.
			_, _ = pool.Exec(ctx,
				`DELETE FROM webhook_deliveries WHERE event_id=$1::uuid AND webhook_id=$2::uuid`,
				event.ID, hook.ID)
			return false, err
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 300 {
			_, _ = pool.Exec(ctx,
				`DELETE FROM webhook_deliveries WHERE event_id=$1::uuid AND webhook_id=$2::uuid`,
				event.ID, hook.ID)
			return false, fmt.Errorf("webhook HTTP %d", resp.StatusCode)
		}
		deliveredAny = true
	}

	// No matching hooks → treat as delivered.
	if !deliveredAny && lastErr == nil {
		return true, nil
	}
	return deliveredAny, lastErr
}

func buildEventBody(event *ClaimedOutboxEvent) []byte {
	var payload any
	_ = json.Unmarshal(event.Payload, &payload)
	body := map[string]any{
		"id":        event.ID,
		"type":      event.EventType,
		"payload":   payload,
		"createdAt": event.CreatedAt,
	}
	if event.ObjectAPIName != nil {
		body["objectApiName"] = *event.ObjectAPIName
	}
	if event.RecordID != nil {
		body["recordId"] = *event.RecordID
	}
	b, _ := json.Marshal(body)
	return b
}

func buildWebhookRequest(
	ctx context.Context,
	url string,
	body []byte,
	secret *string,
	eventType string,
	timeoutMs int,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-One-Event", eventType)
	if secret != nil && *secret != "" {
		req.Header.Set("X-One-Secret", *secret)
	}
	// Apply timeout via context deadline.
	if timeoutMs > 0 {
		deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
		if d, ok := ctx.Deadline(); !ok || deadline.Before(d) {
			req = req.WithContext(contextWithDeadline(ctx, deadline))
		}
	}
	return req, nil
}

// contextWithDeadline creates a context with an absolute deadline without
// holding a cancel func (caller's outer context handles cleanup).
func contextWithDeadline(parent context.Context, deadline time.Time) context.Context {
	ctx, cancel := context.WithDeadline(parent, deadline)
	// We must call cancel eventually; spawn a goroutine that waits for the
	// parent to be done or the deadline to pass.
	go func() {
		select {
		case <-parent.Done():
		case <-ctx.Done():
		}
		cancel()
	}()
	return ctx
}

// markEventPublished marks an outbox event as successfully delivered.
func markEventPublished(ctx context.Context, pool *db.Pool, eventID, workerID string) error {
	tag, err := pool.Exec(ctx, `
UPDATE outbox_events
SET published_at=$2, locked_at=NULL, locked_by=NULL, last_error=NULL
WHERE id=$1::uuid AND published_at IS NULL AND locked_by=$3`, eventID, time.Now(), workerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("outbox event %s lease lost before publish", eventID)
	}
	return nil
}

// markEventFailed records the delivery error and defers the event until its
// current lease expires. Retaining locked_at prevents one poison event from
// being reclaimed immediately and starving newer outbox work.
func markEventFailed(ctx context.Context, pool *db.Pool, eventID, workerID string, deliveryErr error) error {
	msg := "delivery failed"
	if deliveryErr != nil {
		msg = deliveryErr.Error()
	}
	tag, err := pool.Exec(ctx, `
UPDATE outbox_events
SET locked_by=NULL, last_error=$2
WHERE id=$1::uuid AND published_at IS NULL AND locked_by=$3`, eventID, msg, workerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("outbox event %s lease lost before failure recording", eventID)
	}
	return nil
}

// ProcessOutbox claims and delivers a batch of unpublished outbox events.
// Returns the number of events processed.
func ProcessOutbox(ctx context.Context, pool *db.Pool, opts *ProcessOptions) (int, error) {
	if opts == nil {
		opts = &ProcessOptions{}
	}
	workerID := opts.workerID()
	leaseMs := opts.leaseMs()
	webhookTimeoutMs := opts.WebhookTimeoutMs
	if webhookTimeoutMs <= 0 {
		webhookTimeoutMs = 10_000
	}

	if _, err := ReclaimExpiredOutboxLeases(ctx, pool, leaseMs); err != nil {
		log.Printf("[worker] reclaim outbox: %v", err)
	}

	limit := opts.OutboxLimit
	if limit <= 0 {
		limit = 50
	}
	fetchFn := opts.fetch
	var hooks []WebhookRow
	hooksLoaded := false

	processed := 0
	for processed < limit {
		events, err := ClaimOutboxEvents(ctx, pool, workerID, 1, leaseMs)
		if err != nil {
			return processed, err
		}
		if len(events) == 0 {
			break
		}
		if !hooksLoaded {
			hooks, err = loadActiveWebhooks(ctx, pool)
			if err != nil {
				_ = markEventFailed(ctx, pool, events[0].ID, workerID, err)
				return processed, fmt.Errorf("load webhooks: %w", err)
			}
			hooksLoaded = true
		}
		ev := events[0]
		ok, deliveryErr := deliverEvent(ctx, pool, &ev, hooks, opts, webhookTimeoutMs, fetchFn)
		if ok {
			if err := markEventPublished(ctx, pool, ev.ID, workerID); err != nil {
				return processed, err
			}
		} else {
			if err := markEventFailed(ctx, pool, ev.ID, workerID, deliveryErr); err != nil {
				return processed, err
			}
		}
		processed++
	}
	return processed, nil
}

func stringSliceFromAny(v any) []string {
	switch t := v.(type) {
	case []string:
		return append([]string(nil), t...)
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// knownAgentTools is the allowlist of tool names the worker recognizes.
var knownAgentTools = agentharness.KnownAgentTools

// validateAgentAllowlists ensures payload tools are known.
// Empty objectScopes means all objects (checked when tools execute real writes).
func validateAgentAllowlists(tools, objectScopes []string) error {
	if len(tools) == 0 {
		return fmt.Errorf("agent.run: allowedTools must be non-empty")
	}
	for _, t := range tools {
		if _, ok := knownAgentTools[t]; !ok {
			return fmt.Errorf("agent.run: tool %q not allowed", t)
		}
	}
	for _, o := range objectScopes {
		if o == "" {
			return fmt.Errorf("agent.run: empty objectScopes entry")
		}
	}
	return nil
}

// validateAgentSkills ensures each skill names an existing automation (BP-014).
func validateAgentSkills(ctx context.Context, pool *db.Pool, skills []string) error {
	for _, s := range skills {
		if s == "" {
			return fmt.Errorf("agent.run: empty allowedSkills entry")
		}
		var one int
		err := pool.QueryRow(ctx, `SELECT 1 FROM metadata_automations WHERE api_name=$1`, s).Scan(&one)
		if err != nil {
			return fmt.Errorf("agent.run: skill %q is not a known automation", s)
		}
	}
	return nil
}

// ToolAllowed reports whether tool is in the playbook allowlist.
func ToolAllowed(tool string, allowedTools []string) bool {
	for _, t := range allowedTools {
		if t == tool {
			return true
		}
	}
	return false
}

// ObjectInScope reports whether objectAPIName is permitted (empty scopes = all).
func ObjectInScope(objectAPIName string, objectScopes []string) bool {
	if len(objectScopes) == 0 {
		return true
	}
	for _, o := range objectScopes {
		if o == objectAPIName {
			return true
		}
	}
	return false
}

func mcpDeps(pool *db.Pool, opts *ProcessOptions) mcp.Deps {
	deps := mcp.Deps{Pool: pool}
	if opts != nil {
		deps.Meta = opts.Metadata
		deps.Data = opts.DataEngine
		deps.ObjectAz = opts.ObjectAz
		deps.FieldAz = opts.FieldAz
		deps.RecordAccess = opts.RecordAccess
		deps.Actions = opts.Actions
		deps.Deploy = opts.DeployEngine
		deps.SystemAz = opts.SystemAz
		deps.AutomationAz = opts.AutomationAz
	}
	if deps.Meta == nil && pool != nil {
		deps.Meta = metadata.NewService(pool)
	}
	return deps
}
