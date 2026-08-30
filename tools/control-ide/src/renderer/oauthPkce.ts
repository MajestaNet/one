/** PKCE helpers for Control IDE → Majesta One social broker → Majesta One JWT (ADR-015). */

function b64url(buf: ArrayBuffer): string {
  const bytes = new Uint8Array(buf);
  let s = "";
  for (const b of bytes) s += String.fromCharCode(b);
  return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

export async function createPkcePair(): Promise<{ verifier: string; challenge: string }> {
  const arr = new Uint8Array(32);
  crypto.getRandomValues(arr);
  const verifier = b64url(arr.buffer);
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier));
  return { verifier, challenge: b64url(digest) };
}

export function randomOAuthState(): string {
  const arr = new Uint8Array(16);
  crypto.getRandomValues(arr);
  return b64url(arr.buffer);
}

/** Default public Connected App apiName (seeded). */
export const CONTROL_IDE_INTEGRATION = "one.controlIde";

/** Public-app scope that opts Control IDE into opaque refresh tokens. */
export const OFFLINE_ACCESS_SCOPE = "offline_access";

/** Build Majesta One hosted login page URL (provider chooser → authorize). */
export function buildOneLoginUrl(opts: {
  baseUrl: string;
  clientId: string;
  redirectUri: string;
  codeChallenge: string;
  state: string;
}): string {
  const base = opts.baseUrl.replace(/\/$/, "");
  const q = new URLSearchParams({
    client_id: opts.clientId,
    response_type: "code",
    redirect_uri: opts.redirectUri,
    code_challenge_method: "S256",
    code_challenge: opts.codeChallenge,
    state: opts.state,
    scope: OFFLINE_ACCESS_SCOPE,
  });
  return `${base}/auth/v1/login?${q.toString()}`;
}

/** Build Majesta One authorize URL (Google/Apple/dev broker). */
export function buildOneAuthorizeUrl(opts: {
  baseUrl: string;
  provider: "google" | "apple" | "dev" | string;
  clientId: string;
  redirectUri: string;
  codeChallenge: string;
  state: string;
}): string {
  const base = opts.baseUrl.replace(/\/$/, "");
  const q = new URLSearchParams({
    provider: opts.provider,
    client_id: opts.clientId,
    response_type: "code",
    redirect_uri: opts.redirectUri,
    code_challenge_method: "S256",
    code_challenge: opts.codeChallenge,
    state: opts.state,
    scope: OFFLINE_ACCESS_SCOPE,
  });
  return `${base}/auth/v1/authorize?${q.toString()}`;
}

/** Exchange Majesta One authorization code for Majesta One access JWT (PKCE). */
export async function exchangeOneAuthorizationCode(opts: {
  baseUrl: string;
  clientId: string;
  redirectUri: string;
  code: string;
  codeVerifier: string;
}): Promise<OneTokenGrant> {
  const body = new URLSearchParams({
    grant_type: "authorization_code",
    client_id: opts.clientId,
    code: opts.code,
    redirect_uri: opts.redirectUri,
    code_verifier: opts.codeVerifier,
    scope: OFFLINE_ACCESS_SCOPE,
  });
  const res = await fetch(`${opts.baseUrl.replace(/\/$/, "")}/auth/v1/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body,
  });
  const json = (await res.json()) as Record<string, unknown>;
  const grant = readTokenGrant(json);
  if (!res.ok || !grant) {
    throw new Error(JSON.stringify(json));
  }
  return grant;
}

/** External IdP ID token → Majesta One JWT (Okta/Entra/Cognito adapter path). */
export async function exchangeOneIdToken(
  baseUrl: string,
  idToken: string,
): Promise<OneTokenGrant> {
  const body = new URLSearchParams({
    grant_type: "urn:ietf:params:oauth:grant-type:token-exchange",
    subject_token: idToken,
    subject_token_type: "urn:ietf:params:oauth:token-type:id_token",
  });
  const res = await fetch(`${baseUrl.replace(/\/$/, "")}/auth/v1/token/exchange`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body,
  });
  const json = (await res.json()) as Record<string, unknown>;
  const grant = readTokenGrant(json);
  if (!res.ok || !grant) {
    throw new Error(JSON.stringify(json));
  }
  return grant;
}

/** @deprecated Cognito Hosted UI — prefer buildOneAuthorizeUrl */
export function buildHostedUiAuthorizeUrl(opts: {
  cognitoDomain: string;
  clientId: string;
  redirectUri: string;
  codeChallenge: string;
  state: string;
  scopes?: string[];
}): string {
  const domain = opts.cognitoDomain.replace(/\/$/, "");
  const scopes = (opts.scopes ?? ["openid", "email", "profile"]).join(" ");
  const q = new URLSearchParams({
    client_id: opts.clientId,
    response_type: "code",
    scope: scopes,
    redirect_uri: opts.redirectUri,
    code_challenge_method: "S256",
    code_challenge: opts.codeChallenge,
    state: opts.state,
  });
  return `${domain}/oauth2/authorize?${q.toString()}`;
}

/** @deprecated Cognito token endpoint — prefer exchangeOneAuthorizationCode */
export async function exchangeCognitoCode(opts: {
  cognitoDomain: string;
  clientId: string;
  redirectUri: string;
  code: string;
  codeVerifier: string;
}): Promise<{ id_token: string; access_token?: string }> {
  const domain = opts.cognitoDomain.replace(/\/$/, "");
  const body = new URLSearchParams({
    grant_type: "authorization_code",
    client_id: opts.clientId,
    code: opts.code,
    redirect_uri: opts.redirectUri,
    code_verifier: opts.codeVerifier,
  });
  const res = await fetch(`${domain}/oauth2/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body,
  });
  const json = (await res.json()) as { id_token?: string; access_token?: string; error?: string };
  if (!res.ok || !json.id_token) {
    throw new Error(JSON.stringify(json));
  }
  return { id_token: json.id_token, access_token: json.access_token };
}

