export const BULK_COMPOSITE_MAX = 25;

export type CompositeSubrequest = {
  method: string;
  object: string;
  id: string;
  referenceId: string;
  body: Record<string, unknown>;
};

export type CompositeRowResult = {
  referenceId: string;
  status: number;
  ok: boolean;
  forbidden: boolean;
};

export function buildBulkPatchRequests(
  objectApiName: string,
  ids: string[],
  body: Record<string, unknown>,
): { requests: CompositeSubrequest[]; deferred: number } {
  const unique: string[] = [];
  const seen = new Set<string>();
  for (const id of ids) {
    const trimmed = id.trim();
    if (!trimmed || seen.has(trimmed)) continue;
    seen.add(trimmed);
    unique.push(trimmed);
  }
  const slice = unique.slice(0, BULK_COMPOSITE_MAX);
  return {
    requests: slice.map((id, i) => ({
      method: "PATCH",
      object: objectApiName,
      id,
      referenceId: `row${i + 1}`,
      body,
    })),
    deferred: Math.max(0, unique.length - slice.length),
  };
}

export function summarizeCompositeResponse(raw: unknown): {
  updated: number;
  forbidden: number;
  failed: number;
  rows: CompositeRowResult[];
  message: string;
} {
  const body = raw as { compositeResponse?: Array<{ status?: number; referenceId?: string }> };
  const entries = Array.isArray(body?.compositeResponse) ? body.compositeResponse : [];
  const rows: CompositeRowResult[] = entries.map((entry) => {
    const status = Number(entry.status ?? 0);
    return {
      referenceId: String(entry.referenceId ?? ""),
      status,
      ok: status === 200 || status === 201,
      forbidden: status === 403,
    };
  });
  const updated = rows.filter((r) => r.ok).length;
  const forbidden = rows.filter((r) => r.forbidden).length;
  const failed = rows.filter((r) => !r.ok && !r.forbidden).length;
  const parts = [`${updated} updated`];
  if (forbidden) parts.push(`${forbidden} forbidden`);
  if (failed) parts.push(`${failed} failed`);
  return { updated, forbidden, failed, rows, message: parts.join(", ") };
}
