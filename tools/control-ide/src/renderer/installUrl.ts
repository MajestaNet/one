/**
 * Install base-URL validation. Every authenticated call attaches the Majesta One JWT to whatever
 * host this resolves to, so a typo or a hostile peer record is a credential-disclosure bug
 * (CIDE-08). Two layers:
 *
 *  - `assertApiBaseUrl` — hard rules enforced on every request in `apiFetch`.
 *  - `checkInstallBaseUrl` — connect-time policy; requires TLS off loopback unless the
 *    operator explicitly accepts plaintext for that URL.
 */

const LOOPBACK_HOSTS = new Set(["localhost", "127.0.0.1", "::1", "[::1]", "0.0.0.0"]);

export function isLoopbackHost(hostname: string): boolean {
  return LOOPBACK_HOSTS.has(hostname.toLowerCase());
}

/** Normalize for display and comparison: scheme + host + port, no trailing slash. */
export function baseUrlOrigin(raw: string): string {
  try {
    return new URL(raw).origin;
  } catch {
    return "";
  }
}

export type AssertApiBaseUrlOptions = {
  /** Permit plaintext HTTP to a non-loopback host (operator-acked at connect time). */
  allowInsecureHttp?: boolean;
};

/**
 * Hard rules for anything the IDE will send a bearer token to. Throws with an operator-
 * readable message rather than returning a flag, so no caller can forget to check.
 */
export function assertApiBaseUrl(raw: unknown, options: AssertApiBaseUrlOptions = {}): string {
  const value = typeof raw === "string" ? raw.trim() : "";
  if (!value) throw new Error("Install URL is required");

  let url: URL;
  try {
    url = new URL(value);
  } catch {
    throw new Error(`Install URL is not a valid URL: ${value}`);
  }

  if (url.protocol !== "https:" && url.protocol !== "http:") {
    throw new Error(`Install URL must be http(s), got ${url.protocol}`);
  }
  if (url.username || url.password) {
    throw new Error("Install URL must not embed credentials");
  }
  if (url.search || url.hash) {
    throw new Error("Install URL must not include a query string or fragment");
  }
  if (url.protocol === "http:" && !isLoopbackHost(url.hostname) && !options.allowInsecureHttp) {
    throw new Error(
      `Install URL must use https (${url.host} is not loopback). Plaintext HTTP requires an explicit acknowledgement at connect time.`,
    );
  }

  return value.replace(/\/+$/, "");
}

export type BaseUrlVerdict =
  | { ok: true; url: string; insecure: boolean }
  | { ok: false; error: string; needsInsecureAck?: boolean };

/**
 * Connect-time policy. `allowInsecureHttp` is the operator's explicit acceptance that the
 * JWT will cross the network in cleartext — self-hosted installs behind a VPN are a real
 * case, but it must be a decision rather than a default.
 */
export function checkInstallBaseUrl(
  raw: unknown,
  options: { allowInsecureHttp?: boolean } = {},
): BaseUrlVerdict {
  let url: string;
  try {
    // Shape check only here — the insecure-http rule is reported as needsInsecureAck below
    // so the Connect UI can show a confirmation rather than a hard error.
    url = assertApiBaseUrl(raw, { allowInsecureHttp: true });
  } catch (e) {
    return { ok: false, error: e instanceof Error ? e.message : String(e) };
  }

  const parsed = new URL(url);
  const insecure = parsed.protocol === "http:" && !isLoopbackHost(parsed.hostname);
  if (insecure && !options.allowInsecureHttp) {
    return {
      ok: false,
      needsInsecureAck: true,
      error: `${parsed.host} would receive your Majesta One token over plaintext HTTP. Use https, or accept the risk explicitly below.`,
    };
  }

  return { ok: true, url, insecure };
}
