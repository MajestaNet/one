import type { AgentRun } from "../agents/runs";
import type { BoardHandoff, QueryFilter, SortSpec } from "./types";

function asRecord(v: unknown): Record<string, unknown> | null {
  return v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : null;
}

function asStringArray(v: unknown): string[] | undefined {
  if (!Array.isArray(v)) return undefined;
  const out = v.map((x) => String(x)).filter(Boolean);
  return out.length ? out : undefined;
}

function parseFilters(v: unknown): QueryFilter[] | undefined {
  if (!Array.isArray(v)) return undefined;
  const filters: QueryFilter[] = [];
  for (const raw of v) {
    const f = asRecord(raw);
    if (!f || typeof f.field !== "string" || typeof f.op !== "string") continue;
    filters.push({
      field: f.field,
      op: f.op as QueryFilter["op"],
      value: f.value,
    });
  }
  return filters.length ? filters : undefined;
}

function parseSort(v: unknown): SortSpec[] | undefined {
  if (!Array.isArray(v)) return undefined;
  const sort: SortSpec[] = [];
  for (const raw of v) {
    const s = asRecord(raw);
    if (!s || typeof s.field !== "string") continue;
    const direction = s.direction === "desc" ? "desc" : "asc";
    sort.push({ field: s.field, direction });
  }
  return sort.length ? sort : undefined;
}

/** Normalize a loose payload into BoardHandoff, or null if empty. */
export function normalizeBoardHandoff(raw: unknown, defaults: Partial<BoardHandoff> = {}): BoardHandoff | null {
  const obj = asRecord(raw);
  if (!obj && !defaults.recordIds?.length && !defaults.objectApiName) return null;

  const nested =
    asRecord(obj?.boardHandoff) ??
    asRecord(obj?.handoff) ??
    asRecord(obj?.board) ??
    obj;

  const recordIds =
    asStringArray(nested?.recordIds) ??
    asStringArray(nested?.ids) ??
    asStringArray(obj?.recordIds) ??
    defaults.recordIds;

  const records = Array.isArray(nested?.records) ? nested.records : Array.isArray(obj?.records) ? obj.records : null;
  const fromRecords =
    records
      ?.map((r) => {
        const rec = asRecord(r);
        return rec ? String(rec.id ?? rec.Id ?? "") : "";
      })
      .filter(Boolean) ?? [];

  const ids = recordIds?.length ? recordIds : fromRecords.length ? fromRecords : undefined;

  const objectApiName =
    (typeof nested?.objectApiName === "string" && nested.objectApiName) ||
    (typeof nested?.object === "string" && nested.object) ||
    (typeof obj?.objectApiName === "string" && obj.objectApiName) ||
    (typeof obj?.object === "string" && obj.object) ||
    defaults.objectApiName;

  const viewRaw = asRecord(nested?.view) ?? asRecord(obj?.view) ?? asRecord(nested?.query) ?? asRecord(obj?.query);
  const filters = parseFilters(viewRaw?.filters) ?? parseFilters(nested?.filters);
  const sort = parseSort(viewRaw?.sort) ?? parseSort(nested?.sort);
  const limit =
    typeof viewRaw?.limit === "number"
      ? viewRaw.limit
      : typeof nested?.limit === "number"
        ? nested.limit
        : undefined;

  const suggestionsRaw = Array.isArray(nested?.suggestions) ? nested.suggestions : [];
  const suggestions = suggestionsRaw
    .map((s, i) => {
      const row = asRecord(s);
      if (!row) return null;
      const label = String(row.label ?? row.title ?? "");
      if (!label) return null;
      return {
        id: String(row.id ?? `s-${i}`),
        label,
        action: String(row.action ?? row.id ?? "open"),
      };
    })
    .filter((s): s is NonNullable<typeof s> => Boolean(s));

  const proposedRaw = Array.isArray(nested?.proposedMutations) ? nested.proposedMutations : [];
  const proposedMutations: BoardHandoff["proposedMutations"] = [];
  for (const m of proposedRaw) {
    const row = asRecord(m);
    if (!row || typeof row.object !== "string") continue;
    if (row.op !== "create" && row.op !== "update" && row.op !== "delete") continue;
    proposedMutations.push({
      op: row.op,
      object: row.object,
      id: typeof row.id === "string" ? row.id : undefined,
      data: asRecord(row.data) ?? undefined,
    });
  }

  if (!objectApiName && !ids?.length && !filters?.length && !proposedMutations.length && !suggestions.length) {
    return null;
  }

  return {
    source: defaults.source ?? "run",
    runId: defaults.runId ?? (typeof nested?.runId === "string" ? nested.runId : undefined),
    objectApiName,
    view: filters || sort || limit ? { filters, sort, limit } : defaults.view,
    recordIds: ids,
    proposedMutations: proposedMutations.length ? proposedMutations : undefined,
    rationale:
      typeof nested?.rationale === "string"
        ? nested.rationale
        : typeof nested?.summary === "string"
          ? nested.summary
          : typeof obj?.summary === "string"
            ? obj.summary
            : defaults.rationale,
    suggestions: suggestions.length
      ? suggestions
      : defaults.suggestions ??
        (ids?.length
          ? [
              {
                id: "open-ranked",
                label: `Open ${ids.length} ranked record${ids.length === 1 ? "" : "s"}`,
                action: "focus_ids",
              },
            ]
          : undefined),
  };
}

/** Extract BoardHandoff from an agent run output/input. */
export function boardHandoffFromRun(run: AgentRun): BoardHandoff | null {
  const out = typeof run.output === "object" && run.output ? run.output : null;
  const input = run.input ?? null;
  const fromOut = normalizeBoardHandoff(out, { source: "run", runId: run.id });
  if (fromOut) return fromOut;
  const fromIn = normalizeBoardHandoff(input, { source: "run", runId: run.id });
  if (fromIn) return fromIn;
  // Soft default: prioritize Accounts when goal mentions accounts
  const goal = (run.goal ?? "").toLowerCase();
  if (goal.includes("account")) {
    return {
      source: "run",
      runId: run.id,
      objectApiName: "Account",
      rationale: run.goal,
      suggestions: [
        { id: "open-accounts", label: "Show Accounts", action: "open_object" },
        { id: "filter-customer", label: "Filter Type = Customer", action: "filter_type_customer" },
      ],
    };
  }
  if (goal.includes("opportunit") || goal.includes("pipeline")) {
    return {
      source: "run",
      runId: run.id,
      objectApiName: "Opportunity",
      rationale: run.goal,
      suggestions: [{ id: "open-opps", label: "Show Opportunities", action: "open_object" }],
    };
  }
  if (goal.includes("case") || goal.includes("service")) {
    return {
      source: "run",
      runId: run.id,
      objectApiName: "Case",
      rationale: run.goal,
      suggestions: [{ id: "open-cases", label: "Show Cases", action: "open_object" }],
    };
  }
  return {
    source: "run",
    runId: run.id,
    objectApiName: "Account",
    rationale: run.goal ?? `Run ${run.id}`,
    suggestions: [{ id: "show-matches", label: "Show matching records", action: "open_object" }],
  };
}

export const BOARD_HANDOFF_MIME = "application/x-one-board-handoff";
