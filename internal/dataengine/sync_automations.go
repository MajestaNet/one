package dataengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/automation"
	"github.com/jackc/pgx/v5"
)

const (
	maxSyncDepth = automation.MaxSyncDepth
	maxSyncOps   = automation.MaxSyncOps
	syncDeadline = automation.SyncDeadline
)

// txMutator applies record mutations on the open write transaction (ADR-014 Phase 3).
type txMutator struct {
	svc   *Service
	actor *authz.Actor
}

func (m *txMutator) CreateRecord(ctx context.Context, objectAPIName string, data map[string]any) (string, error) {
	if err := bumpSyncOps(ctx); err != nil {
		return "", err
	}
	rec, err := m.svc.createWith(ctx, objectAPIName, data, m.actor)
	if err != nil {
		return "", err
	}
	id, _ := rec["Id"].(string)
	return id, nil
}

func (m *txMutator) UpdateRecord(ctx context.Context, objectAPIName, recordID string, data map[string]any) error {
	if err := bumpSyncOps(ctx); err != nil {
		return err
	}
	_, err := m.svc.updateWith(ctx, objectAPIName, recordID, data, m.actor)
	return err
}

func (m *txMutator) GetRecord(ctx context.Context, objectAPIName, recordID string) (map[string]any, error) {
	rec, err := m.svc.Get(ctx, objectAPIName, recordID)
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *Service) afterWrite(
	ctx context.Context,
	actor *authz.Actor,
	action, objectAPIName, recordID string,
	details map[string]any,
) error {
	q := s.querier(ctx)
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `
INSERT INTO audit_log (actor_id, action, object_api_name, record_id, details)
VALUES ($1::uuid, $2, $3, $4::uuid, $5::jsonb)`,
		actor.ID, "record."+action, objectAPIName, recordID, detailsJSON,
	)
	if err != nil {
		return err
	}

	eventType := "Record" + stringsTitle(action)
	payload := map[string]any{
		"action":        action,
		"objectApiName": objectAPIName,
		"recordId":      recordID,
		"actorId":       actor.ID,
	}
	for k, v := range details {
		payload[k] = v
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `
INSERT INTO outbox_events (event_type, object_api_name, record_id, payload)
VALUES ($1, $2, $3::uuid, $4::jsonb)`,
		eventType, objectAPIName, recordID, payloadJSON,
	)
	if err != nil {
		return err
	}

	if err := s.enqueueSharingRecalc(ctx, map[string]any{
		"scope":         "record",
		"objectApiName": objectAPIName,
		"recordId":      recordID,
	}); err != nil {
		return err
	}

	return s.dispatchAutomations(ctx, actor, action, objectAPIName, recordID, details)
}

// enqueueSharingRecalc uses the current transaction when present. Record data,
// audit/outbox rows, and derived-sharing work must commit or roll back together;
// otherwise an all-or-none ingest can leave jobs for records that never existed.
func (s *Service) enqueueSharingRecalc(ctx context.Context, payload map[string]any) error {
	q := s.querier(ctx)
	var enabled bool
	err := q.QueryRow(ctx, `
SELECT record_sharing_enabled FROM organization_settings WHERE id = true`,
	).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `INSERT INTO jobs (job_type, payload) VALUES ('sharing.recalc', $1::jsonb)`, b)
	return err
}

func (s *Service) dispatchAutomations(
	ctx context.Context,
	actor *authz.Actor,
	action, objectAPIName, recordID string,
	details map[string]any,
) error {
	q := s.querier(ctx)
	rows, err := q.Query(ctx, `
SELECT id::text, api_name, trigger_event, COALESCE(actions, '[]'::jsonb),
       COALESCE(runtime, 'actions'), COALESCE(execution, 'async'),
       COALESCE(source, ''), COALESCE(entry_file, '')
FROM metadata_automations
WHERE object_api_name = $1 AND active = true
ORDER BY api_name`, objectAPIName)
	if err != nil {
		return err
	}
	defer rows.Close()

	type autoRow struct {
		id, apiName, trigger, runtime, execution, source, entryFile string
		actions                                                     []byte
	}
	var list []autoRow
	for rows.Next() {
		var r autoRow
		if err := rows.Scan(&r.id, &r.apiName, &r.trigger, &r.actions, &r.runtime, &r.execution, &r.source, &r.entryFile); err != nil {
			return err
		}
		if r.trigger != action && r.trigger != "write" {
			continue
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	triggerData := map[string]any{}
	if details != nil {
		if d, ok := details["data"].(map[string]any); ok {
			triggerData = d
		} else if p, ok := details["patch"].(map[string]any); ok {
			triggerData = p
		}
	}

	for _, r := range list {
		if s.AutomationAz != nil {
			if err := s.AutomationAz.AssertCanRunAutomation(ctx, actor, r.apiName); err != nil {
				return fmt.Errorf("automation %s: %w", r.apiName, err)
			}
		}
		exec := automation.NormalizeExecution(r.execution)
		actions, err := automation.ActionsFromJSON(r.actions)
		if err != nil {
			return fmt.Errorf("automation %s actions: %w", r.apiName, err)
		}
		if exec == automation.ExecutionSync {
			if err := s.runSyncAutomation(ctx, actor, automation.SyncTrigger{
				Action: action, ObjectAPIName: objectAPIName, RecordID: recordID,
				Data: triggerData, ActorID: actor.ID,
			}, automation.SyncAutomation{
				ID: r.id, APIName: r.apiName, Runtime: r.runtime, Execution: exec,
				Actions: actions, Source: r.source, EntryFile: r.entryFile,
			}); err != nil {
				return err
			}
			continue
		}
		// Async: enqueue on the same transaction so rollback drops the job.
		jobPayload, err := json.Marshal(map[string]any{
			"automationId":  r.id,
			"apiName":       r.apiName,
			"objectApiName": objectAPIName,
			"recordId":      recordID,
			"action":        action,
			"actions":       json.RawMessage(r.actions),
			"runtime":       r.runtime,
			"execution":     exec,
			"actorId":       actor.ID,
			"entryFile":     r.entryFile,
		})
		if err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `
INSERT INTO jobs (job_type, payload) VALUES ('automation.run', $1::jsonb)`, jobPayload); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) runSyncAutomation(ctx context.Context, actor *authz.Actor, trigger automation.SyncTrigger, auto automation.SyncAutomation) error {
	depth := syncDepth(ctx)
	if depth >= maxSyncDepth {
		return fmt.Errorf("automation %s: exceeded max sync depth (%d)", auto.APIName, maxSyncDepth)
	}
	ops := syncOps(ctx)
	if ops == nil {
		ops = &syncOpsCounter{}
		ctx = withSyncOps(ctx, ops)
	}
	ctx = withSyncDepth(ctx, depth+1)

	deadline := syncDeadline
	if dl, ok := ctx.Deadline(); ok {
		if rem := time.Until(dl); rem > 0 && rem < deadline {
			deadline = rem
		}
	}
	runCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	mut := &txMutator{svc: s, actor: actor}
	host := automation.HostBridge(automation.SyncMutatorBridge{Inner: mut})
	host = automation.AuthzHost{
		Inner:  host,
		Object: s.ObjectAz,
		Actor:  actor,
	}
	runCtx = automation.WithSyncGuest(runCtx)
	if s.Actions != nil {
		inv := s.Actions
		act := actor
		host = automation.BindActions(host, func(ctx context.Context, apiName string, input map[string]any) (map[string]any, error) {
			return inv.Invoke(ctx, act, apiName, input)
		})
	}
	auto.Host = host
	return automation.ExecuteSync(runCtx, mut, trigger, auto)
}
