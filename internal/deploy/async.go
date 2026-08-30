package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	DefaultSyncMaxFiles    = 50
	DefaultSyncMaxBytes    = 2097152
	DefaultQueueMax        = 8
	DefaultJobSlotsDeploy  = 1
	JobTypeValidate        = "deploy.validate"
	JobTypeApply           = "deploy.apply"
	JobTypeCustomerTestRun = "customer.test.run"
	WorkPollPrefix         = "/deploy/v1/work/"
)

// DeployQueueJobTypes are counted toward DEPLOY_QUEUE_MAX in Phase 1 (no jobs.class yet).
var DeployQueueJobTypes = []string{JobTypeValidate, JobTypeApply, JobTypeCustomerTestRun}

// QueuedWork is the HTTP 202 body until Phase 3 ExecutionRun objects exist.
type QueuedWork struct {
	Accepted bool   `json:"accepted"`
	Status   string `json:"status"`
	JobID    string `json:"jobId"`
	BundleID string `json:"bundleId,omitempty"`
	Poll     string `json:"poll"`
}

// WorkStatus is GET /deploy/v1/work/{jobId}.
type WorkStatus struct {
	JobID     string          `json:"jobId"`
	JobType   string          `json:"jobType"`
	Status    string          `json:"status"`
	LastError *string         `json:"lastError,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

// ArtifactFileCount is metadata items plus src/ files in a pack.
func ArtifactFileCount(art *BundleArtifact) int {
	if art == nil {
		return 0
	}
	n := len(art.Objects) + len(art.Fields) + len(art.ValidationRules) + len(art.Automations) +
		len(art.AgentPlaybooks) + len(art.Canvases) + len(art.Experiences) + len(art.PermissionSets) +
		len(art.Webhooks) + len(art.Connectors) + len(art.Tests) + len(art.DataRoles) +
		len(art.ObjectSharingSettings) + len(art.SharingRules) + len(art.Sources)
	return n
}

// ArtifactJSONBytes is the serialized artifact size used by the sync byte gate.
func ArtifactJSONBytes(art *BundleArtifact) int64 {
	if art == nil {
		return 0
	}
	b, err := json.Marshal(art)
	if err != nil {
		return 0
	}
	return int64(len(b))
}

// ExceedsSyncGate reports whether file count or byte size is over the in-request cap.
func (e *DeployEngine) ExceedsSyncGate(files int, bytes int64) bool {
	maxFiles := DefaultSyncMaxFiles
	maxBytes := int64(DefaultSyncMaxBytes)
	if e != nil {
		if e.syncMaxFiles > 0 {
			maxFiles = e.syncMaxFiles
		}
		if e.syncMaxBytes > 0 {
			maxBytes = e.syncMaxBytes
		}
	}
	return files > maxFiles || bytes > maxBytes
}

func (e *DeployEngine) bundleExceedsSyncGate(ctx context.Context, bundleID string) (bool, *BundleRow, *BundleArtifact, error) {
	bundle, err := e.GetBundle(ctx, bundleID)
	if err != nil {
		return false, nil, nil, err
	}
	art, err := ParseBundleArtifact(bundle.Artifact)
	if err != nil {
		return false, bundle, nil, err
	}
	bytes := int64(len(bundle.Artifact))
	return e.ExceedsSyncGate(ArtifactFileCount(art), bytes), bundle, art, nil
}

func (e *DeployEngine) assertDeployQueue(ctx context.Context) error {
	if e == nil || e.pool == nil {
		return fmt.Errorf("deploy engine unavailable")
	}
	var n int
	err := e.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM jobs
WHERE job_type = ANY($1)
  AND status IN ('pending', 'running')`, DeployQueueJobTypes).Scan(&n)
	if err != nil {
		return fmt.Errorf("count deploy queue: %w", err)
	}
	if n >= e.queueMax {
		return newBusyError(e.jobSlotsDeploy, e.queueMax)
	}
	return nil
}

