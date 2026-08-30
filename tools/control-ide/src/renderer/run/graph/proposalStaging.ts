import type { ProposedMutation } from "../../operate/types";
import { getHomeRunGraph, putRunGraph, type RunGraphFetch } from "./api";
import type { RunGraphDocument, RunGraphNode } from "./types";

export type ProposalStaging = {
  proposalId: string;
  runId?: string;
  mutations: ProposedMutation[];
  rationale?: string;
  createdAt: number;
  /** Exclusive index of mutations already committed through Client (partial apply resume). */
  appliedThrough?: number;
};

export type StageProposalInput = Omit<ProposalStaging, "createdAt"> & { createdAt?: number };

export type ProposalApplyResult = {
  results: unknown[];
  appliedCount: number;
  total: number;
  complete: boolean;
  /** Exclusive index into staging.mutations that were applied this attempt + prior. */
  appliedThrough: number;
};

export class ProposalApplyPartialError extends Error {
  readonly result: ProposalApplyResult;

  constructor(message: string, result: ProposalApplyResult) {
    super(message);
    this.name = "ProposalApplyPartialError";
    this.result = result;
  }
}

function cloneMutation(mutation: ProposedMutation): ProposedMutation {
  return {
    op: mutation.op,
    object: mutation.object,
    id: mutation.id,
    data: mutation.data ? { ...mutation.data } : undefined,
  };
}

function cloneStaging(staging: ProposalStaging): ProposalStaging {
  return {
    ...staging,
    mutations: staging.mutations.map(cloneMutation),
    appliedThrough: staging.appliedThrough,
  };
}

function validateMutation(mutation: ProposedMutation, index: number): void {
  if (!mutation.object.trim()) throw new Error(`mutations[${index}].object is required`);
  if ((mutation.op === "update" || mutation.op === "delete") && !mutation.id?.trim()) {
    throw new Error(`mutations[${index}].id is required for ${mutation.op}`);
  }
}

/** In-memory only. App creates one store per active IDE session/environment. */
export class ProposalStagingStore {
  private readonly entries = new Map<string, ProposalStaging>();

  stage(input: StageProposalInput): ProposalStaging {
    const proposalId = input.proposalId.trim();
    if (!proposalId) throw new Error("proposalId is required");
    if (!input.mutations.length) throw new Error("proposal mutations are required");
    input.mutations.forEach(validateMutation);
    const staging: ProposalStaging = {
      proposalId,
      runId: input.runId?.trim() || undefined,
      mutations: input.mutations.map(cloneMutation),
      rationale: input.rationale?.trim() || undefined,
      createdAt: input.createdAt ?? Date.now(),
      appliedThrough:
        typeof input.appliedThrough === "number" && Number.isFinite(input.appliedThrough)
          ? Math.max(0, Math.min(input.mutations.length, Math.floor(input.appliedThrough)))
          : undefined,
    };
    this.entries.set(proposalId, staging);
    return cloneStaging(staging);
  }

  get(proposalId: string): ProposalStaging | undefined {
    const staging = this.entries.get(proposalId);
    return staging ? cloneStaging(staging) : undefined;
  }

  /** Records how many leading mutations have already been committed (partial apply). */
  markAppliedThrough(proposalId: string, appliedThrough: number): ProposalStaging | undefined {
    const staging = this.entries.get(proposalId);
    if (!staging) return undefined;
    const next = {
      ...staging,
      appliedThrough: Math.max(0, Math.min(staging.mutations.length, Math.floor(appliedThrough))),
    };
    this.entries.set(proposalId, next);
    return cloneStaging(next);
  }

  drop(proposalId: string): boolean {
    return this.entries.delete(proposalId);
  }

  list(): ProposalStaging[] {
    return [...this.entries.values()].map(cloneStaging);
  }

  get size(): number {
    return this.entries.size;
  }

  clear(): void {
    this.entries.clear();
  }
}

