import type { WorkspaceMode } from "./workspace/types";
import { MODE_WORKSPACE_TOOLS, MODES, SETTINGS_WORKSPACE_TOOLS, type TileId } from "./workspace/types";

/** Normalize Actor.scopes from /client/v1/me into lowercase family names. */
export function normalizeScopes(raw: unknown): string[] {
  if (Array.isArray(raw)) {
    return raw.map((s) => String(s).toLowerCase().trim()).filter(Boolean);
  }
  if (typeof raw === "string" && raw.trim()) {
    return raw
      .split(/[,\s+]+/)
      .map((s) => s.toLowerCase().trim())
      .filter(Boolean);
  }
  return [];
}

/** Exact family scope match (no substring). Admin scope grants all families for chrome. */
export function hasScope(scopes: string[] | undefined, family: string): boolean {
  if (!scopes || scopes.length === 0) return true; // unknown → show all (pre-connect)
  const f = family.toLowerCase();
  return scopes.some((s) => s === f || s === "admin");
}

/** Normalize systemCapabilities / systemPermissions from /me or session. */
export function normalizeCapabilities(raw: unknown): string[] {
  if (Array.isArray(raw)) {
    return raw.map((s) => String(s).toLowerCase().trim()).filter(Boolean);
  }
  return [];
}

export function hasCapability(
  caps: string[] | undefined,
  capability: string,
  opts?: { isAdmin?: boolean; failClosed?: boolean },
): boolean {
  if (opts?.isAdmin) return true;
  const failClosed = Boolean(opts?.failClosed);
  if (!caps || caps.length === 0) {
    // Pre-connect / caps not loaded: allow unless caller requires fail-closed.
    return !failClosed;
  }
  const want = capability.toLowerCase();
  if (caps.some((c) => c === want)) return true;
  // Legacy identity.manage expands to both identity.users and identity.integrations.
  if (
    (want === "identity.users" || want === "identity.integrations") &&
    caps.some((c) => c === "identity.manage")
  ) {
    return true;
  }
  // BP-057: legacy ide.run / ide.ship caps alias to four-tile launcher modes.
  if (want === "ide.operate" && caps.some((c) => c === "ide.run")) return true;
  if (want === "ide.build" && caps.some((c) => c === "ide.ship")) return true;
  if (
    want === "ide.settings.env" &&
    caps.some((c) => c === "ide.ship.env" || c === "ide.govern.env")
  ) {
    return true;
  }
  // Build deploy: ide.ship.deploy aliases under ide.build.
  if (
    want === "ide.ship.deploy" &&
    caps.some((c) => c === "ide.build" || c === "ide.ship")
  ) {
    return true;
  }
  return false;
}

/** Mode → ide.* capability. */
export const MODE_IDE_CAP: Record<WorkspaceMode, string> = {
  operate: "ide.operate",
  build: "ide.build",
  govern: "ide.govern",
};

/** Tool → ide.{mode}.{tile} capability (rail tools). */
export const TOOL_IDE_CAP: Partial<Record<TileId, string>> = {
  query: "ide.operate.query",
  monitor: "ide.operate.monitor",
  explorer: "ide.operate.explorer",
  runGraph: "ide.run.tools",
  runTool: "ide.run.tools",
  objectHome: "ide.run.tools",
  objects: "ide.build.objects",
  packages: "ide.build.packages",
  agentSpecs: "ide.build.agentSpecs",
  tools: "ide.build.tools",
  repo: "ide.build.repo",
  deploy: "ide.ship.deploy",
  env: "ide.settings.env",
  users: "ide.govern.users",
  integrations: "ide.govern.integrations",
  experiences: "ide.govern.experiences",
  installAuth: "ide.govern.installAuth",
  permissions: "ide.govern.permissions",
  account: "ide.settings.account",
  hosting: "ide.settings.hosting",
  inference: "ide.settings.inference",
};

/** Settings section capability (launcher Settings tile). */
export const SETTINGS_IDE_CAP = "ide.settings";

