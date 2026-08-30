/**
 * @one/auth — PKCE + Majesta One JWT helpers for Client Experience apps.
 * See docs/client-experience-security.md
 */

/** Same integer as @one/client PREFERRED_API_REVISION (packages stay independent). */
export const PREFERRED_API_REVISION = 1;

const FORBIDDEN_AUTHORIZE_SCOPES = new Set(["metadata", "deploy", "ops", "admin"]);

export type FetchLike = typeof fetch;

export type OneAuthConfig = {
  baseUrl: string;
  clientId: string;
  redirectUri: string;
  scopes?: string[];
  /** Pinned API revision (One-API-Revision) on /auth/v1/token. */
  apiRevision?: number;
  /** Injected fetch for tests; defaults to globalThis.fetch. */
  fetch?: FetchLike;
};

export type TokenResponse = {
  access_token: string;
  token_type?: string;
  expires_in?: number;
  refresh_token?: string;
};

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
}

async function throwOneAPIError(res: Response): Promise<never> {
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

function origin(baseUrl: string): string {
  return baseUrl.replace(/\/$/, "");
}

function revisionHeader(config: OneAuthConfig): string {
  return String(config.apiRevision ?? PREFERRED_API_REVISION);
}

/** Default Experience scopes: client only. */
export function defaultAuthorizeScopes(config: Pick<OneAuthConfig, "scopes">): string[] {
  return config.scopes?.length ? [...config.scopes] : ["client"];
}

/**
 * Reject Metadata / Deploy / Ops / admin before redirect (public Connected Apps).
 * `offline_access` is allowed when the app opts into refresh (Phase R2).
 */
export function assertExperienceScopes(scopes: string[]): string[] {
  const tokens = scopes.flatMap((s) => s.split(/\s+/)).map((s) => s.trim()).filter(Boolean);
  const forbidden = tokens.filter((t) => FORBIDDEN_AUTHORIZE_SCOPES.has(t.toLowerCase()));
  if (forbidden.length > 0) {
    throw new Error(
      `Client Experience authorize cannot request scope(s) ${forbidden.join(", ")} (metadata, deploy, ops, and admin are not allowed in the browser). Default is client.`,
    );
  }
  return tokens.length ? tokens : ["client"];
}

function base64Url(bytes: Uint8Array): string {
  let s = "";
  for (const b of bytes) s += String.fromCharCode(b);
  return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

/** Generate PKCE verifier + S256 challenge for public Connected Apps. */
export async function generatePKCE(): Promise<{ verifier: string; challenge: string }> {
  const verifier = base64Url(crypto.getRandomValues(new Uint8Array(32)));
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier));
  return { verifier, challenge: base64Url(new Uint8Array(digest)) };
}

/** Build Majesta One `/auth/v1/authorize` URL (authorization_code + PKCE). */
export function buildAuthorizeUrl(
  config: OneAuthConfig,
  state: string,
  challenge: string,
): string {
  const scopes = assertExperienceScopes(defaultAuthorizeScopes(config)).join(" ");
  const u = new URL("/auth/v1/authorize", origin(config.baseUrl) + "/");
  u.searchParams.set("response_type", "code");
  u.searchParams.set("client_id", config.clientId);
  u.searchParams.set("redirect_uri", config.redirectUri);
  u.searchParams.set("scope", scopes);
  u.searchParams.set("state", state);
  u.searchParams.set("code_challenge", challenge);
  u.searchParams.set("code_challenge_method", "S256");
  return u.toString();
}

/** Exchange authorization code for Majesta One JWT. Sends One-API-Revision. */
export async function exchangeAuthorizationCode(
  config: OneAuthConfig,
  code: string,
  verifier: string,
): Promise<TokenResponse> {
  const body = new URLSearchParams({
    grant_type: "authorization_code",
    code,
    redirect_uri: config.redirectUri,
    client_id: config.clientId,
    code_verifier: verifier,
  });
  const fetchImpl = resolveFetch(config.fetch);
  const res = await fetchImpl(new URL("/auth/v1/token", origin(config.baseUrl) + "/").toString(), {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
      Accept: "application/json",
      "One-API-Revision": revisionHeader(config),
    },
    body,
  });
  if (!res.ok) await throwOneAPIError(res);
  return (await res.json()) as TokenResponse;
}

export function createOneAuthClient(config: OneAuthConfig) {
  return {
    config,
    generatePKCE,
    buildAuthorizeUrl: (state: string, challenge: string) =>
      buildAuthorizeUrl(config, state, challenge),
    exchangeAuthorizationCode: (code: string, verifier: string) =>
      exchangeAuthorizationCode(config, code, verifier),
  };
}
