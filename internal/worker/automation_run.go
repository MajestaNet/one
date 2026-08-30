package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/automation"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
)

// processAutomationRun executes an automation.run job (actions or Deno code).
func processAutomationRun(ctx context.Context, pool *db.Pool, payload map[string]any, opts *ProcessOptions) error {
	apiName, _ := payload["apiName"].(string)
	automationID, _ := payload["automationId"].(string)
	actorID, _ := payload["actorId"].(string)
	objectAPIName, _ := payload["objectApiName"].(string)
	recordID, _ := payload["recordId"].(string)
	action, _ := payload["action"].(string)
	runtime, _ := payload["runtime"].(string)
	entryFile, _ := payload["entryFile"].(string)

	source := ""
	actionsRaw, _ := json.Marshal(payload["actions"])
	runAsPrincipalID := ""

	if automationID != "" {
		var actionsJSON []byte
		err := pool.QueryRow(ctx, `
SELECT api_name, COALESCE(runtime, 'actions'), COALESCE(actions, '[]'::jsonb),
       COALESCE(source, ''), COALESCE(entry_file, ''), COALESCE(run_as_principal_id::text, '')
FROM metadata_automations WHERE id=$1::uuid`, automationID).Scan(
			&apiName, &runtime, &actionsJSON, &source, &entryFile, &runAsPrincipalID)
		if err != nil {
			return fmt.Errorf("load automation %s: %w", automationID, err)
		}
		actionsRaw = actionsJSON
	}

	// ADR-014: schedule uses definition run-as; record/manual/API invoke use the starter principal.
	if action == "schedule" && runAsPrincipalID != "" {
		actorID = runAsPrincipalID
	}
	if actorID == "" {
		// Legacy stub jobs (tests) without actor — treat actions-only no-ops as success.
		if automation.NormalizeRuntime(runtime) != automation.RuntimeCode {
			actions, err := automation.ActionsFromJSON(actionsRaw)
			if err != nil {
				return err
			}
			return automation.ExecuteSyncActions(ctx, noopMutator{}, automation.SyncTrigger{
				Action: action, ObjectAPIName: objectAPIName, RecordID: recordID,
			}, apiName, actions)
		}
		return fmt.Errorf("automation.run missing actorId")
	}

	actor, err := resolveAutomationActor(ctx, pool, actorID)
	if err != nil {
		return err
	}

	if apiName != "" && opts != nil && opts.AutomationAz != nil {
		if err := opts.AutomationAz.AssertCanRunAutomation(ctx, actor, apiName); err != nil {
			return err
		}
	}

	triggerData := map[string]any{}
	if input, ok := payload["input"].(map[string]any); ok {
		for k, v := range input {
			triggerData[k] = v
		}
	}
	if recordID != "" && objectAPIName != "" && opts != nil && opts.DataEngine != nil {
		if rec, gerr := opts.DataEngine.Get(ctx, objectAPIName, recordID); gerr == nil {
			for k, v := range rec {
				triggerData[k] = v
			}
		}
	}

	trigger := automation.SyncTrigger{
		Action:        action,
		ObjectAPIName: objectAPIName,
		RecordID:      recordID,
		Data:          triggerData,
		ActorID:       actor.ID,
	}

	actions, err := automation.ActionsFromJSON(actionsRaw)
	if err != nil {
		return fmt.Errorf("automation %s actions: %w", apiName, err)
	}

	if opts == nil || opts.DataEngine == nil {
		return fmt.Errorf("DataEngine not configured for automation.run")
	}
	host := automation.HostBridge(serviceHost{svc: opts.DataEngine, actor: actor})
	if opts.ObjectAz != nil {
		host = automation.AuthzHost{Inner: host, Object: opts.ObjectAz, Actor: actor}
	}
	encKey := ""
	if opts != nil {
		encKey = opts.WebhookEncryptionKey
	}
	host = automation.OutboundHost{Inner: host, Pool: pool, EncryptionKey: encKey}
	if opts.DataEngine != nil && opts.DataEngine.Actions != nil {
		inv := opts.DataEngine.Actions
		act := actor
		host = automation.BindActions(host, func(ctx context.Context, apiName string, input map[string]any) (map[string]any, error) {
			return inv.Invoke(ctx, act, apiName, input)
		})
	}

	rt := automation.NormalizeRuntime(runtime)
	if rt == automation.RuntimeCode {
		if source == "" {
			return fmt.Errorf("automation %s: code runtime missing source", apiName)
		}
		_, err := automation.RunGuest(ctx, host, automation.GuestRequest{
			APIName:   apiName,
			Source:    source,
			EntryFile: entryFile,
			Trigger:   trigger,
			SyncMode:  false,
			Logger: func(message string) {
				log.Printf("[worker] automation %s log: %s", apiName, message)
			},
		})
		if err != nil {
			return err
		}
	} else {
		mut := hostAsMutator{host: host}
		if err := automation.ExecuteSyncActions(ctx, mut, trigger, apiName, actions); err != nil {
			return err
		}
	}

	details, _ := json.Marshal(map[string]any{
		"automationId":  automationID,
		"apiName":       apiName,
		"objectApiName": objectAPIName,
		"recordId":      recordID,
		"runtime":       rt,
	})
	if recordID != "" {
		_, _ = pool.Exec(ctx, `
INSERT INTO audit_log (actor_id, action, object_api_name, record_id, details)
VALUES ($1::uuid, 'automation.run', $2, $3::uuid, $4::jsonb)`,
			actor.ID, objectAPIName, recordID, string(details))
	} else {
		_, _ = pool.Exec(ctx, `
INSERT INTO audit_log (actor_id, action, object_api_name, details)
VALUES ($1::uuid, 'automation.run', $2, $3::jsonb)`,
			actor.ID, objectAPIName, string(details))
	}
	return nil
}

