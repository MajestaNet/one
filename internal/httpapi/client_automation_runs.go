package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/automation"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
)

type automationDefRow struct {
	ID            string
	APIName       string
	Label         string
	ObjectAPIName string
	TriggerEvent  string
	Active        bool
	Runtime       string
	Execution     string
	Source        string
	EntryFile     string
	ActionsJSON   []byte
}

func (s *Server) handleListCallableAutomations(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	rows, err := pool.Query(r.Context(), `
SELECT api_name, COALESCE(label,''), COALESCE(object_api_name,''), COALESCE(trigger_event,''),
       active, COALESCE(runtime,'actions'), COALESCE(execution,'async')
FROM metadata_automations
WHERE active=true
ORDER BY api_name`)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var apiName, label, objectAPIName, triggerEvent, runtime, execution string
		var active bool
		if err := rows.Scan(&apiName, &label, &objectAPIName, &triggerEvent, &active, &runtime, &execution); err != nil {
			writeAPIError(w, err)
			return
		}
		ok := true
		if s.automationAz != nil {
			ok, err = s.automationAz.ActorCanRunAutomation(r.Context(), actor, apiName)
			if err != nil {
				writeAPIError(w, err)
				return
			}
		}
		if !ok {
			continue
		}
		out = append(out, map[string]any{
			"apiName":       apiName,
			"label":         label,
			"objectApiName": objectAPIName,
			"triggerEvent":  triggerEvent,
			"active":        active,
			"runtime":       runtime,
			"execution":     execution,
		})
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"automations": out})
}

