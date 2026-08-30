package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	// SyncDeadline caps in-request sync automation work (ADR-014 Phase 3).
	SyncDeadline = 5 * time.Second
	// MaxSyncDepth limits nested sync automations fired by in-tx side effects.
	MaxSyncDepth = 3
	// MaxSyncOps caps Client-like mutations per sync chain.
	MaxSyncOps = 50
)

// SyncMutator is the in-transaction record API used by sync automations.
// Implementations must apply writes on the open Postgres transaction (no auto-commit HTTP).
type SyncMutator interface {
	CreateRecord(ctx context.Context, objectAPIName string, data map[string]any) (recordID string, err error)
	UpdateRecord(ctx context.Context, objectAPIName, recordID string, data map[string]any) error
	GetRecord(ctx context.Context, objectAPIName, recordID string) (map[string]any, error)
}

// SyncTrigger is the record write that fired the automation.
type SyncTrigger struct {
	Action        string // create|update|delete
	ObjectAPIName string
	RecordID      string
	Data          map[string]any // create data or update merged/patch context
	ActorID       string
}

// SyncAutomation is one automation definition loaded for sync execution.
type SyncAutomation struct {
	ID        string
	APIName   string
	Runtime   string
	Execution string
	Actions   []any
	Source    string
	EntryFile string
	// Optional overrides for code runtime (tests / AuthZ wrapping).
	Host     HostBridge
	DenoPath string
	Logger   func(message string)
}

// ExecuteSync runs a sync automation (actions JSON or Deno guest TypeScript).
func ExecuteSync(ctx context.Context, m SyncMutator, trigger SyncTrigger, auto SyncAutomation) error {
	if m == nil {
		return fmt.Errorf("automation %s: nil sync mutator", auto.APIName)
	}
	rt := NormalizeRuntime(auto.Runtime)
	switch rt {
	case RuntimeCode:
		host := HostBridge(SyncMutatorBridge{Inner: m})
		if auto.Host != nil {
			host = auto.Host
		}
		_, err := RunGuest(ctx, host, GuestRequest{
			APIName:   auto.APIName,
			Source:    auto.Source,
			EntryFile: auto.EntryFile,
			Trigger:   trigger,
			SyncMode:  true,
			DenoPath:  auto.DenoPath,
			Logger:    auto.Logger,
		})
		return err
	default:
		return ExecuteSyncActions(ctx, m, trigger, auto.APIName, auto.Actions)
	}
}

// ExecuteSyncActions runs declarative actions inside the open transaction.
func ExecuteSyncActions(ctx context.Context, m SyncMutator, trigger SyncTrigger, apiName string, actions []any) error {
	if t, ok := HasOutboundAction(actions); ok {
		return fmt.Errorf("automation %s: execution=sync forbids outbound action type %q", apiName, t)
	}
	for i, raw := range actions {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("automation %s: aborted: %w", apiName, err)
		}
		if err := execOneAction(ctx, m, trigger, apiName, i, raw); err != nil {
			return err
		}
	}
	return nil
}

func execOneAction(ctx context.Context, m SyncMutator, trigger SyncTrigger, apiName string, index int, raw any) error {
	switch v := raw.(type) {
	case string:
		t := strings.ToLower(strings.TrimSpace(v))
		if t == "" || t == "noop" || strings.HasPrefix(t, "log") {
			return nil
		}
		if t == "fail" || t == "error" {
			return fmt.Errorf("automation %s action[%d]: forced failure", apiName, index)
		}
		return fmt.Errorf("automation %s action[%d]: unsupported string action %q", apiName, index, v)
	case map[string]any:
		t := ActionType(v)
		switch t {
		case "", "noop", "log":
			return nil
		case "fail", "error":
			msg, _ := v["message"].(string)
			if msg == "" {
				msg = "forced failure"
			}
			return fmt.Errorf("automation %s action[%d]: %s", apiName, index, msg)
		case "createrecord", "create":
			return syncCreateRecord(ctx, m, trigger, apiName, index, v)
		case "updaterecord", "update":
			return syncUpdateRecord(ctx, m, trigger, apiName, index, v)
		default:
			if _, outbound := outboundActionTypes[t]; outbound {
				return fmt.Errorf("automation %s action[%d]: outbound type %q forbidden in sync", apiName, index, t)
			}
			return fmt.Errorf("automation %s action[%d]: unsupported action type %q", apiName, index, t)
		}
	default:
		// Ignore opaque non-object actions (legacy log stubs).
		return nil
	}
}

