import { describe, expect, it, vi } from "vitest";
import { graphSubgraphToToolDocument, publishRunGraphSubgraph } from "./publishSubgraph";
import { RUN_GRAPH_API_VERSION, type RunGraphDocument } from "./types";

const graph: RunGraphDocument = {
  apiVersion: RUN_GRAPH_API_VERSION,
  id: "home",
  title: "My graph",
  nodes: [
    { id: "record-1", kind: "record", ref: { objectApiName: "Account", recordId: "00000000-0000-4000-8000-000000000111" }, layout: { x: 10, y: 20 } },
    { id: "tool-1", kind: "tool", toolRef: { toolSpecApiName: "SourceTool" } },
    { id: "signal-1", kind: "signal", bindingId: "renewals" },
  ],
  edges: [],
  dataBindings: [{
    id: "renewals",
    objectApiName: "Opportunity",
    fields: ["Name", "Amount"],
    filters: [{ field: "IsClosed", op: "eq", value: false }],
    limit: 25,
  }],
};

describe("Run graph subgraph publishing", () => {
  it("creates a reference/query-only ToolDocument", () => {
    const doc = graphSubgraphToToolDocument(graph, ["record-1"], "AccountFocus", "Account focus");
    expect(doc.nodes[0]).toMatchObject({ kind: "recordCard", bindingId: "record-record-1" });
    expect(doc.dataBindings?.[0].query.filters).toEqual([
      { field: "Id", op: "eq", value: "00000000-0000-4000-8000-000000000111" },
    ]);
    expect(JSON.stringify(doc.nodes)).not.toMatch(/fields|rows|cards|hydrated/);
  });

  it("requires can_publish when reusing a mounted Tool", async () => {
    const denied = vi.fn().mockResolvedValue({ apiName: "SourceTool", permissions: { canOpen: true, canInteract: true, canModify: false, canPublish: false } });
    await expect(publishRunGraphSubgraph(denied, graph, ["tool-1"], "DerivedTool", "Derived tool"))
      .rejects.toThrow(/can_publish/);

    const allowed = vi.fn(async (path: string) => path.startsWith("/client/")
      ? { apiName: "SourceTool", permissions: { canOpen: true, canInteract: true, canModify: false, canPublish: true } }
      : { apiName: "DerivedTool" });
    await expect(publishRunGraphSubgraph(allowed, graph, ["tool-1"], "DerivedTool", "Derived tool"))
      .resolves.toEqual({ apiName: "DerivedTool", nodeCount: 1 });
    expect(allowed).toHaveBeenCalledWith("/metadata/v1/tools", expect.objectContaining({ method: "POST" }));
  });

  it("publishes signal definitions with their field projection and no live rows", () => {
    const doc = graphSubgraphToToolDocument(graph, ["signal-1"], "RenewalQueue", "Renewal queue");
    expect(doc.nodes).toEqual([
      expect.objectContaining({ kind: "queryResult", bindingId: "renewals" }),
    ]);
    expect(doc.dataBindings).toEqual([{
      id: "renewals",
      objectApiName: "Opportunity",
      query: {
        select: ["Name", "Amount"],
        filters: [{ field: "IsClosed", op: "eq", value: false }],
        sort: undefined,
        limit: 25,
      },
    }]);
    expect(JSON.stringify(doc)).not.toMatch(/"rows"|"records"|"fields"/);
  });
});
