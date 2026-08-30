import type { ToolDocument, ToolLayout, ToolNode, ToolQueryBinding } from "./types";

/** Stable IDE / future MCP tool names (ADR-021, BP-050 Phase 4). */
export const TOOL_BRIDGE_NAMES = [
  "tool.create",
  "tool.update",
  "tool.get",
  "tool.list",
  "tool.rerun",
  "tool.saveAsSpec",
] as const;

export type ToolBridgeName = (typeof TOOL_BRIDGE_NAMES)[number];

export function isToolBridgeName(value: string): value is ToolBridgeName {
  return (TOOL_BRIDGE_NAMES as readonly string[]).includes(value);
}

export type ToolCreateInput = {
  document: unknown;
  /** When true (default), caller may open the Tool pane after create. */
  openPane?: boolean;
  createdFromRunId?: string;
};

export type ToolCreateResult = {
  ok: true;
  toolId: string;
  title: string;
  document: ToolDocument;
};

export type ToolUpdateInput = {
  toolId: string;
  title?: string;
  layout?: ToolLayout;
  nodes?: ToolNode[];
  dataBindings?: ToolQueryBinding[];
  /** Shallow-merge into existing document fields when set. */
  patch?: {
    title?: string;
    layout?: Partial<ToolLayout>;
    nodes?: ToolNode[];
    dataBindings?: ToolQueryBinding[];
  };
};

export type ToolUpdateResult =
  | { ok: true; toolId: string; document: ToolDocument }
  | { ok: false; error: string };

export type ToolGetInput = { toolId: string };

export type ToolGetResult =
  | { ok: true; document: ToolDocument }
  | { ok: false; error: string };

export type ToolListResult = {
  ok: true;
  tools: Array<{ id: string; title: string; updatedAt?: string; toolSpecApiName?: string }>;
};

export type ToolRerunInput = { toolId: string };

export type ToolRerunResult =
  | { ok: true; toolId: string; document: ToolDocument }
  | { ok: false; error: string };

export type ToolSaveAsSpecInput = {
  toolId: string;
  apiName: string;
  label?: string;
  description?: string;
  icon?: string;
  sortOrder?: number;
};

export type ToolSaveAsSpecResult =
  | { ok: true; toolId: string; apiName: string; label: string }
  | { ok: false; error: string };

export type ToolBridgeError = {
  ok: false;
  error: string;
  issues?: Array<{ path: string; message: string }>;
};

export type ToolBridgeCall = {
  tool: ToolBridgeName | string;
  input?: unknown;
};
