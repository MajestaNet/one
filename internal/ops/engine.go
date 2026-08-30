// Package ops implements install-local product upgrade orchestration (ADR-007).
// It is distinct from the commercial Deploy API (customer metadata promote).
package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/deploy"
)

const (
	StatusPending    = "pending"
	StatusRolling    = "rolling"
	StatusGating     = "gating"
	StatusSucceeded  = "succeeded"
	StatusFailed     = "failed"
	StatusRolledBack = "rolled_back"

	PlatformSmokeSuite    = "PlatformSmoke"
	PostUpgradeSmokeSuite = "PostUpgradeSmoke"
)

// Roller drives infrastructure for product image rolls.
type Roller interface {
	// Mode is "local" or "ecs".
	Mode() string
	// CaptureCurrent returns current task definition ARNs (may be empty in local mode).
	CaptureCurrent(ctx context.Context) (apiTaskDef, workerTaskDef string, err error)
	// Roll registers new task defs (when supported) and updates services.
	Roll(ctx context.Context, req RollRequest) (newAPI, newWorker string, err error)
	// Rollback restores previous task definitions.
	Rollback(ctx context.Context, apiTaskDef, workerTaskDef string) error
}

// RollRequest is the target product version and images.
type RollRequest struct {
	APIImage       string
	WorkerImage    string
	ProductVersion string
}

// HealthChecker probes /healthz and /readyz.
type HealthChecker interface {
	Check(ctx context.Context) error
}

// Engine orchestrates product upgrades.
type Engine struct {
	pool           *db.Pool
	deploy         *deploy.DeployEngine
	roller         Roller
	health         HealthChecker
	productVersion string
	publicURL      string
}

// Options configures Engine.
type Options struct {
	ProductVersion string
	PublicURL      string
	Roller         Roller
	Health         HealthChecker
}

// NewEngine constructs an Ops upgrade engine.
func NewEngine(pool *db.Pool, deployEng *deploy.DeployEngine, opts Options) *Engine {
	roller := opts.Roller
	if roller == nil {
		roller = LocalRoller()
	}
	health := opts.Health
	if health == nil {
		health = HTTPHealthChecker{BaseURL: opts.PublicURL}
	}
	return &Engine{
		pool:           pool,
		deploy:         deployEng,
		roller:         roller,
		health:         health,
		productVersion: opts.ProductVersion,
		publicURL:      strings.TrimRight(opts.PublicURL, "/"),
	}
}

// Available describes what an admin can confirm next.
type Available struct {
	CurrentVersion string `json:"currentVersion"`
	RollerMode     string `json:"rollerMode"`
	PublicURL      string `json:"publicURL,omitempty"`
	PlatformSmoke  string `json:"platformSmokeSuite"`
	OptionalSuite  string `json:"optionalCustomerSuite"`
	Notes          string `json:"notes"`
}

// GetAvailable returns current version and upgrade tips.
func (e *Engine) GetAvailable() *Available {
	return &Available{
		CurrentVersion: e.productVersion,
		RollerMode:     e.roller.Mode(),
		PublicURL:      e.publicURL,
		PlatformSmoke:  PlatformSmokeSuite,
		OptionalSuite:  PostUpgradeSmokeSuite,
		Notes:          "Confirm via POST /ops/v1/upgrades with target images + productVersion. Product rolls are not Deploy promotions.",
	}
}

// UpgradeRow is a platform_upgrades row.
type UpgradeRow struct {
	ID                    string          `json:"id"`
	Status                string          `json:"status"`
	FromVersion           string          `json:"fromVersion"`
	ToVersion             string          `json:"toVersion"`
	APIImage              string          `json:"apiImage"`
	WorkerImage           string          `json:"workerImage"`
	PreviousAPITaskDef    *string         `json:"previousApiTaskDef,omitempty"`
	PreviousWorkerTaskDef *string         `json:"previousWorkerTaskDef,omitempty"`
	NewAPITaskDef         *string         `json:"newApiTaskDef,omitempty"`
	NewWorkerTaskDef      *string         `json:"newWorkerTaskDef,omitempty"`
	TestRunIDs            json.RawMessage `json:"testRunIds"`
	GateResult            json.RawMessage `json:"gateResult,omitempty"`
	Error                 *string         `json:"error,omitempty"`
	CreatedBy             *string         `json:"createdBy,omitempty"`
	CreatedAt             time.Time       `json:"createdAt"`
	StartedAt             *time.Time      `json:"startedAt,omitempty"`
	CompletedAt           *time.Time      `json:"completedAt,omitempty"`
}

// ConfirmInput starts a product upgrade.
type ConfirmInput struct {
	APIImage       string
	WorkerImage    string
	ProductVersion string
	Actor          *authz.Actor
	// SkipRoll skips infrastructure roll (test gate only) — used by unit tests / dry local.
	SkipRoll bool
}

