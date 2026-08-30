import {
  RUN_GRAPH_API_VERSION,
  isRunGraphEdgeKind,
  isRunGraphNodeKind,
  type RunGraphBinding,
  type RunGraphDocument,
  type RunGraphEdge,
  type RunGraphNode,
  type RunGraphValidationIssue,
  type RunGraphValidationResult,
} from "./types";

function isObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function allowedKeys(
  issues: RunGraphValidationIssue[],
  path: string,
  value: Record<string, unknown>,
  allowed: readonly string[],
) {
  const set = new Set(allowed);
  for (const key of Object.keys(value)) {
    if (!set.has(key)) issues.push({ path: `${path}.${key}`, message: `${key} is not allowed` });
  }
}

function nonEmpty(value: unknown): value is string {
  return typeof value === "string" && Boolean(value.trim());
}

function isUUID(value: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value);
}

function parseLayout(
  raw: unknown,
  path: string,
  issues: RunGraphValidationIssue[],
): RunGraphNode["layout"] {
  if (raw === undefined) return undefined;
  if (!isObject(raw)) {
    issues.push({ path, message: "layout must be an object" });
    return undefined;
  }
  allowedKeys(issues, path, raw, ["x", "y", "w", "z"]);
  if (typeof raw.x !== "number" || typeof raw.y !== "number") {
    issues.push({ path, message: "layout requires numeric x and y" });
    return undefined;
  }
  return {
    x: raw.x,
    y: raw.y,
    w: typeof raw.w === "number" ? raw.w : undefined,
    z: typeof raw.z === "number" ? raw.z : undefined,
  };
}

