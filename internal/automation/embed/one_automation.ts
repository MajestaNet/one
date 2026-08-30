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

/** Guest SDK (ADR-014). http/connector are async-only via Go host RPC (BP-014). */
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
  /** Async only — host performs HTTPS; guest has no network. */
  http: (args: {
    method?: string;
    url?: string;
    connectorApiName?: string;
    path?: string;
    headers?: Record<string, string>;
    body?: unknown;
    secretRef?: string;
  }) => Promise<{
    status: number;
    ok: boolean;
    headers: Record<string, string>;
    body: string;
    json?: unknown;
  }>;
  /** Async only — resolves install connector + optional secret ref. */
  connector: (args: {
    apiName: string;
    method?: string;
    path?: string;
    headers?: Record<string, string>;
    body?: unknown;
  }) => Promise<{
    status: number;
    ok: boolean;
    headers: Record<string, string>;
    body: string;
    json?: unknown;
  }>;
};

/** Unit-harness helpers (available in tests/automations guests only). */
export type AutomationUnitContext = AutomationContext & {
  runUnderTest: (args?: { trigger?: AutomationTrigger }) => Promise<{ ok: boolean }>;
  getCalls: (args?: { method?: string }) => Promise<{ calls: Record<string, unknown>[] }>;
  clearCalls: () => Promise<{ ok: boolean }>;
};
