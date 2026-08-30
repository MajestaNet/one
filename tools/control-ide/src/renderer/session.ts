/** Multi-environment session helpers for Control IDE. */

export type EnvConnection = {
  installId: string;
  installRole: string;
  baseUrl: string;
  token: string;
  scopes?: string[];
  isAdmin?: boolean;
  systemPermissions?: string[];
  label?: string;
  productVersion?: string;
  apiRevisionPin?: number;
  apiRevisionMin?: number;
  apiRevisionCurrent?: number;
  compatStatus?: "ok" | "warn" | "block" | "overridden";
  compatCode?: string;
  /** Human identity from `/client/v1/me` — used by the account chip, never the JWT. */
  displayName?: string;
  email?: string;
  principalType?: string;
  /** Opaque refresh token from interactive grants — encrypted session.bin only (CIDE-10). */
  refreshToken?: string;
  /** Epoch ms when the access JWT expires (`Date.now() + expires_in * 1000`). */
  accessExpiresAt?: number;
};

export type Session = {
  /** Active install — drives all API calls and agent runs. */
  activeInstallId: string;
  environments: EnvConnection[];
  repoPath?: string;
  customerRepoUrl?: string;
  deviceId?: string;
  /** Mirrored from the active EnvConnection for convenience / legacy readers. */
  baseUrl: string;
  token: string;
  scopes?: string[];
  isAdmin?: boolean;
  systemPermissions?: string[];
  /**
   * Operator explicitly accepted plaintext HTTP for a non-loopback install URL.
   * Required for apiFetch to talk to that host (CIDE-08).
   */
  allowInsecureHttp?: boolean;
  displayName?: string;
  email?: string;
  principalType?: string;
  refreshToken?: string;
  accessExpiresAt?: number;
};

export type PeerHint = {
  installId: string;
  installRole?: string;
  label?: string;
  baseUrl?: string;
  active?: boolean;
};

function optionalNonEmptyString(value: unknown): string | undefined {
  return typeof value === "string" && value ? value : undefined;
}

