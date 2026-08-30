import { describe, expect, it } from "vitest";
import { toolForOpenedNode } from "./opens";
import { RUN_GRAPH_API_VERSION, type RunGraphDocument } from "./types";

describe("toolForOpenedNode", () => {
  it("resolves the Tool on an opens edge in either direction", () => {
    const document: RunGraphDocument = {
      apiVersion: RUN_GRAPH_API_VERSION,
      id: "home",
      title: "My graph",
      nodes: [
        { id: "account", kind: "record", ref: { objectApiName: "Account", recordId: "a-1" } },
        { id: "tool", kind: "tool", toolRef: { toolSpecApiName: "AccountPlaybook" } },
      ],
      edges: [{ id: "opens-1", from: "tool", to: "account", kind: "opens" }],
    };
    expect(toolForOpenedNode(document, "account")).toEqual(document.nodes[1]);
    document.edges = [{ id: "opens-2", from: "account", to: "tool", kind: "opens" }];
    expect(toolForOpenedNode(document, "account")).toEqual(document.nodes[1]);
  });
});
