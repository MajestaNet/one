package inference

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/MajestaNet/ide/internal/db"
)

// AppendRunEvent inserts the next seq event for a run.
func AppendRunEvent(ctx context.Context, pool *db.Pool, runID, eventType string, payload any) (int, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	var seq int
	err = pool.QueryRow(ctx, `
WITH next AS (
  SELECT COALESCE(MAX(seq), 0) + 1 AS seq FROM agent_run_events WHERE run_id=$1::uuid
)
INSERT INTO agent_run_events (run_id, seq, event_type, payload)
SELECT $1::uuid, next.seq, $2, $3::jsonb FROM next
RETURNING seq`, runID, eventType, string(b)).Scan(&seq)
	return seq, err
}

// ListRunEventsAfter returns events with seq > afterSeq.
func ListRunEventsAfter(ctx context.Context, pool *db.Pool, runID string, afterSeq int) ([]RunEvent, error) {
	rows, err := pool.Query(ctx, `
SELECT seq, event_type, payload, created_at
FROM agent_run_events WHERE run_id=$1::uuid AND seq>$2
ORDER BY seq ASC LIMIT 500`, runID, afterSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RunEvent
	for rows.Next() {
		var e RunEvent
		var raw []byte
		if err := rows.Scan(&e.Seq, &e.Type, &raw, &e.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &e.Payload)
		out = append(out, e)
	}
	return out, rows.Err()
}

// RunEvent is one persisted stream event.
type RunEvent struct {
	Seq       int
	Type      string
	Payload   any
	CreatedAt interface{}
}

// StatusJSON builds a public status view of install inference (no secrets).
func StatusJSON(cfg *InstallConfig, providers []Provider, doTokenConfigured bool) map[string]any {
	modes := map[string]string{}
	for m, id := range DOModeModels {
		modes[string(m)] = id
	}
	var doMode any
	if cfg.DOMode != nil {
		doMode = string(*cfg.DOMode)
	}
	var model any
	if cfg.DOEnabled && cfg.DOMode != nil {
		if id, err := ModelForMode(*cfg.DOMode); err == nil {
			model = id
		}
	}
	plist := make([]map[string]any, 0, len(providers))
	for _, p := range providers {
		m := map[string]any{
			"apiName": p.APIName, "label": p.Label, "baseUrl": p.BaseURL,
			"defaultModel": p.DefaultModel, "active": p.Active,
			"hasSecret": p.SecretRef != nil && *p.SecretRef != "",
			"createdAt": p.CreatedAt, "updatedAt": p.UpdatedAt,
		}
		if p.SecretRef != nil {
			m["secretRef"] = *p.SecretRef
		}
		plist = append(plist, m)
	}
	out := map[string]any{
		"activeSource":      string(cfg.ActiveSource),
		"doEnabled":         cfg.DOEnabled,
		"doMode":            doMode,
		"doModelId":         model,
		"modelId":           model, // plan contract alias of doModelId
		"doTokenConfigured": doTokenConfigured,
		"doModeModels":      modes,
		"billingNotice":     BillingNotice,
		"prepaid":           true,
		"providers":         plist,
		"updatedAt":         cfg.UpdatedAt,
	}
	if cfg.DefaultProviderAPIName != nil {
		out["defaultProviderApiName"] = *cfg.DefaultProviderAPIName
	}
	return out
}

// FormatRouteError maps router errors to API-safe messages.
func FormatRouteError(err error) (code, msg string) {
	if err == nil {
		return "", ""
	}
	switch {
	case errors.Is(err, ErrNotConfigured):
		return "INFERENCE_NOT_CONFIGURED", "Configure install inference: Metadata /metadata/v1/inference/providers + /inference/config (BYO), or Deploy PUT /deploy/v1/cloud/inference (Native DigitalOcean)."
	case errors.Is(err, ErrDOTokenMissing):
		return "DO_TOKEN_MISSING", "Native DigitalOcean Inference requires DIGITALOCEAN_API_TOKEN on this install."
	case errors.Is(err, ErrEgressDenied):
		return "EGRESS_DENIED", "BYO inference host must be on the install egress allowlist (Metadata → egress)."
	default:
		return "INFERENCE_ERROR", "inference request failed"
	}
}