/** All ide.* capabilities for PermissionsPanel checkboxes. */
export const IDE_CAPABILITY_GROUPS: { label: string; caps: { id: string; label: string }[] }[] = [
  {
    label: "Modes",
    caps: [
      { id: "ide.operate", label: "Operate" },
      { id: "ide.build", label: "Build" },
      { id: "ide.govern", label: "Govern" },
      { id: "ide.settings", label: "Settings" },
    ],
  },
  {
    label: "Operate tools",
    caps: [{ id: "ide.run.tools", label: "Graph & Tools" }],
  },
  {
    label: "Build tools",
    caps: [
      { id: "ide.build.objects", label: "Objects" },
      { id: "ide.build.packages", label: "Packages" },
      { id: "ide.build.agentSpecs", label: "Agents" },
      { id: "ide.build.tools", label: "Tools" },
      { id: "ide.build.repo", label: "Repo" },
      { id: "ide.ship.deploy", label: "Deploy Pipeline" },
      { id: "ide.operate.query", label: "Query" },
      { id: "ide.operate.monitor", label: "Monitor" },
      { id: "ide.operate.explorer", label: "Explorer" },
    ],
  },
  {
    label: "Govern tools",
    caps: [
      { id: "ide.govern.users", label: "Users" },
      { id: "ide.govern.integrations", label: "Integrations" },
      { id: "ide.govern.experiences", label: "Experiences" },
      { id: "ide.govern.installAuth", label: "Install auth" },
      { id: "ide.govern.permissions", label: "Permissions" },
    ],
  },
  {
    label: "Settings tools",
    caps: [
      { id: "ide.settings.account", label: "Account" },
      { id: "ide.settings.hosting", label: "Hosting" },
      { id: "ide.settings.inference", label: "Inference" },
      { id: "ide.settings.env", label: "Environments" },
    ],
  },
];

export type ToolsForModeOpts = {
  systemPermissions?: string[];
  isAdmin?: boolean;
};

/** True when Settings launcher tile may open (fail-closed after /me caps load). */
export function canOpenSettings(
  scopes: string[] | undefined,
  opts?: ToolsForModeOpts,
): boolean {
  if (!scopes || scopes.length === 0) return true;
  const caps = normalizeCapabilities(opts?.systemPermissions);
  const isAdmin = Boolean(opts?.isAdmin);
  const capsKnown = caps.length > 0 || isAdmin;
  if (!capsKnown) return true;
  return (
    hasCapability(caps, SETTINGS_IDE_CAP, { isAdmin, failClosed: true }) ||
    hasCapability(caps, "ide.settings.env", { isAdmin, failClosed: true })
  );
}

/** Settings tool rail filtered by ide.settings.* (and optional family scopes). */
export function toolsForSettings(
  scopes: string[] | undefined,
  opts?: ToolsForModeOpts,
): TileId[] {
  const base = SETTINGS_WORKSPACE_TOOLS;
  const caps = normalizeCapabilities(opts?.systemPermissions);
  const isAdmin = Boolean(opts?.isAdmin);
  if (!scopes || scopes.length === 0) return base;
  const capsKnown = caps.length > 0 || isAdmin;
  return base.filter((id) => {
    if (id === "hosting") {
      if (capsKnown && !hasCapability(caps, "ide.settings.hosting", { isAdmin, failClosed: true })) {
        return false;
      }
      return true;
    }
    if (id === "account") {
      if (capsKnown && !hasCapability(caps, "ide.settings.account", { isAdmin, failClosed: true })) {
        return false;
      }
      return true;
    }
    if (id === "inference") {
      if (capsKnown && !hasCapability(caps, "ide.settings.inference", { isAdmin, failClosed: true })) {
        return false;
      }
      return true;
    }
    if (id === "env") {
      if (capsKnown && !hasCapability(caps, "ide.settings.env", { isAdmin, failClosed: true })) {
        return false;
      }
      return hasScope(scopes, "deploy") || hasScope(scopes, "client");
    }
    return true;
  });
}

