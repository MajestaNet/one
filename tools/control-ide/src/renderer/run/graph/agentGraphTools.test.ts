import { describe, expect, it, vi } from "vitest";
import { processRunToolEffects } from "../runToolEffects";
import { executeGraphBridge } from "./agentGraphTools";
import { RUN_GRAPH_API_VERSION, type RunGraphDocument, type RunGraphEnvelope } from "./types";

const ACCOUNT_ID = "00000000-0000-4000-8000-000000000111";

function graphServer() {
  let envelope: RunGraphEnvelope = {
    id: "graph-row-1",
    graphKey: "home",
    title: "My graph",
    revision: 1,
    document: {
      apiVersion: RUN_GRAPH_API_VERSION,
      id: "home",
      title: "My graph",
      nodes: [],
      edges: [],
    },
  };
  const writes: RunGraphDocument[] = [];
  const fetch = vi.fn(async (path: string, init?: RequestInit) => {
    if (path === "/client/v1/run-graphs/home" && !init?.method) return envelope;
    if (path === "/client/v1/run-graphs/home" && init?.method === "PUT") {
      const document = JSON.parse(String(init.body)) as RunGraphDocument;
      writes.push(document);
      envelope = { ...envelope, revision: envelope.revision + 1, document };
      return envelope;
    }
    throw new Error(`unexpected request: ${init?.method ?? "GET"} ${path}`);
  });
  return { fetch, writes, current: () => envelope.document };
}

