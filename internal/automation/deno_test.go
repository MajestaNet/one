package automation_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/automation"
)

type memHost struct {
	memMutator
	createdIDs []string
}

func (m *memHost) CreateRecord(ctx context.Context, objectAPIName string, data map[string]any) (string, error) {
	id, err := m.memMutator.CreateRecord(ctx, objectAPIName, data)
	if err != nil {
		return "", err
	}
	m.createdIDs = append(m.createdIDs, id)
	return id, nil
}

func (m *memHost) DeleteRecord(context.Context, string, string) error {
	return errors.New("no delete")
}

func (m *memHost) Query(context.Context, map[string]any) (map[string]any, error) {
	return nil, errors.New("no query")
}

func (m *memHost) InvokeAction(context.Context, string, map[string]any) (map[string]any, error) {
	return nil, errors.New("no invokeAction")
}

func TestRunGuest_CreateRecord(t *testing.T) {
	if _, err := automation.FindDeno(""); err != nil {
		t.Skip(err.Error())
	}
	host := &memHost{}
	src := `
export default async function run(ctx) {
  const { id } = await ctx.createRecord({
    objectApiName: "Opportunity",
    data: { Name: ctx.trigger.data.Name, Stage: "Prospecting" },
  });
  ctx.log("created", id);
  return { ok: true, id };
}
`
	res, err := automation.RunGuest(context.Background(), host, automation.GuestRequest{
		APIName: "CreateOpp",
		Source:  src,
		Trigger: automation.SyncTrigger{
			Action: "create", ObjectAPIName: "Account", RecordID: "acc-1",
			Data: map[string]any{"Name": "Acme"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || len(host.created) != 1 || host.created[0].Object != "Opportunity" {
		t.Fatalf("res=%+v created=%+v", res, host.created)
	}
	if host.created[0].Data["Name"] != "Acme" {
		t.Fatalf("data=%v", host.created[0].Data)
	}
}

func TestRunGuest_RejectsForbiddenImport(t *testing.T) {
	_, err := automation.RunGuest(context.Background(), &memHost{}, automation.GuestRequest{
		APIName: "Bad",
		Source:  `import _ from "npm:lodash"; export default async function run() { return { ok: true }; }`,
		Trigger: automation.SyncTrigger{},
	})
	if err == nil || !strings.Contains(err.Error(), "forbidden import") {
		t.Fatalf("expected forbidden import, got %v", err)
	}
}

func TestRunGuest_HostCreateDenied(t *testing.T) {
	if _, err := automation.FindDeno(""); err != nil {
		t.Skip(err.Error())
	}
	host := &memHost{}
	host.failCreate = true
	_, err := automation.RunGuest(context.Background(), host, automation.GuestRequest{
		APIName: "Denied",
		Source: `
export default async function run(ctx) {
  await ctx.createRecord({ objectApiName: "Opportunity", data: { Name: "X" } });
  return { ok: true };
}
`,
		Trigger: automation.SyncTrigger{Data: map[string]any{}},
	})
	if err == nil {
		t.Fatal("expected create denial to fail guest")
	}
}

func TestExecuteSync_CodeRuns(t *testing.T) {
	if _, err := automation.FindDeno(""); err != nil {
		t.Skip(err.Error())
	}
	m := &memMutator{}
	err := automation.ExecuteSync(context.Background(), m, automation.SyncTrigger{
		Action: "create", ObjectAPIName: "Account", RecordID: "a1",
		Data: map[string]any{"Name": "N"},
	}, automation.SyncAutomation{
		APIName: "Codey", Runtime: "code",
		Source: `
export default async function run(ctx) {
  await ctx.createRecord({ objectApiName: "Child", data: { Name: ctx.trigger.data.Name } });
  return { ok: true };
}
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.created) != 1 || m.created[0].Object != "Child" {
		t.Fatalf("created=%+v", m.created)
	}
}

func TestDenoVersionPinned(t *testing.T) {
	if automation.DenoVersion == "" {
		t.Fatal("DenoVersion empty")
	}
}
