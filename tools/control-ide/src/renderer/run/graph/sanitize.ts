const BAKED_KEYS = new Set([
  "rows",
  "data",
  "fields",
  "recordIds",
  "cards",
  "messages",
  "recordId",
  "value",
  "operations",
  "hydrated",
  "snapshot",
  "queryResult",
  "records",
]);

type SanitizeContext = { parentKey?: string; inBinding: boolean };

function keepBakedKey(key: string, context: SanitizeContext): boolean {
  if (key === "recordId" && context.parentKey === "ref") return true;
  if (!context.inBinding) return false;
  return key === "fields" || key === "recordIds" || key === "recordId" || key === "value";
}

function sanitizeValue(value: unknown, context: SanitizeContext): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => sanitizeValue(item, context));
  }
  if (!value || typeof value !== "object") return value;

  const out: Record<string, unknown> = {};
  for (const [key, child] of Object.entries(value as Record<string, unknown>)) {
    if (BAKED_KEYS.has(key) && !keepBakedKey(key, context)) continue;
    out[key] = sanitizeValue(child, {
      parentKey: key,
      inBinding: context.inBinding || key === "dataBindings",
    });
  }
  return out;
}

/** Defense in depth only; the Go write sanitizer remains authoritative. */
export function sanitizeRunGraphDocument(input: unknown): unknown {
  return sanitizeValue(input, { inBinding: false });
}