describe("Run graph agent bridge", () => {
  it("supports the typed graph operation set through reference-only PUTs", async () => {
    const server = graphServer();
    const ctx = { fetch: server.fetch };

    const cluster = await executeGraphBridge("graph.cluster", { label: "My day" }, ctx);
    expect(cluster.ok).toBe(true);
    const clusterId = String(cluster.nodeId);

    const pin = await executeGraphBridge(
      "graph.pin",
      { objectApiName: "Account", recordId: ACCOUNT_ID, clusterId },
      ctx,
    );
    expect(pin.ok).toBe(true);
    const recordId = String(pin.nodeId);

    const mounted = await executeGraphBridge("graph.mountTool", { toolSpecApiName: "AccountBrief" }, ctx);
    expect(mounted.ok).toBe(true);
    const toolId = String(mounted.nodeId);

    const linked = await executeGraphBridge(
      "graph.link",
      { from: recordId, to: toolId, kind: "opens" },
      ctx,
    );
    expect(linked.ok).toBe(true);

    expect(
      await executeGraphBridge(
        "graph.annotate",
        { text: "Review renewal risk", kind: "insight", linkToNodeId: recordId },
        ctx,
      ),
    ).toMatchObject({ ok: true });
    expect(
      await executeGraphBridge(
        "graph.layout",
        { positions: { [recordId]: { x: 42, y: 84 } } },
        ctx,
      ),
    ).toMatchObject({ ok: true, positioned: 1 });
    expect(await executeGraphBridge("graph.unlink", { edgeId: String(linked.edgeId) }, ctx)).toMatchObject({
      ok: true,
    });

    const got = await executeGraphBridge("graph.get", {}, ctx);
    expect(got).toMatchObject({ ok: true, graphKey: "home" });
    expect(server.current().nodes.find((node) => node.id === recordId)?.layout).toMatchObject({ x: 42, y: 84 });
    expect(server.current().edges.some((edge) => edge.id === linked.edgeId)).toBe(false);
    expect(JSON.stringify(server.writes)).not.toMatch(/"rows"|"cards"|"hydrated"/);
  });

  it("pins an object collection without baking rows and wires derivedFrom on record pins", async () => {
    const server = graphServer();
    const ctx = { fetch: server.fetch };
    const collection = await executeGraphBridge("graph.pinCollection", { objectApiName: "Account", label: "Accounts" }, ctx);
    expect(collection.ok).toBe(true);
    const collectionId = String(collection.nodeId);
    expect(server.current().nodes).toEqual([
      expect.objectContaining({
        kind: "collection",
        ref: { objectApiName: "Account" },
        label: "Accounts",
      }),
    ]);

    const again = await executeGraphBridge("graph.pinCollection", { objectApiName: "Account" }, ctx);
    expect(again).toMatchObject({ ok: true, nodeId: collectionId });
    expect(server.current().nodes).toHaveLength(1);

    const pin = await executeGraphBridge(
      "graph.pin",
      { objectApiName: "Account", recordId: ACCOUNT_ID, collectionId },
      ctx,
    );
    expect(pin.ok).toBe(true);
    expect(server.current().edges).toEqual([
      expect.objectContaining({ from: String(pin.nodeId), to: collectionId, kind: "derivedFrom" }),
    ]);
    expect(JSON.stringify(server.writes)).not.toMatch(/"rows"|"Name":"Acme"/);
  });

  it("rejects fat annotate and pin payloads before persistence", async () => {
    const annotateServer = graphServer();
    const annotate = await executeGraphBridge(
      "graph.annotate",
      { text: "Do not bake", kind: "insight", fields: { AnnualRevenue: 10 } },
      { fetch: annotateServer.fetch },
    );
    expect(annotate).toMatchObject({ ok: false });
    expect(String(annotate.error)).toMatch(/fields is not allowed/);
    expect(annotateServer.writes).toHaveLength(0);

    const pinServer = graphServer();
    const pin = await executeGraphBridge(
      "graph.pin",
      { objectApiName: "Account", recordId: ACCOUNT_ID, rows: [{ Name: "Secret" }] },
      { fetch: pinServer.fetch },
    );
    expect(pin).toMatchObject({ ok: false });
    expect(String(pin.error)).toMatch(/rows is not allowed/);
    expect(pinServer.writes).toHaveLength(0);
  });

  it("applies graph calls from the shared Run agent toolCalls effect", async () => {
    const server = graphServer();
    const effects = await processRunToolEffects(
      {
        id: "run-graph-1",
        status: "completed",
        goal: "Pin this Account to my graph",
        output: {
          toolCalls: [
            { tool: "graph.pin", input: { objectApiName: "Account", recordId: ACCOUNT_ID } },
          ],
        },
      },
      { mode: "operate", fetch: server.fetch },
    );
    expect(effects.graphChanged).toBe(true);
    expect(effects.graphResults).toHaveLength(1);
    expect(effects.toolResults).toBeUndefined();
    expect(effects.enrichedOutput?.graphResults).toBeTruthy();
    expect(server.current().nodes).toHaveLength(1);
  });

  it("compacts only record nodes confirmed forbidden or missing", async () => {
    const server = graphServer();
    const ctx = { fetch: server.fetch };
    await executeGraphBridge("graph.pin", { objectApiName: "Account", recordId: ACCOUNT_ID }, ctx);
    server.fetch.mockImplementation(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/run-graphs/home" && !init?.method) {
        return { id: "graph-row-1", graphKey: "home", title: "My graph", revision: 2, document: server.current() };
      }
      if (path.endsWith("/resolve")) return { nodes: [{ nodeId: server.current().nodes[0].id, ok: false, code: "NOT_FOUND" }] };
      if (path === "/client/v1/run-graphs/home" && init?.method === "PUT") {
        const document = JSON.parse(String(init.body)) as RunGraphDocument;
        server.writes.push(document);
        return { id: "graph-row-1", graphKey: "home", title: "My graph", revision: 3, document };
      }
      throw new Error(`unexpected request: ${init?.method ?? "GET"} ${path}`);
    });
    const compacted = await executeGraphBridge("graph.compact", { strategy: "demote-stale" }, ctx);
    expect(compacted).toMatchObject({ ok: true, removed: 1 });
    expect(server.writes.at(-1)?.nodes).toHaveLength(0);
  });
});
