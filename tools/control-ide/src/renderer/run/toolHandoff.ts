import type { AgentRun } from "../agents/runs";

/** Structured agent → Run Tool handoff (ADR-021 Phase 4). */
export type ToolHandoff = {
  source: "run" | "tool_result" | "message_excerpt";
  runId?: string;
  toolId: string;
  toolTitle?: string;
  toolSpecApiName?: string;
  rationale?: string;
};

function asRecord(v: unknown): Record<string, unknown> | null {
  return v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : null;
}

/** Normalize a loose payload into ToolHandoff, or null if no tool id. */
export function normalizeToolHandoff(raw: unknown, defaults: Partial<ToolHandoff> = {}): ToolHandoff | null {
  const obj = asRecord(raw);
  const nested =
    asRecord(obj?.toolHandoff) ??
    asRecord(obj?.handoff) ??
    obj;

  const toolId =
    (typeof nested?.toolId === "string" && nested.toolId) ||
    (typeof nested?.canvasId === "string" && nested.canvasId) ||
    defaults.toolId;
  if (!toolId) return null;

  return {
    source: defaults.source ?? "run",
    runId: defaults.runId ?? (typeof nested?.runId === "string" ? nested.runId : undefined),
    toolId,
    toolTitle:
      (typeof nested?.toolTitle === "string" && nested.toolTitle) ||
      (typeof nested?.canvasTitle === "string" && nested.canvasTitle) ||
      defaults.toolTitle,
    toolSpecApiName:
      (typeof nested?.toolSpecApiName === "string" && nested.toolSpecApiName) ||
      (typeof nested?.apiName === "string" && nested.apiName) ||
      defaults.toolSpecApiName,
    rationale:
      (typeof nested?.rationale === "string" && nested.rationale) ||
      (typeof nested?.summary === "string" && nested.summary) ||
      defaults.rationale,
  };
}

/** Extract ToolHandoff from an agent run output/input. */
export function toolHandoffFromRun(run: AgentRun): ToolHandoff | null {
  const out = typeof run.output === "object" && run.output ? run.output : null;
  const input = run.input ?? null;
  const fromOut = normalizeToolHandoff(out, { source: "run", runId: run.id });
  if (fromOut) return fromOut;
  return normalizeToolHandoff(input, { source: "run", runId: run.id });
}
