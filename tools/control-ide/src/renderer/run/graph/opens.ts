import type { RunGraphDocument, RunGraphNode } from "./types";

/** Resolve an `opens` relationship in either authored direction to its Tool pin. */
export function toolForOpenedNode(
  document: RunGraphDocument,
  nodeId: string,
): RunGraphNode | undefined {
  for (const edge of document.edges) {
    if (edge.kind !== "opens") continue;
    const otherId =
      edge.from === nodeId ? edge.to :
        edge.to === nodeId ? edge.from :
          undefined;
    if (!otherId) continue;
    const tool = document.nodes.find((node) => node.id === otherId && node.kind === "tool");
    if (tool) return tool;
  }
  return undefined;
}
