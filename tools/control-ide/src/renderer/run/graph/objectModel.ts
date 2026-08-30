import type { DescribeObject } from "../../operate/types";
import type { GlobalDescribeObject } from "../../operate/describeCache";
import { tidyPositions } from "./layoutBands";
import type { RunGraphDocument, RunGraphEdge, RunGraphNode } from "./types";

export const MODEL_EDGE_PREFIX = "model:";
export const RUN_GRAPH_NODE_LIMIT = 200;

function graphId(prefix: string): string {
  return `${prefix}-${globalThis.crypto.randomUUID()}`;
}

function baseCollection(node: RunGraphNode): boolean {
  return node.kind === "collection" && Boolean(node.ref?.objectApiName) && !node.bindingId && !node.searchQ;
}

function modelEdgeId(from: string, to: string): string {
  return `${MODEL_EDGE_PREFIX}${from}:${to}`;
}

export type ObjectModelMerge = {
  document: RunGraphDocument;
  addedObjects: number;
  relationshipCount: number;
  changed: boolean;
};

/**
 * Ensures one base collection for every object returned by the caller's describe,
 * then derives collection-to-collection topology from lookup/reference fields.
 */
export function mergeAccessibleObjectModel(
  document: RunGraphDocument,
  catalog: readonly GlobalDescribeObject[],
  describes: ReadonlyMap<string, DescribeObject>,
): ObjectModelMerge {
  const nodes = document.nodes.slice();
  const collections = new Map<string, RunGraphNode>();
  for (const node of nodes) {
    if (baseCollection(node) && node.ref?.objectApiName && !collections.has(node.ref.objectApiName)) {
      collections.set(node.ref.objectApiName, node);
    }
  }

  let addedObjects = 0;
  for (const object of catalog) {
    if (collections.has(object.apiName)) continue;
    if (nodes.length >= RUN_GRAPH_NODE_LIMIT) {
      throw new Error(
        `My graph can hold ${RUN_GRAPH_NODE_LIMIT} nodes; ${catalog.length - addedObjects} accessible objects could not be added.`,
      );
    }
    const node: RunGraphNode = {
      id: graphId("collection"),
      kind: "collection",
      ref: { objectApiName: object.apiName },
      label: object.pluralLabel || object.label || object.apiName,
    };
    nodes.push(node);
    collections.set(object.apiName, node);
    addedObjects += 1;
  }

  const positions = tidyPositions(nodes);
  const positionedNodes = nodes.map((node) =>
    node.layout ? node : { ...node, layout: positions[node.id] },
  );
  const relationships = new Map<string, RunGraphEdge>();
  for (const object of catalog) {
    const from = collections.get(object.apiName);
    const describe = describes.get(object.apiName);
    if (!from || !describe) continue;
    for (const field of describe.fields ?? []) {
      const targetApiName = field.referenceTo?.trim();
      const to = targetApiName ? collections.get(targetApiName) : undefined;
      if (!to || to.id === from.id) continue;
      const key = `${from.id}:${to.id}`;
      if (!relationships.has(key)) {
        relationships.set(key, {
          id: modelEdgeId(from.id, to.id),
          from: from.id,
          to: to.id,
          kind: "relates",
        });
      }
    }
  }

  const userEdges = document.edges.filter((edge) => !edge.id.startsWith(MODEL_EDGE_PREFIX));
  const modelEdges = [...relationships.values()];
  const next: RunGraphDocument = { ...document, nodes: positionedNodes, edges: [...userEdges, ...modelEdges] };
  const changed = JSON.stringify(next.nodes) !== JSON.stringify(document.nodes)
    || JSON.stringify(next.edges) !== JSON.stringify(document.edges);
  return { document: next, addedObjects, relationshipCount: modelEdges.length, changed };
}

