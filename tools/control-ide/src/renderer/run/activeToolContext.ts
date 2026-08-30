import { loadToolStore } from "./store";
import type { ToolDocument, ToolNode, ToolQueryBinding } from "./types";

export type ActiveToolNodeSummary = {
  id: string;
  kind: string;
  title?: string;
};

export type ActiveToolContext = {
  toolId: string;
  title: string;
  toolSpecApiName?: string;
  dataBindings: ToolQueryBinding[];
  nodesSummary: ActiveToolNodeSummary[];
};

function nodeTitle(node: ToolNode): string | undefined {
  if (typeof node.title === "string" && node.title.trim()) return node.title;
  if (typeof node.props?.title === "string") return node.props.title;
  if (typeof node.props?.label === "string") return node.props.label;
  return undefined;
}

export function summarizeToolDocument(document: ToolDocument): ActiveToolContext {
  return {
    toolId: document.id,
    title: document.title,
    toolSpecApiName: document.toolSpecApiName,
    dataBindings: document.dataBindings ?? [],
    nodesSummary: document.nodes.map((n) => ({
      id: n.id,
      kind: n.kind,
      title: nodeTitle(n),
    })),
  };
}

export function buildActiveToolContext(
  toolId: string | null | undefined,
  storeEpoch?: number,
): ActiveToolContext | null {
  void storeEpoch;
  if (!toolId) return null;
  const doc = loadToolStore().documents.find((d) => d.id === toolId);
  if (!doc) return null;
  return summarizeToolDocument(doc);
}