function asObject(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function parseMutations(value: unknown): ProposedMutation[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((candidate) => {
    const row = asObject(candidate);
    if (
      !row ||
      (row.op !== "create" && row.op !== "update" && row.op !== "delete") ||
      typeof row.object !== "string"
    ) {
      return [];
    }
    return [{
      op: row.op,
      object: row.object,
      id: typeof row.id === "string" ? row.id : undefined,
      data: asObject(row.data) ?? undefined,
    } satisfies ProposedMutation];
  });
}

/** Parses proposal evidence without mutating graph or session state. */
export function proposalInputFromRunOutput(
  runId: string,
  output: Record<string, unknown> | null | undefined,
): StageProposalInput | null {
  if (!output) return null;
  const proposal = asObject(output.proposal);
  const handoff = asObject(output.boardHandoff) ?? asObject(output.handoff);
  const resolvedMutations = [
    parseMutations(proposal?.mutations),
    parseMutations(output.proposedMutations),
    parseMutations(handoff?.proposedMutations),
  ].find((mutations) => mutations.length > 0) ?? [];
  if (!resolvedMutations.length) return null;
  const proposalId =
    (typeof proposal?.proposalId === "string" && proposal.proposalId.trim()) ||
    (typeof output.proposalId === "string" && output.proposalId.trim()) ||
    `proposal-${runId}`;
  const rationale =
    (typeof proposal?.rationale === "string" && proposal.rationale) ||
    (typeof handoff?.rationale === "string" && handoff.rationale) ||
    (typeof output.summary === "string" && output.summary) ||
    undefined;
  return { proposalId, runId, mutations: resolvedMutations, rationale };
}

function nextId(prefix: string): string {
  return `${prefix}-${globalThis.crypto.randomUUID()}`;
}

/**
 * Stages payload in memory, then persists only a reference-only proposal pin
 * (and optional annotation) into the Run graph.
 */
export async function stageProposalOnGraph(
  fetchFn: RunGraphFetch,
  store: ProposalStagingStore,
  input: StageProposalInput,
): Promise<{ proposalId: string; nodeId: string; revision: number }> {
  const previous = store.get(input.proposalId);
  const staging = store.stage(input);
  try {
    const current = await getHomeRunGraph(fetchFn);
    const existing = current.document.nodes.find(
      (node) => node.kind === "proposal" && node.proposalId === staging.proposalId,
    );
    if (existing) {
      return { proposalId: staging.proposalId, nodeId: existing.id, revision: current.revision };
    }

    const proposalNode: RunGraphNode = {
      id: nextId("proposal"),
      kind: "proposal",
      proposalId: staging.proposalId,
    };
    const document: RunGraphDocument = {
      ...current.document,
      nodes: [...current.document.nodes, proposalNode],
      edges: [...current.document.edges],
    };
    if (staging.rationale) {
      const rationaleNode: RunGraphNode = {
        id: nextId("insight"),
        kind: "insight",
        text: staging.rationale,
      };
      document.nodes.push(rationaleNode);
      document.edges.push({
        id: nextId("edge"),
        from: rationaleNode.id,
        to: proposalNode.id,
        kind: "explains",
      });
    }
    const saved = await putRunGraph(fetchFn, "home", document, current.revision);
    return { proposalId: staging.proposalId, nodeId: proposalNode.id, revision: saved.revision };
  } catch (error) {
    if (previous) store.stage(previous);
    else store.drop(staging.proposalId);
    throw error;
  }
}

export async function stageProposalFromRunOutput(
  fetchFn: RunGraphFetch,
  store: ProposalStagingStore,
  runId: string,
  output: Record<string, unknown> | null | undefined,
): Promise<{ proposalId: string; nodeId: string; revision: number } | null> {
  const input = proposalInputFromRunOutput(runId, output);
  return input ? stageProposalOnGraph(fetchFn, store, input) : null;
}

/** Applies remaining staged Client-shaped operations under the caller's active JWT.
 * Prevalidates the full proposal first. On mid-flight Client failure, records
 * `appliedThrough` for resume and throws ProposalApplyPartialError — the graph pin
 * must stay until a complete apply succeeds.
 */
export async function applyProposalMutations(
  fetchFn: RunGraphFetch,
  staging: ProposalStaging,
  store?: ProposalStagingStore,
): Promise<ProposalApplyResult> {
  staging.mutations.forEach(validateMutation);
  const startAt =
    typeof staging.appliedThrough === "number" && Number.isFinite(staging.appliedThrough)
      ? Math.max(0, Math.min(staging.mutations.length, Math.floor(staging.appliedThrough)))
      : 0;
  const results: unknown[] = [];
  let appliedThrough = startAt;
  for (let index = startAt; index < staging.mutations.length; index += 1) {
    const mutation = staging.mutations[index]!;
    const objectPath = encodeURIComponent(mutation.object);
    try {
      if (mutation.op === "create") {
        results.push(await fetchFn(`/client/v1/sobjects/${objectPath}`, {
          method: "POST",
          body: JSON.stringify(mutation.data ?? {}),
        }));
      } else {
        const recordPath = `${objectPath}/${encodeURIComponent(mutation.id!)}`;
        if (mutation.op === "update") {
          results.push(await fetchFn(`/client/v1/sobjects/${recordPath}`, {
            method: "PATCH",
            body: JSON.stringify(mutation.data ?? {}),
          }));
        } else {
          results.push(await fetchFn(`/client/v1/sobjects/${recordPath}`, { method: "DELETE" }));
        }
      }
      appliedThrough = index + 1;
      store?.markAppliedThrough(staging.proposalId, appliedThrough);
    } catch (reason) {
      store?.markAppliedThrough(staging.proposalId, appliedThrough);
      const message = reason instanceof Error ? reason.message : String(reason);
      const result: ProposalApplyResult = {
        results,
        appliedCount: appliedThrough,
        total: staging.mutations.length,
        complete: false,
        appliedThrough,
      };
      throw new ProposalApplyPartialError(
        `Applied ${appliedThrough} of ${staging.mutations.length} mutations; stopped at mutations[${index}] (${mutation.op} ${mutation.object}): ${message}. Retry Apply to continue from the remaining ops, or Reject to drop the proposal.`,
        result,
      );
    }
  }
  const complete: ProposalApplyResult = {
    results,
    appliedCount: appliedThrough,
    total: staging.mutations.length,
    complete: true,
    appliedThrough,
  };
  return complete;
}
