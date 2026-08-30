package automation_test

import (
	"context"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/automation"
)

func TestRunUnitTest_RunUnderTestCreates(t *testing.T) {
	if _, err := automation.FindDeno(""); err != nil {
		t.Skip(err.Error())
	}
	autoSrc := `
export default async function run(ctx) {
  await ctx.createRecord({
    objectApiName: "Opportunity",
    data: { Name: String(ctx.trigger.data?.Name || ""), Amount: ctx.trigger.data?.Amount },
  });
  return { ok: true };
}
`
	testSrc := `
export default async function run(ctx) {
  await ctx.runUnderTest({
    trigger: {
      action: "create",
      objectApiName: "Account",
      recordId: "acc-1",
      data: { Name: "Acme", Amount: 100 },
    },
  });
  const { calls } = await ctx.getCalls({ method: "createRecord" });
  if (!calls || calls.length !== 1) throw new Error("expected 1 create, got " + (calls?.length ?? 0));
  if (calls[0].objectApiName !== "Opportunity") throw new Error("wrong object");
  if (calls[0].data?.Name !== "Acme") throw new Error("wrong name");
  return { ok: true };
}
`
	res, err := automation.RunUnitTest(context.Background(), automation.UnitTestRequest{
		TestAPIName:       "create_opp_on_account_test",
		TestFile:          "tests/automations/create_opp_on_account_test.ts",
		TestSource:        testSrc,
		AutomationAPIName: "CreateOpp_On_Account",
		AutomationSource:  autoSrc,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("result=%+v", res)
	}
}

func TestRunUnitTest_InvokeActionMock(t *testing.T) {
	if _, err := automation.FindDeno(""); err != nil {
		t.Skip(err.Error())
	}
	autoSrc := `
export default async function run(ctx) {
  const converted = await ctx.invokeAction({
    apiName: "lead.convert",
    input: { leadId: ctx.trigger.recordId, createOpportunity: false },
  });
  await ctx.updateRecord({
    objectApiName: "Account",
    recordId: String(converted.accountId || "acc-1"),
    data: { Description: "from-convert" },
  });
  return { ok: true };
}
`
	testSrc := `
export default async function run(ctx) {
  await ctx.runUnderTest({
    trigger: {
      action: "create",
      objectApiName: "Lead",
      recordId: "lead-1",
      data: { LastName: "Avery", Company: "Acme" },
    },
  });
  const { calls } = await ctx.getCalls({ method: "invokeAction" });
  if (!calls || calls.length !== 1) throw new Error("expected 1 invokeAction, got " + (calls?.length ?? 0));
  if (calls[0].apiName !== "lead.convert") throw new Error("wrong apiName");
  if (calls[0].input?.leadId !== "lead-1") throw new Error("wrong leadId");
  return { ok: true };
}
`
	res, err := automation.RunUnitTest(context.Background(), automation.UnitTestRequest{
		TestAPIName:       "on_lead_convert_copy_region_test",
		TestFile:          "tests/automations/on_lead_convert_copy_region_test.ts",
		TestSource:        testSrc,
		AutomationAPIName: "OnLeadConvert_CopyRegion",
		AutomationSource:  autoSrc,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("result=%+v", res)
	}
}

func TestRunUnitTest_RejectsNpmImport(t *testing.T) {
	_, err := automation.RunUnitTest(context.Background(), automation.UnitTestRequest{
		TestSource: `import _ from "npm:lodash"; export default async function run() { return { ok: true }; }`,
	})
	if err == nil || !strings.Contains(err.Error(), "forbidden import") {
		t.Fatalf("expected forbidden import, got %v", err)
	}
}

func TestFrozenSDKMethods(t *testing.T) {
	for _, m := range []string{"getRecord", "createRecord", "updateRecord", "deleteRecord", "query", "log", "invokeAction"} {
		if !automation.IsFrozenSDKMethod(m) {
			t.Fatalf("missing frozen method %s", m)
		}
	}
	if automation.IsFrozenSDKMethod("http") {
		// http/connector are frozen async methods (BP-014); sync rejects at runtime.
	} else {
		t.Fatal("http must be a frozen SDK method")
	}
}
