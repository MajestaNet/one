export default async function run(ctx) {
  if (ctx.trigger?.data?.Status !== "Converted") {
    return { ok: true, skipped: true };
  }
  const converted = await ctx.invokeAction({
    apiName: "lead.convert",
    input: { leadId: String(ctx.trigger.recordId) },
  });
  return { ok: true, data: converted };
}
