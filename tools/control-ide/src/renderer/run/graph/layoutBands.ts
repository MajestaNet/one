import type { RunGraphDocument, RunGraphNode, RunGraphNodeKind } from "./types";

export type RunGraphBand = "collections" | "tools" | "working";

const X_ORIGIN = 40;
const X_STRIDE = 260;
const Y_STRIDE = 154;
const COLLECTION_COLUMNS = 5;
const WORKING_COLUMNS = 4;

export function bandFor(node: Pick<RunGraphNode, "kind">): RunGraphBand {
  if (node.kind === "collection") return "collections";
  if (node.kind === "tool") return "tools";
  return "working";
}

function sorted(nodes: RunGraphNode[]): RunGraphNode[] {
  return nodes.slice().sort((left, right) => {
    const leftLabel = left.label || left.ref?.objectApiName || left.toolRef?.toolSpecApiName || left.id;
    const rightLabel = right.label || right.ref?.objectApiName || right.toolRef?.toolSpecApiName || right.id;
    return leftLabel.localeCompare(rightLabel);
  });
}

/** Stable, kind-aware layout: object shelf, Tool shelf, then active work. */
export function tidyPositions(nodes: RunGraphNode[]): Record<string, { x: number; y: number }> {
  const collections = sorted(nodes.filter((node) => bandFor(node) === "collections"));
  const tools = sorted(nodes.filter((node) => bandFor(node) === "tools"));
  const working = nodes.filter((node) => bandFor(node) === "working");
  const positions: Record<string, { x: number; y: number }> = {};

  collections.forEach((node, index) => {
    positions[node.id] = {
      x: X_ORIGIN + (index % COLLECTION_COLUMNS) * X_STRIDE,
      y: 36 + Math.floor(index / COLLECTION_COLUMNS) * Y_STRIDE,
    };
  });

  const collectionRows = Math.max(1, Math.ceil(collections.length / COLLECTION_COLUMNS));
  const toolY = 36 + collectionRows * Y_STRIDE + 54;
  tools.forEach((node, index) => {
    positions[node.id] = { x: X_ORIGIN + index * X_STRIDE, y: toolY };
  });

  const toolRows = Math.max(1, Math.ceil(tools.length / COLLECTION_COLUMNS));
  const workingY = toolY + toolRows * Y_STRIDE + 54;
  working.forEach((node, index) => {
    positions[node.id] = {
      x: X_ORIGIN + (index % WORKING_COLUMNS) * X_STRIDE,
      y: workingY + Math.floor(index / WORKING_COLUMNS) * Y_STRIDE,
    };
  });
  return positions;
}

export function nextBandPosition(
  document: Pick<RunGraphDocument, "nodes">,
  kind: RunGraphNodeKind,
): { x: number; y: number } {
  const probe: RunGraphNode = { id: "__new__", kind };
  return tidyPositions([...document.nodes, probe])[probe.id];
}

