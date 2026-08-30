import type { AutomationUnitContext } from "one:automation";

/** Unit harness: mock host, no Postgres. */
export default async function run(ctx: AutomationUnitContext) {
  await ctx.clearCalls();
  await ctx.runUnderTest({
    trigger: {
      action: "create",
      objectApiName: "Contact",
      recordId: "00000000-0000-4000-8000-0000000000cc",
      data: { FirstName: "Ada", LastName: "Lovelace" },
    },
  });
  const { calls } = await ctx.getCalls({ method: "createRecord" });
  if (!calls || calls.length !== 2) {
    throw new Error(`expected 2 createRecord calls, got ${calls?.length ?? 0}`);
  }
  if (calls[0].objectApiName !== "Account") {
    throw new Error(`expected Account first, got ${calls[0].objectApiName}`);
  }
  const acctData = (calls[0].data ?? {}) as Record<string, unknown>;
  if (acctData.Name !== "Ada Lovelace Account") {
    throw new Error(`expected Account Name Ada Lovelace Account, got ${acctData.Name}`);
  }
  if (calls[1].objectApiName !== "Referral__c") {
    throw new Error(`expected Referral__c, got ${calls[1].objectApiName}`);
  }
  const refData = (calls[1].data ?? {}) as Record<string, unknown>;
  if (refData.ContactId !== "00000000-0000-4000-8000-0000000000cc") {
    throw new Error("expected Referral ContactId to match trigger record");
  }
  if (!refData.AccountId) {
    throw new Error("expected Referral AccountId from created Account");
  }
  return { ok: true };
}