export type OneTokenGrant = {
  access_token: string;
  expires_in?: number;
  refresh_token?: string;
  refresh_expires_in?: number;
};

function readTokenGrant(json: Record<string, unknown>): OneTokenGrant | null {
  if (typeof json.access_token !== "string" || !json.access_token) return null;
  const out: OneTokenGrant = { access_token: json.access_token };
  if (typeof json.expires_in === "number" && Number.isFinite(json.expires_in)) {
    out.expires_in = json.expires_in;
  }
  if (typeof json.refresh_token === "string" && json.refresh_token) {
    out.refresh_token = json.refresh_token;
  }
  if (typeof json.refresh_expires_in === "number" && Number.isFinite(json.refresh_expires_in)) {
    out.refresh_expires_in = json.refresh_expires_in;
  }
  return out;
}

export const DEFAULT_REDIRECT_URI = "one-control://oauth/callback";

const PKCE_PENDING_KEY = "one.pkce.pending";

export type PendingPkce = {
  verifier: string;
  state: string;
  baseUrl: string;
  clientId: string;
  redirectUri: string;
};

export function storePendingPkce(p: PendingPkce): void {
  sessionStorage.setItem(PKCE_PENDING_KEY, JSON.stringify(p));
}

export function loadPendingPkce(): PendingPkce | null {
  try {
    const raw = sessionStorage.getItem(PKCE_PENDING_KEY);
    if (!raw) return null;
    return JSON.parse(raw) as PendingPkce;
  } catch {
    return null;
  }
}

export function clearPendingPkce(): void {
  sessionStorage.removeItem(PKCE_PENDING_KEY);
}

/**
 * Read and immediately clear the pending flow. The `one-control://` handler will accept a
 * deep link from any local process, so a pending flow must be single-use: a replayed or
 * injected callback then finds nothing to consume (CIDE-11).
 */
export function takePendingPkce(): PendingPkce | null {
  const pending = loadPendingPkce();
  if (pending) clearPendingPkce();
  return pending;
}

/**
 * Parse an authorization code from a `one-control://` or http callback URL. `state` is
 * mandatory: without it the caller has nothing to compare against, and the old code path
 * silently skipped the check when a callback omitted it (CIDE-11).
 */
export function parseOAuthCallbackUrl(url: string): { code: string; state: string } | null {
  try {
    const normalized = url.replace(/^one-control:/i, "https://one-control");
    const u = new URL(normalized);
    const code = u.searchParams.get("code") ?? "";
    const state = u.searchParams.get("state") ?? "";
    if (!code || !state) return null;
    return { code, state };
  } catch {
    return null;
  }
}

/** Constant-time-ish comparison for the OAuth state parameter. */
export function statesMatch(expected: string | undefined, received: string | undefined): boolean {
  if (!expected || !received || expected.length !== received.length) return false;
  let diff = 0;
  for (let i = 0; i < expected.length; i += 1) {
    diff |= expected.charCodeAt(i) ^ received.charCodeAt(i);
  }
  return diff === 0;
}