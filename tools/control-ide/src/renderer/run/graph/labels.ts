import type { RunGraphEdgeKind, RunGraphNodeKind } from "./types";

const EDGE_LABELS: Record<RunGraphEdgeKind, string> = {
  relates: "related to",
  owns: "contains",
  opens: "opens",
  derivedFrom: "from list",
  watches: "watching",
  blocks: "blocks",
  next: "next",
  explains: "explains",
};

const NODE_LABELS: Record<RunGraphNodeKind, string> = {
  record: "Record",
  cluster: "Group",
  person: "Person",
  tool: "Tool",
  insight: "Note",
  question: "Question",
  proposal: "Review",
  signal: "Live list",
  collection: "List",
};

export function runGraphEdgeLabel(kind: RunGraphEdgeKind): string {
  return EDGE_LABELS[kind];
}

export function runGraphKindLabel(kind: RunGraphNodeKind): string {
  return NODE_LABELS[kind];
}
