/**
 * Silent refresh / revoke for Control IDE (BP-063 Phase 3).
 *
 * Two Electron processes sharing one session.bin is unsupported; the in-flight
 * Promise below is a single-process mutex only (one desktop instance).
 */

import { redactSensitive } from "./errors";
import { assertApiBaseUrl } from "./installUrl";
import { CONTROL_IDE_INTEGRATION } from "./oauthPkce";

export const ACCESS_TOKEN_SKEW_MS = 60_000;

export type RotatedTokens = {
  accessToken: string;
  refreshToken?: string;
  expiresIn?: number;
  refreshExpiresIn?: number;
};

export type RefreshSessionOpts = {
  clientId?: string;
  allowInsecureHttp?: boolean;
};

/** True when access expiry is unknown or within `skewMs` of now. */
export function accessTokenNearingExpiry(
  accessExpiresAt: number | undefined,
  now = Date.now(),
  skewMs = ACCESS_TOKEN_SKEW_MS,
): boolean {
  if (accessExpiresAt == null || !Number.isFinite(accessExpiresAt)) return true;
  return accessExpiresAt - now <= skewMs;
}

let inFlight: Promise<RotatedTokens> | null = null;
let inFlightKey: string | null = null;

function flightKey(origin: string, refreshToken: string, clientId: string): string {
  return `${origin}\0${clientId}\0${refreshToken}`;
}

function parseTokenResponse(text: string, status: number, path: string): RotatedTokens {
  let json: Record<string, unknown>;
  try {
    json = text ? (JSON.parse(text) as Record<string, unknown>) : {};
  } catch {
    json = { error: text };
  }
  if (status < 200 || status >= 300 || typeof json.access_token !== "string" || !json.access_token) {
    const detail = redactSensitive(text || JSON.stringify(json));
    throw new Error(`${status} ${path}: ${detail}`);
  }
  const out: RotatedTokens = { accessToken: json.access_token };
  if (typeof json.refresh_token === "string" && json.refresh_token) {
    out.refreshToken = json.refresh_token;
  }
  if (typeof json.expires_in === "number" && Number.isFinite(json.expires_in)) {
    out.expiresIn = json.expires_in;
  }
  if (typeof json.refresh_expires_in === "number" && Number.isFinite(json.refresh_expires_in)) {
    out.refreshExpiresIn = json.refresh_expires_in;
  }
  return out;
}

async function postRefreshToken(
  origin: string,
  refreshToken: string,
  clientId: string,
): Promise<RotatedTokens> {
  const body = new URLSearchParams({
    grant_type: "refresh_token",
    refresh_token: refreshToken,
    client_id: clientId,
  });
  const path = "/auth/v1/token";
  const res = await fetch(`${origin}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body,
  });
  const text = await res.text();
  return parseTokenResponse(text, res.status, path);
}

/**
 * Mint a new access JWT from a stored refresh token.
 * Concurrent callers with the same origin + token share one POST (no double-rotate).
 */
export function refreshAccessToken(
  baseUrl: string,
  refreshToken: string,
  opts?: RefreshSessionOpts,
): Promise<RotatedTokens> {
  const origin = assertApiBaseUrl(baseUrl, { allowInsecureHttp: opts?.allowInsecureHttp });
  const clientId = opts?.clientId || CONTROL_IDE_INTEGRATION;
  const key = flightKey(origin, refreshToken, clientId);
  if (inFlight && inFlightKey === key) {
    return inFlight;
  }
  inFlightKey = key;
  inFlight = postRefreshToken(origin, refreshToken, clientId).finally(() => {
    inFlight = null;
    inFlightKey = null;
  });
  return inFlight;
}

/** Best-effort RFC 7009-shaped revoke. Network / HTTP errors are ignored. */
export async function revokeRefreshToken(
  baseUrl: string,
  refreshToken: string,
  opts?: RefreshSessionOpts,
): Promise<void> {
  try {
    const origin = assertApiBaseUrl(baseUrl, { allowInsecureHttp: opts?.allowInsecureHttp });
    await fetch(`${origin}/auth/v1/revoke`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token: refreshToken, token_type_hint: "refresh_token" }),
    });
  } catch {
    /* always clear the local session after Sign out, even if revoke fails */
  }
}
