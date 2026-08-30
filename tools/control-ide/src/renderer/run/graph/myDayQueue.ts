import type { RunGraphDocument, RunGraphEdge, RunGraphNode } from "./types";

export type MyDayQueueReason = "blocks" | "next" | "watches" | "my-day";

export type MyDayQueueItem = {
  node: RunGraphNode;
  reason: MyDayQueueReason;
  weight?: number;
  edges: RunGraphEdge[];
  nextEdgeIds: string[];
  membershipEdgeIds: string[];
};

const REASON_RANK: Record<MyDayQueueReason, number> = {
  blocks: 0,
  next: 1,
  watches: 2,
  "my-day": 3,
};

function normalizedClusterLabel(node: RunGraphNode): string {
  return (node.label ?? node.id).trim().toLowerCase().replace(/[\s_]+/g, "-");
}

export function isMyDayCluster(node: RunGraphNode): boolean {
  return node.kind === "cluster" && normalizedClusterLabel(node) === "my-day";
}

function finiteWeight(edge: RunGraphEdge): number | undefined {
  return typeof edge.weight === "number" && Number.isFinite(edge.weight) ? edge.weight : undefined;
}

function strongestReason(edges: RunGraphEdge[], membershipEdgeIds: Set<string>): MyDayQueueReason {
  if (edges.some((edge) => edge.kind === "blocks")) return "blocks";
  if (edges.some((edge) => edge.kind === "next")) return "next";
  if (edges.some((edge) => edge.kind === "watches")) return "watches";
  if (edges.some((edge) => membershipEdgeIds.has(edge.id))) return "my-day";
  return "my-day";
}

/**
 * Builds the My day projection from durable topology only.
 *
 * Work edges contribute their target node. An `owns` edge connected to the
 * canonical My day cluster contributes its non-cluster endpoint as a
 * lower-priority fallback. A node appears once at its strongest reason.
 */
export function rankMyDayQueue(document: RunGraphDocument): MyDayQueueItem[] {
  const nodes = new Map(document.nodes.map((node) => [node.id, node]));
  const myDayClusterIds = new Set(document.nodes.filter(isMyDayCluster).map((node) => node.id));
  const edgesByNode = new Map<string, RunGraphEdge[]>();
  const membershipEdgeIds = new Set<string>();
  const insertionOrder = new Map<string, number>();

  const add = (nodeId: string, edge: RunGraphEdge) => {
    const node = nodes.get(nodeId);
    if (!node || node.kind === "cluster") return;
    if (!insertionOrder.has(nodeId)) insertionOrder.set(nodeId, insertionOrder.size);
    const edges = edgesByNode.get(nodeId) ?? [];
    if (!edges.some((candidate) => candidate.id === edge.id)) edges.push(edge);
    edgesByNode.set(nodeId, edges);
  };

  for (const edge of document.edges) {
    if (edge.kind === "blocks" || edge.kind === "next" || edge.kind === "watches") add(edge.to, edge);
    if (edge.kind === "owns" && myDayClusterIds.has(edge.from)) {
      membershipEdgeIds.add(edge.id);
      add(edge.to, edge);
    } else if (edge.kind === "owns" && myDayClusterIds.has(edge.to)) {
      membershipEdgeIds.add(edge.id);
      add(edge.from, edge);
    }
  }

  return [...edgesByNode.entries()]
    .map(([nodeId, edges]) => {
      const nextEdges = edges.filter((edge) => edge.kind === "next");
      const weights = nextEdges.flatMap((edge) => {
        const weight = finiteWeight(edge);
        return weight === undefined ? [] : [weight];
      });
      return {
        node: nodes.get(nodeId)!,
        reason: strongestReason(edges, membershipEdgeIds),
        weight: weights.length ? Math.max(...weights) : undefined,
        edges,
        nextEdgeIds: nextEdges.map((edge) => edge.id),
        membershipEdgeIds: edges
          .filter((edge) => membershipEdgeIds.has(edge.id))
          .map((edge) => edge.id),
      } satisfies MyDayQueueItem;
    })
    .sort((left, right) => {
      const rank = REASON_RANK[left.reason] - REASON_RANK[right.reason];
      if (rank !== 0) return rank;
      if (left.reason === "next" && right.reason === "next") {
        const weight = (right.weight ?? 0) - (left.weight ?? 0);
        if (weight !== 0) return weight;
      }
      return (insertionOrder.get(left.node.id) ?? 0) - (insertionOrder.get(right.node.id) ?? 0);
    });
}

/** Edge-only completion plan: leave every node and unrelated topology intact. */
export function myDayDoneEdgeIds(document: RunGraphDocument, nodeId: string): string[] {
  const myDayClusterIds = new Set(document.nodes.filter(isMyDayCluster).map((node) => node.id));
  return document.edges.flatMap((edge) => {
    const incomingWork =
      (edge.kind === "next" || edge.kind === "blocks" || edge.kind === "watches") &&
      edge.to === nodeId;
    const clusterMembership =
      edge.kind === "owns" &&
      ((myDayClusterIds.has(edge.from) && edge.to === nodeId) ||
        (myDayClusterIds.has(edge.to) && edge.from === nodeId));
    return incomingWork || clusterMembership ? [edge.id] : [];
  });
}

export function markMyDayItemDoneEdges(document: RunGraphDocument, nodeId: string): RunGraphEdge[] {
  const removed = new Set(myDayDoneEdgeIds(document, nodeId));
  return document.edges.filter((edge) => !removed.has(edge.id));
}

export function myDayPromotionEndpoints(
  item: MyDayQueueItem,
): { from: string; to: string } | undefined {
  const edge =
    item.edges.find((candidate) => candidate.kind === "next") ??
    item.edges.find((candidate) => candidate.kind === "blocks") ??
    item.edges.find((candidate) => candidate.kind === "watches") ??
    item.edges.find((candidate) => item.membershipEdgeIds.includes(candidate.id));
  if (!edge) return undefined;
  if (item.membershipEdgeIds.includes(edge.id) && edge.to !== item.node.id) {
    return { from: edge.to, to: edge.from };
  }
  return { from: edge.from, to: item.node.id };
}

/** Raises an existing next edge or adds one; record/task fields are never introduced. */
export function promoteMyDayItemEdges(
  document: RunGraphDocument,
  item: MyDayQueueItem,
  edgeId = `edge-${globalThis.crypto.randomUUID()}`,
): RunGraphEdge[] {
  const promotedWeight =
    Math.max(0, ...document.edges.flatMap((edge) => {
      if (edge.kind !== "next") return [];
      const weight = finiteWeight(edge);
      return weight === undefined ? [] : [weight];
    })) + 1;
  const incomingNext = item.edges
    .filter((edge) => edge.kind === "next")
    .sort((left, right) => (finiteWeight(right) ?? 0) - (finiteWeight(left) ?? 0))[0];
  if (incomingNext) {
    return document.edges.map((edge) =>
      edge.id === incomingNext.id ? { ...edge, weight: promotedWeight } : edge,
    );
  }

  const endpoints = myDayPromotionEndpoints(item);
  if (!endpoints) throw new Error("My day entry has no source edge");
  return [...document.edges, { id: edgeId, ...endpoints, kind: "next", weight: promotedWeight }];
}