function parseNode(raw: unknown, index: number, issues: RunGraphValidationIssue[]): RunGraphNode | null {
  const path = `nodes[${index}]`;
  if (!isObject(raw)) {
    issues.push({ path, message: "node must be an object" });
    return null;
  }
  allowedKeys(issues, path, raw, [
    "id",
    "kind",
    "ref",
    "toolRef",
    "layout",
    "cardProjection",
    "label",
    "text",
    "proposalId",
    "bindingId",
    "searchQ",
  ]);
  if (!nonEmpty(raw.id)) issues.push({ path: `${path}.id`, message: "id is required" });
  if (!isRunGraphNodeKind(raw.kind)) {
    issues.push({ path: `${path}.kind`, message: `unknown node kind ${JSON.stringify(raw.kind)}` });
    return null;
  }
  const layout = parseLayout(raw.layout, `${path}.layout`, issues);
  const projection = raw.cardProjection;
  if (projection !== undefined && (!Array.isArray(projection) || !projection.every(nonEmpty))) {
    issues.push({ path: `${path}.cardProjection`, message: "cardProjection must be a string array" });
  }

  let ref: RunGraphNode["ref"];
  let toolRef: RunGraphNode["toolRef"];
  if (raw.kind === "record") {
    if (!isObject(raw.ref) || !nonEmpty(raw.ref.objectApiName) || !nonEmpty(raw.ref.recordId)) {
      issues.push({ path: `${path}.ref`, message: "record ref requires objectApiName and recordId" });
    } else {
      allowedKeys(issues, `${path}.ref`, raw.ref, ["objectApiName", "recordId"]);
      if (!isUUID(raw.ref.recordId)) {
        issues.push({ path: `${path}.ref.recordId`, message: "recordId must be a UUID" });
      }
      ref = { objectApiName: raw.ref.objectApiName, recordId: raw.ref.recordId };
    }
  } else if (raw.kind === "collection") {
    if (!isObject(raw.ref) || !nonEmpty(raw.ref.objectApiName)) {
      issues.push({ path: `${path}.ref`, message: "collection ref requires objectApiName" });
    } else {
      allowedKeys(issues, `${path}.ref`, raw.ref, ["objectApiName"]);
      ref = { objectApiName: raw.ref.objectApiName };
    }
    if (raw.searchQ !== undefined) {
      if (!nonEmpty(raw.searchQ)) {
        issues.push({ path: `${path}.searchQ`, message: "searchQ must be non-empty when set" });
      } else if (raw.searchQ.length > 200) {
        issues.push({ path: `${path}.searchQ`, message: "searchQ exceeds 200 characters" });
      }
    }
    if (isObject(raw.ref) && raw.ref.recordId !== undefined) {
      issues.push({ path: `${path}.ref.recordId`, message: "recordId is not allowed on collection nodes" });
    }
  } else if (raw.kind === "person") {
    if (!isObject(raw.ref)) {
      issues.push({ path: `${path}.ref`, message: "person ref is required" });
    } else {
      allowedKeys(issues, `${path}.ref`, raw.ref, ["principalId", "contactRecordId"]);
      const principalId = nonEmpty(raw.ref.principalId) ? raw.ref.principalId : undefined;
      const contactRecordId = nonEmpty(raw.ref.contactRecordId) ? raw.ref.contactRecordId : undefined;
      if (!principalId && !contactRecordId) {
        issues.push({ path: `${path}.ref`, message: "person ref requires principalId or contactRecordId" });
      }
      ref = { principalId, contactRecordId };
    }
  }
  if (raw.kind === "tool") {
    if (!isObject(raw.toolRef)) {
      issues.push({ path: `${path}.toolRef`, message: "toolRef is required" });
    } else {
      allowedKeys(issues, `${path}.toolRef`, raw.toolRef, ["toolSpecApiName", "workingToolId"]);
      const toolSpecApiName = nonEmpty(raw.toolRef.toolSpecApiName)
        ? raw.toolRef.toolSpecApiName
        : undefined;
      const workingToolId = nonEmpty(raw.toolRef.workingToolId) ? raw.toolRef.workingToolId : undefined;
      if (Boolean(toolSpecApiName) === Boolean(workingToolId)) {
        issues.push({ path: `${path}.toolRef`, message: "toolRef requires exactly one reference" });
      }
      toolRef = { toolSpecApiName, workingToolId };
    }
  }
  if (raw.kind === "cluster" && !nonEmpty(raw.label)) {
    issues.push({ path: `${path}.label`, message: "label is required" });
  }
  if ((raw.kind === "insight" || raw.kind === "question") && !nonEmpty(raw.text)) {
    issues.push({ path: `${path}.text`, message: "text is required" });
  }
  if (raw.kind === "proposal" && !nonEmpty(raw.proposalId)) {
    issues.push({ path: `${path}.proposalId`, message: "proposalId is required" });
  }
  if (raw.kind === "signal" && !nonEmpty(raw.bindingId)) {
    issues.push({ path: `${path}.bindingId`, message: "bindingId is required" });
  }
  if (!nonEmpty(raw.id)) return null;
  return {
    id: raw.id,
    kind: raw.kind,
    ref,
    toolRef,
    layout,
    cardProjection: Array.isArray(projection) ? (projection as string[]) : undefined,
    label: typeof raw.label === "string" ? raw.label : undefined,
    text: typeof raw.text === "string" ? raw.text : undefined,
    proposalId: typeof raw.proposalId === "string" ? raw.proposalId : undefined,
    bindingId: typeof raw.bindingId === "string" ? raw.bindingId : undefined,
    searchQ: typeof raw.searchQ === "string" ? raw.searchQ : undefined,
  };
}

function parseBindings(raw: unknown, issues: RunGraphValidationIssue[]): RunGraphBinding[] | undefined {
  if (raw === undefined) return undefined;
  if (!Array.isArray(raw)) {
    issues.push({ path: "dataBindings", message: "dataBindings must be an array" });
    return undefined;
  }
  const out: RunGraphBinding[] = [];
  raw.forEach((item, index) => {
    const path = `dataBindings[${index}]`;
    if (!isObject(item)) {
      issues.push({ path, message: "binding must be an object" });
      return;
    }
    allowedKeys(issues, path, item, ["id", "objectApiName", "fields", "filters", "sort", "limit"]);
    if (!nonEmpty(item.id) || !nonEmpty(item.objectApiName)) {
      issues.push({ path, message: "binding requires id and objectApiName" });
      return;
    }
    out.push({
      id: item.id,
      objectApiName: item.objectApiName,
      fields: Array.isArray(item.fields) && item.fields.every(nonEmpty) ? (item.fields as string[]) : undefined,
      filters: Array.isArray(item.filters) ? item.filters : undefined,
      sort: Array.isArray(item.sort) ? item.sort : undefined,
      limit: typeof item.limit === "number" ? item.limit : undefined,
    });
  });
  return out;
}

