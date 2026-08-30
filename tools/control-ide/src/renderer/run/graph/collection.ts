import type { QueryFilter } from "../../operate/types";
import type { RunGraphBinding, RunGraphDocument, RunGraphNode } from "./types";

export const DEFAULT_COLLECTION_OBJECTS = ["Account", "Contact", "Opportunity"] as const;

export function collectionIdentity(input: {
  objectApiName: string;
  bindingId?: string;
  searchQ?: string;
}): string {
  return `${input.objectApiName}::${input.bindingId ?? ""}::${input.searchQ ?? ""}`;
}

export function collectionNodeIdentity(node: RunGraphNode): string | null {
  if (node.kind !== "collection" || !node.ref?.objectApiName) return null;
  return collectionIdentity({
    objectApiName: node.ref.objectApiName,
    bindingId: node.bindingId,
    searchQ: node.searchQ,
  });
}

export function findCollectionNode(
  document: RunGraphDocument,
  input: { objectApiName: string; bindingId?: string; searchQ?: string },
): RunGraphNode | undefined {
  const identity = collectionIdentity(input);
  return document.nodes.find((node) => collectionNodeIdentity(node) === identity);
}

export function bindingFiltersAsQuery(binding?: RunGraphBinding): QueryFilter[] {
  if (!Array.isArray(binding?.filters)) return [];
  const allowed = new Set<QueryFilter["op"]>([
    "eq",
    "ne",
    "gt",
    "gte",
    "lt",
    "lte",
    "like",
    "in",
    "is_null",
    "is_not_null",
  ]);
  return binding.filters.flatMap((item) => {
    if (!item || typeof item !== "object" || Array.isArray(item)) return [];
    const row = item as Record<string, unknown>;
    if (typeof row.field !== "string" || !row.field.trim()) return [];
    if (typeof row.op !== "string" || !allowed.has(row.op as QueryFilter["op"])) return [];
    return [{ field: row.field, op: row.op as QueryFilter["op"], value: row.value }];
  });
}

export function collectionListFilters(
  node: RunGraphNode,
  binding?: RunGraphBinding,
): QueryFilter[] {
  const search = node.searchQ?.trim();
  const searchFilters: QueryFilter[] = search ? [{ field: "Name", op: "like", value: search }] : [];
  return [...searchFilters, ...bindingFiltersAsQuery(binding)];
}

export function nextCollectionLayout(document: RunGraphDocument): { x: number; y: number } {
  const count = document.nodes.filter((node) => node.kind === "collection").length;
  return { x: 48 + (count % 4) * 250, y: 48 + Math.floor(count / 4) * 170 };
}
