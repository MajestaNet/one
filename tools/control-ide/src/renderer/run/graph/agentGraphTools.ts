import { getRunGraph, putRunGraph, resolveRunGraphCards, type RunGraphFetch } from "./api";
import {
  isGraphBridgeName,
  type GraphAnnotateInput,
  type GraphBridgeCall,
  type GraphBridgeError,
  type GraphBridgeName,
  type GraphClusterInput,
  type GraphCompactInput,
  type GraphGetInput,
  type GraphLayoutInput,
  type GraphLinkInput,
  type GraphMountToolInput,
  type GraphPinCollectionInput,
  type GraphPinInput,
  type GraphPublishSubgraphInput,
  type GraphUnlinkInput,
} from "./agentGraphContracts";
import { findCollectionNode } from "./collection";
import { nextBandPosition } from "./layoutBands";
import { publishRunGraphSubgraph } from "./publishSubgraph";
import {
  isRunGraphEdgeKind,
  type RunGraphDocument,
  type RunGraphEdge,
  type RunGraphNode,
} from "./types";

export type GraphBridgeContext = { fetch: RunGraphFetch };
export type GraphBridgeResult = Record<string, unknown> & { ok: boolean };

const MAX_ANNOTATION_TEXT = 4096;

function err(message: string): GraphBridgeError {
  return { ok: false, error: message };
}

function asObject(value: unknown, label: string): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error(`${label} must be an object`);
  }
  return value as Record<string, unknown>;
}

function exactObject(
  value: unknown,
  label: string,
  allowed: readonly string[],
): Record<string, unknown> {
  const row = asObject(value ?? {}, label);
  const allowedSet = new Set(allowed);
  const extra = Object.keys(row).find((key) => !allowedSet.has(key));
  if (extra) throw new Error(`${label}.${extra} is not allowed`);
  return row;
}

function requiredString(value: unknown, label: string): string {
  if (typeof value !== "string" || !value.trim()) throw new Error(`${label} is required`);
  return value.trim();
}

function optionalString(value: unknown, label: string): string | undefined {
  if (value === undefined) return undefined;
  return requiredString(value, label);
}

function stringArray(value: unknown, label: string): string[] | undefined {
  if (value === undefined) return undefined;
  if (!Array.isArray(value)) throw new Error(`${label} must be a string array`);
  return value.map((item, index) => requiredString(item, `${label}[${index}]`));
}

function nextID(prefix: string): string {
  return `${prefix}-${globalThis.crypto.randomUUID()}`;
}

function graphKey(input: Record<string, unknown>): string {
  return optionalString(input.graphKey, "graphKey") ?? "home";
}

function requireNode(document: RunGraphDocument, nodeID: string, label = "nodeId") {
  if (!document.nodes.some((node) => node.id === nodeID)) {
    throw new Error(`${label} does not reference a graph node: ${nodeID}`);
  }
}

async function save(
  ctx: GraphBridgeContext,
  key: string,
  document: RunGraphDocument,
  expectedRevision: number,
): Promise<number> {
  return (await putRunGraph(ctx.fetch, key, document, expectedRevision)).revision;
}

function parseGet(input: unknown): GraphGetInput {
  const row = exactObject(input, "graph.get input", ["graphKey"]);
  return { graphKey: optionalString(row.graphKey, "graphKey") };
}

function parsePin(input: unknown): GraphPinInput {
  const row = exactObject(input, "graph.pin input", ["objectApiName", "recordId", "clusterId", "collectionId"]);
  return {
    objectApiName: requiredString(row.objectApiName, "objectApiName"),
    recordId: requiredString(row.recordId, "recordId"),
    clusterId: optionalString(row.clusterId, "clusterId"),
    collectionId: optionalString(row.collectionId, "collectionId"),
  };
}

function parsePinCollection(input: unknown): GraphPinCollectionInput {
  const row = exactObject(input, "graph.pinCollection input", ["objectApiName", "bindingId", "searchQ", "label"]);
  return {
    objectApiName: requiredString(row.objectApiName, "objectApiName"),
    bindingId: optionalString(row.bindingId, "bindingId"),
    searchQ: optionalString(row.searchQ, "searchQ"),
    label: optionalString(row.label, "label"),
  };
}

function parseCluster(input: unknown): GraphClusterInput {
  const row = exactObject(input, "graph.cluster input", ["label", "nodeIds"]);
  return {
    label: requiredString(row.label, "label"),
    nodeIds: stringArray(row.nodeIds, "nodeIds"),
  };
}

