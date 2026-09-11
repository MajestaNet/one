export default async function run(ctx) {
  if (ctx.trigger?.data?.Status !== "Accepted") {
    return { ok: true, skipped: true };
  }
  const accepted = await ctx.invokeAction({
    apiName: "quote.accept",
    input: { quoteId: String(ctx.trigger.recordId) },
  });
  return { ok: true, data: accepted };
}
