/** Defense-in-depth: promote oneEffects / JSON envelopes embedded in summary text. */

const EFFECT_KEYS = [
  "graphCalls",
  "graphBridgeCalls",
  "toolCalls",
  "toolBridgeCalls",
  "proposal",
  "proposalId",
  "proposedMutations",
  "boardHandoff",
  "handoff",
  "toolHandoff",
] as const;

function asObject(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function hasEffectKey(obj: Record<string, unknown>): boolean {
  return EFFECT_KEYS.some((key) => key in obj);
}

function filterEffects(obj: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const key of EFFECT_KEYS) {
    if (key in obj) out[key] = obj[key];
  }
  return out;
}

function extractFenced(text: string, tag: string): { obj: Record<string, unknown>; rest: string } | null {
  const lower = text.toLowerCase();
  const needle = "```" + tag.toLowerCase();
  const idx = lower.indexOf(needle);
  if (idx < 0) return null;
  let start = idx + needle.length;
  while (start < text.length && (text[start] === " " || text[start] === "\t" || text[start] === "\r")) {
    start += 1;
  }
  if (text[start] === "\n") start += 1;
  const endRel = lower.indexOf("```", start);
  if (endRel < 0) return null;
  const raw = text.slice(start, endRel).trim();
  try {
    const parsed = JSON.parse(raw) as unknown;
    const obj = asObject(parsed);
    if (!obj) return null;
    const rest = `${text.slice(0, idx)}${text.slice(endRel + 3)}`.trim();
    return { obj, rest };
  } catch {
    return null;
  }
}

function extractTrailingObject(text: string): Record<string, unknown> | null {
  const trimmed = text.trim();
  const start = trimmed.lastIndexOf("{");
  if (start < 0) return null;
  try {
    return asObject(JSON.parse(trimmed.slice(start)));
  } catch {
    return null;
  }
}

/**
 * If `output.summary` embeds a oneEffects / json fence (or trailing effects object),
 * merge those keys onto the output and strip the fence from summary.
 * Existing structured keys win (do not clobber).
 */
export function enrichRunOutputFromSummary(
  output: Record<string, unknown> | null | undefined,
): Record<string, unknown> | null | undefined {
  if (!output) return output;
  const summary = typeof output.summary === "string" ? output.summary : "";
  if (!summary.trim()) return output;

  let effects: Record<string, unknown> | null = null;
  let nextSummary = summary;

  const one = extractFenced(summary, "oneEffects");
  if (one && hasEffectKey(one.obj)) {
    effects = filterEffects(one.obj);
    nextSummary = one.rest || (typeof one.obj.summary === "string" ? one.obj.summary : nextSummary);
  } else {
    const jsonFence = extractFenced(summary, "json");
    if (jsonFence && hasEffectKey(jsonFence.obj)) {
      effects = filterEffects(jsonFence.obj);
      nextSummary =
        jsonFence.rest || (typeof jsonFence.obj.summary === "string" ? jsonFence.obj.summary : nextSummary);
    } else {
      const trailing = extractTrailingObject(summary);
      if (trailing && hasEffectKey(trailing)) {
        effects = filterEffects(trailing);
      }
    }
  }

  if (!effects) return output;
  const enriched: Record<string, unknown> = { ...output, summary: nextSummary };
  for (const [key, value] of Object.entries(effects)) {
    if (key === "summary") continue;
    if (key in enriched) continue;
    enriched[key] = value;
  }
  return enriched;
}
