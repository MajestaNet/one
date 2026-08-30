import { describe, expect, it } from "vitest";
import { heuristicToolCallsFromGoal } from "./runToolHeuristics";

describe("heuristicToolCallsFromGoal", () => {
  it("returns update+rerun for refresh top N accounts", () => {
    const calls = heuristicToolCallsFromGoal(
      "please refresh my account list but find the 5 accounts with the highest ranking",
      "tool-1",
      [{ id: "main", objectApiName: "Account", query: { limit: 100 } }],
    );
    expect(calls?.length).toBe(2);
    expect(calls?.[0]?.tool).toBe("tool.update");
    expect(calls?.[1]?.tool).toBe("tool.rerun");
    const patch = (calls?.[0]?.input as { patch?: { dataBindings?: Array<{ query?: { limit?: number } }> } })
      .patch;
    expect(patch?.dataBindings?.[0]?.query?.limit).toBe(5);
  });

  it("returns null for unrelated prompts", () => {
    expect(heuristicToolCallsFromGoal("write a poem", "tool-1", [])).toBeNull();
  });

  it("parses top N from top-first phrasing", () => {
    const calls = heuristicToolCallsFromGoal("refresh and show top 3 accounts", "tool-1", []);
    expect(calls?.[0]?.tool).toBe("tool.update");
    const patch = (calls?.[0]?.input as { patch?: { dataBindings?: Array<{ query?: { limit?: number } }> } })
      .patch;
    expect(patch?.dataBindings?.[0]?.query?.limit).toBe(3);
  });
});
