import { hasCapability } from "../scopes";
import type { RunGraphDocument, RunGraphNode } from "./graph/types";

export const KERNEL_USER_OBJECT = "User";

export function isKernelUserObject(apiName: string | undefined | null): boolean {
  return String(apiName ?? "").trim() === KERNEL_USER_OBJECT;
}

/** Operate may mount Users only when Client identity.users would succeed. */
export function canMountOperateUserCollection(opts: {
  systemPermissions?: string[];
  isAdmin?: boolean;
}): boolean {
  return hasCapability(opts.systemPermissions, "identity.users", {
    isAdmin: opts.isAdmin,
    failClosed: true,
  });
}

export function filterOperateCatalog<T extends { apiName: string }>(
  catalog: readonly T[],
  opts: { systemPermissions?: string[]; isAdmin?: boolean },
): T[] {
  if (canMountOperateUserCollection(opts)) return [...catalog];
  return catalog.filter((object) => !isKernelUserObject(object.apiName));
}

export function pruneInaccessibleUserCollections(
  document: RunGraphDocument,
  opts: { systemPermissions?: string[]; isAdmin?: boolean },
): RunGraphDocument {
  if (canMountOperateUserCollection(opts)) return document;
  const removed = new Set(
    document.nodes
      .filter((node) => node.kind === "collection" && isKernelUserObject(node.ref?.objectApiName))
      .map((node) => node.id),
  );
  if (removed.size === 0) return document;
  return {
    ...document,
    nodes: document.nodes.filter((node: RunGraphNode) => !removed.has(node.id)),
    edges: document.edges.filter((edge) => !removed.has(edge.from) && !removed.has(edge.to)),
  };
}

export function isCapabilityRequiredError(err: unknown): boolean {
  const message = err instanceof Error ? err.message : String(err ?? "");
  return /CAPABILITY_REQUIRED/i.test(message);
}
