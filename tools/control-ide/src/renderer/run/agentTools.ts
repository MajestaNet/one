import type { FetchFn } from "../agents/runs";
import { resolveToolDocumentBindings, sanitizeToolDocumentForMetadata } from "./resolveBindings";
import {
  loadToolStore,
  type ToolStoreSnapshot,
  upsertToolDocument,
} from "./store";
import type { ToolDocument } from "./types";
import { validateToolDocument } from "./validate";
import type {
  ToolBridgeCall,
  ToolBridgeError,
  ToolBridgeName,
  ToolCreateInput,
  ToolCreateResult,
  ToolGetInput,
  ToolGetResult,
  ToolListResult,
  ToolRerunInput,
  ToolRerunResult,
  ToolSaveAsSpecInput,
  ToolSaveAsSpecResult,
  ToolUpdateInput,
  ToolUpdateResult,
} from "./toolContracts";
import { isToolBridgeName } from "./toolContracts";

export type ToolBridgeContext = {
  store?: ToolStoreSnapshot;
  fetch?: FetchFn;
};

function snapshot(ctx: ToolBridgeContext): ToolStoreSnapshot {
  return ctx.store ?? loadToolStore();
}

function withStore(ctx: ToolBridgeContext, next: ToolStoreSnapshot): ToolStoreSnapshot {
  ctx.store = next;
  return next;
}

function err(message: string, issues?: ToolBridgeError["issues"]): ToolBridgeError {
  return { ok: false, error: message, issues };
}

export function toolCreate(
  input: ToolCreateInput,
  ctx: ToolBridgeContext = {},
): ToolCreateResult | ToolBridgeError {
  const result = validateToolDocument(input.document);
  if (!result.ok) return err("Invalid ToolDocument", result.issues);
  const document: ToolDocument = {
    ...result.document,
    meta: {
      ...result.document.meta,
      createdFromRunId: input.createdFromRunId ?? result.document.meta?.createdFromRunId,
      updatedAt: new Date().toISOString(),
    },
  };
  const next = upsertToolDocument(snapshot(ctx), document);
  withStore(ctx, next);
  return { ok: true, toolId: document.id, title: document.title, document };
}

export function toolUpdate(
  input: ToolUpdateInput,
  ctx: ToolBridgeContext = {},
): ToolUpdateResult | ToolBridgeError {
  const store = snapshot(ctx);
  const existing = store.documents.find((d) => d.id === input.toolId);
  if (!existing) return err(`Tool not found: ${input.toolId}`);

  const document: ToolDocument = { ...existing };
  if (input.title) document.title = input.title;
  if (input.layout) document.layout = input.layout;
  if (input.nodes) document.nodes = input.nodes;
  if (input.dataBindings) document.dataBindings = input.dataBindings;

  if (input.patch) {
    if (input.patch.title) document.title = input.patch.title;
    if (input.patch.layout) document.layout = { ...document.layout, ...input.patch.layout };
    if (input.patch.dataBindings) {
      const byId = new Map((document.dataBindings ?? []).map((b) => [b.id, { ...b }]));
      for (const binding of input.patch.dataBindings) {
        const prev = byId.get(binding.id);
        if (prev) {
          byId.set(binding.id, {
            ...prev,
            ...binding,
            query: binding.query
              ? { ...(prev.query ?? {}), ...binding.query }
              : prev.query,
          });
        } else {
          byId.set(binding.id, binding);
        }
      }
      document.dataBindings = Array.from(byId.values());
    }
    if (input.patch.nodes) {
      const byId = new Map(document.nodes.map((n) => [n.id, n]));
      for (const node of input.patch.nodes) {
        byId.set(node.id, node);
      }
      document.nodes = Array.from(byId.values());
    }
  }

  const validated = validateToolDocument(document);
  if (!validated.ok) return err("Invalid ToolDocument after update", validated.issues);
  const next = upsertToolDocument(store, validated.document);
  withStore(ctx, next);
  return { ok: true, toolId: validated.document.id, document: validated.document };
}

