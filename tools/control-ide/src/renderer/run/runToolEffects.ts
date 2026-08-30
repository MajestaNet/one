import type { AgentRun } from "../agents/runs";
import { demoAccountsTool } from "./fixtures";
import { enrichRunOutputFromSummary } from "./agentEffectsFromSummary";
import { applyToolEffectsFromRunOutput, toolCreate, type ToolBridgeContext, type ToolRunEffects } from "./agentTools";
import { applyGraphEffectsFromRunOutput } from "./graph/agentGraphTools";
import {
  stageProposalFromRunOutput,
  type ProposalStagingStore,
} from "./graph/proposalStaging";
import { heuristicToolCallsFromGoal } from "./runToolHeuristics";
import { TOOL_DOCUMENT_API_VERSION, type ToolDocument, type ToolQueryBinding } from "./types";

const TOOL_GOAL_RE =
  /\b(interactive tool|as a tool|tool with|build a tool|compose a tool|show .+ tool|open pipeline tool)\b/i;

function asRecord(v: unknown): Record<string, unknown> | null {
  return v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : null;
}

function asOutputRecord(run: AgentRun): Record<string, unknown> | null {
  const out = run.output;
  if (!out || typeof out !== "object") return null;
  return out as Record<string, unknown>;
}

/** Build a starter Tool from a completed Run chat when the user asked for one. */
export function synthesizeToolDocumentFromRun(run: AgentRun): ToolDocument | null {
  const goal = run.goal ?? "";
  if (!TOOL_GOAL_RE.test(goal)) return null;

  const objectApiName = goal.toLowerCase().includes("opportunit") ? "Opportunity" : "Account";
  const id = `run-${run.id}`;
  const base = demoAccountsTool();
  return {
    ...base,
    apiVersion: TOOL_DOCUMENT_API_VERSION,
    id,
    title: `${objectApiName} tool`,
    nodes: base.nodes.map((n) =>
      n.id === "note"
        ? {
            ...n,
            props: {
              ...n.props,
              text: goal,
            },
          }
        : n,
    ),
    meta: { createdFromRunId: run.id, updatedAt: new Date().toISOString() },
  };
}

export type ProcessRunToolOptions = ToolBridgeContext & {
  mode?: string;
  synthesizeWhenPrompted?: boolean;
  activeToolId?: string;
  activeToolBindings?: ToolQueryBinding[];
  proposalStore?: ProposalStagingStore;
};

/** Process a completed agent run: explicit Tool output, tool calls, or prompt synthesis. */
export async function processRunToolEffects(
  run: AgentRun,
  opts: ProcessRunToolOptions = {},
): Promise<ToolRunEffects & { enrichedOutput?: Record<string, unknown>; proposalId?: string }> {
  const rawOutput = asOutputRecord(run);
  const output = enrichRunOutputFromSummary(rawOutput) ?? rawOutput;
  let enrichedOutput = output ? { ...output } : undefined;

  const callsFromOutput = parseToolCallsFromOutput(output);
  if (
    callsFromOutput.length === 0 &&
    opts.activeToolId &&
    run.goal &&
    run.status === "completed"
  ) {
    const heuristic = heuristicToolCallsFromGoal(
      run.goal,
      opts.activeToolId,
      opts.activeToolBindings ?? [],
    );
    if (heuristic?.length) {
      enrichedOutput = { ...(output ?? {}), toolCalls: heuristic };
    }
  }

  let effects = await applyToolEffectsFromRunOutput(enrichedOutput ?? output, opts);
  const graphEffects = await applyGraphEffectsFromRunOutput(enrichedOutput ?? output, {
    fetch:
      opts.fetch ??
      (async () => {
        throw new Error("graph.* requires an active Client API session");
      }),
  });
  if (graphEffects.graphResults?.length) {
    effects = {
      ...effects,
      graphChanged: graphEffects.graphChanged,
      graphResults: graphEffects.graphResults,
    };
    enrichedOutput = {
      ...(enrichedOutput ?? output ?? {}),
      graphResults: graphEffects.graphResults,
    };
  }

  let proposalId: string | undefined;
  if (opts.fetch && opts.proposalStore && run.status === "completed") {
    const proposal = await stageProposalFromRunOutput(
      opts.fetch,
      opts.proposalStore,
      run.id,
      enrichedOutput ?? output,
    );
    if (proposal) {
      proposalId = proposal.proposalId;
      effects = { ...effects, graphChanged: true };
      enrichedOutput = { ...(enrichedOutput ?? output ?? {}), proposalId };
    }
  }

  const toolsPlanned = Array.isArray(output?.toolsPlanned)
    ? (output.toolsPlanned as unknown[]).map(String)
    : [];
  const wantsTool =
    opts.synthesizeWhenPrompted !== false &&
    opts.mode === "operate" &&
    run.status === "completed" &&
    !effects.toolId &&
    (toolsPlanned.some((t) => t.startsWith("tool.")) || TOOL_GOAL_RE.test(run.goal ?? ""));

  if (wantsTool) {
    const synthesized = synthesizeToolDocumentFromRun(run);
    if (synthesized) {
      const toolCtx: ToolBridgeContext = { ...opts, store: opts.store ?? effects.store };
      const created = toolCreate({ document: synthesized, createdFromRunId: run.id }, toolCtx);
      if (created.ok) {
        effects = {
          ...effects,
          store: toolCtx.store ?? effects.store,
          toolId: created.toolId,
          toolTitle: created.title,
          toolResults: [...(effects.toolResults ?? []), { tool: "tool.create", result: created }],
        };
      }
    }
  }

  if (effects.toolId && (enrichedOutput ?? output)) {
    const base = enrichedOutput ?? output ?? {};
    enrichedOutput = {
      ...base,
      toolHandoff: {
        ...(asRecord(base.toolHandoff) ?? {}),
        toolId: effects.toolId,
        toolTitle: effects.toolTitle,
        toolSpecApiName: effects.toolSpecApiName,
        source: "tool_result",
        runId: run.id,
        rationale: typeof base.summary === "string" ? base.summary : run.goal,
      },
    };
  }

  return { ...effects, enrichedOutput, proposalId };
}

