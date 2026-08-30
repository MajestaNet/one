import type { RunGraphDocument, RunGraphEdge, RunGraphNode } from "./types";
import { isMyDayCluster } from "./myDayQueue";

export type RunGraphLensId = "all" | "objects" | "watching" | "tools" | "my-day";

export const RUN_GRAPH_LENSES: Array<{ id: RunGraphLensId; label: string }> = [
  { id: "all", label: "All" },
  { id: "objects", label: "Objects" },
  { id: "watching", label: "Watching" },
  { id: "tools", label: "Tools" },
  { id: "my-day", label: "My day" },
];

export type RunGraphViewDocument = { nodes: RunGraphNode[]; edges: RunGraphEdge[] };

export function applyRunGraphLens(
  document: RunGraphDocument,
  lens: RunGraphLensId,
): RunGraphViewDocument {
  if (lens === "all") return { nodes: document.nodes, edges: document.edges };
  if (lens === "tools") {
    const nodes = document.nodes.filter((node) => node.kind === "tool");
    const ids = new Set(nodes.map((node) => node.id));
    return { nodes, edges: document.edges.filter((edge) => ids.has(edge.from) && ids.has(edge.to)) };
  }
  if (lens === "objects") {
    const nodes = document.nodes.filter((node) => node.kind === "collection");
    const ids = new Set(nodes.map((node) => node.id));
    return { nodes, edges: document.edges.filter((edge) => ids.has(edge.from) && ids.has(edge.to)) };
  }
  if (lens === "watching") {
    const edges = document.edges.filter((edge) => edge.kind === "watches");
    const ids = new Set(edges.flatMap((edge) => [edge.from, edge.to]));
    return { nodes: document.nodes.filter((node) => ids.has(node.id)), edges };
  }

  const included = new Set(document.nodes.filter(isMyDayCluster).map((node) => node.id));
  const myDayClusterIds = new Set(included);
  for (const edge of document.edges) {
    const queueEdge = edge.kind === "blocks" || edge.kind === "next" || edge.kind === "watches";
    const clusterMembership =
      edge.kind === "owns" && (myDayClusterIds.has(edge.from) || myDayClusterIds.has(edge.to));
    if (queueEdge || clusterMembership) {
      included.add(edge.from);
      included.add(edge.to);
    }
  }
  const nodes = document.nodes.filter((node) => included.has(node.id));
  return {
    nodes,
    edges: document.edges.filter((edge) => included.has(edge.from) && included.has(edge.to)),
  };
}
