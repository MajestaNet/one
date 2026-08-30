import type { RunGraphDocument } from "./types";

const ATTENTION_EDGE_KINDS = new Set(["next", "watches", "blocks", "opens"]);

function myDayMemberIds(document: RunGraphDocument): Set<string> {
  const clusters = new Set(
    document.nodes
      .filter((node) => node.kind === "cluster" && (node.label || "").trim().toLowerCase() === "my day")
      .map((node) => node.id),
  );
  return new Set(
    document.edges.flatMap((edge) =>
      edge.kind === "owns" && clusters.has(edge.from) ? [edge.to] : [],
    ),
  );
}

export function orphanDerivedRecordIds(document: RunGraphDocument): Set<string> {
  const myDay = myDayMemberIds(document);
  const derived = new Set(
    document.edges.flatMap((edge) => edge.kind === "derivedFrom" ? [edge.from] : []),
  );
  const attended = new Set(
    document.edges.flatMap((edge) =>
      ATTENTION_EDGE_KINDS.has(edge.kind) ? [edge.from, edge.to] : [],
    ),
  );
  return new Set(
    document.nodes.flatMap((node) =>
      node.kind === "record" && derived.has(node.id) && !attended.has(node.id) && !myDay.has(node.id)
        ? [node.id]
        : [],
    ),
  );
}

export function tidyAttentionDocument(
  document: RunGraphDocument,
  staleNodeIds: ReadonlySet<string>,
): { document: RunGraphDocument; removed: number } {
  const remove = new Set([...staleNodeIds, ...orphanDerivedRecordIds(document)]);
  if (!remove.size) return { document, removed: 0 };
  return {
    document: {
      ...document,
      nodes: document.nodes.filter((node) => !remove.has(node.id)),
      edges: document.edges.filter((edge) => !remove.has(edge.from) && !remove.has(edge.to)),
    },
    removed: remove.size,
  };
}

