/** Resolve ToolSpec dataBindings through Client query (FLS + sharing enforced server-side). */

import { flattenRecordRow } from "../operate/queryAutocomplete";
import type { FetchFn } from "./tools";
import type { ToolDocument, ToolNode, ToolQueryBinding } from "./types";

const RECORD_DATA_KINDS = new Set([
  "stat",
  "recordTable",
  "recordCard",
  "relatedList",
  "queryResult",
  "pipelineLane",
  "messageThread",
  "mutationProposal",
]);

/** Props that must never be persisted into Metadata ToolSpecs (AuthZ snapshots). */
const BAKED_PROP_KEYS = [
  "rows",
  "fields",
  "recordIds",
  "cards",
  "messages",
  "recordId",
  "value",
  "operations",
] as const;

/** Clear baked record payloads so connected Tools never paint metadata snapshots. */
export function stripBakedRecordPayloads(node: ToolNode): ToolNode {
  const props = { ...node.props };
  let changed = false;
  for (const key of BAKED_PROP_KEYS) {
    if (key in props) {
      delete props[key];
      changed = true;
    }
  }
  if (node.kind === "stat" && RECORD_DATA_KINDS.has(node.kind)) {
    if (props.value !== null) {
      props.value = null;
      changed = true;
    }
  }
  if (!changed && !RECORD_DATA_KINDS.has(node.kind)) return node;
  if (node.kind === "stat") props.value = null;
  return { ...node, props };
}

/**
 * Durable Metadata shape: layout + bindings + chrome props only — no Client query snapshots.
 * Used by tool.saveAsSpec and Build ToolsPanel writes.
 */
export function sanitizeToolDocumentForMetadata(document: ToolDocument): ToolDocument {
  return {
    ...document,
    nodes: document.nodes.map(stripBakedRecordPayloads),
  };
}

/** Sanitize an unknown nodes JSON array (Build panel / raw payloads). */
export function sanitizeToolNodesForMetadata(nodes: unknown): unknown[] {
  if (!Array.isArray(nodes)) return [];
  return nodes.map((raw) => {
    if (!raw || typeof raw !== "object" || Array.isArray(raw)) return raw;
    const node = raw as ToolNode;
    if (typeof node.kind !== "string") return raw;
    return stripBakedRecordPayloads({
      id: typeof node.id === "string" ? node.id : "",
      kind: node.kind as ToolNode["kind"],
      title: node.title,
      bindingId: node.bindingId,
      props:
        node.props && typeof node.props === "object" && !Array.isArray(node.props) ? node.props : {},
    });
  });
}

export function applyBindingToNode(
  node: ToolNode,
  records: Record<string, unknown>[],
  objectApiName: string,
): ToolNode {
  if (node.kind === "stat") {
    return { ...node, props: { ...node.props, value: records.length } };
  }
  if (node.kind === "recordTable" || node.kind === "relatedList") {
    return { ...node, props: { ...node.props, rows: records, objectApiName } };
  }
  if (node.kind === "recordCard") {
    const first = records[0];
    if (!first) {
      return {
        ...node,
        props: { ...node.props, objectApiName, fields: {}, recordId: undefined },
      };
    }
    const recordId = String(first.id ?? first.Id ?? "");
    return {
      ...node,
      props: {
        ...node.props,
        objectApiName,
        recordId: recordId || undefined,
        fields: first,
      },
    };
  }
  if (node.kind === "queryResult") {
    const recordIds = records
      .map((r) => String(r.id ?? r.Id ?? ""))
      .filter(Boolean);
    return {
      ...node,
      props: { ...node.props, objectApiName, recordIds },
    };
  }
  if (node.kind === "pipelineLane") {
    const cards = records.map((r) => ({
      id: String(r.id ?? r.Id ?? ""),
      title: String(r.Name ?? r.name ?? r.Subject ?? r.id ?? r.Id ?? "Record"),
    }));
    return { ...node, props: { ...node.props, cards } };
  }
  return node;
}

export type ResolveBindingsResult = {
  document: ToolDocument;
  errors: string[];
};

async function queryBinding(
  fetchFn: FetchFn,
  binding: ToolQueryBinding,
): Promise<Record<string, unknown>[]> {
  const raw = (await fetchFn("/client/v1/query", {
    method: "POST",
    body: JSON.stringify({
      object: binding.objectApiName,
      select: binding.query.select,
      filters: binding.query.filters,
      sort: binding.query.sort,
      limit: binding.query.limit ?? 25,
    }),
  })) as { records?: Record<string, unknown>[] };
  return (raw.records ?? []).map(flattenRecordRow);
}

/**
 * AuthZ-honest Tool hydration for connected sessions:
 * 1. Strip baked rows/fields from record-bearing nodes
 * 2. Resolve each dataBinding via Client query (caller JWT)
 * 3. Apply results only to nodes that declare bindingId
 */
export async function resolveToolDocumentBindings(
  document: ToolDocument,
  fetchFn: FetchFn,
): Promise<ResolveBindingsResult> {
  const errors: string[] = [];
  let nodes = document.nodes.map(stripBakedRecordPayloads);

  const bindings = document.dataBindings ?? [];
  if (bindings.length === 0) {
    return {
      document: { ...document, nodes },
      errors,
    };
  }

  for (const binding of bindings) {
    try {
      const records = await queryBinding(fetchFn, binding);
      nodes = nodes.map((n) =>
        n.bindingId === binding.id ? applyBindingToNode(n, records, binding.objectApiName) : n,
      );
    } catch (e) {
      errors.push(`Binding ${binding.id} (${binding.objectApiName}): ${String(e)}`);
      // Leave stripped empty state — never fall back to baked metadata rows.
    }
  }

  return {
    document: {
      ...document,
      nodes,
      meta: { ...document.meta, updatedAt: new Date().toISOString() },
    },
    errors,
  };
}
