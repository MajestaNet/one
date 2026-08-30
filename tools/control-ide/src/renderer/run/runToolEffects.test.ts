import { describe, expect, it } from "vitest";
import { pendingToolActionsFromRun } from "./runToolEffects";

describe("pendingToolActionsFromRun", () => {
  it("lists graph and tool calls, not the playbook allowlist", () => {
    expect(
      pendingToolActionsFromRun({
        id: "r1",
        status: "completed",
        output: {
          toolsPlanned: ["query", "update"],
          graphCalls: [{ tool: "graph.pin", input: { ref: { objectApiName: "Account", recordId: "a1" } } }],
          toolCalls: [{ tool: "tool.update", input: { toolId: "t1" } }],
        },
      }),
    ).toEqual(["graph.pin", "tool.update"]);
  });

  it("lists proposal mutations", () => {
    expect(
      pendingToolActionsFromRun({
        id: "r2",
        status: "completed",
        output: {
          proposal: {
            mutations: [{ op: "update", object: "Account", id: "a1", data: { Name: "Acme" } }],
          },
        },
      }),
    ).toEqual(["Account update"]);
  });

  it("returns empty when the reply has no actionable effects", () => {
    expect(
      pendingToolActionsFromRun({
        id: "r3",
        status: "completed",
        output: { summary: "Acme is an Account.", toolsPlanned: ["query"] },
      }),
    ).toEqual([]);
  });

  it("asks to approve tool.create when the prompt requested a Tool", () => {
    expect(
      pendingToolActionsFromRun({
        id: "r4",
        status: "completed",
        goal: "Compose a tool for accounts",
        output: { summary: "Here is a pipeline tool." },
      }),
    ).toEqual(["tool.create"]);
  });
});
