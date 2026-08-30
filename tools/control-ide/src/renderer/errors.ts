/**
 * Error text that is safe to render. API error bodies were surfaced verbatim, which can echo
 * bearer material and internal detail into the UI (CIDE-18).
 */

const MAX_LENGTH = 400;

/** JWTs and other long opaque credentials that should never reach the screen. */
const JWT_PATTERN = /\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/g;
const BEARER_PATTERN = /\b(bearer\s+)[A-Za-z0-9._~+/=-]{12,}/gi;
const SECRET_FIELD_PATTERN =
  /("?(?:access_token|refresh_token|id_token|client_secret|password|token)"?\s*[:=]\s*"?)[^",}\s]+/gi;

/** Strip credential-shaped substrings and cap the length. */
export function redactSensitive(raw: string): string {
  const redacted = raw
    .replace(JWT_PATTERN, "[redacted-jwt]")
    .replace(BEARER_PATTERN, "$1[redacted]")
    .replace(SECRET_FIELD_PATTERN, "$1[redacted]");
  return redacted.length > MAX_LENGTH ? `${redacted.slice(0, MAX_LENGTH)}… (truncated)` : redacted;
}

/** Turn anything thrown into a single line fit for an error banner. */
export function formatError(err: unknown): string {
  const message =
    err instanceof Error ? err.message : typeof err === "string" ? err : JSON.stringify(err);
  return redactSensitive(message ?? "Unknown error");
}
