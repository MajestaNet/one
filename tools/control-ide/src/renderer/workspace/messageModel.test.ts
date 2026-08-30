import { describe, expect, it } from "vitest";
import {
  defaultTileForMode,
  formatMessageTime,
  mergeStreamedRunReplies,
  messagesFromRun,
  roleLabel,
  runningAgentPlaceholder,
  toolsPlannedFromRun,
} from "./messageModel";
import type { AgentRun } from "../agents/runs";

describe("messageModel", () => {
  it("labels roles for transcript chrome", () => {
    expect(roleLabel("human")).toBe("You");
    expect(roleLabel("tool")).toBe("Tool");
    expect(roleLabel("approval")).toBe("Approval");
  });

  it("formats message times", () => {
    expect(formatMessageTime(undefined)).toBeUndefined();
    expect(formatMessageTime("not-a-date")).toBeUndefined();
    expect(formatMessageTime("2026-07-23T12:00:00.000Z")).toMatch(/\d/);
  });

  it("extracts toolsPlanned from run output", () => {
    expect(toolsPlannedFromRun({ id: "1", status: "completed" })).toEqual([]);
    expect(
      toolsPlannedFromRun({
        id: "1",
        status: "completed",
        output: { toolsPlanned: ["query", "update"] },
      }),
    ).toEqual(["query", "update"]);
  });

  it("builds tool + agent bubbles with Operate handoff", () => {
    const run: AgentRun = {
      id: "run-1",
      status: "completed",
      goal: "Prioritize accounts",
      output: {
        summary: "Top 3 accounts ranked",
        toolsPlanned: ["queryAccounts"],
        boardHandoff: {
          objectApiName: "Account",
          recordIds: ["a1", "a2"],
        },
      },
    };
    const msgs = messagesFromRun(run, { mode: "build", agentLabel: "QueryAssistant", now: 1000 });
    expect(msgs).toHaveLength(2);
    expect(msgs[0].role).toBe("tool");
    expect(msgs[0].steps?.map((s) => s.label)).toEqual(["queryAccounts"]);
    expect(msgs[0].boardHandoff?.source).toBe("tool_result");
    expect(msgs[0].tileActionLabel).toMatch(/matching records/i);
    expect(msgs[1].role).toBe("agent");
    expect(msgs[1].body).toMatch(/Top 3/);
    expect(msgs[1].agentLabel).toBe("QueryAssistant");
    expect(msgs[1].boardHandoff?.recordIds).toEqual(["a1", "a2"]);
  });

  it("emits approval role when awaiting approval", () => {
    const msgs = messagesFromRun(
      { id: "r2", status: "awaiting_approval", goal: "update records" },
      { mode: "build", now: 2 },
    );
    expect(msgs.some((m) => m.role === "approval")).toBe(true);
    expect(defaultTileForMode("build").tileAction).toBe("objects");
  });

  it("parks completed runs with pending tool actions for approval", () => {
    const msgs = messagesFromRun(
      {
        id: "r-fx",
        status: "completed",
        output: { summary: "Pin Acme" },
      },
      { mode: "operate", now: 4, pendingToolActions: ["graph.pin"] },
    );
    expect(msgs.some((m) => m.role === "approval")).toBe(true);
    expect(msgs.find((m) => m.role === "approval")?.pendingToolApply).toBe(true);
    expect(msgs.find((m) => m.role === "approval")?.runStatus).toBe("awaiting_approval");
    expect(msgs.find((m) => m.role === "tool")?.steps?.map((s) => s.label)).toEqual(["graph.pin"]);
  });

  it("builds tool handoff bubbles in Operate mode", () => {
    const run: AgentRun = {
      id: "run-2",
      status: "completed",
      goal: "Open pipeline tool",
      output: {
        summary: "Pipeline Tool ready",
        toolsPlanned: ["tool.create"],
        toolHandoff: {
          toolId: "run-run-2",
          toolTitle: "Pipeline tool",
        },
      },
    };
    const msgs = messagesFromRun(run, { mode: "operate", now: 3 });
    expect(msgs[0].toolHandoff?.toolId).toBe("run-run-2");
    expect(msgs[0].tileAction).toBe("runTool");
    expect(msgs[1].toolHandoff?.toolTitle).toBe("Pipeline tool");
    expect(defaultTileForMode("operate").tileAction).toBe("runTool");
  });

  it("keeps the streamed bubble id and body when merging finalized replies", () => {
    const stream = runningAgentPlaceholder({ id: "stream-1", agentLabel: "QueryAssistant" });
    stream.body = "Ranked **Acme** first.";
    const replies = messagesFromRun(
      {
        id: "run-9",
        status: "completed",
        output: { summary: "Top accounts", toolsPlanned: ["queryAccounts"] },
      },
      { mode: "build", now: 9 },
    );
    const merged = mergeStreamedRunReplies(stream, replies);
    expect(merged).toHaveLength(1);
    expect(merged[0].id).toBe("stream-1");
    expect(merged[0].body).toBe("Ranked **Acme** first.");
    expect(merged[0].runStatus).toBe("completed");
    expect(merged[0].steps?.map((s) => s.label)).toEqual(["queryAccounts"]);
  });

  it("uses the finalized summary when the stream body is still empty", () => {
    const stream = runningAgentPlaceholder({ id: "stream-2" });
    const merged = mergeStreamedRunReplies(stream, [
      { id: "a-final", role: "agent", body: "Done.", runStatus: "completed" },
    ]);
    expect(merged[0].id).toBe("stream-2");
    expect(merged[0].body).toBe("Done.");
  });
});