function parseMountTool(input: unknown): GraphMountToolInput {
  const row = exactObject(input, "graph.mountTool input", ["toolSpecApiName", "workingToolId"]);
  const toolSpecApiName = optionalString(row.toolSpecApiName, "toolSpecApiName");
  const workingToolId = optionalString(row.workingToolId, "workingToolId");
  if (Boolean(toolSpecApiName) === Boolean(workingToolId)) {
    throw new Error("graph.mountTool requires exactly one Tool reference");
  }
  return { toolSpecApiName, workingToolId };
}

function parseLink(input: unknown): GraphLinkInput {
  const row = exactObject(input, "graph.link input", ["from", "to", "kind"]);
  if (!isRunGraphEdgeKind(row.kind)) throw new Error("kind must be an allowlisted graph edge kind");
  return {
    from: requiredString(row.from, "from"),
    to: requiredString(row.to, "to"),
    kind: row.kind,
  };
}

function parseUnlink(input: unknown): GraphUnlinkInput {
  const row = exactObject(input, "graph.unlink input", ["edgeId"]);
  return { edgeId: requiredString(row.edgeId, "edgeId") };
}

function parseAnnotate(input: unknown): GraphAnnotateInput {
  const row = exactObject(input, "graph.annotate input", ["text", "kind", "linkToNodeId"]);
  const text = requiredString(row.text, "text");
  if (text.length > MAX_ANNOTATION_TEXT) throw new Error("text exceeds 4096 characters");
  if (row.kind !== "insight" && row.kind !== "question") {
    throw new Error("kind must be insight or question");
  }
  return {
    text,
    kind: row.kind,
    linkToNodeId: optionalString(row.linkToNodeId, "linkToNodeId"),
  };
}

function parseLayout(input: unknown): GraphLayoutInput {
  const row = exactObject(input, "graph.layout input", ["positions"]);
  const positionsRaw = asObject(row.positions, "positions");
  const positions: GraphLayoutInput["positions"] = {};
  for (const [nodeID, raw] of Object.entries(positionsRaw)) {
    requiredString(nodeID, "positions nodeId");
    const position = exactObject(raw, `positions.${nodeID}`, ["x", "y"]);
    if (typeof position.x !== "number" || !Number.isFinite(position.x)) {
      throw new Error(`positions.${nodeID}.x must be a finite number`);
    }
    if (typeof position.y !== "number" || !Number.isFinite(position.y)) {
      throw new Error(`positions.${nodeID}.y must be a finite number`);
    }
    positions[nodeID] = { x: position.x, y: position.y };
  }
  return { positions };
}

function parsePublishSubgraph(input: unknown): GraphPublishSubgraphInput {
  const row = exactObject(input, "graph.publishSubgraph input", ["nodeIds", "apiName", "label"]);
  return {
    nodeIds: stringArray(row.nodeIds, "nodeIds") ?? [],
    apiName: requiredString(row.apiName, "apiName"),
    label: requiredString(row.label, "label"),
  };
}

function parseCompact(input: unknown): GraphCompactInput {
  const row = exactObject(input, "graph.compact input", ["strategy"]);
  if (row.strategy !== "demote-stale") throw new Error("strategy must be demote-stale");
  return { strategy: row.strategy };
}