function callName(raw: unknown): string | null {
  if (!raw || typeof raw !== "object") return null;
  const tool = (raw as { tool?: unknown }).tool;
  return typeof tool === "string" && tool.trim() ? tool.trim() : null;
}

function mutationLabel(raw: unknown, index: number): string {
  const row = asRecord(raw);
  const objectName = typeof row?.object === "string" && row.object.trim() ? row.object.trim() : "record";
  const op = typeof row?.op === "string" && row.op.trim() ? row.op.trim() : "mutation";
  return `${objectName} ${op}`.trim() || `mutation ${index + 1}`;
}

/**
 * Tool / graph / record actions the IDE would apply from this run.
 * Does not include playbook allowlist `toolsPlanned`.
 */
export function pendingToolActionsFromRun(
  run: AgentRun,
  opts: { activeToolId?: string; activeToolBindings?: ToolQueryBinding[] } = {},
): string[] {
  const rawOutput = asOutputRecord(run);
  const output = enrichRunOutputFromSummary(rawOutput) ?? rawOutput;
  const names: string[] = [];
  const seen = new Set<string>();
  const add = (name: string) => {
    if (!name || seen.has(name)) return;
    seen.add(name);
    names.push(name);
  };

  for (const key of ["graphCalls", "graphBridgeCalls"] as const) {
    const list = output?.[key];
    if (!Array.isArray(list)) continue;
    for (const item of list) {
      const name = callName(item);
      if (name) add(name);
    }
  }
  for (const item of parseToolCallsFromOutput(output)) {
    const name = callName(item);
    if (name) add(name);
  }

  const proposal = asRecord(output?.proposal);
  const mutations = [
    ...(Array.isArray(proposal?.mutations) ? proposal.mutations : []),
    ...(Array.isArray(output?.proposedMutations) ? output.proposedMutations : []),
  ];
  mutations.forEach((item, index) => add(mutationLabel(item, index)));

  if (names.length === 0 && opts.activeToolId && run.goal) {
    const heuristic = heuristicToolCallsFromGoal(
      run.goal,
      opts.activeToolId,
      opts.activeToolBindings ?? [],
    );
    for (const call of heuristic ?? []) {
      add(call.tool);
    }
  }
  if (TOOL_GOAL_RE.test(run.goal ?? "")) {
    add("tool.create");
  }

  return names;
}

function parseToolCallsFromOutput(output: Record<string, unknown> | null): unknown[] {
  if (!output) return [];
  const raw = output.toolCalls ?? output.toolBridgeCalls ?? output.canvasToolCalls;
  return Array.isArray(raw) ? raw : [];
}
