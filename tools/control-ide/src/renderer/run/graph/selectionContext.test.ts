import { describe, expect, it } from "vitest";
import { graphSelectionToContextExcerpt } from "./selectionContext";
import { RUN_GRAPH_API_VERSION, type RunGraphDocument } from "./types";

describe("graphSelectionToContextExcerpt", () => {
  it("includes only node ids and record refs, never hydrated or baked fields", () => {
    const document: RunGraphDocument = {
      apiVersion: RUN_GRAPH_API_VERSION,
      id: "home",
      title: "My graph",
      nodes: [
        {
          id: "account-1",
          kind: "record",
          ref: { objectApiName: "Account", recordId: "a-1" },
          fields: { Name: "must not escape" },
          hydrated: { Industry: "must not escape" },
        } as never,
        { id: "question-1", kind: "question", text: "Sensitive annotation not needed in context" },
        { id: "accounts", kind: "collection", ref: { objectApiName: "Account" } },
      ],
      edges: [],
    };

    const excerpt = graphSelectionToContextExcerpt(document, ["account-1", "question-1", "accounts"]);
    expect(excerpt?.structured?.records).toEqual([
      { nodeId: "account-1", kind: "record", objectApiName: "Account", recordId: "a-1" },
      { nodeId: "question-1", kind: "question" },
      { nodeId: "accounts", kind: "collection", objectApiName: "Account" },
    ]);
    expect(excerpt?.text).not.toMatch(/"(?:Name|Industry|fields|hydrated)"|Sensitive annotation/);
  });

  it("returns null for an empty selection", () => {
    const document: RunGraphDocument = {
      apiVersion: RUN_GRAPH_API_VERSION,
      id: "home",
      title: "My graph",
      nodes: [],
      edges: [],
    };
    expect(graphSelectionToContextExcerpt(document, [])).toBeNull();
  });
});
