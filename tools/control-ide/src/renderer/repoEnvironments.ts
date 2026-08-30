/** Parse environments/*.yaml pointers from a customer repo (non-secret). */

export type RepoEnvironment = {
  alias: string;
  installId: string;
  installRole: string;
  baseUrl: string;
};

const ROLE_RANK: Record<string, number> = {
  test: 0,
  dev: 0,
  development: 0,
  staging: 1,
  uat: 1,
  stage: 1,
  prod: 2,
  production: 2,
};

function parseSimpleYaml(text: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const line of text.split(/\r?\n/)) {
    const m = /^([A-Za-z][\w]*)\s*:\s*(.*)$/.exec(line.trim());
    if (!m) continue;
    let v = m[2]!.trim();
    if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) {
      v = v.slice(1, -1);
    }
    out[m[1]!] = v;
  }
  return out;
}

export function rankInstallRole(role: string): number {
  return ROLE_RANK[role.toLowerCase()] ?? 10;
}

/** Load and order environments/*.yaml via listTree + readText. */
export async function loadRepoEnvironments(
  root: string,
  api: {
    listTree: (root: string, rel?: string) => Promise<string[]>;
    readText: (root: string, rel: string) => Promise<string>;
  },
): Promise<RepoEnvironment[]> {
  let files: string[];
  try {
    files = await api.listTree(root, "environments");
  } catch {
    return [];
  }
  const envs: RepoEnvironment[] = [];
  for (const rel of files) {
    const n = rel.replace(/\\/g, "/");
    if (!n.startsWith("environments/") || !/\.ya?ml$/i.test(n)) continue;
    const stem = n.replace(/^environments\//, "").replace(/\.ya?ml$/i, "");
    try {
      const text = await api.readText(root, n);
      const raw = parseSimpleYaml(text);
      const installRole = raw.installRole || stem;
      const alias = raw.alias || stem;
      envs.push({
        alias,
        installId: raw.installId || "",
        installRole,
        baseUrl: raw.baseUrl || "",
      });
    } catch {
      /* skip unreadable */
    }
  }
  envs.sort((a, b) => {
    const d = rankInstallRole(a.installRole) - rankInstallRole(b.installRole);
    if (d !== 0) return d;
    return a.alias.localeCompare(b.alias);
  });
  return envs;
}

/** Order session env connections using repo environments/*.yaml when present. */
export function orderStagesByRepoEnv<T extends { installId: string; installRole?: string; label?: string; baseUrl?: string }>(
  sessionEnvs: T[],
  repoEnvs: RepoEnvironment[],
): T[] {
  if (!sessionEnvs.length) return sessionEnvs;
  if (!repoEnvs.length) {
    return [...sessionEnvs].sort((a, b) => {
      const ra = rankInstallRole(a.installRole || a.label || "");
      const rb = rankInstallRole(b.installRole || b.label || "");
      if (ra !== rb) return ra - rb;
      return (a.installRole || "").localeCompare(b.installRole || "");
    });
  }
  const byId = new Map(sessionEnvs.map((e) => [e.installId, e]));
  const byRole = new Map(sessionEnvs.map((e) => [(e.installRole || "").toLowerCase(), e]));
  const byUrl = new Map(
    sessionEnvs.map((e) => [(e.baseUrl || "").replace(/\/$/, "").toLowerCase(), e]),
  );
  const ordered: T[] = [];
  const used = new Set<string>();
  for (const r of repoEnvs) {
    const match =
      (r.installId && byId.get(r.installId)) ||
      byRole.get(r.installRole.toLowerCase()) ||
      byRole.get(r.alias.toLowerCase()) ||
      (r.baseUrl ? byUrl.get(r.baseUrl.replace(/\/$/, "").toLowerCase()) : undefined);
    if (match && !used.has(match.installId)) {
      ordered.push(match);
      used.add(match.installId);
    }
  }
  for (const e of sessionEnvs) {
    if (!used.has(e.installId)) ordered.push(e);
  }
  return ordered;
}