export function toolGet(input: ToolGetInput, ctx: ToolBridgeContext = {}): ToolGetResult {
  const doc = snapshot(ctx).documents.find((d) => d.id === input.toolId);
  if (!doc) return { ok: false, error: `Tool not found: ${input.toolId}` };
  return { ok: true, document: doc };
}

export function toolList(ctx: ToolBridgeContext = {}): ToolListResult {
  const docs = snapshot(ctx).documents;
  return {
    ok: true,
    tools: docs.map((d) => ({
      id: d.id,
      title: d.title,
      updatedAt: d.meta?.updatedAt,
      toolSpecApiName: d.toolSpecApiName,
    })),
  };
}

export async function toolRerun(
  input: ToolRerunInput,
  ctx: ToolBridgeContext,
): Promise<ToolRerunResult | ToolBridgeError> {
  const got = toolGet(input, ctx);
  if (!got.ok) return got;
  if (!ctx.fetch) return err("tool.rerun requires an active API session");
  if (!got.document.dataBindings?.length) {
    return { ok: true, toolId: got.document.id, document: got.document };
  }

  const { document, errors } = await resolveToolDocumentBindings(got.document, ctx.fetch);
  if (errors.length) {
    return err(errors[0]);
  }
  const validated = validateToolDocument(document);
  if (!validated.ok) return err("Invalid ToolDocument after rerun", validated.issues);
  const next = upsertToolDocument(snapshot(ctx), validated.document);
  withStore(ctx, next);
  return { ok: true, toolId: validated.document.id, document: validated.document };
}

export async function toolSaveAsSpec(
  input: ToolSaveAsSpecInput,
  ctx: ToolBridgeContext,
): Promise<ToolSaveAsSpecResult | ToolBridgeError> {
  const got = toolGet({ toolId: input.toolId }, ctx);
  if (!got.ok) return got;
  if (!ctx.fetch) return err("tool.saveAsSpec requires metadata scope and ide.build.tools");

  const apiName = input.apiName.trim();
  if (!apiName) return err("apiName is required for tool.saveAsSpec");
  const label = input.label?.trim() || got.document.title;
  const durable = sanitizeToolDocumentForMetadata(got.document);

  try {
    await ctx.fetch("/metadata/v1/tools", {
      method: "POST",
      body: JSON.stringify({
        apiName,
        label,
        description: input.description,
        icon: input.icon,
        sortOrder: input.sortOrder,
        layout: durable.layout,
        nodes: durable.nodes,
        dataBindings: durable.dataBindings ?? [],
      }),
    });
  } catch (e) {
    return err(`Failed to save ToolSpec: ${String(e)}`);
  }

  const linked: ToolDocument = {
    ...got.document,
    toolSpecApiName: apiName,
    title: label,
    meta: { ...got.document.meta, updatedAt: new Date().toISOString() },
  };
  const next = upsertToolDocument(snapshot(ctx), linked);
  withStore(ctx, next);
  return { ok: true, toolId: linked.id, apiName, label };
}

export async function executeToolBridge(
  tool: ToolBridgeName,
  input: unknown,
  ctx: ToolBridgeContext,
): Promise<unknown> {
  switch (tool) {
    case "tool.create":
      return toolCreate((input ?? {}) as ToolCreateInput, ctx);
    case "tool.update":
      return toolUpdate((input ?? {}) as ToolUpdateInput, ctx);
    case "tool.get":
      return toolGet((input ?? {}) as ToolGetInput, ctx);
    case "tool.list":
      return toolList(ctx);
    case "tool.rerun":
      return toolRerun((input ?? {}) as ToolRerunInput, ctx);
    case "tool.saveAsSpec":
      return toolSaveAsSpec((input ?? {}) as ToolSaveAsSpecInput, ctx);
    default:
      return err(`Unknown tool bridge: ${tool}`);
  }
}