func (e *DeployEngine) insertJob(ctx context.Context, jobType string, payload any) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	var id string
	err = e.pool.QueryRow(ctx, `
INSERT INTO jobs (job_type, payload) VALUES ($1, $2::jsonb)
RETURNING id::text`, jobType, string(raw)).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("enqueue %s: %w", jobType, err)
	}
	return id, nil
}

func queued(jobID, bundleID string) *QueuedWork {
	return &QueuedWork{
		Accepted: true,
		Status:   "queued",
		JobID:    jobID,
		BundleID: bundleID,
		Poll:     WorkPollPrefix + jobID,
	}
}

// EnqueueValidate decides sync vs 202 for org validate (local artifact or stored bundle).
func (e *DeployEngine) EnqueueValidate(ctx context.Context, input struct {
	Artifact  any
	BundleID  string
	Label     *string
	CreatedBy *string
}, rawBytes int64) (*ValidateLocalResult, *QueuedWork, error) {
	over := false
	bundleID := strings.TrimSpace(input.BundleID)

	if bundleID != "" {
		exceeds, _, _, err := e.bundleExceedsSyncGate(ctx, bundleID)
		if err != nil {
			return nil, nil, err
		}
		over = exceeds
	} else if input.Artifact != nil {
		art, err := ParseBundleArtifact(input.Artifact)
		if err != nil {
			return nil, nil, err
		}
		bytes := rawBytes
		if artifactBytes := ArtifactJSONBytes(art); artifactBytes > bytes {
			// Zip uploads can be much smaller than their expanded artifact. Gate on
			// the larger representation so compression cannot force expensive
			// validation back onto the API goroutine.
			bytes = artifactBytes
		}
		over = e.ExceedsSyncGate(ArtifactFileCount(art), bytes)
	} else {
		return nil, nil, newValidationError("bundleId, artifact, or zip body is required")
	}

	if !over {
		res, err := e.ValidateLocal(ctx, input)
		return res, nil, err
	}
	if err := e.assertDeployQueue(ctx); err != nil {
		return nil, nil, err
	}
	if bundleID == "" {
		row, err := e.CreateBundleFromArtifact(ctx, struct {
			Artifact  any
			Label     *string
			CreatedBy *string
			Origin    string
			Signature *string
		}{
			Artifact:  input.Artifact,
			Label:     input.Label,
			CreatedBy: input.CreatedBy,
			Origin:    "customer-package",
		})
		if err != nil {
			return nil, nil, err
		}
		bundleID = row.ID
	}
	jobID, err := e.insertJob(ctx, JobTypeValidate, map[string]any{
		"bundleId":  bundleID,
		"createdBy": stringOrEmpty(input.CreatedBy),
	})
	if err != nil {
		return nil, nil, err
	}
	return nil, queued(jobID, bundleID), nil
}

// EnqueueValidateBundle is POST /bundles/{id}/validate — sync keeps {bundleId,report}.
func (e *DeployEngine) EnqueueValidateBundle(ctx context.Context, bundleID string) (*ValidateBundleResult, *QueuedWork, error) {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return nil, nil, newValidationError("bundleId is required")
	}
	over, _, _, err := e.bundleExceedsSyncGate(ctx, bundleID)
	if err != nil {
		return nil, nil, err
	}
	if !over {
		res, err := e.ValidateBundle(ctx, bundleID)
		return res, nil, err
	}
	if err := e.assertDeployQueue(ctx); err != nil {
		return nil, nil, err
	}
	jobID, err := e.insertJob(ctx, JobTypeValidate, map[string]any{"bundleId": bundleID})
	if err != nil {
		return nil, nil, err
	}
	return nil, queued(jobID, bundleID), nil
}

