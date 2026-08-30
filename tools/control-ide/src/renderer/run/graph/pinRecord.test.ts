import { describe, expect, it, vi } from "vitest";
import { pinBoardHandoffToHomeGraph, pinRecordToHomeGraph } from "./pinRecord";
import type { RunGraphDocument } from "./types";

describe("pinRecordToHomeGraph", () => {
  it("PUTs a record node onto the home graph", async () => {
    const document: RunGraphDocument = {
      apiVersion: "one.runGraph/v1",
      id: "home",
      title: "My graph",
      nodes: [],
      edges: [],
    };
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/run-graphs/home" && !init?.method) {
        return { graphKey: "home", revision: 1, document };
      }
      if (path === "/client/v1/run-graphs/home" && init?.method === "PUT") {
        const body = JSON.parse(String(init.body)) as RunGraphDocument;
        return { graphKey: "home", revision: 2, document: body };
      }
      throw new Error(`unexpected ${path} ${init?.method}`);
    });

    const result = await pinRecordToHomeGraph(fetchFn, {
      objectApiName: "Account",
      recordId: "00000000-0000-4000-8000-000000000111",
    });
    expect(result.ok).toBe(true);
    expect(result).toMatchObject({ graphKey: "home", revision: 2 });
    const put = fetchFn.mock.calls.find(([, init]) => init?.method === "PUT");
    const body = JSON.parse(String(put?.[1]?.body)) as RunGraphDocument;
    expect(body.nodes).toHaveLength(1);
    expect(body.nodes[0]).toMatchObject({
      kind: "record",
      ref: { objectApiName: "Account", recordId: "00000000-0000-4000-8000-000000000111" },
    });
  });

  it("pins every unique BoardHandoff record id as a reference-only node", async () => {
    let document: RunGraphDocument = {
      apiVersion: "one.runGraph/v1",
      id: "home",
      title: "My graph",
      nodes: [],
      edges: [],
    };
    let revision = 1;
    const putBodies: RunGraphDocument[] = [];
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path !== "/client/v1/run-graphs/home") throw new Error(`unexpected ${path}`);
      if (init?.method === "PUT") {
        document = JSON.parse(String(init.body)) as RunGraphDocument;
        putBodies.push(document);
        revision += 1;
      }
      return { graphKey: "home", revision, document };
    });

    const result = await pinBoardHandoffToHomeGraph(fetchFn, {
      source: "run",
      objectApiName: "Account",
      recordIds: [
        "00000000-0000-4000-8000-000000000111",
        "00000000-0000-4000-8000-000000000222",
        "00000000-0000-4000-8000-000000000111",
      ],
    });

    expect(result.nodeIds).toHaveLength(2);
    expect(document.nodes).toEqual([
      expect.objectContaining({
        kind: "record",
        ref: { objectApiName: "Account", recordId: "00000000-0000-4000-8000-000000000111" },
      }),
      expect.objectContaining({
        kind: "record",
        ref: { objectApiName: "Account", recordId: "00000000-0000-4000-8000-000000000222" },
      }),
    ]);
    expect(putBodies.every((body) =>
      body.nodes.every((node) => Object.keys(node).every((key) => ["id", "kind", "ref", "layout"].includes(key))),
    )).toBe(true);
    expect(document.nodes.every((node) => node.layout && Number.isFinite(node.layout.x) && Number.isFinite(node.layout.y))).toBe(true);
  });
});
