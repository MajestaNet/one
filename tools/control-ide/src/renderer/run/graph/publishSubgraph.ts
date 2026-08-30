import { sanitizeToolDocumentForMetadata } from "../resolveBindings";
import { TOOL_DOCUMENT_API_VERSION, type ToolDocument, type ToolNode, type ToolQueryBinding } from "../types";
import { getClientTool } from "../tools";
import type { RunGraphFetch } from "./api";
import type { RunGraphDocument, RunGraphNode } from "./types";

function nodeTitle(node: RunGraphNode): string {
  return node.label || node.text || node.toolRef?.toolSpecApiName || node.toolRef?.workingToolId || node.kind;
}

/** Translate a selected graph subset into reference/query-only ToolDocument chrome. */
export function graphSubgraphToToolDocument(
  graph: RunGraphDocument,
  nodeIds: string[],
  apiName: string,
  label: string,
): ToolDocument {
  const selected = new Set(nodeIds);
  if (!selected.size) throw new Error("nodeIds must select at least one graph node");
  const graphNodes = graph.nodes.filter((node) => selected.has(node.id));
  if (graphNodes.length !== selected.size) throw new Error("nodeIds contains an unknown graph node");

  const nodes: ToolNode[] = [];
  const dataBindings: ToolQueryBinding[] = [];
  const positions: NonNullable<ToolDocument["layout"]["positions"]> = {};
  for (const node of graphNodes) {
    const id = `graph-${node.id}`;
    if (node.layout) positions[id] = { x: node.layout.x, y: node.layout.y, w: node.layout.w };
    if (node.kind === "record" && node.ref?.objectApiName && node.ref.recordId) {
      const bindingId = `record-${node.id}`;
      dataBindings.push({
        id: bindingId,
        objectApiName: node.ref.objectApiName,
        query: { filters: [{ field: "Id", op: "eq", value: node.ref.recordId }], limit: 1 },
      });
      nodes.push({
        id,
        kind: "recordCard",
        title: node.ref.objectApiName,
        bindingId,
        props: { objectApiName: node.ref.objectApiName },
      });
      continue;
    }
    if (node.kind === "signal") {
      const binding = graph.dataBindings?.find((candidate) => candidate.id === node.bindingId);
      if (binding) {
        dataBindings.push({
          id: binding.id,
          objectApiName: binding.objectApiName,
          query: {
            select: binding.fields,
            filters: binding.filters,
            sort: binding.sort,
            limit: binding.limit,
          },
        });
        nodes.push({ id, kind: "queryResult", title: nodeTitle(node), bindingId: binding.id, props: { objectApiName: binding.objectApiName } });
        continue;
      }
    }
    nodes.push(node.kind === "cluster"
      ? { id, kind: "sectionHeader", title: nodeTitle(node), props: { subtitle: "Published graph cluster" } }
      : { id, kind: "markdownNote", title: node.kind, props: { text: nodeTitle(node) } });
  }

  return sanitizeToolDocumentForMetadata({
    apiVersion: TOOL_DOCUMENT_API_VERSION,
    id: apiName,
    title: label,
    toolSpecApiName: apiName,
    layout: { mode: "spatial", positions },
    nodes,
    dataBindings,
  });
}

export async function publishRunGraphSubgraph(
  fetchFn: RunGraphFetch,
  graph: RunGraphDocument,
  nodeIds: string[],
  apiName: string,
  label: string,
): Promise<{ apiName: string; nodeCount: number }> {
  if (!/^[A-Za-z][A-Za-z0-9_]*$/.test(apiName)) {
    throw new Error("apiName must start with a letter and contain only letters, numbers, or underscore");
  }
  const selected = new Set(nodeIds);
  const mountedSpecs = graph.nodes.flatMap((node) =>
    selected.has(node.id) && node.kind === "tool" && node.toolRef?.toolSpecApiName
      ? [node.toolRef.toolSpecApiName]
      : [],
  );
  for (const mountedApiName of new Set(mountedSpecs)) {
    const source = await getClientTool(fetchFn, mountedApiName);
    if (!source.permissions?.canPublish) {
      throw new Error(`can_publish is required to publish mounted Tool ${mountedApiName}`);
    }
  }
  const document = graphSubgraphToToolDocument(graph, nodeIds, apiName, label);
  await fetchFn("/metadata/v1/tools", {
    method: "POST",
    body: JSON.stringify({
      apiName,
      label,
      description: `Published from ${nodeIds.length} node${nodeIds.length === 1 ? "" : "s"} in My graph`,
      icon: "graph",
      layout: document.layout,
      nodes: document.nodes,
      dataBindings: document.dataBindings ?? [],
    }),
  });
  return { apiName, nodeCount: document.nodes.length };
}
