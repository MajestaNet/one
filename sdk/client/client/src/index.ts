/**
 * @one/client — Client API fetch wrapper (/client/v1 only).
 * Wire matches install QueryRequest + /sobjects + /describe (ADR-019 / ADR-025).
 */

export const PREFERRED_API_REVISION = 1;
/** @deprecated Use PREFERRED_API_REVISION. Kept for existing Experience imports. */
export const CLIENT_PREFERRED_API_REVISION = PREFERRED_API_REVISION;

export type FetchLike = typeof fetch;

export type OneClientConfig = {
  baseUrl: string;
  getAccessToken: () => string | Promise<string>;
  /** Pinned API revision (One-API-Revision). Defaults to package preferred revision. */
  apiRevision?: number;
  /** Injected fetch for tests; defaults to globalThis.fetch. */
  fetch?: FetchLike;
};

export type FilterOp =
  | "eq"
  | "ne"
  | "gt"
  | "gte"
  | "lt"
  | "lte"
  | "like"
  | "in"
  | "is_null"
  | "is_not_null";

export type QueryFilter = {
  field: string;
  op: FilterOp;
  value?: unknown;
};

export type SortSpec = {
  field: string;
  direction?: string;
};

export type RelationshipQuery = {
  type: string;
  field: string;
  object?: string;
  alias?: string;
  filters?: QueryFilter[];
  select?: string[];
  limit?: number;
};

/** POST /client/v1/query body — matches internal/dataengine QueryRequest. */
export type QueryRequest = {
  object: string;
  select?: string[];
  filters?: QueryFilter[];
  sort?: SortSpec[];
  relationships?: RelationshipQuery[];
  limit?: number;
  cursor?: string;
  includeDeleted?: boolean;
  mode?: string;
};

export type QueryResponse = {
  records?: unknown[];
  totalSize?: number;
  done?: boolean;
  queryPlan?: unknown;
  nextCursor?: string;
};

export type SearchRequest = {
  q: string;
  objects?: string[];
  limit?: number;
};

export type SearchResponse = {
  query?: string;
  hits?: unknown[];
};

export type SObjectRecord = Record<string, unknown>;

export type VersionProbe = {
  productVersion?: string;
  version?: string;
  runtime?: string;
  apiRevision?: {
    min: number;
    current: number;
    recommended?: number;
  };
  httpApi?: Record<string, string>;
};

/**
 * Install JSON error `{ error, message, … }` (including API_REVISION_UNSUPPORTED + cta).
 */
export class OneAPIError extends Error {
  readonly status: number;
  readonly code: string;
  readonly body: unknown;

  constructor(status: number, code: string, message: string, body: unknown) {
    super(message);
    this.name = "OneAPIError";
    this.status = status;
    this.code = code;
    this.body = body;
  }

  /** Operator hint on API_REVISION_UNSUPPORTED (install `cta` field). */
  get cta(): string | undefined {
    if (this.body && typeof this.body === "object" && "cta" in this.body) {
      const v = (this.body as { cta?: unknown }).cta;
      return typeof v === "string" ? v : undefined;
    }
    return undefined;
  }
}

export async function throwOneAPIError(res: Response): Promise<never> {
  const text = await res.text();
  let body: unknown = text;
  let code = "";
  let message = text || res.statusText || `HTTP ${res.status}`;
  if (text) {
    try {
      const parsed = JSON.parse(text) as { error?: unknown; message?: unknown };
      body = parsed;
      if (typeof parsed.error === "string") code = parsed.error;
      if (typeof parsed.message === "string" && parsed.message) message = parsed.message;
    } catch {
      /* keep raw text */
    }
  }
  throw new OneAPIError(res.status, code, message, body);
}

function resolveFetch(fetchImpl?: FetchLike): FetchLike {
  const fn = fetchImpl ?? globalThis.fetch;
  if (typeof fn !== "function") {
    throw new Error("fetch is not available; pass config.fetch");
  }
  return fn;
}

async function resolveToken(getAccessToken: OneClientConfig["getAccessToken"]): Promise<string> {
  const t = await getAccessToken();
  if (!t) throw new Error("missing access token");
  return t;
}

function origin(baseUrl: string): string {
  return baseUrl.replace(/\/$/, "");
}

/**
 * GET /version (revision-agnostic; no One-API-Revision header).
 * Does not hard-block like `one` exit 3 — callers send a pin and surface API_REVISION_UNSUPPORTED.
 */
export async function probeVersion(
  baseUrl: string,
  options?: { fetch?: FetchLike },
): Promise<VersionProbe> {
  const fetchImpl = resolveFetch(options?.fetch);
  const url = new URL("/version", origin(baseUrl) + "/").toString();
  const res = await fetchImpl(url, { method: "GET", headers: { Accept: "application/json" } });
  if (!res.ok) await throwOneAPIError(res);
  return (await res.json()) as VersionProbe;
}

export function createOneClient(config: OneClientConfig) {
  const apiRoot = origin(config.baseUrl) + "/client/v1";
  const apiRevision = config.apiRevision ?? PREFERRED_API_REVISION;
  const fetchImpl = () => resolveFetch(config.fetch);

  async function request<T>(path: string, init?: RequestInit): Promise<T> {
    const token = await resolveToken(config.getAccessToken);
    const res = await fetchImpl()(apiRoot + path, {
      ...init,
      headers: {
        Accept: "application/json",
        ...(init?.headers ?? {}),
        Authorization: `Bearer ${token}`,
        "One-API-Revision": String(apiRevision),
      },
    });
    if (!res.ok) await throwOneAPIError(res);
    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  }

  function jsonBody(body: unknown): RequestInit {
    return {
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    };
  }

  function sobjectPath(object: string, id?: string): string {
    const base = `/sobjects/${encodeURIComponent(object)}`;
    return id === undefined ? base : `${base}/${encodeURIComponent(id)}`;
  }

  return {
    query: (body: QueryRequest) =>
      request<QueryResponse>("/query", { method: "POST", ...jsonBody(body) }),
    search: (body: SearchRequest) =>
      request<SearchResponse>("/search", { method: "POST", ...jsonBody(body) }),
    getRecord: (object: string, id: string) => request<SObjectRecord>(sobjectPath(object, id)),
    createRecord: (object: string, body: SObjectRecord) =>
      request<SObjectRecord>(sobjectPath(object), { method: "POST", ...jsonBody(body) }),
    updateRecord: (object: string, id: string, body: SObjectRecord) =>
      request<SObjectRecord>(sobjectPath(object, id), { method: "PATCH", ...jsonBody(body) }),
    deleteRecord: (object: string, id: string) =>
      request<void>(sobjectPath(object, id), { method: "DELETE" }),
    describe: () => request<unknown>("/describe"),
    describeObject: (object: string) =>
      request<unknown>(`/describe/${encodeURIComponent(object)}`),
    me: () => request<unknown>("/me"),
    probeVersion: () => probeVersion(config.baseUrl, { fetch: config.fetch }),
  };
}
