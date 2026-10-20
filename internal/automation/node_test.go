package automation_test

import (
	"context"
	"errors"
	"os"
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

func requireNodeGuest(t *testing.T) {
	t.Helper()
	if _, err := automation.FindNode(""); err != nil {
		if errors.Is(err, automation.ErrNodeUnavailable) {
			t.Skip(err.Error())
		}
		t.Fatal(err)
	}
}

func TestRunGuest_CreateRecord(t *testing.T) {
	requireNodeGuest(t)
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
	if len(res.Logs) != 1 || !strings.Contains(res.Logs[0], "created") {
		t.Fatalf("logs=%v", res.Logs)
	}
}

func TestRunGuest_TypeOnlyImport(t *testing.T) {
	requireNodeGuest(t)
	src := `
import type { AutomationContext } from "one:automation";
export default async function run(ctx: AutomationContext) {
  ctx.log(ctx.trigger.objectApiName);
  return { ok: true };
}
`
	res, err := automation.RunGuest(context.Background(), &memHost{}, automation.GuestRequest{
		APIName: "Typed",
		Source:  src,
		Trigger: automation.SyncTrigger{ObjectAPIName: "Account"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || len(res.Logs) != 1 || res.Logs[0] != "Account" {
		t.Fatalf("res=%+v", res)
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

func TestRunGuest_DynamicNodeImportDenied(t *testing.T) {
	requireNodeGuest(t)
	src := `
export default async function run() {
  const spec = "node:" + "net";
  await import(spec);
  return { ok: true };
}
`
	_, err := automation.RunGuest(context.Background(), &memHost{}, automation.GuestRequest{
		APIName: "EscapeNet",
		Source:  src,
	})
	if err == nil || !strings.Contains(err.Error(), "forbidden import") {
		t.Fatalf("expected forbidden import, got %v", err)
	}
}

func TestRunGuest_FileURLEscapeDenied(t *testing.T) {
	requireNodeGuest(t)
	src := `
export default async function run() {
  const spec = "file://" + "/etc/passwd";
  await import(spec);
  return { ok: true };
}
`
	_, err := automation.RunGuest(context.Background(), &memHost{}, automation.GuestRequest{
		APIName: "EscapeFile",
		Source:  src,
	})
	if err == nil || !strings.Contains(err.Error(), "forbidden import") {
		t.Fatalf("expected forbidden import, got %v", err)
	}
}

func TestRunGuest_FetchDenied(t *testing.T) {
	requireNodeGuest(t)
	src := `
export default async function run() {
  await fetch("http://127.0.0.1:9/");
  return { ok: true };
}
`
	_, err := automation.RunGuest(context.Background(), &memHost{}, automation.GuestRequest{
		APIName: "EscapeFetch",
		Source:  src,
	})
	if err == nil || !strings.Contains(err.Error(), "guest network is disabled") {
		t.Fatalf("expected network denial, got %v", err)
	}
}

func TestRunGuest_BuiltinAndAddonDenied(t *testing.T) {
	requireNodeGuest(t)
	src := `
export default async function run() {
  try {
    process.getBuiltinModule("node:child_process");
    return { ok: false, error: "child_process builtin allowed" };
  } catch (e) {
    const msg = String(e && e.message || e);
    if (!msg.includes("forbidden builtin")) throw new Error("unexpected builtin error: " + msg);
  }
  try {
    process.getBuiltinModule("node:fs").readFileSync("/etc/passwd", "utf8");
    return { ok: false, error: "fs builtin allowed" };
  } catch (e) {
    const msg = String(e && e.message || e);
    if (!msg.includes("forbidden builtin")) throw new Error("unexpected fs error: " + msg);
  }
  try {
    process.dlopen({ exports: {} }, "/etc/passwd");
    return { ok: false, error: "dlopen allowed" };
  } catch (e) {
    const msg = String(e && e.message || e);
    if (!msg.includes("native addons are disabled")) throw new Error("unexpected dlopen error: " + msg);
  }
  return { ok: true };
}
`
	res, err := automation.RunGuest(context.Background(), &memHost{}, automation.GuestRequest{
		APIName: "EscapeBuiltin",
		Source:  src,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("res=%+v", res)
	}
}

func TestRunGuest_HostCreateDenied(t *testing.T) {
	requireNodeGuest(t)
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
	requireNodeGuest(t)
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

func TestNodeVersionPinned(t *testing.T) {
	if automation.NodeVersion != "22.22.2" {
		t.Fatalf("NodeVersion=%s", automation.NodeVersion)
	}
	body, err := os.ReadFile("../../deploy/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "NODE_VERSION="+automation.NodeVersion) {
		t.Fatalf("Dockerfile NODE_VERSION does not match NodeVersion %s", automation.NodeVersion)
	}
	if strings.Contains(string(body), "DENO_VERSION") || strings.Contains(string(body), "deno") {
		t.Fatal("Dockerfile still references Deno")
	}
}
