import dagre from "@dagrejs/dagre";
import type { DescribeField, DescribeObject } from "./types";
import type { GlobalDescribeObject } from "./describeCache";

export type GraphNode = {
  id: string;
  apiName: string;
  label: string;
  packageName?: string;
  fieldCount: number;
  /** False when the node comes from a package catalog that is not enabled yet. */
  enabled: boolean;
  x: number;
  y: number;
  width: number;
  height: number;
};

export type GraphEdge = {
  id: string;
  from: string;
  to: string;
  fieldApiName: string;
  relationshipType: "lookup" | "masterDetail";
  points: { x: number; y: number }[];
};

export type ObjectGraph = {
  nodes: GraphNode[];
  edges: GraphEdge[];
  width: number;
  height: number;
};

const NODE_W = 160;
const NODE_H = 48;
const MAX_NODES_DEFAULT = 80;

export function buildEdgesFromDescribe(
  objectApiName: string,
  fields: DescribeField[],
  knownObjects: Set<string>,
): Array<Pick<GraphEdge, "id" | "from" | "to" | "fieldApiName" | "relationshipType">> {
  const edges: Array<Pick<GraphEdge, "id" | "from" | "to" | "fieldApiName" | "relationshipType">> =
    [];
  for (const f of fields) {
    const ref = f.referenceTo?.trim();
    if (!ref || !knownObjects.has(ref)) continue;
    const relType: "lookup" | "masterDetail" = String(f.fieldType ?? "")
      .toLowerCase()
      .includes("master")
      ? "masterDetail"
      : "lookup";
    edges.push({
      id: `${objectApiName}.${f.apiName}->${ref}`,
      from: objectApiName,
      to: ref,
      fieldApiName: f.apiName,
      relationshipType: relType,
    });
  }
  return edges;
}

export function selectVisibleObjects(
  objects: GlobalDescribeObject[],
  opts?: { search?: string; packages?: string[]; maxNodes?: number },
): GlobalDescribeObject[] {
  const maxNodes = opts?.maxNodes ?? MAX_NODES_DEFAULT;
  const search = (opts?.search ?? "").trim().toLowerCase();
  const packages = opts?.packages?.filter(Boolean);
  let list = objects;
  if (packages?.length) {
    const set = new Set(packages.map((p) => p.toLowerCase()));
    list = list.filter((o) => set.has(String(o.packageName ?? "core").toLowerCase()));
  }
  if (search) {
    list = list.filter(
      (o) =>
        o.apiName.toLowerCase().includes(search) ||
        String(o.label ?? "")
          .toLowerCase()
          .includes(search),
    );
  }
  return list.slice(0, maxNodes);
}

export function layoutObjectGraph(
  objects: GlobalDescribeObject[],
  describes: Map<string, DescribeObject>,
  opts?: { search?: string; packages?: string[]; maxNodes?: number },
): ObjectGraph {
  const visible = selectVisibleObjects(objects, opts);
  const known = new Set(visible.map((o) => o.apiName));
  const g = new dagre.graphlib.Graph();
  g.setGraph({ rankdir: "LR", nodesep: 28, ranksep: 56, marginx: 24, marginy: 24 });
  g.setDefaultEdgeLabel(() => ({}));

  for (const o of visible) {
    const desc = describes.get(o.apiName);
    const extra = o as GlobalDescribeObject & { fieldCount?: number; enabled?: boolean };
    g.setNode(o.apiName, {
      label: o.label ?? o.apiName,
      width: NODE_W,
      height: NODE_H,
      packageName: o.packageName,
      enabled: extra.enabled !== false,
      fieldCount: Math.max(desc?.fields?.length ?? 0, extra.fieldCount ?? 0),
    });
  }

  for (const o of visible) {
    const desc = describes.get(o.apiName);
    if (!desc?.fields?.length) continue;
    for (const e of buildEdgesFromDescribe(o.apiName, desc.fields, known)) {
      if (!g.hasNode(e.from) || !g.hasNode(e.to)) continue;
      g.setEdge(e.from, e.to, {
        fieldApiName: e.fieldApiName,
        relationshipType: e.relationshipType,
      });
    }
  }

  dagre.layout(g);

  const nodes: GraphNode[] = visible.map((o) => {
    const extra = o as GlobalDescribeObject & { fieldCount?: number; enabled?: boolean };
    const n = g.node(o.apiName) as {
      x: number;
      y: number;
      width: number;
      height: number;
      fieldCount?: number;
      enabled?: boolean;
    };
    return {
      id: o.apiName,
      apiName: o.apiName,
      label: o.label ?? o.apiName,
      packageName: o.packageName,
      enabled: extra.enabled !== false,
      fieldCount: n.fieldCount ?? Math.max(describes.get(o.apiName)?.fields?.length ?? 0, extra.fieldCount ?? 0),
      x: n.x,
      y: n.y,
      width: n.width ?? NODE_W,
      height: n.height ?? NODE_H,
    };
  });

  const edges: GraphEdge[] = [];
  for (const e of g.edges()) {
    const data = g.edge(e) as {
      points?: { x: number; y: number }[];
      fieldApiName?: string;
      relationshipType?: "lookup" | "masterDetail";
    };
    edges.push({
      id: `${e.v}->${e.w}:${data.fieldApiName ?? ""}`,
      from: e.v,
      to: e.w,
      fieldApiName: data.fieldApiName ?? "",
      relationshipType: data.relationshipType ?? "lookup",
      points: data.points ?? [],
    });
  }

  const graph = g.graph() as { width?: number; height?: number };
  return {
    nodes,
    edges,
    width: Math.max(graph.width ?? 400, 400),
    height: Math.max(graph.height ?? 240, 240),
  };
}
