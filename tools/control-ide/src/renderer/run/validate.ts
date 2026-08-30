import {
  TOOL_DOCUMENT_API_VERSION,
  isToolNodeKind,
  type ToolDocument,
  type ToolLayout,
  type ToolNode,
  type ToolQueryBinding,
  type ToolValidationIssue,
  type ToolValidationResult,
} from "./types";

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function push(issues: ToolValidationIssue[], path: string, message: string) {
  issues.push({ path, message });
}

function parseLayout(raw: unknown, issues: ToolValidationIssue[]): ToolLayout | null {
  if (!isPlainObject(raw)) {
    push(issues, "layout", "layout must be an object");
    return null;
  }
  // Accept layout.mode (Go/validate) or layout.kind (YAML examples).
  const modeRaw = raw.mode ?? raw.kind;
  if (modeRaw !== "sections" && modeRaw !== "spatial") {
    push(issues, "layout.mode", 'layout.mode must be "sections" or "spatial"');
    return null;
  }
  const layout: ToolLayout = { mode: modeRaw };
  if (raw.sections !== undefined) {
    if (!Array.isArray(raw.sections)) {
      push(issues, "layout.sections", "layout.sections must be an array");
    } else {
      layout.sections = [];
      raw.sections.forEach((sec, i) => {
        if (!isPlainObject(sec)) {
          push(issues, `layout.sections[${i}]`, "section must be an object");
          return;
        }
        if (typeof sec.id !== "string" || !sec.id.trim()) {
          push(issues, `layout.sections[${i}].id`, "section.id is required");
          return;
        }
        if (!Array.isArray(sec.nodeIds) || !sec.nodeIds.every((id) => typeof id === "string")) {
          push(issues, `layout.sections[${i}].nodeIds`, "section.nodeIds must be a string array");
          return;
        }
        layout.sections!.push({
          id: sec.id,
          title: typeof sec.title === "string" ? sec.title : undefined,
          nodeIds: sec.nodeIds as string[],
        });
      });
    }
  }
  if (raw.positions !== undefined) {
    if (!isPlainObject(raw.positions)) {
      push(issues, "layout.positions", "layout.positions must be an object");
    } else {
      layout.positions = {};
      for (const [key, pos] of Object.entries(raw.positions)) {
        if (!isPlainObject(pos) || typeof pos.x !== "number" || typeof pos.y !== "number") {
          push(issues, `layout.positions.${key}`, "position requires numeric x and y");
          continue;
        }
        layout.positions[key] = {
          x: pos.x,
          y: pos.y,
          w: typeof pos.w === "number" ? pos.w : undefined,
          h: typeof pos.h === "number" ? pos.h : undefined,
        };
      }
    }
  }
  return layout;
}

function parseNode(raw: unknown, index: number, issues: ToolValidationIssue[]): ToolNode | null {
  const path = `nodes[${index}]`;
  if (!isPlainObject(raw)) {
    push(issues, path, "node must be an object");
    return null;
  }
  if (typeof raw.id !== "string" || !raw.id.trim()) {
    push(issues, `${path}.id`, "node.id is required");
    return null;
  }
  if (!isToolNodeKind(raw.kind)) {
    push(
      issues,
      `${path}.kind`,
      `unknown node kind ${JSON.stringify(raw.kind)} — allowlisted kinds only (ADR-021)`,
    );
    return null;
  }
  if (!isPlainObject(raw.props)) {
    push(issues, `${path}.props`, "node.props must be an object");
    return null;
  }
  return {
    id: raw.id,
    kind: raw.kind,
    title: typeof raw.title === "string" ? raw.title : undefined,
    bindingId: typeof raw.bindingId === "string" ? raw.bindingId : undefined,
    props: raw.props,
  };
}

function parseBindings(raw: unknown, issues: ToolValidationIssue[]): ToolQueryBinding[] | undefined {
  if (raw === undefined) return undefined;
  if (!Array.isArray(raw)) {
    push(issues, "dataBindings", "dataBindings must be an array");
    return undefined;
  }
  const out: ToolQueryBinding[] = [];
  raw.forEach((b, i) => {
    if (!isPlainObject(b)) {
      push(issues, `dataBindings[${i}]`, "binding must be an object");
      return;
    }
    if (typeof b.id !== "string" || !b.id.trim()) {
      push(issues, `dataBindings[${i}].id`, "binding.id is required");
      return;
    }
    if (typeof b.objectApiName !== "string" || !b.objectApiName.trim()) {
      push(issues, `dataBindings[${i}].objectApiName`, "objectApiName is required");
      return;
    }
    if (!isPlainObject(b.query)) {
      push(issues, `dataBindings[${i}].query`, "query must be an object");
      return;
    }
    out.push({
      id: b.id,
      objectApiName: b.objectApiName,
      query: b.query as ToolQueryBinding["query"],
    });
  });
  return out;
}

/** Validate unknown JSON into a ToolDocument. Rejects unknown node kinds. */
export function validateToolDocument(input: unknown): ToolValidationResult {
  const issues: ToolValidationIssue[] = [];
  if (!isPlainObject(input)) {
    return { ok: false, issues: [{ path: "", message: "document must be an object" }] };
  }
  if (input.apiVersion !== TOOL_DOCUMENT_API_VERSION) {
    push(issues, "apiVersion", `apiVersion must be "${TOOL_DOCUMENT_API_VERSION}"`);
  }
  if (typeof input.id !== "string" || !input.id.trim()) {
    push(issues, "id", "id is required");
  }
  if (typeof input.title !== "string" || !input.title.trim()) {
    push(issues, "title", "title is required");
  }
  const layout = parseLayout(input.layout, issues);
  if (!Array.isArray(input.nodes)) {
    push(issues, "nodes", "nodes must be an array");
  }
  const nodes: ToolNode[] = [];
  if (Array.isArray(input.nodes)) {
    input.nodes.forEach((n, i) => {
      const parsed = parseNode(n, i, issues);
      if (parsed) nodes.push(parsed);
    });
  }
  const dataBindings = parseBindings(input.dataBindings, issues);

  if (issues.length > 0 || !layout || typeof input.id !== "string" || typeof input.title !== "string") {
    return { ok: false, issues };
  }

  const ids = new Set<string>();
  for (const n of nodes) {
    if (ids.has(n.id)) {
      push(issues, `nodes`, `duplicate node id "${n.id}"`);
    }
    ids.add(n.id);
  }
  if (issues.length > 0) return { ok: false, issues };

  const document: ToolDocument = {
    apiVersion: TOOL_DOCUMENT_API_VERSION,
    id: input.id,
    title: input.title,
    toolSpecApiName: typeof input.toolSpecApiName === "string" ? input.toolSpecApiName : undefined,
    layout,
    nodes,
    dataBindings,
    meta: isPlainObject(input.meta)
      ? {
          createdFromRunId:
            typeof input.meta.createdFromRunId === "string" ? input.meta.createdFromRunId : undefined,
          updatedAt: typeof input.meta.updatedAt === "string" ? input.meta.updatedAt : undefined,
        }
      : undefined,
  };
  return { ok: true, document };
}
