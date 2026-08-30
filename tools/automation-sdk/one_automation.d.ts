/**
 * Vendor-plane TypeScript stubs for Control IDE / editors.
 * Not an npm package for customers — guests resolve one:automation via Deno import map.
 */

export type AutomationResult = {
  ok?: boolean;
  error?: string;
  [key: string]: unknown;
};

export type AutomationTrigger = {
  action: string;
  objectApiName: string;
  recordId: string;
  data?: Record<string, unknown>;
  actorId?: string;
};

export type AutomationContext = {
  trigger: AutomationTrigger;
  createRecord: (args: {
    objectApiName: string;
    data?: Record<string, unknown>;
  }) => Promise<{ id: string }>;
  updateRecord: (args: {
    objectApiName: string;
    recordId: string;
    data?: Record<string, unknown>;
  }) => Promise<{ ok: boolean }>;
  getRecord: (args: {
    objectApiName: string;
    recordId: string;
  }) => Promise<Record<string, unknown>>;
  deleteRecord: (args: {
    objectApiName: string;
    recordId: string;
  }) => Promise<{ ok: boolean }>;
  query: (args: Record<string, unknown>) => Promise<{
    records: Record<string, unknown>[];
    totalSize: number;
  }>;
  log: (...args: unknown[]) => void;
  /** Product platform actions (ADR-029). Package-gated; AuthZ as run-as. */
  invokeAction: (args: {
    apiName: string;
    input?: Record<string, unknown>;
  }) => Promise<Record<string, unknown>>;
};

export type AutomationUnitContext = AutomationContext & {
  runUnderTest: (args?: { trigger?: AutomationTrigger }) => Promise<{ ok: boolean }>;
  getCalls: (args?: { method?: string }) => Promise<{ calls: Record<string, unknown>[] }>;
  clearCalls: () => Promise<{ ok: boolean }>;
};