async function executeGraphMutation(
  tool: Exclude<GraphBridgeName, "graph.get" | "graph.publishSubgraph">,
  input: unknown,
  ctx: GraphBridgeContext,
): Promise<GraphBridgeResult> {
  const key = "home";
  const envelope = await getRunGraph(ctx.fetch, key);
  const expectedRevision = envelope.revision;
  const document: RunGraphDocument = {
    ...envelope.document,
    nodes: [...envelope.document.nodes],
    edges: [...envelope.document.edges],
  };

  switch (tool) {
    case "graph.pin": {
      const parsed = parsePin(input);
      const existing = document.nodes.find(
        (node) =>
          node.kind === "record" &&
          node.ref?.objectApiName === parsed.objectApiName &&
          node.ref.recordId === parsed.recordId,
      );
      const node: RunGraphNode =
        existing ?? {
          id: nextID("record"),
          kind: "record",
          ref: { objectApiName: parsed.objectApiName, recordId: parsed.recordId },
          layout: nextBandPosition(document, "record"),
        };
      if (!existing) document.nodes.push(node);
      if (parsed.clusterId) {
        const cluster = document.nodes.find(
          (candidate) => candidate.id === parsed.clusterId && candidate.kind === "cluster",
        );
        if (!cluster) throw new Error(`clusterId does not reference a cluster: ${parsed.clusterId}`);
        if (!document.edges.some((edge) => edge.from === cluster.id && edge.to === node.id && edge.kind === "owns")) {
          document.edges.push({ id: nextID("edge"), from: cluster.id, to: node.id, kind: "owns" });
        }
      }
      if (parsed.collectionId) {
        const collection = document.nodes.find(
          (candidate) => candidate.id === parsed.collectionId && candidate.kind === "collection",
        );
        if (!collection) throw new Error(`collectionId does not reference a collection: ${parsed.collectionId}`);
        if (!document.edges.some((edge) => edge.from === node.id && edge.to === collection.id && edge.kind === "derivedFrom")) {
          document.edges.push({ id: nextID("edge"), from: node.id, to: collection.id, kind: "derivedFrom" });
        }
      }
      return { ok: true, graphKey: key, nodeId: node.id, revision: await save(ctx, key, document, expectedRevision) };
    }
    case "graph.pinCollection": {
      const parsed = parsePinCollection(input);
      if (parsed.bindingId) {
        const binding = document.dataBindings?.find((candidate) => candidate.id === parsed.bindingId);
        if (!binding) throw new Error(`bindingId does not reference a dataBinding: ${parsed.bindingId}`);
        if (binding.objectApiName !== parsed.objectApiName) {
          throw new Error(`bindingId object ${binding.objectApiName} does not match collection ${parsed.objectApiName}`);
        }
      }
      const existing = findCollectionNode(document, parsed);
      const node: RunGraphNode =
        existing ?? {
          id: nextID("collection"),
          kind: "collection",
          ref: { objectApiName: parsed.objectApiName },
          bindingId: parsed.bindingId,
          searchQ: parsed.searchQ,
          label: parsed.label || parsed.objectApiName,
          layout: nextBandPosition(document, "collection"),
        };
      if (!existing) document.nodes.push(node);
      return { ok: true, graphKey: key, nodeId: node.id, revision: await save(ctx, key, document, expectedRevision) };
    }
    case "graph.cluster": {
      const parsed = parseCluster(input);
      for (const nodeID of parsed.nodeIds ?? []) requireNode(document, nodeID);
      const node: RunGraphNode = {
        id: nextID("cluster"),
        kind: "cluster",
        label: parsed.label,
        layout: nextBandPosition(document, "cluster"),
      };
      document.nodes.push(node);
      for (const nodeID of new Set(parsed.nodeIds ?? [])) {
        document.edges.push({ id: nextID("edge"), from: node.id, to: nodeID, kind: "owns" });
      }
      return { ok: true, graphKey: key, nodeId: node.id, revision: await save(ctx, key, document, expectedRevision) };
    }
    case "graph.mountTool": {
      const parsed = parseMountTool(input);
      const existing = document.nodes.find(
        (node) =>
          node.kind === "tool" &&
          node.toolRef?.toolSpecApiName === parsed.toolSpecApiName &&
          node.toolRef?.workingToolId === parsed.workingToolId,
      );
      const node: RunGraphNode =
        existing ?? {
          id: nextID("tool"),
          kind: "tool",
          toolRef: parsed,
          layout: nextBandPosition(document, "tool"),
        };
      if (!existing) document.nodes.push(node);
      return { ok: true, graphKey: key, nodeId: node.id, revision: await save(ctx, key, document, expectedRevision) };
    }
    case "graph.link": {
      const parsed = parseLink(input);
      requireNode(document, parsed.from, "from");
      requireNode(document, parsed.to, "to");
      const existing = document.edges.find(
        (edge) => edge.from === parsed.from && edge.to === parsed.to && edge.kind === parsed.kind,
      );
      const edge: RunGraphEdge =
        existing ?? { id: nextID("edge"), from: parsed.from, to: parsed.to, kind: parsed.kind };
      if (!existing) document.edges.push(edge);
      return { ok: true, graphKey: key, edgeId: edge.id, revision: await save(ctx, key, document, expectedRevision) };
    }
    case "graph.unlink": {
      const parsed = parseUnlink(input);
      const index = document.edges.findIndex((edge) => edge.id === parsed.edgeId);
      if (index < 0) throw new Error(`edgeId does not reference a graph edge: ${parsed.edgeId}`);
      document.edges.splice(index, 1);
      return { ok: true, graphKey: key, edgeId: parsed.edgeId, revision: await save(ctx, key, document, expectedRevision) };
    }
    case "graph.annotate": {
      const parsed = parseAnnotate(input);
      if (parsed.linkToNodeId) requireNode(document, parsed.linkToNodeId, "linkToNodeId");
      const node: RunGraphNode = {
        id: nextID(parsed.kind),
        kind: parsed.kind,
        text: parsed.text,
        layout: nextBandPosition(document, parsed.kind),
      };
      document.nodes.push(node);
      if (parsed.linkToNodeId) {
        document.edges.push({
          id: nextID("edge"),
          from: node.id,
          to: parsed.linkToNodeId,
          kind: "explains",
        });
      }
      return { ok: true, graphKey: key, nodeId: node.id, revision: await save(ctx, key, document, expectedRevision) };
    }
    case "graph.layout": {
      const parsed = parseLayout(input);
      for (const nodeID of Object.keys(parsed.positions)) requireNode(document, nodeID);
      document.nodes = document.nodes.map((node) => {
        const position = parsed.positions[node.id];
        return position ? { ...node, layout: { ...node.layout, ...position } } : node;
      });
      return {
        ok: true,
        graphKey: key,
        positioned: Object.keys(parsed.positions).length,
        revision: await save(ctx, key, document, expectedRevision),
      };
    }
    case "graph.compact": {
      parseCompact(input);
      const refs = document.nodes.flatMap((node) =>
        node.kind === "record" && node.ref?.objectApiName && node.ref.recordId
          ? [{ nodeId: node.id, objectApiName: node.ref.objectApiName, recordId: node.ref.recordId }]
          : [],
      );
      const resolved = await resolveRunGraphCards(ctx.fetch, refs);
      const stale = new Set(
        resolved.filter((result) => !result.ok && (result.code === "NOT_FOUND" || result.code === "FORBIDDEN"))
          .map((result) => result.nodeId),
      );
      if (!stale.size) return { ok: true, graphKey: key, removed: 0, revision: expectedRevision };
      document.nodes = document.nodes.filter((node) => !stale.has(node.id));
      document.edges = document.edges.filter((edge) => !stale.has(edge.from) && !stale.has(edge.to));
      return {
        ok: true,
        graphKey: key,
        removed: stale.size,
        revision: await save(ctx, key, document, expectedRevision),
      };
    }
  }
}

