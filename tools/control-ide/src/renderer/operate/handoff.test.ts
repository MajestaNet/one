import { describe, expect, it } from "vitest";
import { boardHandoffFromRun, normalizeBoardHandoff } from "./handoff";

describe("normalizeBoardHandoff", () => {
  it("reads nested boardHandoff with recordIds and suggestions", () => {
    const h = normalizeBoardHandoff({
      summary: "Top accounts",
      boardHandoff: {
        objectApiName: "Account",
        recordIds: ["a1", "a2"],
        suggestions: [{ id: "open", label: "Open ranked", action: "focus_ids" }],
      },
    });
    expect(h?.objectApiName).toBe("Account");
    expect(h?.recordIds).toEqual(["a1", "a2"]);
    expect(h?.suggestions?.[0]?.action).toBe("focus_ids");
    expect(h?.rationale).toBe("Top accounts");
  });

  it("derives ids from records array", () => {
    const h = normalizeBoardHandoff({
      object: "Opportunity",
      records: [{ id: "o1" }, { Id: "o2" }],
      filters: [{ field: "StageName", op: "eq", value: "Discovery" }],
    });
    expect(h?.objectApiName).toBe("Opportunity");
    expect(h?.recordIds).toEqual(["o1", "o2"]);
    expect(h?.view?.filters?.[0]?.field).toBe("StageName");
  });

  it("returns null for empty payload", () => {
    expect(normalizeBoardHandoff({})).toBeNull();
  });

  it("returns null for canvas-id-only payloads (canvas removed)", () => {
    const h = normalizeBoardHandoff({
      boardHandoff: { canvasId: "c-1", canvasTitle: "Pipeline board" },
    });
    expect(h).toBeNull();
  });
});

describe("boardHandoffFromRun", () => {
  it("prefers structured output", () => {
    const h = boardHandoffFromRun({
      id: "run-1",
      status: "completed",
      goal: "anything",
      output: { boardHandoff: { objectApiName: "Case", recordIds: ["c1"] } },
    });
    expect(h?.objectApiName).toBe("Case");
    expect(h?.runId).toBe("run-1");
  });

  it("falls back to goal heuristics", () => {
    const h = boardHandoffFromRun({
      id: "run-2",
      status: "completed",
      goal: "Prioritize the accounts I should call",
    });
    expect(h?.objectApiName).toBe("Account");
    expect(h?.suggestions?.length).toBeGreaterThan(0);
  });

  it("maps pipeline and case goals and soft-defaults otherwise", () => {
    expect(
      boardHandoffFromRun({ id: "r3", status: "completed", goal: "Show my opportunity pipeline" })
        ?.objectApiName,
    ).toBe("Opportunity");
    expect(
      boardHandoffFromRun({ id: "r4", status: "completed", goal: "Triage open cases" })?.objectApiName,
    ).toBe("Case");
    expect(
      boardHandoffFromRun({ id: "r5", status: "completed", goal: "Do something useful" })?.objectApiName,
    ).toBe("Account");
  });

  it("parses proposed mutations and input handoff", () => {
    const fromInput = boardHandoffFromRun({
      id: "r6",
      status: "completed",
      input: {
        handoff: {
          object: "Contact",
          proposedMutations: [{ op: "update", object: "Contact", id: "c1", data: { Email: "a@b.c" } }],
        },
      },
    });
    expect(fromInput?.objectApiName).toBe("Contact");
    expect(fromInput?.proposedMutations?.[0]?.op).toBe("update");
  });
});
