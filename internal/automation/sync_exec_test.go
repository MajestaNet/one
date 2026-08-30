package automation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MajestaNet/ide/internal/automation"
)

type memMutator struct {
	created []struct {
		Object string
		Data   map[string]any
	}
	failCreate bool
}

func (m *memMutator) CreateRecord(_ context.Context, objectAPIName string, data map[string]any) (string, error) {
	if m.failCreate {
		return "", errors.New("create denied")
	}
	m.created = append(m.created, struct {
		Object string
		Data   map[string]any
	}{objectAPIName, data})
	return "new-id", nil
}

func (m *memMutator) UpdateRecord(context.Context, string, string, map[string]any) error { return nil }

func (m *memMutator) GetRecord(context.Context, string, string) (map[string]any, error) {
	return map[string]any{}, nil
}

func TestExecuteSyncActions_CreateRecordMapping(t *testing.T) {
	m := &memMutator{}
	trigger := automation.SyncTrigger{
		Action: "create", ObjectAPIName: "Account", RecordID: "acc-1",
		Data: map[string]any{"Name": "Acme", "Amount": 100},
	}
	actions := []any{
		map[string]any{
			"type":          "createRecord",
			"objectApiName": "Opportunity",
			"fieldMap":      map[string]any{"Name": "Name", "Amount": "Amount"},
			"data":          map[string]any{"Stage": "Prospecting", "AccountId": "{{trigger.Id}}"},
		},
	}
	if err := automation.ExecuteSyncActions(context.Background(), m, trigger, "CreateOpp", actions); err != nil {
		t.Fatal(err)
	}
	if len(m.created) != 1 || m.created[0].Object != "Opportunity" {
		t.Fatalf("created=%+v", m.created)
	}
	d := m.created[0].Data
	if d["Name"] != "Acme" || d["Amount"] != 100 || d["Stage"] != "Prospecting" || d["AccountId"] != "acc-1" {
		t.Fatalf("data=%v", d)
	}
}

func TestExecuteSyncActions_FailRollsError(t *testing.T) {
	m := &memMutator{}
	err := automation.ExecuteSyncActions(context.Background(), m, automation.SyncTrigger{}, "X", []any{"fail"})
	if err == nil {
		t.Fatal("expected fail")
	}
}

func TestExecuteSync_CodeRejectedWithoutSource(t *testing.T) {
	err := automation.ExecuteSync(context.Background(), &memMutator{}, automation.SyncTrigger{}, automation.SyncAutomation{
		APIName: "Codey", Runtime: "code", Actions: []any{},
	})
	if err == nil || err.Error() == "" {
		t.Fatal("expected empty source rejection")
	}
}

func TestExecuteSyncActions_OutboundRejected(t *testing.T) {
	err := automation.ExecuteSyncActions(context.Background(), &memMutator{}, automation.SyncTrigger{}, "X", []any{
		map[string]any{"type": "http", "url": "https://x"},
	})
	if err == nil {
		t.Fatal("expected outbound reject")
	}
}