/** Modes visible given JWT family scopes + ide.* caps. Empty scopes = all (not yet loaded). */
export function modesForScopes(
  scopes: string[] | undefined,
  opts?: { systemPermissions?: string[]; isAdmin?: boolean },
): WorkspaceMode[] {
  if (!scopes || scopes.length === 0) return MODES.map((m) => m.id);
  const caps = normalizeCapabilities(opts?.systemPermissions);
  const isAdmin = Boolean(opts?.isAdmin);
  const capsKnown = caps.length > 0 || isAdmin;
  const out: WorkspaceMode[] = [];
  if (
    hasScope(scopes, "client") &&
    (!capsKnown || hasCapability(caps, MODE_IDE_CAP.operate, { isAdmin, failClosed: true }))
  ) {
    out.push("operate");
  }
  if (
    (hasScope(scopes, "metadata") || hasScope(scopes, "deploy")) &&
    (!capsKnown || hasCapability(caps, MODE_IDE_CAP.build, { isAdmin, failClosed: true }))
  ) {
    out.push("build");
  }
  if (
    (hasScope(scopes, "client") ||
      hasScope(scopes, "deploy") ||
      hasScope(scopes, "ops") ||
      hasScope(scopes, "admin")) &&
    (!capsKnown || hasCapability(caps, MODE_IDE_CAP.govern, { isAdmin, failClosed: true }))
  ) {
    out.push("govern");
  }
  return out.length > 0 ? out : [];
}

export function toolsForMode(
  mode: WorkspaceMode,
  scopes: string[] | undefined,
  opts?: ToolsForModeOpts,
): TileId[] {
  const base = MODE_WORKSPACE_TOOLS[mode];
  const caps = normalizeCapabilities(opts?.systemPermissions);
  const isAdmin = Boolean(opts?.isAdmin);
  if (!scopes || scopes.length === 0) {
    return base;
  }
  const capsKnown = caps.length > 0 || isAdmin;
  return base.filter((id) => {
    const ideCap = TOOL_IDE_CAP[id];
    // Family scope gates (API prerequisite).
    let familyOk: boolean;
    switch (id) {
      case "crm":
      case "client":
      case "query":
      case "explorer":
      case "monitor":
        familyOk = hasScope(scopes, "client") || hasScope(scopes, "metadata");
        break;
      case "runGraph":
      case "runTool":
      case "objectHome":
        familyOk = hasScope(scopes, "client") || hasScope(scopes, "metadata");
        break;
      case "objects":
      case "packages":
      case "agentSpecs":
      case "tools":
      case "automations":
      case "metadata":
      case "repo":
        familyOk = hasScope(scopes, "metadata") || hasScope(scopes, "client");
        break;
      case "deploy":
        familyOk = hasScope(scopes, "deploy");
        break;
      case "users":
      case "integrations":
      case "experiences":
      case "installAuth":
      case "permissions":
        familyOk = hasScope(scopes, "client") || hasScope(scopes, "metadata") || hasScope(scopes, "admin");
        break;
      default:
        familyOk = true;
    }
    if (!familyOk) return false;

    if (capsKnown && ideCap) {
      if (!hasCapability(caps, ideCap, { isAdmin, failClosed: true })) return false;
    }

    // Extra server-capability gates for useful Monitor data.
    if (id === "monitor" && capsKnown) {
      return (
        hasCapability(caps, "debug.read", { isAdmin, failClosed: true }) ||
        hasCapability(caps, "debug.trace", { isAdmin, failClosed: true })
      );
    }
    if (id === "automations" && capsKnown) {
      return hasCapability(caps, "metadata.build", { isAdmin, failClosed: true });
    }
    return true;
  });
}

export function extractActorScopes(me: Record<string, unknown>): {
  scopes: string[];
  isAdmin: boolean;
} {
  const scopes = normalizeScopes(me.scopes ?? me.scope ?? me.api_scopes);
  const isAdmin = Boolean(me.isAdmin ?? me.is_admin ?? scopes.includes("admin"));
  return { scopes, isAdmin };
}