export async function executeGraphBridge(
  tool: GraphBridgeName,
  input: unknown,
  ctx: GraphBridgeContext,
): Promise<GraphBridgeResult | GraphBridgeError> {
  try {
    if (tool === "graph.get") {
      const parsed = parseGet(input);
      const envelope = await getRunGraph(ctx.fetch, graphKey(parsed as Record<string, unknown>));
      return { ok: true, graphKey: envelope.graphKey, revision: envelope.revision, document: envelope.document };
    }
    if (tool === "graph.publishSubgraph") {
      const parsed = parsePublishSubgraph(input);
      const envelope = await getRunGraph(ctx.fetch, "home");
      const published = await publishRunGraphSubgraph(
        ctx.fetch, envelope.document, parsed.nodeIds, parsed.apiName, parsed.label,
      );
      return { ok: true, graphKey: "home", ...published };
    }
    return await executeGraphMutation(tool, input, ctx);
  } catch (reason) {
    return err(reason instanceof Error ? reason.message : String(reason));
  }
}

export async function executeGraphBridgeCalls(
  calls: GraphBridgeCall[],
  ctx: GraphBridgeContext,
): Promise<Array<{ tool: string; result: GraphBridgeResult | GraphBridgeError }>> {
  const results: Array<{ tool: string; result: GraphBridgeResult | GraphBridgeError }> = [];
  for (const call of calls) {
    if (!isGraphBridgeName(call.tool)) {
      results.push({ tool: call.tool, result: err(`Unknown graph bridge: ${call.tool}`) });
      continue;
    }
    results.push({ tool: call.tool, result: await executeGraphBridge(call.tool, call.input, ctx) });
  }
  return results;
}

function asCallRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function parseGraphCalls(raw: unknown): GraphBridgeCall[] {
  if (!Array.isArray(raw)) return [];
  return raw.flatMap((item) => {
    const row = asCallRecord(item);
    if (!row || typeof row.tool !== "string" || !row.tool.startsWith("graph.")) return [];
    return [{ tool: row.tool, input: row.input ?? row.arguments ?? row.params }];
  });
}

export async function applyGraphEffectsFromRunOutput(
  output: Record<string, unknown> | null | undefined,
  ctx: GraphBridgeContext,
): Promise<{
  graphChanged: boolean;
  graphResults?: Array<{ tool: string; result: GraphBridgeResult | GraphBridgeError }>;
}> {
  const raw = output?.graphCalls ?? output?.graphBridgeCalls ?? output?.toolCalls;
  const calls = parseGraphCalls(raw);
  if (!calls.length) return { graphChanged: false };
  const graphResults = await executeGraphBridgeCalls(calls, ctx);
  const graphChanged = graphResults.some(
    (row) => row.tool !== "graph.get" && row.result.ok,
  );
  return { graphChanged, graphResults };
}
