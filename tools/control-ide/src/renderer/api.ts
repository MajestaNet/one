export type {
  EnvConnection,
  Session,
  PeerHint,
} from "./session";
export {
  activeConnection,
  isConnected,
  normalizeSession,
  upsertEnvironment,
  switchActiveEnvironment,
  mergePeerHints,
  envDisplayName,
  connectionFromActor,
  withActiveMirrors,
  withRotatedTokens,
  sessionIdentity,
  initialsFromName,
  withActorIdentity,
  isUnauthorizedError,
} from "./session";
export type { AccessTokenMeta } from "./session";

import { redactSensitive } from "./errors";
import { assertApiBaseUrl } from "./installUrl";
import { CONTROL_IDE_INTEGRATION } from "./oauthPkce";
import { refreshAccessToken } from "./refreshSession";

export { assertApiBaseUrl, baseUrlOrigin, checkInstallBaseUrl, isLoopbackHost } from "./installUrl";
export { formatError, redactSensitive } from "./errors";
export { probeInstallVersion } from "./versionProbe";

export type UpdateStatus = {
  state: string;
  message: string;
  version?: string;
  progress?: number;
};

export type OneAPI = {
  getSession: () => Promise<import("./session").Session | null>;
  setSession: (
    s: import("./session").Session | null,
  ) => Promise<boolean | { ok: boolean; error?: string }>;
  isSessionEncryptionAvailable?: () => Promise<boolean>;
  registerRepoRoot?: (repoPath: string) => Promise<{ ok: boolean; path?: string; error?: string }>;
  chooseRepoRoot?: () => Promise<{ ok: boolean; path?: string; canceled?: boolean; error?: string }>;
  /** Pick any usable folder (empty OK). Does not require .git / one.yaml. */
  chooseLocalFolder?: () => Promise<{ ok: boolean; path?: string; canceled?: boolean; error?: string }>;
  openExternal?: (url: string) => Promise<{ ok: boolean; error?: string }>;
  gitStatus: (repoPath: string) => Promise<{ ok: boolean; branch?: string; status?: string; error?: string }>;
  exportRepoZip?: (
    repoPath: string,
    paths?: string[],
  ) => Promise<{ ok: boolean; base64?: string; error?: string }>;
  gitClone?: (
    url: string,
    destPath: string,
  ) => Promise<{ ok: boolean; path?: string; error?: string }>;
  gitPull?: (repoPath: string) => Promise<{ ok: boolean; branch?: string; error?: string }>;
  gitCreateBranch?: (
    repoPath: string,
    branch: string,
  ) => Promise<{ ok: boolean; branch?: string; error?: string }>;
  gitPush?: (repoPath: string) => Promise<{ ok: boolean; error?: string }>;
  gitCommit?: (
    repoPath: string,
    message: string,
  ) => Promise<{ ok: boolean; branch?: string; error?: string }>;
  initSampleRepo?: (
    destPath: string,
  ) => Promise<{ ok: boolean; path?: string; template?: string; error?: string }>;
  importExportZip?: (
    destPath: string,
    base64: string,
  ) => Promise<{ ok: boolean; path?: string; error?: string }>;
  openInEditor?: (
    repoPath: string,
    editor?: "code" | "auto" | string,
  ) => Promise<{ ok: boolean; editor?: string; error?: string }>;
  listTree: (root: string, rel?: string) => Promise<string[]>;
  readText: (root: string, rel: string) => Promise<string>;
  writeText: (root: string, rel: string, content: string) => Promise<boolean>;
  getUpdateStatus?: () => Promise<UpdateStatus>;
  checkForUpdates?: () => Promise<UpdateStatus>;
  installUpdate?: () => Promise<{ ok: boolean; error?: string }>;
  onOAuthCallback?: (handler: (url: string) => void) => () => void;
};

declare global {
  interface Window {
    one: OneAPI;
  }
}

export type ApiFetchOpts = {
  deviceId?: string;
  allowInsecureHttp?: boolean;
  apiRevisionPin?: number;
  refreshToken?: string;
  clientId?: string;
  onRotated?: (tokens: {
    accessToken: string;
    refreshToken?: string;
    expiresIn?: number;
  }) => void | Promise<void>;
  /** Set on the post-refresh retry so a second 401 cannot loop. */
  skipRefresh?: boolean;
};

export async function apiFetch(
  baseUrl: string,
  token: string,
  path: string,
  init: RequestInit = {},
  opts?: ApiFetchOpts,
) {
  // Single chokepoint for "where is this bearer token going" (CIDE-08).
  const url = `${assertApiBaseUrl(baseUrl, { allowInsecureHttp: opts?.allowInsecureHttp })}${path}`;
  const headers = new Headers(init.headers);
  headers.set("Authorization", `Bearer ${token}`);
  if (opts?.deviceId) {
    headers.set("X-One-Device-Id", opts.deviceId);
  }
  if (opts?.apiRevisionPin != null && Number.isFinite(opts.apiRevisionPin)) {
    headers.set("One-API-Revision", String(opts.apiRevisionPin));
  }
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const res = await fetch(url, { ...init, headers });
  const text = await res.text();
  let body: unknown = text;
  try {
    body = text ? JSON.parse(text) : null;
  } catch {
    /* keep text */
  }
  if (!res.ok) {
    const canRefresh =
      res.status === 401 &&
      !opts?.skipRefresh &&
      Boolean(opts?.refreshToken);
    if (canRefresh && opts?.refreshToken) {
      const rotated = await refreshAccessToken(baseUrl, opts.refreshToken, {
        allowInsecureHttp: opts.allowInsecureHttp,
        clientId: opts.clientId || CONTROL_IDE_INTEGRATION,
      });
      await opts.onRotated?.({
        accessToken: rotated.accessToken,
        refreshToken: rotated.refreshToken,
        expiresIn: rotated.expiresIn,
      });
      return apiFetch(baseUrl, rotated.accessToken, path, init, { ...opts, skipRefresh: true });
    }
    const detail = typeof body === "string" ? body : JSON.stringify(body);
    throw new Error(`${res.status} ${path}: ${redactSensitive(detail ?? "")}`);
  }
  return body;
}