func syncCreateRecord(ctx context.Context, m SyncMutator, trigger SyncTrigger, apiName string, index int, v map[string]any) error {
	objectAPIName, _ := v["objectApiName"].(string)
	if objectAPIName == "" {
		objectAPIName, _ = v["object"].(string)
	}
	if objectAPIName == "" {
		return fmt.Errorf("automation %s action[%d]: createRecord requires objectApiName", apiName, index)
	}
	data := map[string]any{}
	if raw, ok := v["data"].(map[string]any); ok {
		for k, val := range raw {
			data[k] = resolveTemplate(val, trigger)
		}
	}
	if fmap, ok := v["fieldMap"].(map[string]any); ok {
		for target, srcField := range fmap {
			srcName, _ := srcField.(string)
			if srcName == "" {
				continue
			}
			data[target] = triggerField(trigger, srcName)
		}
	}
	_, err := m.CreateRecord(ctx, objectAPIName, data)
	if err != nil {
		return fmt.Errorf("automation %s action[%d] createRecord %s: %w", apiName, index, objectAPIName, err)
	}
	return nil
}

func syncUpdateRecord(ctx context.Context, m SyncMutator, trigger SyncTrigger, apiName string, index int, v map[string]any) error {
	objectAPIName, _ := v["objectApiName"].(string)
	if objectAPIName == "" {
		objectAPIName, _ = v["object"].(string)
	}
	if objectAPIName == "" {
		objectAPIName = trigger.ObjectAPIName
	}
	recordID, _ := v["recordId"].(string)
	if recordID == "" {
		if s, ok := resolveTemplate(v["recordId"], trigger).(string); ok {
			recordID = s
		}
	}
	if recordID == "" {
		recordID = trigger.RecordID
	}
	if recordID == "" {
		return fmt.Errorf("automation %s action[%d]: updateRecord requires recordId", apiName, index)
	}
	data := map[string]any{}
	if raw, ok := v["data"].(map[string]any); ok {
		for k, val := range raw {
			data[k] = resolveTemplate(val, trigger)
		}
	}
	if err := m.UpdateRecord(ctx, objectAPIName, recordID, data); err != nil {
		return fmt.Errorf("automation %s action[%d] updateRecord %s/%s: %w", apiName, index, objectAPIName, recordID, err)
	}
	return nil
}

func triggerField(trigger SyncTrigger, name string) any {
	switch name {
	case "Id", "id", "recordId":
		return trigger.RecordID
	case "ObjectApiName", "objectApiName":
		return trigger.ObjectAPIName
	}
	if trigger.Data == nil {
		return nil
	}
	return trigger.Data[name]
}

func resolveTemplate(val any, trigger SyncTrigger) any {
	s, ok := val.(string)
	if !ok {
		return val
	}
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") {
		inner := strings.TrimSpace(trimmed[2 : len(trimmed)-2])
		inner = strings.TrimPrefix(inner, "trigger.")
		inner = strings.TrimPrefix(inner, "Trigger.")
		return triggerField(trigger, inner)
	}
	// Lightweight replace of {{trigger.X}} substrings.
	out := s
	for {
		start := strings.Index(out, "{{")
		if start < 0 {
			break
		}
		end := strings.Index(out[start:], "}}")
		if end < 0 {
			break
		}
		end += start
		inner := strings.TrimSpace(out[start+2 : end])
		inner = strings.TrimPrefix(inner, "trigger.")
		inner = strings.TrimPrefix(inner, "Trigger.")
		repl := fmt.Sprint(triggerField(trigger, inner))
		out = out[:start] + repl + out[end+2:]
	}
	return out
}

// ActionsFromJSON unmarshals actions jsonb into []any.
func ActionsFromJSON(raw []byte) ([]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []any{}, nil
	}
	var out []any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []any{}
	}
	return out, nil
}
