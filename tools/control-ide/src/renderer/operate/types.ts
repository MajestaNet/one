/** Operate CRM types — BoardHandoff + describe/query shapes (operate-agentic-crm.md). */

export type QueryFilter = {
  field: string;
  op: "eq" | "ne" | "gt" | "gte" | "lt" | "lte" | "like" | "in" | "is_null" | "is_not_null";
  value?: unknown;
};

export type SortSpec = {
  field: string;
  direction: "asc" | "desc";
};

export type DescribeField = {
  apiName: string;
  label?: string;
  fieldType?: string;
  required?: boolean;
  filterable?: boolean;
  sortable?: boolean;
  picklistValues?: string[];
  referenceTo?: string | null;
  relationshipName?: string | null;
  length?: number | null;
};

export type DescribeObject = {
  apiName?: string;
  label?: string;
  pluralLabel?: string;
  fields?: DescribeField[];
};

export type BoardHandoffSuggestion = {
  id: string;
  label: string;
  action: string;
};

export type ProposedMutation = {
  op: "create" | "update" | "delete";
  object: string;
  id?: string;
  data?: Record<string, unknown>;
};

/** Structured agent → CRM tile handoff (BP-024). */
export type BoardHandoff = {
  source: "run" | "tool_result" | "message_excerpt" | "approval_bundle";
  runId?: string;
  objectApiName?: string;
  view?: {
    filters?: QueryFilter[];
    sort?: SortSpec[];
    limit?: number;
  };
  recordIds?: string[];
  proposedMutations?: ProposedMutation[];
  rationale?: string;
  suggestions?: BoardHandoffSuggestion[];
};

export type SavedView = {
  id: string;
  name: string;
  objectApiName: string;
  filters: QueryFilter[];
  sort: SortSpec[];
  limit: number;
};

export type ObjectTab = {
  id: string;
  objectApiName: string;
  label: string;
  /** Package that must be enabled (omit = always when core). */
  packageName?: string;
  /** Pipeline/kanban field when set (e.g. StageName). */
  boardField?: string;
};

export type RelatedListDef = {
  objectApiName: string;
  lookupField: string;
  label: string;
  packageName?: string;
};

export const SYSTEM_FIELD_SKIP = new Set([
  "Id",
  "id",
  "CreatedAt",
  "created_at",
  "UpdatedAt",
  "updated_at",
  "CreatedById",
  "LastModifiedById",
  "LastModifiedAt",
  "OwnerId",
]);

export function displayName(rec: Record<string, unknown>): string {
  return String(
    rec.Name ?? rec.name ?? rec.Subject ?? rec.LastName ?? rec.CaseNumber ?? rec.label ?? rec.id ?? rec.Id ?? "Untitled",
  );
}

export function recordId(rec: Record<string, unknown>): string {
  return String(rec.id ?? rec.Id ?? "");
}
