export const RUN_GRAPH_API_VERSION = "one.runGraph/v1" as const;

export const RUN_GRAPH_NODE_KINDS = [
  "record",
  "cluster",
  "tool",
  "insight",
  "question",
  "proposal",
  "signal",
  "person",
  "collection",
] as const;

export const RUN_GRAPH_EDGE_KINDS = [
  "relates",
  "owns",
  "watches",
  "blocks",
  "next",
  "explains",
  "opens",
  "derivedFrom",
] as const;

export type RunGraphNodeKind = (typeof RUN_GRAPH_NODE_KINDS)[number];
export type RunGraphEdgeKind = (typeof RUN_GRAPH_EDGE_KINDS)[number];

export type RunGraphLayout = { x: number; y: number; w?: number; z?: number };

export type RunGraphNode = {
  id: string;
  kind: RunGraphNodeKind;
  ref?: {
    objectApiName?: string;
    recordId?: string;
    principalId?: string;
    contactRecordId?: string;
  };
  toolRef?: { toolSpecApiName?: string; workingToolId?: string };
  layout?: RunGraphLayout;
  cardProjection?: string[];
  label?: string;
  text?: string;
  proposalId?: string;
  bindingId?: string;
  searchQ?: string;
};

export type RunGraphEdge = {
  id: string;
  from: string;
  to: string;
  kind: RunGraphEdgeKind;
  weight?: number;
};

export type RunGraphBinding = {
  id: string;
  objectApiName: string;
  fields?: string[];
  filters?: unknown[];
  sort?: unknown[];
  limit?: number;
};

export type RunGraphLens = {
  id: string;
  label: string;
  filter: Record<string, unknown>;
};

export type RunGraphDocument = {
  apiVersion: typeof RUN_GRAPH_API_VERSION;
  id: string;
  title: string;
  revision?: number;
  nodes: RunGraphNode[];
  edges: RunGraphEdge[];
  dataBindings?: RunGraphBinding[];
  lenses?: RunGraphLens[];
  viewport?: { x: number; y: number; zoom: number };
};

export type RunGraphEnvelope = {
  id: string;
  graphKey: string;
  title: string;
  document: RunGraphDocument;
  revision: number;
  createdAt?: string;
  updatedAt?: string;
};

export type RunGraphResolveRef = {
  nodeId: string;
  objectApiName: string;
  recordId: string;
};

export type RunGraphResolveResult = {
  nodeId: string;
  ok: boolean;
  record?: Record<string, unknown>;
  code?: "FORBIDDEN" | "NOT_FOUND" | "UNAVAILABLE";
};

export type RunGraphValidationIssue = { path: string; message: string };
export type RunGraphValidationResult =
  | { ok: true; document: RunGraphDocument }
  | { ok: false; issues: RunGraphValidationIssue[] };

export function isRunGraphNodeKind(value: unknown): value is RunGraphNodeKind {
  return typeof value === "string" && (RUN_GRAPH_NODE_KINDS as readonly string[]).includes(value);
}

export function isRunGraphEdgeKind(value: unknown): value is RunGraphEdgeKind {
  return typeof value === "string" && (RUN_GRAPH_EDGE_KINDS as readonly string[]).includes(value);
}
