/** Metadata ToolSpec helpers for Run mode (ADR-021 / BP-050). */

import { loadToolStore } from "./store";
import { TOOL_DOCUMENT_API_VERSION, type ToolDocument } from "./types";
import { validateToolDocument } from "./validate";

export type FetchFn = (path: string, init?: RequestInit) => Promise<unknown>;

export type ToolSpecSummary = {
  apiName: string;
  label: string;
  description?: string;
  icon?: string;
  sortOrder?: number;
  active?: boolean;
  permissions?: ToolPermissions;
};

export type ToolPermissions = {
  canOpen: boolean;
  canInteract: boolean;
  canModify: boolean;
  canPublish: boolean;
};

export type ToolSpecPayload = ToolSpecSummary & {
  layout?: unknown;
  nodes?: unknown;
  dataBindings?: unknown;
  document?: unknown;
};

/** Stable rail / panel id for a live ToolSpec. */
export function toolRailId(apiName: string): string {
  return `tool:${apiName}`;
}

export function parseToolRailId(id: string): string | null {
  if (!id.startsWith("tool:")) return null;
  const apiName = id.slice("tool:".length).trim();
  return apiName || null;
}

/** Session working Tool id for the Run rail (agent tool.create). */
export function sessionToolRailId(toolId: string): string {
  return `session:${toolId}`;
}

export function parseSessionToolRailId(id: string): string | null {
  if (!id.startsWith("session:")) return null;
  const toolId = id.slice("session:".length).trim();
  return toolId || null;
}

export type SessionToolSummary = {
  id: string;
  title: string;
  updatedAt?: string;
};

export function listSessionTools(store = loadToolStore()): SessionToolSummary[] {
  return store.documents
    .slice()
    .sort((a, b) => (b.meta?.updatedAt ?? "").localeCompare(a.meta?.updatedAt ?? ""))
    .map((d) => ({
      id: d.id,
      title: d.title,
      updatedAt: d.meta?.updatedAt,
    }));
}

export async function listTools(fetchFn: FetchFn): Promise<ToolSpecSummary[]> {
  const row = (await fetchFn("/metadata/v1/tools")) as { tools?: ToolSpecSummary[] };
  const list = Array.isArray(row?.tools) ? row.tools : [];
  return list
    .filter((t) => t?.apiName && t.active !== false)
    .slice()
    .sort((a, b) => {
      const ao = a.sortOrder ?? 0;
      const bo = b.sortOrder ?? 0;
      if (ao !== bo) return ao - bo;
      return (a.label || a.apiName).localeCompare(b.label || b.apiName);
    });
}

/** Run-mode list: Client tools filtered by tool_permissions (fail-closed). */
export async function listClientTools(fetchFn: FetchFn): Promise<ToolSpecSummary[]> {
  const row = (await fetchFn("/client/v1/tools")) as { tools?: ToolSpecSummary[] };
  const list = Array.isArray(row?.tools) ? row.tools : [];
  return list
    .filter((t) => t?.apiName)
    .slice()
    .sort((a, b) => {
      const ao = a.sortOrder ?? 0;
      const bo = b.sortOrder ?? 0;
      if (ao !== bo) return ao - bo;
      return (a.label || a.apiName).localeCompare(b.label || b.apiName);
    });
}

export async function getTool(fetchFn: FetchFn, apiName: string): Promise<ToolSpecPayload> {
  return (await fetchFn(`/metadata/v1/tools/${encodeURIComponent(apiName)}`)) as ToolSpecPayload;
}

/** Run-mode get: AuthZ-filtered ToolSpec body under caller JWT. */
export async function getClientTool(fetchFn: FetchFn, apiName: string): Promise<ToolSpecPayload> {
  return (await fetchFn(`/client/v1/tools/${encodeURIComponent(apiName)}`)) as ToolSpecPayload;
}

/**
 * Build a ToolDocument from a Metadata ToolSpec payload.
 * Prefer nested `document`; otherwise synthesize from top-level layout/nodes.
 */
export function toolSpecToDocument(spec: ToolSpecPayload): ToolDocument | null {
  const nested = spec.document;
  if (nested && typeof nested === "object") {
    const doc = nested as Record<string, unknown>;
    const candidate = {
      ...doc,
      apiVersion: doc.apiVersion ?? TOOL_DOCUMENT_API_VERSION,
      id: typeof doc.id === "string" && doc.id.trim() ? doc.id : spec.apiName,
      title:
        typeof doc.title === "string" && doc.title.trim()
          ? doc.title
          : spec.label || spec.apiName,
      toolSpecApiName: spec.apiName,
    };
    const result = validateToolDocument(candidate);
    return result.ok ? result.document : null;
  }

  const candidate = {
    apiVersion: TOOL_DOCUMENT_API_VERSION,
    id: spec.apiName,
    title: spec.label || spec.apiName,
    toolSpecApiName: spec.apiName,
    layout: spec.layout ?? { mode: "sections", sections: [] },
    nodes: Array.isArray(spec.nodes) ? spec.nodes : [],
    dataBindings: Array.isArray(spec.dataBindings) ? spec.dataBindings : [],
  };
  const result = validateToolDocument(candidate);
  return result.ok ? result.document : null;
}