export async function executeToolBridgeCalls(
  calls: ToolBridgeCall[],
  ctx: ToolBridgeContext,
): Promise<Array<{ tool: string; result: unknown }>> {
  const results: Array<{ tool: string; result: unknown }> = [];
  for (const call of calls) {
    if (!isToolBridgeName(call.tool)) {
      results.push({ tool: call.tool, result: err(`Unknown tool bridge: ${call.tool}`) });
      continue;
    }
    results.push({ tool: call.tool, result: await executeToolBridge(call.tool, call.input, ctx) });
  }
  return results;
}

function asRecord(v: unknown): Record<string, unknown> | null {
  return v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : null;
}

function parseToolCalls(raw: unknown): ToolBridgeCall[] {
  if (!Array.isArray(raw)) return [];
  const out: ToolBridgeCall[] = [];
  for (const item of raw) {
    const row = asRecord(item);
    if (!row || typeof row.tool !== "string") continue;
    out.push({ tool: row.tool, input: row.input ?? row.arguments ?? row.params });
  }
  return out;
}

export type ToolRunEffects = {
  store: ToolStoreSnapshot;
  toolId?: string;
  toolTitle?: string;
  toolSpecApiName?: string;
  toolResults?: Array<{ tool: string; result: unknown }>;
  graphChanged?: boolean;
  graphResults?: Array<{ tool: string; result: unknown }>;
};

/** Apply explicit Tool payloads / tool calls from an agent run output (IDE bridge). */
export async function applyToolEffectsFromRunOutput(
  output: Record<string, unknown> | null | undefined,
  ctx: ToolBridgeContext,
): Promise<ToolRunEffects> {
  const toolCtx: ToolBridgeContext = { ...ctx, store: ctx.store ?? loadToolStore() };
  let toolId: string | undefined;
  let toolTitle: string | undefined;
  let toolSpecApiName: string | undefined;
  let toolResults: Array<{ tool: string; result: unknown }> | undefined;

  const handoff = asRecord(output?.toolHandoff) ?? asRecord(output?.handoff);
  if (typeof handoff?.toolId === "string") {
    toolId = handoff.toolId;
    toolTitle = typeof handoff.toolTitle === "string" ? handoff.toolTitle : undefined;
    toolSpecApiName =
      typeof handoff.toolSpecApiName === "string" ? handoff.toolSpecApiName : undefined;
  }

  const docRaw = output?.toolDocument ?? output?.document ?? output?.canvasDocument;
  if (docRaw) {
    const created = toolCreate(
      {
        document: docRaw,
        createdFromRunId: typeof output?.runId === "string" ? output.runId : undefined,
      },
      toolCtx,
    );
    if (created.ok) {
      toolId = created.toolId;
      toolTitle = created.title;
      toolResults = [{ tool: "tool.create", result: created }];
    }
  }

  const calls = parseToolCalls(
    output?.toolCalls ?? output?.toolBridgeCalls ?? output?.canvasToolCalls,
  ).filter((call) => !call.tool.startsWith("graph."));
  if (calls.length) {
    const executed = await executeToolBridgeCalls(calls, toolCtx);
    toolResults = [...(toolResults ?? []), ...executed];
    for (const row of executed) {
      const result = row.result as {
        ok?: boolean;
        toolId?: string;
        title?: string;
        apiName?: string;
        document?: ToolDocument;
      };
      if (result?.ok && result.toolId) {
        toolId = result.toolId;
        toolTitle = result.title ?? result.document?.title ?? toolTitle;
        if (typeof result.apiName === "string") toolSpecApiName = result.apiName;
      }
    }
  }

  return {
    store: toolCtx.store ?? loadToolStore(),
    toolId,
    toolTitle,
    toolSpecApiName,
    toolResults,
  };
}
