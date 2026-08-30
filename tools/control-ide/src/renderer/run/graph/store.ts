import type { RunGraphResolveResult } from "./types";

export const RUN_GRAPH_CARD_TTL_MS = 60_000;

type CacheEntry = { result: RunGraphResolveResult; fetchedAt: number };

export class RunGraphHydrateCache {
  private readonly entries = new Map<string, CacheEntry>();

  get(objectApiName: string, recordId: string, now = Date.now()): RunGraphResolveResult | undefined {
    const key = `${objectApiName}\u0000${recordId}`;
    const entry = this.entries.get(key);
    if (!entry) return undefined;
    if (now - entry.fetchedAt > RUN_GRAPH_CARD_TTL_MS) {
      this.entries.delete(key);
      return undefined;
    }
    return entry.result;
  }

  set(objectApiName: string, recordId: string, result: RunGraphResolveResult, now = Date.now()) {
    this.entries.set(`${objectApiName}\u0000${recordId}`, { result, fetchedAt: now });
  }

  clear() {
    this.entries.clear();
  }
}
