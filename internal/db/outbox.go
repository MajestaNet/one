package db

import (
	"context"
	"encoding/json"
)

// System outbox event types for customer BYO alerting (BP-038). No product mailer.
const (
	EventInstallClaimed           = "install.claimed"
	EventPrincipalCreated         = "principal.created"
	EventPrincipalPasswordChanged = "principal.password_changed"
)

// EnqueueOutbox inserts an unpublished outbox event for webhook delivery.
// recordID may be empty (stored as NULL). payload must be JSON-serializable.
func EnqueueOutbox(ctx context.Context, pool *Pool, eventType string, recordID string, payload map[string]any) error {
	if pool == nil {
		return nil
	}
	if payload == nil {
		payload = map[string]any{}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var rid any
	if recordID != "" {
		rid = recordID
	}
	_, err = pool.Exec(ctx, `
INSERT INTO outbox_events (event_type, record_id, payload)
VALUES ($1, $2::uuid, $3::jsonb)`, eventType, rid, raw)
	return err
}