function optionalFiniteNumber(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

/** Copy refresh fields from a persisted env blob; drop non-string / non-number junk. */
function withRefreshFields<T extends { refreshToken?: unknown; accessExpiresAt?: unknown }>(
  env: T,
): T & { refreshToken?: string; accessExpiresAt?: number } {
  return {
    ...env,
    refreshToken: optionalNonEmptyString(env.refreshToken),
    accessExpiresAt: optionalFiniteNumber(env.accessExpiresAt),
  };
}

function installIdFromBaseUrl(baseUrl: string): string {
  const cleaned = baseUrl.replace(/\/$/, "");
  try {
    const u = new URL(cleaned);
    return `host-${u.host.replace(/[^a-zA-Z0-9.-]/g, "-")}`;
  } catch {
    return `host-${cleaned.replace(/[^a-zA-Z0-9.-]/g, "-").slice(0, 48) || "unknown"}`;
  }
}

/** Apply active env fields onto top-level session mirrors. */
export function withActiveMirrors(session: Session): Session {
  const active =
    session.environments.find((e) => e.installId === session.activeInstallId) ??
    session.environments[0];
  if (!active) {
    return session;
  }
  return {
    ...session,
    activeInstallId: active.installId,
    baseUrl: active.baseUrl,
    token: active.token,
    scopes: active.scopes,
    isAdmin: active.isAdmin,
    systemPermissions: active.systemPermissions,
    displayName: active.displayName,
    email: active.email,
    principalType: active.principalType,
    refreshToken: active.refreshToken,
    accessExpiresAt: active.accessExpiresAt,
  };
}

export function activeConnection(session: Session | null | undefined): EnvConnection | null {
  if (!session?.environments?.length) return null;
  return (
    session.environments.find((e) => e.installId === session.activeInstallId) ??
    session.environments[0] ??
    null
  );
}

export function isConnected(session: Session | null | undefined): boolean {
  const a = activeConnection(session);
  return Boolean(a?.baseUrl && a?.token);
}

/** Migrate legacy single-env session blobs and normalize mirrors. */
export function normalizeSession(raw: unknown): Session | null {
  if (!raw || typeof raw !== "object") return null;
  const s = raw as Record<string, unknown>;

  if (Array.isArray(s.environments) && s.environments.length > 0) {
    const environments = (s.environments as EnvConnection[])
      .filter((e) => e && typeof e.baseUrl === "string" && typeof e.token === "string")
      .map((e) => withRefreshFields(e));
    if (!environments.length) return null;
    const activeInstallId =
      typeof s.activeInstallId === "string" &&
      environments.some((e) => e.installId === s.activeInstallId)
        ? (s.activeInstallId as string)
        : environments[0].installId;
    return withActiveMirrors({
      activeInstallId,
      environments,
      repoPath: typeof s.repoPath === "string" ? s.repoPath : undefined,
      customerRepoUrl: typeof s.customerRepoUrl === "string" ? s.customerRepoUrl : undefined,
      deviceId: typeof s.deviceId === "string" ? s.deviceId : undefined,
      allowInsecureHttp: s.allowInsecureHttp === true ? true : undefined,
      baseUrl: environments[0].baseUrl,
      token: environments[0].token,
    });
  }

  // Legacy flat session
  if (typeof s.baseUrl === "string" && typeof s.token === "string" && s.baseUrl && s.token) {
    const installId =
      typeof s.installId === "string" && s.installId
        ? s.installId
        : installIdFromBaseUrl(s.baseUrl);
    const env: EnvConnection = withRefreshFields({
      installId,
      installRole: typeof s.installRole === "string" && s.installRole ? s.installRole : "local",
      baseUrl: s.baseUrl,
      token: s.token,
      scopes: Array.isArray(s.scopes) ? (s.scopes as string[]) : undefined,
      isAdmin: Boolean(s.isAdmin),
      systemPermissions: Array.isArray(s.systemPermissions)
        ? (s.systemPermissions as string[])
        : undefined,
      refreshToken: s.refreshToken,
      accessExpiresAt: s.accessExpiresAt,
    });
    return withActiveMirrors({
      activeInstallId: installId,
      environments: [env],
      repoPath: typeof s.repoPath === "string" ? s.repoPath : undefined,
      customerRepoUrl: typeof s.customerRepoUrl === "string" ? s.customerRepoUrl : undefined,
      deviceId: typeof s.deviceId === "string" ? s.deviceId : undefined,
      allowInsecureHttp: s.allowInsecureHttp === true ? true : undefined,
      baseUrl: env.baseUrl,
      token: env.token,
    });
  }

  return null;
}

export function upsertEnvironment(
  session: Session | null,
  conn: EnvConnection,
  opts?: { makeActive?: boolean; repoPath?: string; customerRepoUrl?: string; deviceId?: string },
): Session {
  const makeActive = opts?.makeActive !== false;
  const existing = session?.environments ?? [];
  const idx = existing.findIndex(
    (e) => e.installId === conn.installId || e.baseUrl.replace(/\/$/, "") === conn.baseUrl.replace(/\/$/, ""),
  );
  const environments = [...existing];
  if (idx >= 0) {
    environments[idx] = { ...environments[idx], ...conn, installId: environments[idx].installId || conn.installId };
  } else {
    environments.push(conn);
  }
  const activeInstallId = makeActive
    ? idx >= 0
      ? environments[idx].installId
      : conn.installId
    : session?.activeInstallId && environments.some((e) => e.installId === session.activeInstallId)
      ? session.activeInstallId
      : environments[0].installId;

  return withActiveMirrors({
    activeInstallId,
    environments,
    repoPath: opts?.repoPath ?? session?.repoPath,
    customerRepoUrl: opts?.customerRepoUrl ?? session?.customerRepoUrl,
    deviceId: opts?.deviceId ?? session?.deviceId,
    baseUrl: conn.baseUrl,
    token: conn.token,
  });
}

export function switchActiveEnvironment(session: Session, installId: string): Session {
  if (!session.environments.some((e) => e.installId === installId)) {
    return session;
  }
  return withActiveMirrors({ ...session, activeInstallId: installId });
}

export function mergePeerHints(session: Session, peers: PeerHint[]): Session {
  const environments = [...session.environments];
  for (const p of peers) {
    if (!p.installId) continue;
    const idx = environments.findIndex((e) => e.installId === p.installId);
    if (idx >= 0) {
      environments[idx] = {
        ...environments[idx],
        installRole: p.installRole || environments[idx].installRole,
        label: p.label || environments[idx].label,
        baseUrl: p.baseUrl || environments[idx].baseUrl,
      };
    } else if (p.baseUrl) {
      environments.push({
        installId: p.installId,
        installRole: p.installRole || p.label || p.installId,
        label: p.label,
        baseUrl: p.baseUrl,
        token: "", // not connected yet
      });
    }
  }
  return withActiveMirrors({ ...session, environments });
}

export function envDisplayName(env: EnvConnection): string {
  return env.installRole || env.label || env.installId;
}

export type AccessTokenMeta = {
  refreshToken?: string;
  expiresIn?: number;
  now?: number;
};

/** Build EnvConnection after /me (+ optional /environment) for Connect. */
export function connectionFromActor(
  baseUrl: string,
  token: string,
  actor: Record<string, unknown>,
  envInfo?: Record<string, unknown> | null,
  tokenMeta?: AccessTokenMeta,
): EnvConnection {
  const scopes = Array.isArray(actor.scopes)
    ? (actor.scopes as string[]).map(String)
    : undefined;
  const systemPermissions = Array.isArray(actor.systemPermissions)
    ? (actor.systemPermissions as string[])
    : undefined;
  const installId =
    (typeof envInfo?.installId === "string" && envInfo.installId) ||
    (typeof actor.installId === "string" && actor.installId) ||
    installIdFromBaseUrl(baseUrl);
  const installRole =
    (typeof envInfo?.installRole === "string" && envInfo.installRole) ||
    (typeof actor.installRole === "string" && actor.installRole) ||
    "local";
  const now = tokenMeta?.now ?? Date.now();
  const expiresIn = optionalFiniteNumber(tokenMeta?.expiresIn);
  return {
    installId,
    installRole,
    baseUrl: baseUrl.replace(/\/$/, ""),
    token,
    scopes,
    isAdmin: Boolean(actor.isAdmin ?? actor.is_admin),
    systemPermissions,
    label: typeof envInfo?.label === "string" ? envInfo.label : undefined,
    displayName: pickActorDisplayName(actor),
    email: pickActorEmail(actor),
    principalType: typeof actor.principalType === "string" ? actor.principalType : undefined,
    refreshToken: optionalNonEmptyString(tokenMeta?.refreshToken),
    accessExpiresAt: expiresIn != null ? now + expiresIn * 1000 : undefined,
  };
}

/** Apply a rotated access JWT (and optional new refresh token) onto the active env. */
export function withRotatedTokens(
  session: Session,
  tokens: { accessToken: string; refreshToken?: string; expiresIn?: number },
  now = Date.now(),
): Session {
  const active = activeConnection(session);
  if (!active) return session;
  const expiresIn = optionalFiniteNumber(tokens.expiresIn);
  return upsertEnvironment(session, {
    ...active,
    token: tokens.accessToken,
    refreshToken: optionalNonEmptyString(tokens.refreshToken) ?? active.refreshToken,
    accessExpiresAt: expiresIn != null ? now + expiresIn * 1000 : active.accessExpiresAt,
  });
}

function firstNonEmptyString(...values: unknown[]): string | undefined {
  for (const v of values) {
    if (typeof v === "string" && v.trim()) return v.trim();
  }
  return undefined;
}

function pickActorEmail(actor: Record<string, unknown>): string | undefined {
  return firstNonEmptyString(actor.email, actor.userEmail);
}

function pickActorDisplayName(actor: Record<string, unknown>): string | undefined {
  return firstNonEmptyString(
    actor.displayName,
    actor.name,
    actor.principal,
    pickActorEmail(actor),
    typeof actor.apiKeyName === "string" ? actor.apiKeyName : undefined,
  );
}

export type SessionIdentity = {
  displayName: string;
  initials: string;
  email?: string;
};

/** Initials for a small avatar (two letters when possible). */
export function initialsFromName(name: string): string {
  const cleaned = name.trim();
  if (!cleaned) return "?";
  const emailLocal = cleaned.includes("@") ? cleaned.slice(0, cleaned.indexOf("@")) : "";
  const source = emailLocal || cleaned;
  const words = source.split(/[\s._-]+/).filter(Boolean);
  if (words.length >= 2) {
    return `${words[0]![0] ?? ""}${words[1]![0] ?? ""}`.toUpperCase();
  }
  const compact = source.replace(/[^A-Za-z0-9]/g, "");
  if (compact.length >= 2) return compact.slice(0, 2).toUpperCase();
  return (compact[0] ?? "?").toUpperCase();
}

/** Account chip copy from the active env / `/me` actor — never JWT/auth-method labels. */
export function sessionIdentity(session: Session | null | undefined): SessionIdentity | null {
  if (!isConnected(session)) return null;
  const active = activeConnection(session);
  const displayName =
    firstNonEmptyString(active?.displayName, session?.displayName, active?.email, session?.email) ??
    "Signed in";
  const email = firstNonEmptyString(active?.email, session?.email);
  return {
    displayName,
    initials: initialsFromName(displayName),
    email,
  };
}

/** Merge `/client/v1/me` identity onto the active environment. */
export function withActorIdentity(session: Session, actor: Record<string, unknown>): Session {
  const active = activeConnection(session);
  if (!active) return session;
  const conn: EnvConnection = {
    ...active,
    displayName: pickActorDisplayName(actor) ?? active.displayName,
    email: pickActorEmail(actor) ?? active.email,
    principalType:
      typeof actor.principalType === "string" ? actor.principalType : active.principalType,
    scopes: Array.isArray(actor.scopes) ? (actor.scopes as unknown[]).map(String) : active.scopes,
    isAdmin: actor.isAdmin === true || actor.is_admin === true || active.isAdmin,
    systemPermissions: Array.isArray(actor.systemPermissions)
      ? (actor.systemPermissions as string[])
      : active.systemPermissions,
  };
  return upsertEnvironment(session, conn, { makeActive: true });
}

export function isUnauthorizedError(err: unknown): boolean {
  const message = err instanceof Error ? err.message : String(err);
  return /^\s*401\b/.test(message);
}
