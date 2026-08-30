/** Pure helpers for gated electron-updater (testable without Electron). */

export type UpdateState =
  | "disabled"
  | "idle"
  | "checking"
  | "available"
  | "not-available"
  | "downloading"
  | "downloaded"
  | "error";

export type UpdateStatus = {
  state: UpdateState;
  message: string;
  version?: string;
  progress?: number;
};

export type UpdateGateInput = {
  packaged: boolean;
  feedUrl: string | undefined;
  feedError?: string;
};

/**
 * Validate UPDATE_FEED_URL. Requires https, no embedded credentials, and a host that appears
 * in UPDATE_FEED_HOST_ALLOWLIST (comma-separated). Until signed artifacts exist (ADR-030 freeze), the
 * allowlist is the only authenticity control on the feed (CIDE-07).
 */
export function validateFeedUrl(raw: string, allowlistRaw?: string): string {
  const trimmed = raw.trim();
  if (!trimmed) throw new Error("UPDATE_FEED_URL is empty");

  let url: URL;
  try {
    url = new URL(trimmed);
  } catch {
    throw new Error("UPDATE_FEED_URL is not a valid URL");
  }
  if (url.protocol !== "https:") {
    throw new Error("UPDATE_FEED_URL must use https");
  }
  if (url.username || url.password) {
    throw new Error("UPDATE_FEED_URL must not embed credentials");
  }

  const hosts = (allowlistRaw ?? "")
    .split(",")
    .map((h) => h.trim().toLowerCase())
    .filter(Boolean);
  if (hosts.length === 0) {
    throw new Error(
      "UPDATE_FEED_HOST_ALLOWLIST is required (comma-separated hosts) before updates can be enabled",
    );
  }
  if (!hosts.includes(url.hostname.toLowerCase())) {
    throw new Error(
      `UPDATE_FEED_URL host "${url.hostname}" is not in UPDATE_FEED_HOST_ALLOWLIST`,
    );
  }

  return trimmed.replace(/\/+$/, "");
}

export type ResolvedFeed =
  | { url: string }
  | { url: undefined; error?: string };

export function resolveFeedUrl(env: NodeJS.ProcessEnv = process.env): ResolvedFeed {
  const raw = env.UPDATE_FEED_URL?.trim();
  if (!raw) return { url: undefined };
  try {
    return { url: validateFeedUrl(raw, env.UPDATE_FEED_HOST_ALLOWLIST) };
  } catch (err) {
    return { url: undefined, error: err instanceof Error ? err.message : String(err) };
  }
}

export function shouldEnableUpdates(input: UpdateGateInput): boolean {
  return Boolean(input.packaged && input.feedUrl && !input.feedError);
}

export function disabledStatus(reason: string): UpdateStatus {
  return { state: "disabled", message: reason };
}

export function gateDisabledReason(input: UpdateGateInput): string {
  if (!input.packaged) {
    return "Updates require a packaged desktop build.";
  }
  if (input.feedError) {
    return input.feedError;
  }
  if (!input.feedUrl) {
    return "Updates disabled until UPDATE_FEED_URL and UPDATE_FEED_HOST_ALLOWLIST are configured (see ADR-030).";
  }
  return "Updates disabled.";
}

/** Minimal surface we need from electron-updater for wiring + tests. */
export type UpdaterLike = {
  setFeedURL: (opts: { provider: "generic"; url: string }) => void;
  checkForUpdates: () => Promise<unknown>;
  quitAndInstall: (isSilent?: boolean, isForceRunAfter?: boolean) => void;
  on: (event: string, listener: (...args: never[]) => void) => unknown;
  autoDownload: boolean;
};

/**
 * Wire the updater. `autoDownload` stays off until installers are signed (ADR-030 freeze) — an
 * unsigned generic feed is integrity-by-HTTPS-only, so the operator must opt in to download.
 */
export function configureUpdater(updater: UpdaterLike, feedUrl: string): void {
  updater.autoDownload = false;
  updater.setFeedURL({ provider: "generic", url: feedUrl.replace(/\/$/, "") });
}
