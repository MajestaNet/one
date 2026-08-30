import { describe, expect, it, vi } from "vitest";
import {
  executeRunGraphSignalBinding,
  RUN_GRAPH_SIGNAL_TTL_MS,
  RunGraphSignalCache,
} from "./signalBindings";
import type { RunGraphBinding, RunGraphNode } from "./types";

const node: RunGraphNode = { id: "signal-1", kind: "signal", bindingId: "renewals" };
const binding: RunGraphBinding = {
  id: "renewals",
  objectApiName: "Opportunity",
  fields: ["Name", "Amount"],
  filters: [{ field: "IsClosed", op: "eq", value: false }],
  sort: [{ field: "Amount", direction: "desc" }],
  limit: 10,
};

describe("Run graph signal bindings", () => {
  it("executes a definition through Client query and keeps rows in memory", async () => {
    const fetchFn = vi.fn().mockResolvedValue({
      records: [{ id: "00000000-0000-4000-8000-000000000111", data: { Name: "Renewal" } }],
    });

    const result = await executeRunGraphSignalBinding(fetchFn, node, binding, 100);

    expect(fetchFn).toHaveBeenCalledWith("/client/v1/query", {
      method: "POST",
      body: JSON.stringify({
        object: "Opportunity",
        select: ["Name", "Amount"],
        filters: [{ field: "IsClosed", op: "eq", value: false }],
        sort: [{ field: "Amount", direction: "desc" }],
        limit: 10,
      }),
    });
    expect(result.rows).toEqual([
      { id: "00000000-0000-4000-8000-000000000111", data: JSON.stringify({ Name: "Renewal" }) },
    ]);
  });

  it("expires results after the same one-minute display TTL as graph cards", () => {
    const cache = new RunGraphSignalCache();
    const result = {
      nodeId: node.id,
      bindingId: binding.id,
      objectApiName: binding.objectApiName,
      rows: [{ id: "r-1" }],
      fetchedAt: 100,
    };
    cache.set(node.id, binding, result);
    expect(cache.get(node.id, binding, 100 + RUN_GRAPH_SIGNAL_TTL_MS)).toEqual(result);
    expect(cache.get(node.id, binding, 101 + RUN_GRAPH_SIGNAL_TTL_MS)).toBeUndefined();
  });

  it("bounds signal query and display pages to 50 rows", async () => {
    const fetchFn = vi.fn().mockResolvedValue({
      records: Array.from({ length: 60 }, (_, index) => ({ Id: `row-${index}` })),
    });
    const result = await executeRunGraphSignalBinding(
      fetchFn,
      node,
      { ...binding, limit: 1_000 },
    );
    expect(JSON.parse(String(fetchFn.mock.calls[0]?.[1]?.body))).toMatchObject({ limit: 50 });
    expect(result.rows).toHaveLength(50);
  });
});
