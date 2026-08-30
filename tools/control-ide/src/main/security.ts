/**
 * Pure security-policy helpers for the Electron shell. Kept free of `electron` imports so
 * both the main process and `vite.config.ts` can use them, and so every rule is unit
 * testable. See docs/architecture/control-ide-security-audit.md.
 */

export type CspMode = "production" | "development";

export type FrameTrust = {
  /** Vite dev server origin, when running `npm run dev` / `electron:dev`. */
  devServerUrl?: string;
  /** `file://` URL of the packaged renderer entry point. */
  appIndexUrl?: string;
};

/**
 * Content-Security-Policy for the renderer (CIDE-03).
 *
 * `connect-src` stays scheme-restricted rather than host-pinned: the install base URL is
 * operator-configurable at runtime and a document CSP cannot be widened without a reload.
 * Host-level control for API traffic is the base-URL validation in the renderer; this
 * policy's job is to stop injected remote script, framing, and form exfiltration.
 */
export function buildCsp(mode: CspMode, devServerUrl?: string): string {
  const dev = mode === "development";
  const loopback = ["http://localhost:*", "http://127.0.0.1:*"];
  const script = dev ? ["'self'", "'unsafe-inline'", "'unsafe-eval'"] : ["'self'"];
  const connect = ["'self'", "https:", ...loopback];
  if (dev) {
    connect.push("ws://localhost:*", "ws://127.0.0.1:*");
    if (devServerUrl) connect.push(devServerUrl);
  }

  return [
    "default-src 'none'",
    `script-src ${script.join(" ")}`,
    // Monaco injects <style> elements at runtime; it has no nonce hook.
    "style-src 'self' 'unsafe-inline'",
    "font-src 'self' data:",
    "img-src 'self' data: blob:",
    `connect-src ${connect.join(" ")}`,
    // Monaco language workers are constructed from blob URLs.
    "worker-src 'self' blob:",
    "frame-src 'none'",
    "child-src 'none'",
    "object-src 'none'",
    "base-uri 'none'",
    "form-action 'none'",
  ].join("; ");
}

function withoutFragment(raw: string): string {
  const hash = raw.indexOf("#");
  return hash === -1 ? raw : raw.slice(0, hash);
}

function isLoopbackHost(host: string): boolean {
  return host === "localhost" || host === "127.0.0.1" || host === "[::1]" || host === "::1";
}

/**
 * True only for the app's own renderer document. Used to gate navigation and to validate
 * the sender of every IPC message (CIDE-06).
 */
export function isTrustedFrameUrl(raw: string, trust: FrameTrust): boolean {
  const url = withoutFragment(String(raw ?? ""));
  if (!url) return false;

  if (trust.devServerUrl) {
    try {
      if (new URL(url).origin === new URL(trust.devServerUrl).origin) return true;
    } catch {
      /* not a parseable URL — fall through */
    }
  }

  if (trust.appIndexUrl) {
    const expected = withoutFragment(trust.appIndexUrl);
    const [path] = url.split("?");
    if (url === expected || path === expected) return true;
  }

  return false;
}

/**
 * URLs the main process is willing to hand to the OS browser. Loopback `http` is allowed
 * because the default install base URL is `http://localhost:8080` (CIDE-01).
 */
export function isSafeExternalUrl(raw: string): boolean {
  let url: URL;
  try {
    url = new URL(String(raw ?? ""));
  } catch {
    return false;
  }
  if (url.username || url.password) return false;
  if (url.protocol === "https:") return true;
  return url.protocol === "http:" && isLoopbackHost(url.hostname);
}

/**
 * Reject values that `git` would parse as an option rather than an operand. Without this,
 * a `--upload-pack=<cmd>` clone URL is executed as a shell command (CIDE-05).
 */
export function assertSafeGitArg(value: unknown, label: string): string {
  const arg = typeof value === "string" ? value.trim() : "";
  if (!arg) throw new Error(`${label} is required`);
  if (arg.startsWith("-")) throw new Error(`${label} must not start with "-"`);
  if (/[\0\n\r]/.test(arg)) throw new Error(`${label} contains a control character`);
  return arg;
}

/** Customer change branches must use the `change/` prefix (also rejects option-shaped names). */
export function assertChangeBranchName(value: unknown): string {
  const name = assertSafeGitArg(value, "Branch");
  if (!name.startsWith("change/")) {
    throw new Error("Branch must start with change/");
  }
  return name;
}

const CLONE_URL_ALLOWED = /^(https:\/\/[^\s]+|ssh:\/\/[^\s]+|git@[^\s:]+:[^\s]+)$/;

/** Clone sources are API-supplied (`customerRepoUrl`), so the scheme is allowlisted (CIDE-13). */
export function assertSafeCloneUrl(value: unknown): string {
  const url = assertSafeGitArg(value, "Clone URL");
  if (!CLONE_URL_ALLOWED.test(url)) {
    throw new Error("Clone URL must be https://, ssh://, or git@host:path");
  }
  return url;
}

/**
 * Config applied to `git clone` only, where the remote is untrusted: no `ext::`/`file::`
 * remote helpers, no hook execution, no implicit submodule fetch.
 */
export const GIT_CLONE_SAFE_CONFIG = [
  "-c",
  "protocol.ext.allow=never",
  "-c",
  "protocol.file.allow=never",
  "-c",
  "core.hooksPath=/dev/null",
] as const;

/** Environment for every git invocation: never block on an interactive prompt. */
export const GIT_SAFE_ENV = {
  GIT_TERMINAL_PROMPT: "0",
  GIT_ASKPASS: "",
} as const;

/** Environment for `git clone`: restrict the transport set at the git level too. */
export const GIT_CLONE_SAFE_ENV = {
  ...GIT_SAFE_ENV,
  GIT_ALLOW_PROTOCOL: "https:ssh",
} as const;
