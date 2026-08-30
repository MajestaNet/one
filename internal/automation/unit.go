package automation

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed embed/unit_bootstrap.ts
var unitBootstrapTS string

// UnitTestRequest runs one tests/automations/** guest against a mock host.
type UnitTestRequest struct {
	TestAPIName       string
	TestSource        string
	TestFile          string
	AutomationAPIName string
	AutomationSource  string
	Trigger           SyncTrigger
	DenoPath          string
	Logger            func(message string)
}

// MockHost is an in-memory HostBridge for unit tests (no Postgres).
type MockHost struct {
	mu      sync.Mutex
	Creates []map[string]any
	Updates []map[string]any
	Deletes []map[string]any
	Gets    []map[string]any
	Queries []map[string]any
	Invokes []map[string]any
	Records map[string]map[string]any // "Object/id" -> record map

	InvokeResults map[string]map[string]any // apiName -> result
	InvokeErr     error

	UnderTestAPIName string
	UnderTestSource  string
	DenoPath         string
	Logger           func(message string)
}

// NewMockHost constructs an empty mock host.
func NewMockHost() *MockHost {
	return &MockHost{Records: map[string]map[string]any{}}
}

func (m *MockHost) key(objectAPIName, id string) string {
	return objectAPIName + "/" + id
}

func (m *MockHost) CreateRecord(_ context.Context, objectAPIName string, data map[string]any) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := mockRecordID()
	rec := map[string]any{"Id": id}
	for k, v := range data {
		rec[k] = v
	}
	m.Records[m.key(objectAPIName, id)] = rec
	m.Creates = append(m.Creates, map[string]any{
		"objectApiName": objectAPIName,
		"data":          data,
		"id":            id,
	})
	return id, nil
}

func (m *MockHost) UpdateRecord(_ context.Context, objectAPIName, recordID string, data map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := m.key(objectAPIName, recordID)
	rec := m.Records[key]
	if rec == nil {
		rec = map[string]any{"Id": recordID}
	}
	for k, v := range data {
		rec[k] = v
	}
	m.Records[key] = rec
	m.Updates = append(m.Updates, map[string]any{
		"objectApiName": objectAPIName,
		"recordId":      recordID,
		"data":          data,
	})
	return nil
}

func (m *MockHost) GetRecord(_ context.Context, objectAPIName, recordID string) (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Gets = append(m.Gets, map[string]any{
		"objectApiName": objectAPIName,
		"recordId":      recordID,
	})
	rec, ok := m.Records[m.key(objectAPIName, recordID)]
	if !ok {
		return nil, fmt.Errorf("record not found: %s/%s", objectAPIName, recordID)
	}
	out := map[string]any{}
	for k, v := range rec {
		out[k] = v
	}
	return out, nil
}

func (m *MockHost) DeleteRecord(_ context.Context, objectAPIName, recordID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Records, m.key(objectAPIName, recordID))
	m.Deletes = append(m.Deletes, map[string]any{
		"objectApiName": objectAPIName,
		"recordId":      recordID,
	})
	return nil
}

func (m *MockHost) Query(_ context.Context, req map[string]any) (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Queries = append(m.Queries, req)
	obj, _ := req["objectApiName"].(string)
	if obj == "" {
		obj, _ = req["object"].(string)
	}
	var records []map[string]any
	prefix := obj + "/"
	for k, rec := range m.Records {
		if strings.HasPrefix(k, prefix) {
			records = append(records, rec)
		}
	}
	return map[string]any{"records": records, "totalSize": len(records)}, nil
}

func (m *MockHost) InvokeAction(_ context.Context, apiName string, input map[string]any) (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if input == nil {
		input = map[string]any{}
	}
	m.Invokes = append(m.Invokes, map[string]any{
		"apiName": apiName,
		"input":   input,
	})
	if m.InvokeErr != nil {
		return nil, m.InvokeErr
	}
	if m.InvokeResults != nil {
		if out, ok := m.InvokeResults[apiName]; ok {
			return out, nil
		}
	}
	return map[string]any{"ok": true, "apiName": apiName}, nil
}

func (m *MockHost) clearCalls() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Creates = nil
	m.Updates = nil
	m.Deletes = nil
	m.Gets = nil
	m.Queries = nil
	m.Invokes = nil
}

func (m *MockHost) callsFor(method string) []map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch strings.TrimSpace(method) {
	case "", "createRecord", "create":
		return append([]map[string]any{}, m.Creates...)
	case "updateRecord", "update":
		return append([]map[string]any{}, m.Updates...)
	case "deleteRecord", "delete":
		return append([]map[string]any{}, m.Deletes...)
	case "getRecord", "get":
		return append([]map[string]any{}, m.Gets...)
	case "query":
		return append([]map[string]any{}, m.Queries...)
	case "invokeAction", "invoke":
		return append([]map[string]any{}, m.Invokes...)
	default:
		return nil
	}
}

