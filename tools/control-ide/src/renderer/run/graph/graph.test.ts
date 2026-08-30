import { describe, expect, it, vi } from "vitest";
import { getHomeRunGraph, putRunGraph, resolveRunGraphCards } from "./api";
import { applyRunGraphLens } from "./lenses";
import { sanitizeRunGraphDocument } from "./sanitize";
import { RUN_GRAPH_CARD_TTL_MS, RunGraphHydrateCache } from "./store";
import { RUN_GRAPH_API_VERSION, type RunGraphDocument } from "./types";
import { validateRunGraphDocument } from "./validate";

function document(): RunGraphDocument {
  return {
    apiVersion: RUN_GRAPH_API_VERSION,
    id: "home",
    title: "My graph",
    nodes: [
      {
        id: "my-day",
        kind: "cluster",
        label: "My day",
        layout: { x: 0, y: 0 },
      },
      {
        id: "account-1",
        kind: "record",
        ref: { objectApiName: "Account", recordId: "00000000-0000-4000-8000-000000000111" },
        layout: { x: 240, y: 0 },
      },
      {
        id: "tool-1",
        kind: "tool",
        toolRef: { toolSpecApiName: "AccountWorkspace__c" },
      },
    ],
    edges: [
      { id: "edge-1", from: "my-day", to: "account-1", kind: "next", weight: 1 },
    ],
    dataBindings: [
      {
        id: "accounts",
        objectApiName: "Account",
        fields: ["Name"],
        filters: [{ field: "Type", op: "eq", value: "Customer" }],
      },
    ],
  };
}

describe("Run graph validation and sanitization", () => {
  it("accepts the closed reference-only schema and rejects unknown kinds", () => {
    const valid = validateRunGraphDocument(document());
    expect(valid.ok).toBe(true);

    const invalid = document() as unknown as Record<string, unknown>;
    const nodes = invalid.nodes as Array<Record<string, unknown>>;
    nodes[0] = { id: "unsafe", kind: "remoteReact" };
    const result = validateRunGraphDocument(invalid);
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.issues.some((issue) => issue.path.includes("kind"))).toBe(true);
  });

  it("strips baked payloads recursively while preserving refs and binding definitions", () => {
    const raw = document() as unknown as Record<string, unknown>;
    raw.rows = [{ Name: "must not persist" }];
    const nodes = raw.nodes as Array<Record<string, unknown>>;
    nodes[1].data = { Name: "must not persist" };
    nodes[1].fields = { Name: "must not persist" };
    nodes[1].hydrated = { cards: [{ Name: "must not persist" }] };
    const sanitized = sanitizeRunGraphDocument(raw) as Record<string, unknown>;
    expect(sanitized.rows).toBeUndefined();
    const cleanRecord = (sanitized.nodes as Array<Record<string, unknown>>)[1];
    expect(cleanRecord.data).toBeUndefined();
    expect(cleanRecord.fields).toBeUndefined();
    expect(cleanRecord.hydrated).toBeUndefined();
    expect((cleanRecord.ref as Record<string, unknown>).recordId).toBeTruthy();
    const binding = (sanitized.dataBindings as Array<Record<string, unknown>>)[0];
    expect(binding.fields).toEqual(["Name"]);
    expect(((binding.filters as Array<Record<string, unknown>>)[0]).value).toBe("Customer");
  });
});

