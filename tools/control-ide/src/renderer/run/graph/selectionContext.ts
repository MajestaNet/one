import {
  CONTEXT_EXCERPT_MIME,
  type ContextExcerpt,
} from "../../workspace/contextExcerpt";
import type { RunGraphDocument } from "./types";

type GraphSelectionRef = {
  nodeId: string;
  kind: string;
  objectApiName?: string;
  recordId?: string;
};

/** Builds an allowlisted, refs-only excerpt; hydrated card data is not accepted. */
export function graphSelectionToContextExcerpt(
  document: RunGraphDocument,
  selectedNodeIds: readonly string[],
): ContextExcerpt | null {
  const selected = new Set(selectedNodeIds);
  const refs: GraphSelectionRef[] = document.nodes.flatMap((node) => {
    if (!selected.has(node.id)) return [];
    const ref: GraphSelectionRef = { nodeId: node.id, kind: node.kind };
    if (node.kind === "record" && node.ref?.objectApiName && node.ref.recordId) {
      ref.objectApiName = node.ref.objectApiName;
      ref.recordId = node.ref.recordId;
    }
    if (node.kind === "collection" && node.ref?.objectApiName) {
      ref.objectApiName = node.ref.objectApiName;
    }
    return [ref];
  });
  if (!refs.length) return null;
  const label = `${refs.length} graph node${refs.length === 1 ? "" : "s"} selected`;
  return {
    id: `run-graph-selection:${refs.map((ref) => ref.nodeId).join(",")}`,
    mime: CONTEXT_EXCERPT_MIME,
    label,
    text: JSON.stringify({ graphKey: "home", nodes: refs }),
    source: "selection",
    structured: { records: refs },
  };
}