export function validateRunGraphDocument(input: unknown): RunGraphValidationResult {
  const issues: RunGraphValidationIssue[] = [];
  if (!isObject(input)) {
    return { ok: false, issues: [{ path: "", message: "document must be an object" }] };
  }
  allowedKeys(issues, "document", input, [
    "apiVersion",
    "id",
    "title",
    "revision",
    "nodes",
    "edges",
    "dataBindings",
    "lenses",
    "viewport",
  ]);
  if (input.apiVersion !== RUN_GRAPH_API_VERSION) {
    issues.push({ path: "apiVersion", message: `apiVersion must be ${RUN_GRAPH_API_VERSION}` });
  }
  if (!nonEmpty(input.id)) issues.push({ path: "id", message: "id is required" });
  if (!nonEmpty(input.title)) issues.push({ path: "title", message: "title is required" });
  if (!Array.isArray(input.nodes)) issues.push({ path: "nodes", message: "nodes must be an array" });
  if (!Array.isArray(input.edges)) issues.push({ path: "edges", message: "edges must be an array" });

  const nodes = Array.isArray(input.nodes)
    ? input.nodes.flatMap((node, index) => {
        const parsed = parseNode(node, index, issues);
        return parsed ? [parsed] : [];
      })
    : [];
  const nodeIds = new Set<string>();
  for (const node of nodes) {
    if (nodeIds.has(node.id)) issues.push({ path: "nodes", message: `duplicate node id ${node.id}` });
    nodeIds.add(node.id);
  }

  const edges: RunGraphEdge[] = [];
  if (Array.isArray(input.edges)) {
    input.edges.forEach((item, index) => {
      const path = `edges[${index}]`;
      if (!isObject(item)) {
        issues.push({ path, message: "edge must be an object" });
        return;
      }
      allowedKeys(issues, path, item, ["id", "from", "to", "kind", "weight"]);
      if (!nonEmpty(item.id) || !nonEmpty(item.from) || !nonEmpty(item.to) || !isRunGraphEdgeKind(item.kind)) {
        issues.push({ path, message: "edge requires id, from, to, and an allowlisted kind" });
        return;
      }
      if (!nodeIds.has(item.from) || !nodeIds.has(item.to)) {
        issues.push({ path, message: "edge endpoints must reference graph nodes" });
      }
      edges.push({
        id: item.id,
        from: item.from,
        to: item.to,
        kind: item.kind,
        weight: typeof item.weight === "number" ? item.weight : undefined,
      });
    });
  }
  const dataBindings = parseBindings(input.dataBindings, issues);
  const bindingsById = new Map((dataBindings ?? []).map((binding) => [binding.id, binding]));
  nodes.forEach((node, index) => {
    if (node.kind !== "collection" || !node.bindingId) return;
    const binding = bindingsById.get(node.bindingId);
    if (!binding) {
      issues.push({
        path: `nodes[${index}].bindingId`,
        message: `bindingId does not reference a dataBinding`,
      });
      return;
    }
    if (binding.objectApiName !== node.ref?.objectApiName) {
      issues.push({
        path: `nodes[${index}].bindingId`,
        message: `binding object ${binding.objectApiName} does not match collection ${node.ref?.objectApiName ?? ""}`,
      });
    }
  });
  if (issues.length || !nonEmpty(input.id) || !nonEmpty(input.title)) return { ok: false, issues };

  const document: RunGraphDocument = {
    apiVersion: RUN_GRAPH_API_VERSION,
    id: input.id,
    title: input.title,
    revision: typeof input.revision === "number" ? input.revision : undefined,
    nodes,
    edges,
    dataBindings,
    lenses: Array.isArray(input.lenses) ? (input.lenses as RunGraphDocument["lenses"]) : undefined,
    viewport:
      isObject(input.viewport) &&
      typeof input.viewport.x === "number" &&
      typeof input.viewport.y === "number" &&
      typeof input.viewport.zoom === "number"
        ? { x: input.viewport.x, y: input.viewport.y, zoom: input.viewport.zoom }
        : undefined,
  };
  return { ok: true, document };
}