// RunUnitTest executes a tests/automations guest with MockHost + unit helpers.
func RunUnitTest(ctx context.Context, req UnitTestRequest) (*GuestResult, error) {
	if strings.TrimSpace(req.TestSource) == "" {
		return nil, fmt.Errorf("unit test: empty test source")
	}
	label := req.TestFile
	if label == "" {
		label = "tests/automations/" + req.TestAPIName + ".ts"
	}
	if err := ValidateSourceImports(label, req.TestSource); err != nil {
		return nil, err
	}
	if req.AutomationSource != "" {
		entry := "src/automations/" + req.AutomationAPIName + ".ts"
		if err := ValidateSourceImports(entry, req.AutomationSource); err != nil {
			return nil, err
		}
	}

	host := NewMockHost()
	host.UnderTestAPIName = req.AutomationAPIName
	host.UnderTestSource = req.AutomationSource
	host.DenoPath = req.DenoPath
	host.Logger = req.Logger

	trigger := req.Trigger
	if trigger.Action == "" {
		trigger.Action = "create"
	}

	return RunGuest(ctx, unitRPCHost{mock: host}, GuestRequest{
		APIName:    req.TestAPIName,
		Source:     req.TestSource,
		EntryFile:  label,
		Trigger:    trigger,
		SyncMode:   true,
		DenoPath:   req.DenoPath,
		Logger:     req.Logger,
		Bootstrap:  unitBootstrapTS,
		ExtraFiles: unitExtraFiles(req.AutomationSource),
	})
}

func unitExtraFiles(underTestSource string) map[string]string {
	if underTestSource == "" {
		return nil
	}
	return map[string]string{"under_test.ts": underTestSource}
}

// unitRPCHost adapts MockHost and handles unit-only RPC methods.
type unitRPCHost struct {
	mock *MockHost
}

func (h unitRPCHost) CreateRecord(ctx context.Context, objectAPIName string, data map[string]any) (string, error) {
	return h.mock.CreateRecord(ctx, objectAPIName, data)
}
func (h unitRPCHost) UpdateRecord(ctx context.Context, objectAPIName, recordID string, data map[string]any) error {
	return h.mock.UpdateRecord(ctx, objectAPIName, recordID, data)
}
func (h unitRPCHost) GetRecord(ctx context.Context, objectAPIName, recordID string) (map[string]any, error) {
	return h.mock.GetRecord(ctx, objectAPIName, recordID)
}
func (h unitRPCHost) DeleteRecord(ctx context.Context, objectAPIName, recordID string) error {
	return h.mock.DeleteRecord(ctx, objectAPIName, recordID)
}
func (h unitRPCHost) Query(ctx context.Context, req map[string]any) (map[string]any, error) {
	return h.mock.Query(ctx, req)
}
func (h unitRPCHost) InvokeAction(ctx context.Context, apiName string, input map[string]any) (map[string]any, error) {
	return h.mock.InvokeAction(ctx, apiName, input)
}

func forwardUnitRPC(inner HostBridge, ctx context.Context, method string, argsJSON json.RawMessage) (any, bool, error) {
	u, ok := inner.(UnitRPCHandler)
	if !ok {
		return nil, false, nil
	}
	return u.HandleUnitRPC(ctx, method, argsJSON)
}

// HandleUnitRPC implements optional unit helpers (used by dispatchHostRPC).
func (h unitRPCHost) HandleUnitRPC(ctx context.Context, method string, argsJSON json.RawMessage) (any, bool, error) {
	switch method {
	case "runUnderTest":
		if strings.TrimSpace(h.mock.UnderTestSource) == "" {
			return nil, true, fmt.Errorf("runUnderTest: no automation source configured for this unit test")
		}
		var args struct {
			Trigger *SyncTrigger `json:"trigger"`
		}
		if len(argsJSON) > 0 && string(argsJSON) != "null" {
			_ = json.Unmarshal(argsJSON, &args)
		}
		trigger := SyncTrigger{Action: "create"}
		if args.Trigger != nil {
			trigger = *args.Trigger
		}
		_, err := RunGuest(ctx, h.mock, GuestRequest{
			APIName:   h.mock.UnderTestAPIName,
			Source:    h.mock.UnderTestSource,
			EntryFile: "src/automations/" + h.mock.UnderTestAPIName + ".ts",
			Trigger:   trigger,
			SyncMode:  true,
			DenoPath:  h.mock.DenoPath,
			Logger:    h.mock.Logger,
		})
		if err != nil {
			return nil, true, err
		}
		return map[string]any{"ok": true}, true, nil
	case "getCalls":
		var args struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(argsJSON, &args)
		return map[string]any{"calls": h.mock.callsFor(args.Method)}, true, nil
	case "clearCalls":
		h.mock.clearCalls()
		return map[string]any{"ok": true}, true, nil
	default:
		return nil, false, nil
	}
}

// UnitRPCHandler is implemented by hosts that expose unit-harness helpers.
type UnitRPCHandler interface {
	HandleUnitRPC(ctx context.Context, method string, argsJSON json.RawMessage) (result any, handled bool, err error)
}

func mockRecordID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	// RFC-ish UUID string without importing google/uuid.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
