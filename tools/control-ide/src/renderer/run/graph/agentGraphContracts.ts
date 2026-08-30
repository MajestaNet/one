import type { RunGraphEdgeKind } from "./types";

/** Stable IDE bridge names. These are not hosted BP-006 server tools. */
export const GRAPH_BRIDGE_NAMES = [
  "graph.get",
  "graph.pin",
  "graph.pinCollection",
  "graph.cluster",
  "graph.mountTool",
  "graph.link",
  "graph.unlink",
  "graph.annotate",
  "graph.layout",
  "graph.publishSubgraph",
  "graph.compact",
] as const;

export type GraphBridgeName = (typeof GRAPH_BRIDGE_NAMES)[number];

export function isGraphBridgeName(value: string): value is GraphBridgeName {
  return (GRAPH_BRIDGE_NAMES as readonly string[]).includes(value);
}

export type GraphGetInput = { graphKey?: string };
export type GraphPinInput = { objectApiName: string; recordId: string; clusterId?: string; collectionId?: string };
export type GraphPinCollectionInput = {
  objectApiName: string;
  bindingId?: string;
  searchQ?: string;
  label?: string;
};
export type GraphClusterInput = { label: string; nodeIds?: string[] };
export type GraphMountToolInput = { toolSpecApiName?: string; workingToolId?: string };
export type GraphLinkInput = { from: string; to: string; kind: RunGraphEdgeKind };
export type GraphUnlinkInput = { edgeId: string };
export type GraphAnnotateInput = {
  text: string;
  kind: "insight" | "question";
  linkToNodeId?: string;
};
export type GraphLayoutInput = {
  positions: Record<string, { x: number; y: number }>;
};
export type GraphPublishSubgraphInput = { nodeIds: string[]; apiName: string; label: string };
export type GraphCompactInput = { strategy: "demote-stale" };

export type GraphBridgeCall = { tool: GraphBridgeName | string; input?: unknown };
export type GraphBridgeError = { ok: false; error: string };