// Confirm creates an upgrade run, rolls (unless SkipRoll / local), gates, and rolls back on failure.
func (e *Engine) Confirm(ctx context.Context, in ConfirmInput) (*UpgradeRow, error) {
	if in.APIImage == "" || in.WorkerImage == "" || in.ProductVersion == "" {
		return nil, &APIError{Status: http.StatusBadRequest, Code: "VALIDATION_ERROR", Message: "apiImage, workerImage, and productVersion are required"}
	}
	var createdBy *string
	if in.Actor != nil {
		createdBy = &in.Actor.ID
	}

	var id string
	err := e.pool.QueryRow(ctx, `
INSERT INTO platform_upgrades (status, from_version, to_version, api_image, worker_image, created_by, started_at)
VALUES ($1,$2,$3,$4,$5,$6,now())
RETURNING id::text`,
		StatusPending, e.productVersion, in.ProductVersion, in.APIImage, in.WorkerImage, createdBy,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert upgrade: %w", err)
	}

	row, err := e.runUpgrade(ctx, id, in)
	if err != nil {
		_ = e.markFailed(ctx, id, err.Error())
		got, getErr := e.Get(ctx, id)
		if getErr == nil {
			return got, err
		}
		return nil, err
	}
	return row, nil
}

func (e *Engine) runUpgrade(ctx context.Context, id string, in ConfirmInput) (*UpgradeRow, error) {
	prevAPI, prevWorker, err := e.roller.CaptureCurrent(ctx)
	if err != nil {
		return nil, fmt.Errorf("capture current: %w", err)
	}
	_, _ = e.pool.Exec(ctx, `
UPDATE platform_upgrades SET
  status=$2,
  previous_api_task_def=NULLIF($3,''),
  previous_worker_task_def=NULLIF($4,'')
WHERE id=$1::uuid`, id, StatusRolling, prevAPI, prevWorker)

	var newAPI, newWorker string
	if !in.SkipRoll {
		newAPI, newWorker, err = e.roller.Roll(ctx, RollRequest{
			APIImage: in.APIImage, WorkerImage: in.WorkerImage, ProductVersion: in.ProductVersion,
		})
		if err != nil {
			_ = e.rollbackBestEffort(ctx, id, prevAPI, prevWorker)
			return nil, fmt.Errorf("roll: %w", err)
		}
		_, _ = e.pool.Exec(ctx, `
UPDATE platform_upgrades SET new_api_task_def=NULLIF($2,''), new_worker_task_def=NULLIF($3,'')
WHERE id=$1::uuid`, id, newAPI, newWorker)
		if w, ok := e.roller.(interface {
			WaitStable(context.Context) error
		}); ok {
			if err := w.WaitStable(ctx); err != nil {
				_ = e.rollbackBestEffort(ctx, id, prevAPI, prevWorker)
				return nil, fmt.Errorf("wait stable: %w", err)
			}
		}
	}

	_, _ = e.pool.Exec(ctx, `UPDATE platform_upgrades SET status=$2 WHERE id=$1::uuid`, id, StatusGating)

	gate, testIDs, gateErr := e.runGate(ctx, in.Actor)
	gateJSON, _ := json.Marshal(gate)
	idsJSON, _ := json.Marshal(testIDs)
	_, _ = e.pool.Exec(ctx, `
UPDATE platform_upgrades SET test_run_ids=$2::jsonb, gate_result=$3::jsonb WHERE id=$1::uuid`,
		id, string(idsJSON), string(gateJSON))

	if gateErr != nil {
		_ = e.rollbackBestEffort(ctx, id, prevAPI, prevWorker)
		return nil, gateErr
	}

	_, err = e.pool.Exec(ctx, `
UPDATE platform_upgrades SET status=$2, completed_at=now(), error=NULL WHERE id=$1::uuid`,
		id, StatusSucceeded)
	if err != nil {
		return nil, err
	}
	// Local/dev: stamp in-process version so Available reflects the confirmed target.
	if e.roller.Mode() == "local" {
		e.productVersion = in.ProductVersion
	}
	return e.Get(ctx, id)
}

func (e *Engine) runGate(ctx context.Context, actor *authz.Actor) (map[string]any, []string, error) {
	out := map[string]any{"suites": []any{}}
	var ids []string

	if e.health != nil {
		if err := e.health.Check(ctx); err != nil {
			out["health"] = map[string]any{"ok": false, "error": err.Error()}
			return out, ids, fmt.Errorf("health gate: %w", err)
		}
		out["health"] = map[string]any{"ok": true}
	}

	if e.deploy == nil {
		out["note"] = "deploy engine unavailable; skipped test suites"
		return out, ids, nil
	}

	runOne := func(name string, required bool) error {
		res, err := e.deploy.StartTestRun(ctx, struct {
			SuiteAPIName string
			Actor        *authz.Actor
			Async        bool
			Trigger      string
		}{SuiteAPIName: name, Actor: actor, Async: false, Trigger: "product_upgrade"})
		if err != nil {
			if !required && isNotFoundish(err) {
				suites, _ := out["suites"].([]any)
				out["suites"] = append(suites, map[string]any{"suite": name, "skipped": true})
				return nil
			}
			return err
		}
		if res.Run != nil {
			ids = append(ids, res.Run.ID)
			suites, _ := out["suites"].([]any)
			out["suites"] = append(suites, map[string]any{
				"suite": name, "status": res.Run.Status, "id": res.Run.ID,
			})
			if res.Run.Status != "passed" {
				return fmt.Errorf("suite %s status=%s", name, res.Run.Status)
			}
		}
		return nil
	}

	if err := runOne(PlatformSmokeSuite, true); err != nil {
		return out, ids, err
	}
	if err := runOne(PostUpgradeSmokeSuite, false); err != nil {
		return out, ids, err
	}
	return out, ids, nil
}

