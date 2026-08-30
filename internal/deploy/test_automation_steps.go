package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/automation"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
)

func executeAutomationUnitPass(ctx context.Context, step *TestStep, deps testRunDeps) (map[string]any, error) {
	if deps.pool == nil {
		return nil, newValidationError("pool required for automationUnitPass steps")
	}
	testFile := step.TestFile
	if testFile == "" {
		return nil, newValidationError("automationUnitPass requires testFile")
	}
	testSrc, err := LoadCustomerSource(ctx, deps.pool, testFile)
	if err != nil {
		return nil, err
	}
	autoName := step.AutomationAPIName
	autoSrc := ""
	if autoName != "" {
		autoSrc, err = loadAutomationSource(ctx, deps.pool, autoName)
		if err != nil {
			return nil, err
		}
	}
	res, err := automation.RunUnitTest(ctx, automation.UnitTestRequest{
		TestAPIName:       autoName + "_unit",
		TestFile:          testFile,
		TestSource:        testSrc,
		AutomationAPIName: autoName,
		AutomationSource:  autoSrc,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": res.OK, "testFile": testFile, "automationApiName": autoName}, nil
}

func executeAutomationContract(ctx context.Context, step *TestStep, deps testRunDeps) (map[string]any, error) {
	if deps.data == nil {
		return nil, newValidationError("DataEngine required for automationContract steps")
	}
	if deps.pool == nil {
		return nil, newValidationError("pool required for automationContract steps")
	}
	autoName := step.AutomationAPIName
	if autoName == "" {
		return nil, newValidationError("automationContract requires automationApiName")
	}
	def, err := loadAutomationDef(ctx, deps.pool, autoName)
	if err != nil {
		return nil, err
	}
	triggerObject := step.ObjectAPIName
	if triggerObject == "" {
		triggerObject = def.objectAPIName
	}
	if triggerObject == "" {
		return nil, newValidationError("automationContract requires objectApiName")
	}
	payload := map[string]any{}
	for k, v := range step.Data {
		payload[k] = v
	}
	// Avoid double-fire: create fixture with the automation inactive, then invoke explicitly.
	_, _ = deps.pool.Exec(ctx, `UPDATE metadata_automations SET active=false WHERE api_name=$1`, autoName)
	defer func() {
		_, _ = deps.pool.Exec(ctx, `UPDATE metadata_automations SET active=true WHERE api_name=$1`, autoName)
	}()
	rec, err := deps.data.Create(ctx, triggerObject, payload, deps.actor)
	if err != nil {
		return nil, fmt.Errorf("contract fixture create: %w", err)
	}
	triggerID, _ := rec["Id"].(string)

	triggerData := map[string]any{}
	for k, v := range rec {
		triggerData[k] = v
	}
	trigger := automation.SyncTrigger{
		Action:        "create",
		ObjectAPIName: triggerObject,
		RecordID:      triggerID,
		Data:          triggerData,
	}
	if deps.actor != nil {
		trigger.ActorID = deps.actor.ID
	}

	host := automation.HostBridge(contractDataHost{svc: deps.data, actor: deps.actor})
	if deps.data.ObjectAz != nil {
		host = automation.AuthzHost{Inner: host, Object: deps.data.ObjectAz, Actor: deps.actor}
	}
	if deps.data.Actions != nil {
		inv := deps.data.Actions
		act := deps.actor
		host = automation.BindActions(host, func(ctx context.Context, apiName string, input map[string]any) (map[string]any, error) {
			return inv.Invoke(ctx, act, apiName, input)
		})
	}

	rt := automation.NormalizeRuntime(def.runtime)
	switch rt {
	case automation.RuntimeCode:
		if def.source == "" {
			return nil, fmt.Errorf("automation %s: missing source", autoName)
		}
		_, err = automation.RunGuest(ctx, host, automation.GuestRequest{
			APIName:   autoName,
			Source:    def.source,
			EntryFile: def.entryFile,
			Trigger:   trigger,
			SyncMode:  false,
		})
	default:
		actions, aerr := automation.ActionsFromJSON(def.actions)
		if aerr != nil {
			return nil, aerr
		}
		err = automation.ExecuteSyncActions(ctx, hostAsSyncMutator{host: host}, trigger, autoName, actions)
	}
	if err != nil {
		return nil, err
	}

	expectObject := step.ExpectObjectAPIName
	if expectObject == "" {
		expectObject = step.Object
	}
	if expectObject == "" {
		return map[string]any{"triggerId": triggerID, "automationApiName": autoName}, nil
	}
	minRows := step.ExpectMinRows
	if minRows < 1 {
		minRows = 1
	}
	filters := make([]queryFilter, 0, len(step.Filters))
	for _, f := range step.Filters {
		op := f.Op
		if op == "" {
			op = "eq"
		}
		val := f.Value
		if s, ok := val.(string); ok && strings.HasPrefix(s, "$trigger.") {
			field := strings.TrimPrefix(s, "$trigger.")
			if field == "Id" || field == "id" {
				val = triggerID
			} else {
				val = triggerData[field]
			}
		}
		filters = append(filters, queryFilter{Field: f.Field, Op: op, Value: val})
	}
	qreqMap := map[string]any{
		"object":  expectObject,
		"filters": filters,
		"limit":   minRows,
	}
	qreqJSON, err := json.Marshal(qreqMap)
	if err != nil {
		return nil, err
	}
	result, err := deps.data.Query(ctx, json.RawMessage(qreqJSON), dataengine.QueryVisibility{})
	if err != nil {
		return nil, err
	}
	if len(result.Records) < minRows {
		return nil, newValidationErrorf(
			"Expected at least %d %s rows, got %d", minRows, expectObject, len(result.Records))
	}
	return map[string]any{
		"triggerId":         triggerID,
		"automationApiName": autoName,
		"expectObject":      expectObject,
		"rowCount":          len(result.Records),
	}, nil
}

type autoDef struct {
	apiName, objectAPIName, runtime, execution, entryFile, source string
	actions                                                       []byte
}

func loadAutomationDef(ctx context.Context, pool *db.Pool, apiName string) (*autoDef, error) {
	var d autoDef
	err := pool.QueryRow(ctx, `
SELECT api_name, object_api_name, COALESCE(runtime,'actions'), COALESCE(execution,'async'),
       COALESCE(entry_file,''), COALESCE(source,''), COALESCE(actions,'[]'::jsonb)
FROM metadata_automations WHERE api_name=$1`, apiName).Scan(
		&d.apiName, &d.objectAPIName, &d.runtime, &d.execution, &d.entryFile, &d.source, &d.actions)
	if err != nil {
		return nil, fmt.Errorf("automation %s not found", apiName)
	}
	return &d, nil
}

func loadAutomationSource(ctx context.Context, pool *db.Pool, apiName string) (string, error) {
	d, err := loadAutomationDef(ctx, pool, apiName)
	if err != nil {
		return "", err
	}
	if d.source != "" {
		return d.source, nil
	}
	if d.entryFile != "" {
		return LoadCustomerSource(ctx, pool, d.entryFile)
	}
	return "", fmt.Errorf("automation %s has no source", apiName)
}

type contractDataHost struct {
	svc   *dataengine.Service
	actor *authz.Actor
}

func (h contractDataHost) CreateRecord(ctx context.Context, objectAPIName string, data map[string]any) (string, error) {
	rec, err := h.svc.Create(ctx, objectAPIName, data, h.actor)
	if err != nil {
		return "", err
	}
	id, _ := rec["Id"].(string)
	return id, nil
}

func (h contractDataHost) UpdateRecord(ctx context.Context, objectAPIName, recordID string, data map[string]any) error {
	_, err := h.svc.Update(ctx, objectAPIName, recordID, data, h.actor)
	return err
}

func (h contractDataHost) GetRecord(ctx context.Context, objectAPIName, recordID string) (map[string]any, error) {
	return h.svc.Get(ctx, objectAPIName, recordID)
}

func (h contractDataHost) DeleteRecord(ctx context.Context, objectAPIName, recordID string) error {
	return h.svc.Delete(ctx, objectAPIName, recordID, h.actor)
}

func (h contractDataHost) Query(ctx context.Context, req map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	// Normalize objectApiName → object for the query planner.
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	if _, ok := m["object"]; !ok {
		if o, ok := m["objectApiName"].(string); ok {
			m["object"] = o
		}
	}
	raw, _ = json.Marshal(m)
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

func (h contractDataHost) InvokeAction(context.Context, string, map[string]any) (map[string]any, error) {
	return nil, fmt.Errorf("invokeAction is not available on this host")
}

type hostAsSyncMutator struct {
	host automation.HostBridge
}

func (m hostAsSyncMutator) CreateRecord(ctx context.Context, objectAPIName string, data map[string]any) (string, error) {
	return m.host.CreateRecord(ctx, objectAPIName, data)
}
func (m hostAsSyncMutator) UpdateRecord(ctx context.Context, objectAPIName, recordID string, data map[string]any) error {
	return m.host.UpdateRecord(ctx, objectAPIName, recordID, data)
}
func (m hostAsSyncMutator) GetRecord(ctx context.Context, objectAPIName, recordID string) (map[string]any, error) {
	return m.host.GetRecord(ctx, objectAPIName, recordID)
}
