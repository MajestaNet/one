package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
)

// queryFilter matches the dataengine QueryFilter shape for JSON encoding.
type queryFilter struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value,omitempty"`
}

// StepResult is the outcome of one test step.
type StepResult struct {
	Index      int            `json:"index"`
	Type       string         `json:"type"`
	Status     string         `json:"status"` // passed|failed|skipped
	Message    string         `json:"message,omitempty"`
	DurationMs int64          `json:"durationMs"`
	Detail     map[string]any `json:"detail,omitempty"`
}

// TestRunSummary aggregates step outcomes.
type TestRunSummary struct {
	Total      int   `json:"total"`
	Passed     int   `json:"passed"`
	Failed     int   `json:"failed"`
	Skipped    int   `json:"skipped"`
	DurationMs int64 `json:"durationMs"`
}

// TestStep represents one step in a suite (union via Type).
type TestStep struct {
	Type          string         `json:"type"`
	ObjectAPIName string         `json:"objectApiName,omitempty"`
	FieldAPIName  string         `json:"fieldApiName,omitempty"`
	Data          map[string]any `json:"data,omitempty"`
	StoreAs       string         `json:"storeAs,omitempty"`
	ExpectError   bool           `json:"expectError,omitempty"`
	ErrorContains string         `json:"errorContains,omitempty"`
	Object        string         `json:"object,omitempty"`
	Filters       []queryFilter  `json:"filters,omitempty"`
	ExpectMinRows int            `json:"expectMinRows,omitempty"`
	// ADR-014 Phase 5 automation steps
	TestFile            string `json:"testFile,omitempty"`
	AutomationAPIName   string `json:"automationApiName,omitempty"`
	ExpectObjectAPIName string `json:"expectObjectApiName,omitempty"`
}

// RequireSuiteActive validates the suite is active before running.
func RequireSuiteActive(suite map[string]any) error {
	if suite == nil {
		return newNotFoundError("Test suite not found")
	}
	active, _ := suite["active"].(bool)
	if !active {
		apiName, _ := suite["apiName"].(string)
		return newValidationErrorf("Test suite %q is inactive", apiName)
	}
	return nil
}

type testRunDeps struct {
	meta  *metadata.Service
	data  *dataengine.Service
	actor *authz.Actor
	pool  *db.Pool
}

// RunTestSteps executes all steps and returns results and summary.
func RunTestSteps(
	ctx context.Context,
	rawSteps []any,
	meta *metadata.Service,
	data *dataengine.Service,
	actor *authz.Actor,
) ([]StepResult, TestRunSummary, error) {
	return RunTestStepsWithPool(ctx, rawSteps, meta, data, actor, nil)
}

// RunTestStepsWithPool is RunTestSteps with access to persisted customer_source_files.
func RunTestStepsWithPool(
	ctx context.Context,
	rawSteps []any,
	meta *metadata.Service,
	data *dataengine.Service,
	actor *authz.Actor,
	pool *db.Pool,
) ([]StepResult, TestRunSummary, error) {
	deps := testRunDeps{meta: meta, data: data, actor: actor, pool: pool}
	results := make([]StepResult, 0, len(rawSteps))
	vars := map[string]any{}
	started := time.Now()

	for i, rawStep := range rawSteps {
		stepStarted := time.Now()
		step, err := parseTestStep(rawStep)
		if err != nil {
			results = append(results, StepResult{
				Index:      i,
				Type:       "unknown",
				Status:     "failed",
				Message:    err.Error(),
				DurationMs: time.Since(stepStarted).Milliseconds(),
			})
			continue
		}

		detail, err := executeStep(ctx, step, deps, vars)
		durMs := time.Since(stepStarted).Milliseconds()
		if err != nil {
			results = append(results, StepResult{
				Index:      i,
				Type:       step.Type,
				Status:     "failed",
				Message:    err.Error(),
				DurationMs: durMs,
			})
		} else {
			results = append(results, StepResult{
				Index:      i,
				Type:       step.Type,
				Status:     "passed",
				DurationMs: durMs,
				Detail:     detail,
			})
		}
	}

	summary := TestRunSummary{
		Total:      len(results),
		DurationMs: time.Since(started).Milliseconds(),
	}
	for _, r := range results {
		switch r.Status {
		case "passed":
			summary.Passed++
		case "failed":
			summary.Failed++
		case "skipped":
			summary.Skipped++
		}
	}
	return results, summary, nil
}