func isNotFoundish(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, deploy.ErrNotFound) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found")
}

func (e *Engine) rollbackBestEffort(ctx context.Context, id, prevAPI, prevWorker string) error {
	if prevAPI != "" || prevWorker != "" || e.roller.Mode() == "local" {
		if err := e.roller.Rollback(ctx, prevAPI, prevWorker); err != nil {
			_ = e.markFailed(ctx, id, "rollback failed: "+err.Error())
			return err
		}
	}
	_, _ = e.pool.Exec(ctx, `
UPDATE platform_upgrades SET status=$2, completed_at=now() WHERE id=$1::uuid`, id, StatusRolledBack)
	return nil
}

func (e *Engine) markFailed(ctx context.Context, id, msg string) error {
	_, err := e.pool.Exec(ctx, `
UPDATE platform_upgrades SET status=$2, error=$3, completed_at=now() WHERE id=$1::uuid AND status NOT IN ($4,$5)`,
		id, StatusFailed, msg, StatusSucceeded, StatusRolledBack)
	return err
}

// Rollback forces a rollback for an existing run.
func (e *Engine) Rollback(ctx context.Context, id string, actor *authz.Actor) (*UpgradeRow, error) {
	row, err := e.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	// Explicit rollback is allowed after success or a prior rollback (operator decision);
	// no status gate — operator-initiated force rollback.
	prevAPI := ""
	prevWorker := ""
	if row.PreviousAPITaskDef != nil {
		prevAPI = *row.PreviousAPITaskDef
	}
	if row.PreviousWorkerTaskDef != nil {
		prevWorker = *row.PreviousWorkerTaskDef
	}
	if err := e.roller.Rollback(ctx, prevAPI, prevWorker); err != nil {
		_ = e.markFailed(ctx, id, "rollback failed: "+err.Error())
		return e.Get(ctx, id)
	}
	_, _ = e.pool.Exec(ctx, `
UPDATE platform_upgrades SET status=$2, completed_at=now(), error=NULL WHERE id=$1::uuid`,
		id, StatusRolledBack)
	_ = actor // reserved for audit
	return e.Get(ctx, id)
}

// Get returns one upgrade run.
func (e *Engine) Get(ctx context.Context, id string) (*UpgradeRow, error) {
	var row UpgradeRow
	err := e.pool.QueryRow(ctx, `
SELECT id::text, status, from_version, to_version, api_image, worker_image,
  previous_api_task_def, previous_worker_task_def, new_api_task_def, new_worker_task_def,
  test_run_ids, gate_result, error, created_by::text, created_at, started_at, completed_at
FROM platform_upgrades WHERE id=$1::uuid`, id).Scan(
		&row.ID, &row.Status, &row.FromVersion, &row.ToVersion, &row.APIImage, &row.WorkerImage,
		&row.PreviousAPITaskDef, &row.PreviousWorkerTaskDef, &row.NewAPITaskDef, &row.NewWorkerTaskDef,
		&row.TestRunIDs, &row.GateResult, &row.Error, &row.CreatedBy, &row.CreatedAt, &row.StartedAt, &row.CompletedAt,
	)
	if err != nil {
		return nil, &APIError{Status: http.StatusNotFound, Code: "NOT_FOUND", Message: "upgrade run not found"}
	}
	return &row, nil
}

// List returns recent upgrade runs.
func (e *Engine) List(ctx context.Context, limit int) ([]UpgradeRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := e.pool.Query(ctx, `
SELECT id::text, status, from_version, to_version, api_image, worker_image,
  previous_api_task_def, previous_worker_task_def, new_api_task_def, new_worker_task_def,
  test_run_ids, gate_result, error, created_by::text, created_at, started_at, completed_at
FROM platform_upgrades ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UpgradeRow
	for rows.Next() {
		var row UpgradeRow
		if err := rows.Scan(
			&row.ID, &row.Status, &row.FromVersion, &row.ToVersion, &row.APIImage, &row.WorkerImage,
			&row.PreviousAPITaskDef, &row.PreviousWorkerTaskDef, &row.NewAPITaskDef, &row.NewWorkerTaskDef,
			&row.TestRunIDs, &row.GateResult, &row.Error, &row.CreatedBy, &row.CreatedAt, &row.StartedAt, &row.CompletedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// APIError is an HTTP-mapped error.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string    { return e.Message }
func (e *APIError) CodeName() string { return e.Code }
