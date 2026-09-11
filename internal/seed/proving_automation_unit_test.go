package seed

import (
	"context"
	"testing"

	"github.com/MajestaNet/ide/internal/automation"
)

func TestLeadConvertOnConvertedStatusGuestSkipsUnlessConverted(t *testing.T) {
	if _, err := automation.FindDeno(""); err != nil {
		t.Skip(err.Error())
	}
	src := leadConvertOnConvertedStatusSrc
	skipSrc := `
export default async function run(ctx) {
  await ctx.runUnderTest({
    trigger: {
      action: "update",
      objectApiName: "Lead",
      recordId: "lead-1",
      data: { Status: "Qualified" },
    },
  });
  const { calls } = await ctx.getCalls({ method: "invokeAction" });
  if (calls && calls.length !== 0) throw new Error("expected skip, got " + (calls?.length ?? 0));
  return { ok: true };
}
`
	res, err := automation.RunUnitTest(context.Background(), automation.UnitTestRequest{
		TestAPIName:       "lead_convert_skip_test",
		TestFile:          "tests/automations/lead_convert_skip_test.ts",
		TestSource:        skipSrc,
		AutomationAPIName: "Lead_ConvertOnConvertedStatus",
		AutomationSource:  src,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("skip result=%+v", res)
	}

	invokeSrc := `
export default async function run(ctx) {
  await ctx.runUnderTest({
    trigger: {
      action: "update",
      objectApiName: "Lead",
      recordId: "lead-1",
      data: { Status: "Converted" },
    },
  });
  const { calls } = await ctx.getCalls({ method: "invokeAction" });
  if (!calls || calls.length !== 1) throw new Error("expected 1 invokeAction, got " + (calls?.length ?? 0));
  if (calls[0].apiName !== "lead.convert") throw new Error("wrong apiName");
  if (calls[0].input?.leadId !== "lead-1") throw new Error("wrong leadId");
  if (calls[0].input?.createOpportunity) throw new Error("v1 wrap must not pass createOpportunity");
  return { ok: true };
}
`
	res, err = automation.RunUnitTest(context.Background(), automation.UnitTestRequest{
		TestAPIName:       "lead_convert_invoke_test",
		TestFile:          "tests/automations/lead_convert_invoke_test.ts",
		TestSource:        invokeSrc,
		AutomationAPIName: "Lead_ConvertOnConvertedStatus",
		AutomationSource:  src,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("invoke result=%+v", res)
	}
}

func TestQuoteAcceptOnStatusAcceptedGuestSkipsUnlessAccepted(t *testing.T) {
	if _, err := automation.FindDeno(""); err != nil {
		t.Skip(err.Error())
	}
	src := quoteAcceptOnStatusAcceptedSrc
	skipSrc := `
export default async function run(ctx) {
  await ctx.runUnderTest({
    trigger: {
      action: "update",
      objectApiName: "Quote",
      recordId: "quote-1",
      data: { Status: "Presented" },
    },
  });
  const { calls } = await ctx.getCalls({ method: "invokeAction" });
  if (calls && calls.length !== 0) throw new Error("expected skip, got " + (calls?.length ?? 0));
  return { ok: true };
}
`
	res, err := automation.RunUnitTest(context.Background(), automation.UnitTestRequest{
		TestAPIName:       "quote_accept_skip_test",
		TestFile:          "tests/automations/quote_accept_skip_test.ts",
		TestSource:        skipSrc,
		AutomationAPIName: "Quote_AcceptOnStatusAccepted",
		AutomationSource:  src,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("skip result=%+v", res)
	}

	invokeSrc := `
export default async function run(ctx) {
  await ctx.runUnderTest({
    trigger: {
      action: "update",
      objectApiName: "Quote",
      recordId: "quote-1",
      data: { Status: "Accepted" },
    },
  });
  const { calls } = await ctx.getCalls({ method: "invokeAction" });
  if (!calls || calls.length !== 1) throw new Error("expected 1 invokeAction, got " + (calls?.length ?? 0));
  if (calls[0].apiName !== "quote.accept") throw new Error("wrong apiName");
  if (calls[0].input?.quoteId !== "quote-1") throw new Error("wrong quoteId");
  if (Object.prototype.hasOwnProperty.call(calls[0].input || {}, "createOrder")) {
    throw new Error("v1 wrap must not pass createOrder");
  }
  return { ok: true };
}
`
	res, err = automation.RunUnitTest(context.Background(), automation.UnitTestRequest{
		TestAPIName:       "quote_accept_invoke_test",
		TestFile:          "tests/automations/quote_accept_invoke_test.ts",
		TestSource:        invokeSrc,
		AutomationAPIName: "Quote_AcceptOnStatusAccepted",
		AutomationSource:  src,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("invoke result=%+v", res)
	}
}
