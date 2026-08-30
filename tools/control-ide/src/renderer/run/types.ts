/** ToolSpec document types — ADR-021 / one.canvas/v1 (evolves ADR-018). */

export const TOOL_DOCUMENT_API_VERSION = "one.canvas/v1" as const;

export const TOOL_NODE_KINDS = [
  "stat",
  "recordTable",
  "recordCard",
  "relatedList",
  "queryResult",
  "mutationProposal",
  "messageThread",
  "pipelineLane",
  "actionChipGroup",
  "markdownNote",
  "sectionHeader",
] as const;

export type ToolNodeKind = (typeof TOOL_NODE_KINDS)[number];

/** Forbidden kinds — never render even if present in a payload. */
export const FORBIDDEN_NODE_KINDS = ["rawHtml", "iframe", "remoteReact", "customScript"] as const;

export type ToolQueryBinding = {
  id: string;
  objectApiName: string;
  query: { select?: string[]; filters?: unknown[]; sort?: unknown[]; limit?: number };
};

export type ToolLayout = {
  mode: "spatial" | "sections";
  sections?: Array<{ id: string; title?: string; nodeIds: string[] }>;
  positions?: Record<string, { x: number; y: number; w?: number; h?: number }>;
};

export type ToolNode = {
  id: string;
  kind: ToolNodeKind;
  title?: string;
  bindingId?: string;
  props: Record<string, unknown>;
};

export type ToolDocument = {
  apiVersion: typeof TOOL_DOCUMENT_API_VERSION;
  id: string;
  title: string;
  toolSpecApiName?: string;
  layout: ToolLayout;
  nodes: ToolNode[];
  dataBindings?: ToolQueryBinding[];
  meta?: { createdFromRunId?: string; updatedAt?: string };
};

export type ToolValidationIssue = {
  path: string;
  message: string;
};

export type ToolValidationResult =
  | { ok: true; document: ToolDocument }
  | { ok: false; issues: ToolValidationIssue[] };

export function isToolNodeKind(value: unknown): value is ToolNodeKind {
  return typeof value === "string" && (TOOL_NODE_KINDS as readonly string[]).includes(value);
}

export type ActionChip = {
  label: string;
  prompt?: string;
  type?: string;
  automationApiName?: string;
  input?: Record<string, unknown>;
};

export function automationApiNameFromChip(chip: ActionChip): string | null {
  const apiName = chip.automationApiName?.trim();
  if (chip.type === "automationRun" || apiName) {
    return apiName || null;
  }
  return null;
}

export function isAutomationRunChip(chip: ActionChip): boolean {
  return chip.type === "automationRun" || Boolean(chip.automationApiName?.trim());
}

export type MutationOp = {
  op: string;
  object: string;
  id?: string;
};

export function parseActionChips(raw: unknown): ActionChip[] {
  if (!Array.isArray(raw)) return [];
  const out: ActionChip[] = [];
  for (const item of raw) {
    if (!item || typeof item !== "object") continue;
    const o = item as Record<string, unknown>;
    if (typeof o.label !== "string" || !o.label.trim()) continue;
    out.push({
      label: o.label,
      prompt: typeof o.prompt === "string" ? o.prompt : undefined,
      type: typeof o.type === "string" ? o.type : undefined,
      automationApiName: typeof o.automationApiName === "string" ? o.automationApiName : undefined,
      input:
        o.input && typeof o.input === "object" && !Array.isArray(o.input)
          ? (o.input as Record<string, unknown>)
          : undefined,
    });
  }
  return out;
}

export function operationsFromMutationNode(node: ToolNode): MutationOp[] {
  const raw = node.props.operations;
  if (!Array.isArray(raw)) return [];
  const out: MutationOp[] = [];
  for (const item of raw) {
    if (!item || typeof item !== "object") continue;
    const o = item as Record<string, unknown>;
    const op = typeof o.op === "string" ? o.op : typeof o.action === "string" ? o.action : "";
    const object =
      typeof o.object === "string"
        ? o.object
        : typeof o.objectApiName === "string"
          ? o.objectApiName
          : "";
    if (!op || !object) continue;
    out.push({
      op,
      object,
      id: typeof o.id === "string" ? o.id : typeof o.recordId === "string" ? o.recordId : undefined,
    });
  }
  return out;
}
