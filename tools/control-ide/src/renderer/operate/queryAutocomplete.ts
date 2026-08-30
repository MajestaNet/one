import type { DescribeField } from "./types";
import { FILTER_OPS } from "./describeCache";

export type AutocompleteSuggestion = {
  label: string;
  insertText: string;
  detail?: string;
  kind: "object" | "field" | "op" | "keyword";
};

/** Rank object / field suggestions for a typed prefix (case-insensitive). */
export function rankSuggestions(
  items: AutocompleteSuggestion[],
  prefix: string,
  limit = 25,
): AutocompleteSuggestion[] {
  const q = prefix.trim().toLowerCase();
  const scored = items
    .map((item) => {
      const label = item.label.toLowerCase();
      if (!q) return { item, score: 1 };
      if (label === q) return { item, score: 100 };
      if (label.startsWith(q)) return { item, score: 80 };
      if (label.includes(q)) return { item, score: 40 };
      return null;
    })
    .filter((x): x is { item: AutocompleteSuggestion; score: number } => x != null)
    .sort((a, b) => b.score - a.score || a.item.label.localeCompare(b.item.label));
  return scored.slice(0, limit).map((s) => s.item);
}

export function objectSuggestions(apiNames: string[]): AutocompleteSuggestion[] {
  return apiNames.map((apiName) => ({
    label: apiName,
    insertText: apiName,
    kind: "object" as const,
    detail: "object",
  }));
}

export function fieldSuggestions(fields: DescribeField[]): AutocompleteSuggestion[] {
  return fields.map((f) => ({
    label: f.apiName,
    insertText: f.apiName,
    kind: "field" as const,
    detail: f.fieldType ?? "field",
  }));
}

export function opSuggestions(): AutocompleteSuggestion[] {
  return FILTER_OPS.map((op) => ({
    label: op,
    insertText: op,
    kind: "op" as const,
    detail: "filter op",
  }));
}

export function defaultQueryJson(objectApiName: string): string {
  return JSON.stringify(
    {
      object: objectApiName || "Account",
      filters: [],
      sort: [],
      limit: 25,
    },
    null,
    2,
  );
}

/** Prefer Name-like columns, then Id, then remaining keys (capped). */
export function resultColumns(rows: Record<string, unknown>[], maxCols = 8): string[] {
  if (!rows.length) return ["id"];
  const keys = Object.keys(rows[0] ?? {});
  const preferred = ["Name", "name", "Subject", "LastName", "CaseNumber", "label", "Id", "id"];
  const ordered: string[] = [];
  for (const p of preferred) {
    if (keys.includes(p) && !ordered.includes(p)) ordered.push(p);
  }
  for (const k of keys) {
    if (!ordered.includes(k)) ordered.push(k);
    if (ordered.length >= maxCols) break;
  }
  return ordered.slice(0, maxCols);
}

/** Flatten one level of relationship payloads for list display. */
export function flattenRecordRow(rec: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(rec)) {
    if (v != null && typeof v === "object" && !Array.isArray(v)) {
      out[k] = JSON.stringify(v);
    } else if (Array.isArray(v)) {
      out[k] = `[${v.length}]`;
    } else {
      out[k] = v;
    }
  }
  return out;
}
