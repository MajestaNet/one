import type { AgentRun } from "../agents/runs";
import { summarizeRunOutput } from "../agents/runs";
import { boardHandoffFromRun } from "../operate/handoff";
import type { BoardHandoff } from "../operate/types";
import { toolHandoffFromRun } from "../run/toolHandoff";
import type { ToolHandoff } from "../run/toolHandoff";
import type { StreamMessage, StreamRole, TileId, AppSection } from "./types";

export function roleLabel(role: StreamRole): string {
  switch (role) {
    case "human":
      return "You";
    case "agent":
      return "Agent";
    case "system":
      return "System";
    case "tool":
      return "Tool";
    case "approval":
      return "Approval";
  }
}

export function formatMessageTime(iso?: string): string | undefined {
  if (!iso) return undefined;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return undefined;
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

export function toolsPlannedFromRun(run: AgentRun): string[] {
  const out = run.output;
  if (!out || typeof out !== "object") return [];
  const tools = (out as { toolsPlanned?: unknown }).toolsPlanned;
  if (!Array.isArray(tools)) return [];
  return tools.map((t) => String(t)).filter(Boolean);
}

export function defaultTileForMode(mode: AppSection): { tileAction: TileId; tileActionLabel: string } {
  switch (mode) {
    case "operate":
      return { tileAction: "runTool", tileActionLabel: "Open Tool" };
    case "build":
      return { tileAction: "objects", tileActionLabel: "Open Objects" };
    case "govern":
      return { tileAction: "env", tileActionLabel: "Open Environments" };
    case "settings":
      return { tileAction: "account", tileActionLabel: "Open Account settings" };
  }
}

export type RunMessageOpts = {
  mode: AppSection;
  agentLabel?: string;
  now?: number;
  /** Concrete tool/graph/record actions waiting on explicit user approval. */
  pendingToolActions?: string[];
};

/** Build tool + agent/approval bubbles from a completed (or awaiting) run. */
export function messagesFromRun(run: AgentRun, opts: RunMessageOpts): StreamMessage[] {
  const now = opts.now ?? Date.now();
  const tile = defaultTileForMode(opts.mode);
  const pendingActions = (opts.pendingToolActions ?? []).map((label) => label.trim()).filter(Boolean);
  const tools = pendingActions.length ? pendingActions : toolsPlannedFromRun(run);
  const handoff: BoardHandoff | null = opts.mode === "build" ? boardHandoffFromRun(run) : null;
  const toolHandoff: ToolHandoff | null = opts.mode === "operate" ? toolHandoffFromRun(run) : null;
  const pendingToolApply = pendingActions.length > 0 && run.status !== "awaiting_approval";
  const needsApproval = run.status === "awaiting_approval" || pendingToolApply;
  const out: StreamMessage[] = [];

  if (tools.length) {
    out.push({
      id: `t-${run.id}-${now}`,
      role: "tool",
      body: needsApproval
        ? "Tools planned — awaiting approval before execution."
        : "Tools used in this run.",
      runId: run.id,
      runStatus: needsApproval ? "awaiting_approval" : run.status,
      toolsPlanned: tools,
      steps: tools.map((label, i) => ({
        id: `step-${i}`,
        label,
        state: needsApproval ? "pending" : "done",
      })),
      createdAt: run.completedAt ?? run.createdAt ?? new Date(now).toISOString(),
      agentLabel: opts.agentLabel,
      pendingToolApply: pendingToolApply || undefined,
      boardHandoff: handoff
        ? { ...handoff, source: handoff.source === "run" ? "tool_result" : handoff.source }
        : undefined,
      toolHandoff: toolHandoff ?? undefined,
      tileAction: tile.tileAction,
      tileActionLabel:
        opts.mode === "build" && handoff
          ? "Show matching records"
          : opts.mode === "operate" && toolHandoff
            ? "Open Tool"
            : tile.tileActionLabel,
    });
  }

  out.push({
    id: `a-${run.id}-${now}`,
    role: needsApproval ? "approval" : "agent",
    body: summarizeRunOutput(run),
    runId: run.id,
    runStatus: needsApproval ? "awaiting_approval" : run.status,
    toolsPlanned: tools.length ? tools : undefined,
    createdAt: run.completedAt ?? run.createdAt ?? new Date(now).toISOString(),
    agentLabel: opts.agentLabel,
    pendingToolApply: pendingToolApply || undefined,
    boardHandoff: handoff ?? undefined,
    toolHandoff: toolHandoff ?? undefined,
    tileAction: tile.tileAction,
    tileActionLabel:
      opts.mode === "build" && handoff
        ? "Show matching records"
        : opts.mode === "operate" && toolHandoff
          ? "Open Tool"
          : tile.tileActionLabel,
  });

  return out;
}

/**
 * Keep the in-flight assistant bubble (stable id) and fold finalized run
 * chrome onto it so the transcript does not remount on completion.
 */
export function mergeStreamedRunReplies(
  streamMessage: StreamMessage,
  replies: StreamMessage[],
): StreamMessage[] {
  const primary =
    replies.find((m) => m.role === "agent" || m.role === "approval") ?? replies.at(-1);
  const tool = replies.find((m) => m.role === "tool");
  if (!primary) {
    return [
      {
        ...streamMessage,
        runStatus: streamMessage.runStatus === "running" ? "completed" : streamMessage.runStatus,
      },
    ];
  }
  const streamedBody = streamMessage.body.trim();
  return [
    {
      ...streamMessage,
      ...primary,
      id: streamMessage.id,
      body: streamedBody || primary.body,
      createdAt: streamMessage.createdAt ?? primary.createdAt,
      steps: primary.steps ?? tool?.steps ?? streamMessage.steps,
      toolsPlanned: primary.toolsPlanned ?? tool?.toolsPlanned ?? streamMessage.toolsPlanned,
      pendingToolApply: primary.pendingToolApply ?? streamMessage.pendingToolApply,
    },
  ];
}

export function runningAgentPlaceholder(opts: {
  id: string;
  agentLabel?: string;
  createdAt?: string;
}): StreamMessage {
  return {
    id: opts.id,
    role: "agent",
    body: "",
    createdAt: opts.createdAt ?? new Date().toISOString(),
    agentLabel: opts.agentLabel,
    runStatus: "running",
  };
}

export function runningStatusMessage(agentLabel?: string): StreamMessage {
  return {
    id: `run-status-${Date.now()}`,
    role: "system",
    body: agentLabel ? `${agentLabel} is working…` : "Agent run in progress…",
    runStatus: "running",
    createdAt: new Date().toISOString(),
    agentLabel,
  };
}