describe("Run graph API", () => {
  it("loads and defense-in-depth sanitizes home", async () => {
    const baked = document() as unknown as Record<string, unknown>;
    (baked.nodes as Array<Record<string, unknown>>)[1].fields = { Name: "must not display" };
    const fetchFn = vi.fn().mockResolvedValue({
      id: "row-id",
      graphKey: "home",
      title: "My graph",
      document: baked,
      revision: 2,
    });
    const graph = await getHomeRunGraph(fetchFn);
    expect(fetchFn).toHaveBeenCalledWith("/client/v1/run-graphs/home");
    expect(graph.revision).toBe(2);
    expect((graph.document.nodes[1] as unknown as Record<string, unknown>).fields).toBeUndefined();
  });

  it("sanitizes PUT and batches card resolve", async () => {
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path.endsWith("/resolve")) {
        return { nodes: [{ nodeId: "account-1", ok: false, code: "FORBIDDEN" }] };
      }
      const sent = JSON.parse(String(init?.body)) as RunGraphDocument;
      return { id: "row-id", graphKey: "home", title: sent.title, document: sent, revision: 3 };
    });
    const unsafe = document() as RunGraphDocument & { rows?: unknown[] };
    unsafe.rows = [{ Name: "no" }];
    const saved = await putRunGraph(fetchFn, "home", unsafe, 2);
    expect((saved.document as unknown as Record<string, unknown>).rows).toBeUndefined();
    expect(fetchFn).toHaveBeenCalledWith("/client/v1/run-graphs/home", expect.objectContaining({
      headers: { "If-Match": '"2"' },
    }));
    const results = await resolveRunGraphCards(fetchFn, [
      { nodeId: "account-1", objectApiName: "Account", recordId: "00000000-0000-4000-8000-000000000111" },
    ]);
    expect(results).toEqual([{ nodeId: "account-1", ok: false, code: "FORBIDDEN", record: undefined }]);
  });
});

describe("Run graph lenses and cache", () => {
  it("filters Tools and My day without creating another source of truth", () => {
    expect(applyRunGraphLens(document(), "tools").nodes.map((node) => node.id)).toEqual(["tool-1"]);
    const withCollection = document();
    withCollection.nodes.push({
      id: "accounts",
      kind: "collection",
      ref: { objectApiName: "Account" },
      label: "Accounts",
    });
    expect(applyRunGraphLens(withCollection, "objects").nodes.map((node) => node.id)).toEqual(["accounts"]);
    expect(applyRunGraphLens(document(), "my-day").nodes.map((node) => node.id)).toEqual([
      "my-day",
      "account-1",
    ]);
  });

  it("filters Watching to nodes joined by watches edges", () => {
    const graph = document();
    graph.nodes.push(
      {
        id: "contact-1",
        kind: "record",
        ref: { objectApiName: "Contact", recordId: "00000000-0000-4000-8000-000000000222" },
      },
      { id: "unwatched", kind: "insight", text: "Not in this lens" },
    );
    graph.edges.push(
      { id: "watch-1", from: "account-1", to: "contact-1", kind: "watches" },
      { id: "relates-1", from: "account-1", to: "contact-1", kind: "relates" },
    );

    const watching = applyRunGraphLens(graph, "watching");
    expect(watching.nodes.map((node) => node.id)).toEqual(["account-1", "contact-1"]);
    expect(watching.edges).toEqual([
      { id: "watch-1", from: "account-1", to: "contact-1", kind: "watches" },
    ]);
  });

  it("includes blocked topology in the My day viewport", () => {
    const graph = document();
    graph.nodes.push({ id: "blocker", kind: "question", text: "Resolve escalation" });
    graph.edges.push({ id: "blocks-1", from: "blocker", to: "account-1", kind: "blocks" });

    const myDay = applyRunGraphLens(graph, "my-day");
    expect(myDay.nodes.map((node) => node.id)).toEqual(expect.arrayContaining(["blocker", "account-1"]));
    expect(myDay.edges).toEqual(expect.arrayContaining([
      expect.objectContaining({ id: "blocks-1", kind: "blocks" }),
    ]));
  });

  it("expires hydrated cards after the TTL", () => {
    const cache = new RunGraphHydrateCache();
    const result = { nodeId: "account-1", ok: true, record: { Name: "Acme" } };
    cache.set("Account", "1", result, 100);
    expect(cache.get("Account", "1", 100 + RUN_GRAPH_CARD_TTL_MS)).toEqual(result);
    expect(cache.get("Account", "1", 101 + RUN_GRAPH_CARD_TTL_MS)).toBeUndefined();
  });
});