func resolveAutomationActor(ctx context.Context, pool *db.Pool, userID string) (*authz.Actor, error) {
	store := db.NewUserStore(pool)
	u, err := store.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("resolve run-as actor: %w", err)
	}
	psIDs, err := store.ListPermissionSetIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	scopes, roleAdmin, _, err := store.ListRoleGrants(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &authz.Actor{
		ID:               u.ID,
		Email:            u.Email,
		DisplayName:      u.DisplayName,
		PrincipalType:    u.PrincipalType,
		IsAdmin:          u.IsAdmin || roleAdmin,
		PermissionSetIDs: psIDs,
		Scopes:           scopes,
		AuthMethod:       "automation",
	}, nil
}

type serviceHost struct {
	svc   *dataengine.Service
	actor *authz.Actor
}

func (h serviceHost) CreateRecord(ctx context.Context, objectAPIName string, data map[string]any) (string, error) {
	if h.svc == nil {
		return "", fmt.Errorf("DataEngine not configured for automation.run")
	}
	rec, err := h.svc.Create(ctx, objectAPIName, data, h.actor)
	if err != nil {
		return "", err
	}
	id, _ := rec["Id"].(string)
	return id, nil
}

func (h serviceHost) UpdateRecord(ctx context.Context, objectAPIName, recordID string, data map[string]any) error {
	if h.svc == nil {
		return fmt.Errorf("DataEngine not configured for automation.run")
	}
	_, err := h.svc.Update(ctx, objectAPIName, recordID, data, h.actor)
	return err
}

func (h serviceHost) GetRecord(ctx context.Context, objectAPIName, recordID string) (map[string]any, error) {
	if h.svc == nil {
		return nil, fmt.Errorf("DataEngine not configured for automation.run")
	}
	return h.svc.Get(ctx, objectAPIName, recordID)
}

func (h serviceHost) DeleteRecord(ctx context.Context, objectAPIName, recordID string) error {
	if h.svc == nil {
		return fmt.Errorf("DataEngine not configured for automation.run")
	}
	return h.svc.Delete(ctx, objectAPIName, recordID, h.actor)
}

func (h serviceHost) Query(ctx context.Context, req map[string]any) (map[string]any, error) {
	if h.svc == nil {
		return nil, fmt.Errorf("DataEngine not configured for automation.run")
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
	for _, r := range res.Records {
		records = append(records, r)
	}
	return map[string]any{"records": records, "totalSize": res.TotalSize}, nil
}

func (h serviceHost) InvokeAction(context.Context, string, map[string]any) (map[string]any, error) {
	return nil, fmt.Errorf("invokeAction is not available on this host")
}

type hostAsMutator struct {
	host automation.HostBridge
}

func (m hostAsMutator) CreateRecord(ctx context.Context, objectAPIName string, data map[string]any) (string, error) {
	return m.host.CreateRecord(ctx, objectAPIName, data)
}

func (m hostAsMutator) UpdateRecord(ctx context.Context, objectAPIName, recordID string, data map[string]any) error {
	return m.host.UpdateRecord(ctx, objectAPIName, recordID, data)
}

func (m hostAsMutator) GetRecord(ctx context.Context, objectAPIName, recordID string) (map[string]any, error) {
	return m.host.GetRecord(ctx, objectAPIName, recordID)
}

type noopMutator struct{}

func (noopMutator) CreateRecord(context.Context, string, map[string]any) (string, error) {
	return "", fmt.Errorf("createRecord requires DataEngine")
}
func (noopMutator) UpdateRecord(context.Context, string, string, map[string]any) error {
	return fmt.Errorf("updateRecord requires DataEngine")
}
func (noopMutator) GetRecord(context.Context, string, string) (map[string]any, error) {
	return nil, fmt.Errorf("getRecord requires DataEngine")
}