// EnqueuePromote decides sync vs 202 for repo→org apply. forceAsync never runs apply on the caller.
func (e *DeployEngine) EnqueuePromote(ctx context.Context, input struct {
	BundleID  string
	DryRun    bool
	CreatedBy *string
}, forceAsync bool) (*PromoteBundleResult, *QueuedWork, error) {
	if strings.TrimSpace(input.BundleID) == "" {
		return nil, nil, newValidationError("bundleId is required")
	}
	over, bundle, _, err := e.bundleExceedsSyncGate(ctx, input.BundleID)
	if err != nil {
		return nil, nil, err
	}
	if !over && !forceAsync {
		res, err := e.PromoteBundle(ctx, input)
		return res, nil, err
	}
	if err := e.assertDeployQueue(ctx); err != nil {
		return nil, nil, err
	}
	var sourceID *string
	if bundle != nil {
		sourceID = &bundle.SourceInstallID
	}
	promo, err := e.insertPromotion(ctx, input.BundleID, input.DryRun, "local", sourceID, nil, input.CreatedBy)
	if err != nil {
		return nil, nil, err
	}
	jobID, err := e.insertJob(ctx, JobTypeApply, map[string]any{
		"promotionId": promo.ID,
		"bundleId":    input.BundleID,
		"dryRun":      input.DryRun,
		"createdBy":   stringOrEmpty(input.CreatedBy),
	})
	if err != nil {
		return nil, nil, err
	}
	return nil, queued(jobID, input.BundleID), nil
}

// GetDeployWork returns a job owned by this install's deploy class.
func (e *DeployEngine) GetDeployWork(ctx context.Context, jobID string) (*WorkStatus, error) {
	if e == nil || e.pool == nil {
		return nil, fmt.Errorf("deploy engine unavailable")
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return nil, newNotFoundError("Work not found")
	}
	var (
		out     WorkStatus
		payload []byte
	)
	err := e.pool.QueryRow(ctx, `
SELECT id::text, job_type, status, last_error, payload
FROM jobs
WHERE id=$1::uuid AND job_type = ANY($2)`, jobID, DeployQueueJobTypes).Scan(
		&out.JobID, &out.JobType, &out.Status, &out.LastError, &payload,
	)
	if err != nil {
		return nil, newNotFoundError("Work not found")
	}
	if len(payload) > 0 {
		var wrap struct {
			Result json.RawMessage `json:"result"`
		}
		if json.Unmarshal(payload, &wrap) == nil && len(wrap.Result) > 0 && string(wrap.Result) != "null" {
			out.Result = wrap.Result
		}
	}
	return &out, nil
}

// RunValidateJob is the worker handler for deploy.validate (idempotent).
func (e *DeployEngine) RunValidateJob(ctx context.Context, payload map[string]any) (*ValidateLocalResult, error) {
	if existing := resultFromPayload[ValidateLocalResult](payload); existing != nil && existing.BundleID != "" {
		return existing, nil
	}
	bundleID, _ := payload["bundleId"].(string)
	if bundleID == "" {
		return nil, fmt.Errorf("deploy.validate missing bundleId")
	}
	var createdBy *string
	if s, _ := payload["createdBy"].(string); s != "" {
		createdBy = &s
	}
	return e.ValidateLocal(ctx, struct {
		Artifact  any
		BundleID  string
		Label     *string
		CreatedBy *string
	}{BundleID: bundleID, CreatedBy: createdBy})
}

// RunApplyJob is the worker handler for deploy.apply (idempotent).
func (e *DeployEngine) RunApplyJob(ctx context.Context, payload map[string]any) (*PromoteBundleResult, error) {
	if existing := resultFromPayload[PromoteBundleResult](payload); existing != nil && existing.Promotion != nil {
		return existing, nil
	}
	promotionID, _ := payload["promotionId"].(string)
	if promotionID == "" {
		return nil, fmt.Errorf("deploy.apply missing promotionId")
	}
	return e.ApplyPendingPromotion(ctx, promotionID)
}