func parseTestStep(raw any) (*TestStep, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal step: %w", err)
	}
	var s TestStep
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse step: %w", err)
	}
	if s.Type == "" {
		return nil, fmt.Errorf("step missing type")
	}
	return &s, nil
}

func resolveVars(data map[string]any, vars map[string]any) map[string]any {
	out := make(map[string]any, len(data))
	for k, v := range data {
		if s, ok := v.(string); ok && len(s) > 1 && s[0] == '$' {
			if val, found := vars[s[1:]]; found {
				out[k] = val
				continue
			}
		}
		out[k] = v
	}
	return out
}

func executeStep(
	ctx context.Context,
	step *TestStep,
	deps testRunDeps,
	vars map[string]any,
) (map[string]any, error) {
	switch step.Type {
	case "objectExists":
		if _, err := deps.meta.GetObject(ctx, step.ObjectAPIName); err != nil {
			if errors.Is(err, metadata.ErrNotFound) {
				return nil, fmt.Errorf("object %q does not exist", step.ObjectAPIName)
			}
			return nil, err
		}
		return map[string]any{"objectApiName": step.ObjectAPIName}, nil

	case "fieldExists":
		if _, err := deps.meta.GetField(ctx, step.ObjectAPIName, step.FieldAPIName); err != nil {
			if errors.Is(err, metadata.ErrNotFound) {
				return nil, fmt.Errorf("field %q.%q does not exist", step.ObjectAPIName, step.FieldAPIName)
			}
			return nil, err
		}
		return map[string]any{"objectApiName": step.ObjectAPIName, "fieldApiName": step.FieldAPIName}, nil

	case "createRecord":
		if deps.data == nil {
			return nil, newValidationError("DataEngine required for createRecord steps")
		}
		payload := resolveVars(step.Data, vars)
		record, err := deps.data.Create(ctx, step.ObjectAPIName, payload, deps.actor)
		if err != nil {
			return nil, err
		}
		id, _ := record["Id"].(string)
		if step.StoreAs != "" {
			vars[step.StoreAs] = id
		}
		return map[string]any{"Id": id, "objectApiName": step.ObjectAPIName}, nil

	case "assertValidation":
		if deps.data == nil {
			return nil, newValidationError("DataEngine required for assertValidation steps")
		}
		payload := resolveVars(step.Data, vars)
		_, createErr := deps.data.Create(ctx, step.ObjectAPIName, payload, deps.actor)
		if step.ExpectError {
			if createErr == nil {
				return nil, newValidationError("Expected validation error but create succeeded")
			}
			if step.ErrorContains != "" {
				if msg := createErr.Error(); len(msg) > 0 {
					if !containsString(msg, step.ErrorContains) {
						return nil, newValidationErrorf(
							"Expected error containing %q, got: %s", step.ErrorContains, msg)
					}
				}
			}
			return map[string]any{"error": createErr.Error()}, nil
		}
		if createErr != nil {
			return nil, createErr
		}
		return map[string]any{"ok": true}, nil

	case "query":
		if deps.data == nil {
			return nil, newValidationError("DataEngine required for query steps")
		}
		limit := step.ExpectMinRows
		if limit < 1 {
			limit = 1
		}
		// Build filter list with default op.
		filters := make([]queryFilter, 0, len(step.Filters))
		for _, f := range step.Filters {
			op := f.Op
			if op == "" {
				op = "eq"
			}
			filters = append(filters, queryFilter{Field: f.Field, Op: op, Value: f.Value})
		}
		// Encode as JSON for the dataengine query method.
		qreqMap := map[string]any{
			"object":  step.Object,
			"filters": filters,
			"limit":   limit,
		}
		qreqJSON, err := json.Marshal(qreqMap)
		if err != nil {
			return nil, fmt.Errorf("marshal query request: %w", err)
		}
		result, err := deps.data.Query(ctx, json.RawMessage(qreqJSON), dataengine.QueryVisibility{})
		if err != nil {
			return nil, err
		}
		if len(result.Records) < step.ExpectMinRows {
			return nil, newValidationErrorf(
				"Expected at least %d rows, got %d", step.ExpectMinRows, len(result.Records))
		}
		return map[string]any{"rowCount": len(result.Records)}, nil

	case "automationUnitPass":
		return executeAutomationUnitPass(ctx, step, deps)

	case "automationContract":
		return executeAutomationContract(ctx, step, deps)

	default:
		return nil, newValidationErrorf("Unsupported step type: %s", step.Type)
	}
}

func containsString(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && (s == substr ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
