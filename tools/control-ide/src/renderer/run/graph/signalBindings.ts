import { flattenRecordRow } from "../../operate/queryAutocomplete";
import { queryRecords } from "../../operate/recordClient";
import type { RunGraphFetch } from "./api";
import type { RunGraphBinding, RunGraphNode } from "./types";

export const RUN_GRAPH_SIGNAL_TTL_MS = 60_000;

export type RunGraphSignalResult = {
  nodeId: string;
  bindingId: string;
  objectApiName: string;
  rows: Record<string, unknown>[];
  fetchedAt: number;
};

type SignalCacheEntry = {
  signature: string;
  result: RunGraphSignalResult;
};

function bindingSignature(binding: RunGraphBinding): string {
  return JSON.stringify(binding);
}

function cloneResult(result: RunGraphSignalResult): RunGraphSignalResult {
  return { ...result, rows: result.rows.map((row) => ({ ...row })) };
}

/** In-memory display cache only; signal rows never enter RunGraphDocument. */
export class RunGraphSignalCache {
  private readonly entries = new Map<string, SignalCacheEntry>();

  get(
    nodeId: string,
    binding: RunGraphBinding,
    now = Date.now(),
  ): RunGraphSignalResult | undefined {
    const entry = this.entries.get(nodeId);
    if (!entry || entry.signature !== bindingSignature(binding)) return undefined;
    if (now - entry.result.fetchedAt > RUN_GRAPH_SIGNAL_TTL_MS) {
      this.entries.delete(nodeId);
      return undefined;
    }
    return cloneResult(entry.result);
  }

  set(nodeId: string, binding: RunGraphBinding, result: RunGraphSignalResult): void {
    this.entries.set(nodeId, {
      signature: bindingSignature(binding),
      result: cloneResult(result),
    });
  }

  clear(): void {
    this.entries.clear();
  }
}

export async function executeRunGraphSignalBinding(
  fetchFn: RunGraphFetch,
  node: RunGraphNode,
  binding: RunGraphBinding,
  now = Date.now(),
): Promise<RunGraphSignalResult> {
  if (node.kind !== "signal" || node.bindingId !== binding.id) {
    throw new Error("Signal node does not reference this binding");
  }
  const limit = Math.min(50, Math.max(1, Math.trunc(binding.limit ?? 25)));
  const raw = await queryRecords(fetchFn, {
    object: binding.objectApiName,
    select: binding.fields,
    filters: binding.filters ?? [],
    sort: binding.sort ?? [],
    limit,
  });
  return {
    nodeId: node.id,
    bindingId: binding.id,
    objectApiName: binding.objectApiName,
    rows: (raw.records ?? []).slice(0, limit).map(flattenRecordRow),
    fetchedAt: now,
  };
}