// ApplyPendingPromotion runs validate+apply against an existing pending promotion row.
func (e *DeployEngine) ApplyPendingPromotion(ctx context.Context, promotionID string) (*PromoteBundleResult, error) {
	promo, err := e.GetPromotion(ctx, promotionID)
	if err != nil {
		return nil, err
	}
	switch promo.Status {
	case "applied", "validated", "failed":
		return promotionToResult(promo), nil
	}
	createdBy := promo.CreatedBy
	res, err := e.promoteInto(ctx, promo.ID, promo.BundleID, promo.DryRun, createdBy)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (e *DeployEngine) promoteInto(ctx context.Context, promotionID, bundleID string, dryRun bool, createdBy *string) (*PromoteBundleResult, error) {
	bundle, err := e.GetBundle(ctx, bundleID)
	if err != nil {
		failed, _ := e.failPromotion(ctx, promotionID, err.Error())
		return &PromoteBundleResult{Promotion: failed}, err
	}
	artifact, err := ParseBundleArtifact(bundle.Artifact)
	if err != nil {
		failed, _ := e.failPromotion(ctx, promotionID, err.Error())
		return &PromoteBundleResult{Promotion: failed}, err
	}
	validation, err := ValidateBundleArtifact(ctx, e.meta, artifact, e.productVersion, bundle.ProductVersionRange)
	if err != nil {
		failed, _ := e.failPromotion(ctx, promotionID, err.Error())
		return &PromoteBundleResult{Promotion: failed}, err
	}
	vrJSON, _ := json.Marshal(validation)
	_, _ = e.pool.Exec(ctx, `UPDATE deploy_promotions SET validation_report=$2 WHERE id=$1::uuid`, promotionID, string(vrJSON))

	if !validation.OK {
		failed, _ := e.failPromotion(ctx, promotionID, "Validation failed")
		return &PromoteBundleResult{Promotion: failed, Validation: validation}, nil
	}
	if !dryRun {
		if err := e.requireConfiguredSuitesGreen(ctx, createdBy); err != nil {
			failed, _ := e.failPromotion(ctx, promotionID, err.Error())
			return &PromoteBundleResult{Promotion: failed, Validation: validation}, nil
		}
	}
	applyReport, applyErr := ApplyBundleArtifact(ctx, e.pool, e.meta, artifact, dryRun)
	if applyErr != nil {
		failed, _ := e.failPromotion(ctx, promotionID, applyErr.Error())
		return nil, fmt.Errorf("%w; promotion failed: %s", applyErr, failed.ID)
	}
	status := "applied"
	if dryRun {
		status = "validated"
	}
	done, err := e.completePromotion(ctx, promotionID, status, applyReport)
	if err != nil {
		return nil, err
	}
	return &PromoteBundleResult{Promotion: done, Validation: validation, Apply: applyReport}, nil
}

func promotionToResult(row *PromotionRow) *PromoteBundleResult {
	out := &PromoteBundleResult{Promotion: row}
	if len(row.ValidationReport) > 0 && string(row.ValidationReport) != "null" {
		var v ValidationReport
		if json.Unmarshal(row.ValidationReport, &v) == nil {
			out.Validation = &v
		}
	}
	if len(row.ApplyReport) > 0 && string(row.ApplyReport) != "null" {
		var a ApplyReport
		if json.Unmarshal(row.ApplyReport, &a) == nil {
			out.Apply = &a
		}
	}
	return out
}

func resultFromPayload[T any](payload map[string]any) *T {
	raw, ok := payload["result"]
	if !ok || raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return &out
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func testSuiteStepCount(steps json.RawMessage) int {
	if len(steps) == 0 {
		return 0
	}
	var raw []any
	if err := json.Unmarshal(steps, &raw); err != nil {
		return 0
	}
	return len(raw)
}

// ForceAsyncTestRun reports whether a suite should leave the API goroutine.
func (e *DeployEngine) ForceAsyncTestRun(steps json.RawMessage) bool {
	return e.ExceedsSyncGate(testSuiteStepCount(steps), 0)
}
