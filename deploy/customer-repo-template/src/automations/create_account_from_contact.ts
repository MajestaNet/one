import type { AutomationContext, AutomationResult } from "one:automation";

/**
 * Sample (ADR-014): on Contact create, open an Account and a Referral__c
 * linking Contact → Account (lookup fields on the custom object).
 */
export default async function run(ctx: AutomationContext): Promise<AutomationResult> {
  const first = String(ctx.trigger.data?.FirstName ?? "").trim();
  const last = String(ctx.trigger.data?.LastName ?? "Contact").trim();
  const name = [first, last].filter(Boolean).join(" ") || "New Contact";

  const account = await ctx.createRecord({
    objectApiName: "Account",
    data: { Name: `${name} Account` },
  });

  await ctx.createRecord({
    objectApiName: "Referral__c",
    data: {
      ContactId: ctx.trigger.recordId,
      AccountId: account.id,
    },
  });

  ctx.log("CreateAccount_From_Contact created Account + Referral for", name);
  return { ok: true };
}