func (s *Server) handleCreateAutomationRun(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	apiName := strings.TrimSpace(r.PathValue("apiName"))
	if apiName == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "apiName required")
		return
	}
	var body struct {
		Input map[string]any `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if body.Input == nil {
		body.Input = map[string]any{}
	}

	def, err := loadActiveAutomation(r.Context(), pool, apiName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "automation not found or inactive")
			return
		}
		writeAPIError(w, err)
		return
	}
	if s.automationAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "automation authorization not configured")
		return
	}
	if err := s.automationAz.AssertCanRunAutomation(r.Context(), actor, apiName); err != nil {
		writeAPIError(w, err)
		return
	}

	if strings.EqualFold(strings.TrimSpace(def.Execution), "sync") {
		s.runAutomationSyncInline(w, r, pool, actor, def, body.Input)
		return
	}

	payload, err := json.Marshal(map[string]any{
		"automationId":  def.ID,
		"apiName":       def.APIName,
		"objectApiName": def.ObjectAPIName,
		"action":        "manual",
		"actorId":       actor.ID,
		"input":         body.Input,
		"runtime":       def.Runtime,
		"execution":     def.Execution,
	})
	if err != nil {
		writeAPIError(w, err)
		return
	}
	var jobID, status string
	var createdAt time.Time
	err = pool.QueryRow(r.Context(), `
INSERT INTO jobs (job_type, payload)
VALUES ('automation.run', $1::jsonb)
RETURNING id::text, status, created_at`, string(payload)).Scan(&jobID, &status, &createdAt)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":                jobID,
		"automationApiName": def.APIName,
		"status":            status,
		"execution":         def.Execution,
		"input":             body.Input,
		"createdAt":         createdAt,
	})
}

func (s *Server) runAutomationSyncInline(w http.ResponseWriter, r *http.Request, pool *db.Pool, actor *authz.Actor, def *automationDefRow, input map[string]any) {
	if s.data == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data engine not configured")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), automation.SyncDeadline)
	defer cancel()

	trigger := automation.SyncTrigger{
		Action:        "manual",
		ObjectAPIName: def.ObjectAPIName,
		Data:          input,
		ActorID:       actor.ID,
	}
	host := automation.HostBridge(manualInvokeHost{svc: s.data, actor: actor})
	if s.objectAz != nil {
		host = automation.AuthzHost{Inner: host, Object: s.objectAz, Actor: actor}
	}
	host = automation.SyncOutboundBan{Inner: host}
	ctx = automation.WithSyncGuest(ctx)
	if s.actions != nil {
		act := actor
		inv := s.actions
		host = automation.BindActions(host, func(ctx context.Context, apiName string, input map[string]any) (map[string]any, error) {
			return inv.Invoke(ctx, act, apiName, input)
		})
	}

	actions, err := automation.ActionsFromJSON(def.ActionsJSON)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	auto := automation.SyncAutomation{
		ID:        def.ID,
		APIName:   def.APIName,
		Runtime:   def.Runtime,
		Execution: def.Execution,
		Actions:   actions,
		Source:    def.Source,
		EntryFile: def.EntryFile,
		Host:      host,
	}
	mut := manualSyncMutator{host: host}
	started := time.Now().UTC()
	if err := automation.ExecuteSync(ctx, mut, trigger, auto); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"automationApiName": def.APIName,
			"status":            "failed",
			"execution":         "sync",
			"input":             input,
			"lastError":         err.Error(),
			"createdAt":         started,
			"completedAt":       time.Now().UTC(),
		})
		return
	}
	details, _ := json.Marshal(map[string]any{
		"apiName": def.APIName, "action": "manual", "runtime": def.Runtime, "execution": "sync",
	})
	_, _ = pool.Exec(r.Context(), `
INSERT INTO audit_log (actor_id, action, object_api_name, details)
VALUES ($1::uuid, 'automation.run', $2, $3::jsonb)`,
		actor.ID, def.ObjectAPIName, string(details))
	writeJSON(w, http.StatusOK, map[string]any{
		"automationApiName": def.APIName,
		"status":            "completed",
		"execution":         "sync",
		"input":             input,
		"createdAt":         started,
		"completedAt":       time.Now().UTC(),
	})
}

func (s *Server) handleGetAutomationRun(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "id required")
		return
	}
	var apiName, status string
	var lastError *string
	var createdAt time.Time
	var completedAt *time.Time
	var payload []byte
	err := pool.QueryRow(r.Context(), `
SELECT COALESCE(payload->>'apiName',''), status, last_error, created_at, completed_at, payload
FROM jobs
WHERE id=$1::uuid AND job_type='automation.run'
  AND ($2::boolean OR payload->>'actorId'=$3)`,
		id, actor.IsAdmin, actor.ID,
	).Scan(&apiName, &status, &lastError, &createdAt, &completedAt, &payload)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "automation run not found")
			return
		}
		writeAPIError(w, err)
		return
	}
	var p map[string]any
	_ = json.Unmarshal(payload, &p)
	input, _ := p["input"].(map[string]any)
	out := map[string]any{
		"id":                id,
		"automationApiName": apiName,
		"status":            status,
		"input":             input,
		"createdAt":         createdAt,
	}
	if lastError != nil && *lastError != "" {
		out["lastError"] = *lastError
	}
	if completedAt != nil {
		out["completedAt"] = *completedAt
	}
	writeJSON(w, http.StatusOK, out)
}

func loadActiveAutomation(ctx context.Context, pool *db.Pool, apiName string) (*automationDefRow, error) {
	var def automationDefRow
	err := pool.QueryRow(ctx, `
SELECT id::text, api_name, COALESCE(label,''), COALESCE(object_api_name,''), COALESCE(trigger_event,''),
       active, COALESCE(runtime,'actions'), COALESCE(execution,'async'),
       COALESCE(source,''), COALESCE(entry_file,''), COALESCE(actions, '[]'::jsonb)
FROM metadata_automations
WHERE api_name=$1 AND active=true`, apiName).Scan(
		&def.ID, &def.APIName, &def.Label, &def.ObjectAPIName, &def.TriggerEvent,
		&def.Active, &def.Runtime, &def.Execution, &def.Source, &def.EntryFile, &def.ActionsJSON)
	if err != nil {
		return nil, err
	}
	return &def, nil
}

type manualInvokeHost struct {
	svc   *dataengine.Service
	actor *authz.Actor
}

func (h manualInvokeHost) CreateRecord(ctx context.Context, objectAPIName string, data map[string]any) (string, error) {
	if h.svc == nil {
		return "", fmt.Errorf("data engine not configured")
	}
	rec, err := h.svc.Create(ctx, objectAPIName, data, h.actor)
	if err != nil {
		return "", err
	}
	id, _ := rec["Id"].(string)
	return id, nil
}

func (h manualInvokeHost) UpdateRecord(ctx context.Context, objectAPIName, recordID string, data map[string]any) error {
	if h.svc == nil {
		return fmt.Errorf("data engine not configured")
	}
	_, err := h.svc.Update(ctx, objectAPIName, recordID, data, h.actor)
	return err
}

func (h manualInvokeHost) GetRecord(ctx context.Context, objectAPIName, recordID string) (map[string]any, error) {
	if h.svc == nil {
		return nil, fmt.Errorf("data engine not configured")
	}
	return h.svc.Get(ctx, objectAPIName, recordID)
}

func (h manualInvokeHost) DeleteRecord(ctx context.Context, objectAPIName, recordID string) error {
	if h.svc == nil {
		return fmt.Errorf("data engine not configured")
	}
	return h.svc.Delete(ctx, objectAPIName, recordID, h.actor)
}

func (h manualInvokeHost) Query(ctx context.Context, req map[string]any) (map[string]any, error) {
	if h.svc == nil {
		return nil, fmt.Errorf("data engine not configured")
	}
	m := map[string]any{}
	for k, v := range req {
		m[k] = v
	}
	if _, ok := m["object"]; !ok {
		if o, ok := m["objectApiName"].(string); ok {
			m["object"] = o
		}
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	res, err := h.svc.Query(ctx, raw, dataengine.QueryVisibility{})
	if err != nil {
		return nil, err
	}
	records := make([]map[string]any, 0, len(res.Records))
	for _, rec := range res.Records {
		records = append(records, rec)
	}
	return map[string]any{"records": records, "totalSize": res.TotalSize}, nil
}

func (h manualInvokeHost) InvokeAction(context.Context, string, map[string]any) (map[string]any, error) {
	return nil, fmt.Errorf("invokeAction is not available on this host")
}

type manualSyncMutator struct {
	host automation.HostBridge
}

func (m manualSyncMutator) CreateRecord(ctx context.Context, objectAPIName string, data map[string]any) (string, error) {
	return m.host.CreateRecord(ctx, objectAPIName, data)
}
func (m manualSyncMutator) UpdateRecord(ctx context.Context, objectAPIName, recordID string, data map[string]any) error {
	return m.host.UpdateRecord(ctx, objectAPIName, recordID, data)
}
func (m manualSyncMutator) GetRecord(ctx context.Context, objectAPIName, recordID string) (map[string]any, error) {
	return m.host.GetRecord(ctx, objectAPIName, recordID)
}
