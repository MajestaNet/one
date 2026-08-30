/**
 * Minimal Majesta One HTTP client: client_credentials → JWT, then family API calls.
 * AuthZ stays on the install — this client invents no permissions.
 */

export type OneClientOptions = {
  baseUrl: string;
  clientId: string;
  clientSecret: string;
  /** Optional static bearer (skips client_credentials). */
  accessToken?: string;
  /** Pinned API revision (One-API-Revision). Defaults to 1. */
  apiRevision?: number;
};

const DEFAULT_API_REVISION = 1;

function resolveApiRevision(n?: number): number {
  if (typeof n === "number" && Number.isInteger(n) && n > 0) {
    return n;
  }
  return DEFAULT_API_REVISION;
}

function parseEnvApiRevision(raw: string | undefined): number {
  const n = Number.parseInt(raw ?? "", 10);
  return Number.isInteger(n) && n > 0 ? n : DEFAULT_API_REVISION;
}

export class OneClient {
  readonly baseUrl: string;
  readonly apiRevision: number;
  private readonly clientId: string;
  private readonly clientSecret: string;
  private accessToken: string | undefined;
  private tokenExpiresAt = 0;

  constructor(opts: OneClientOptions) {
    this.baseUrl = opts.baseUrl.replace(/\/+$/, "");
    this.clientId = opts.clientId;
    this.clientSecret = opts.clientSecret;
    this.accessToken = opts.accessToken;
    this.apiRevision = resolveApiRevision(opts.apiRevision);
  }

  private revisionHeaders(): Record<string, string> {
    return { "One-API-Revision": String(this.apiRevision) };
  }

  async getToken(): Promise<string> {
    if (this.accessToken && Date.now() < this.tokenExpiresAt - 30_000) {
      return this.accessToken;
    }
    if (this.accessToken && !this.clientId) {
      return this.accessToken;
    }
    const body = new URLSearchParams({
      grant_type: "client_credentials",
      client_id: this.clientId,
      client_secret: this.clientSecret,
    });
    const res = await fetch(`${this.baseUrl}/auth/v1/token`, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
        ...this.revisionHeaders(),
      },
      body,
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(`token mint failed: ${res.status} ${text}`);
    }
    const json = (await res.json()) as { access_token: string; expires_in?: number };
    this.accessToken = json.access_token;
    const ttl = typeof json.expires_in === "number" ? json.expires_in : 3600;
    this.tokenExpiresAt = Date.now() + ttl * 1000;
    return this.accessToken;
  }

  async request<T = unknown>(method: string, path: string, body?: unknown): Promise<T> {
    const token = await this.getToken();
    const res = await fetch(`${this.baseUrl}${path}`, {
      method,
      headers: {
        Authorization: `Bearer ${token}`,
        Accept: "application/json",
        ...this.revisionHeaders(),
        ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
      },
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
    const text = await res.text();
    let parsed: unknown = text;
    if (text) {
      try {
        parsed = JSON.parse(text);
      } catch {
        /* keep text */
      }
    }
    if (!res.ok) {
      throw new Error(`${method} ${path} → ${res.status}: ${typeof parsed === "string" ? parsed : JSON.stringify(parsed)}`);
    }
    return parsed as T;
  }

  /** Forward a JSON-RPC call to the product MCP gateway (Streamable HTTP, stateless JSON). */
  async mcpRpc(method: string, params: Record<string, unknown> = {}, id: number | string = 1): Promise<unknown> {
    const token = await this.getToken();
    const res = await fetch(`${this.baseUrl}/mcp`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
        Accept: "application/json, text/event-stream",
        ...this.revisionHeaders(),
      },
      body: JSON.stringify({ jsonrpc: "2.0", id, method, params }),
    });
    const json = (await res.json()) as { result?: unknown; error?: { message?: string } };
    if (!res.ok || json.error) {
      throw new Error(json.error?.message ?? `MCP ${method} failed: ${res.status}`);
    }
    return json.result;
  }
}

export function clientFromEnv(): OneClient {
  const baseUrl = process.env.ONE_BASE_URL;
  if (!baseUrl) {
    throw new Error("ONE_BASE_URL is required");
  }
  const accessToken = process.env.ONE_ACCESS_TOKEN;
  const clientId = process.env.ONE_CLIENT_ID ?? "";
  const clientSecret = process.env.ONE_CLIENT_SECRET ?? "";
  if (!accessToken && (!clientId || !clientSecret)) {
    throw new Error("Set ONE_ACCESS_TOKEN or ONE_CLIENT_ID + ONE_CLIENT_SECRET");
  }
  return new OneClient({
    baseUrl,
    clientId,
    clientSecret,
    accessToken,
    apiRevision: parseEnvApiRevision(process.env.ONE_API_REVISION),
  });
}
