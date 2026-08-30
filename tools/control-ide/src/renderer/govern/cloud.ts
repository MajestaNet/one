/** Host-agnostic Deploy cloud helpers (JWT client of /deploy/v1/cloud/*). */

export type CloudBinding = {
  host?: string;
  appResourceId?: string;
  databaseResourceId?: string;
  region?: string;
  displayName?: string;
  providerMeta?: Record<string, unknown>;
  /** Compatibility aliases */
  appId?: string;
  databaseId?: string;
  appName?: string;
};

export type CloudStatus = {
  host?: string;
  configured?: boolean;
  reachable?: boolean;
  binding?: CloudBinding;
  capabilities?: Record<string, boolean>;
};

export type AppSummary = {
  appResourceId?: string;
  name?: string;
  region?: string;
  publicUrl?: string;
  apiInstances?: number;
  apiSizeClass?: string;
  workerInstances?: number;
  workerSizeClass?: string;
  apiImageTag?: string;
  apiImageDigest?: string;
  workerImageTag?: string;
  workerImageDigest?: string;
  /** Compatibility */
  appId?: string;
  apiSize?: string;
  workerSize?: string;
  apiInstanceCount?: number;
  apiInstanceSizeSlug?: string;
  workerInstanceCount?: number;
  workerInstanceSizeSlug?: string;
};

export type EnvironmentsPayload = {
  peers?: unknown[];
  provisionRuns?: Array<{
    id?: string;
    host?: string;
    peerInstallId?: string;
    installRole?: string;
    appResourceId?: string;
    databaseResourceId?: string;
    appId?: string;
    databaseId?: string;
    baseUrl?: string;
    status?: string;
    error?: string;
  }>;
};

export type FetchFn = (path: string, init?: RequestInit) => Promise<unknown>;

const CLOUD_PREFIX = "/deploy/v1/cloud";

/** True when any cloud adapter is active (prefer `cloud` / `cloudHost`; keep DO alias). */
export function cloudEnabled(env: Record<string, unknown> | null): boolean {
  if (!env) return false;
  const host = typeof env.cloudHost === "string" ? env.cloudHost.trim() : "";
  if (host) return true;
  const caps = env.capabilities;
  if (caps && typeof caps === "object" && !Array.isArray(caps)) {
    const c = caps as Record<string, unknown>;
    return Boolean(c.cloud || c.digitaloceanCloud || c.awsCloud);
  }
  if (Array.isArray(caps)) {
    const set = caps.map(String);
    return set.includes("cloud") || set.includes("digitaloceanCloud") || set.includes("awsCloud");
  }
  return false;
}

/** @deprecated Prefer cloudEnabled — DO-named alias during migration. */
export function digitaloceanCloudEnabled(env: Record<string, unknown> | null): boolean {
  if (!env) return false;
  if (typeof env.cloudHost === "string" && env.cloudHost === "digitalocean") return true;
  const caps = env.capabilities;
  if (caps && typeof caps === "object" && !Array.isArray(caps)) {
    return Boolean((caps as Record<string, unknown>).digitaloceanCloud);
  }
  if (Array.isArray(caps)) {
    return caps.map(String).includes("digitaloceanCloud");
  }
  return false;
}

export function normalizeAppSummary(raw: AppSummary): AppSummary {
  return {
    ...raw,
    appResourceId: raw.appResourceId || raw.appId,
    appId: raw.appId || raw.appResourceId,
    apiInstances: raw.apiInstances ?? raw.apiInstanceCount,
    apiInstanceCount: raw.apiInstanceCount ?? raw.apiInstances,
    apiSizeClass: raw.apiSizeClass || raw.apiSize || raw.apiInstanceSizeSlug,
    apiInstanceSizeSlug: raw.apiInstanceSizeSlug || raw.apiSize || raw.apiSizeClass,
    workerInstances: raw.workerInstances ?? raw.workerInstanceCount,
    workerInstanceCount: raw.workerInstanceCount ?? raw.workerInstances,
    workerSizeClass: raw.workerSizeClass || raw.workerSize || raw.workerInstanceSizeSlug,
    workerInstanceSizeSlug: raw.workerInstanceSizeSlug || raw.workerSize || raw.workerSizeClass,
  };
}

export function normalizeBinding(raw: CloudBinding | undefined): CloudBinding | undefined {
  if (!raw) return raw;
  return {
    ...raw,
    appResourceId: raw.appResourceId || raw.appId,
    appId: raw.appId || raw.appResourceId,
    databaseResourceId: raw.databaseResourceId || raw.databaseId,
    databaseId: raw.databaseId || raw.databaseResourceId,
    displayName: raw.displayName || raw.appName,
    appName: raw.appName || raw.displayName,
  };
}

export async function getCloudStatus(fetchApi: FetchFn): Promise<CloudStatus> {
  const st = (await fetchApi(`${CLOUD_PREFIX}/status`)) as CloudStatus;
  return { ...st, binding: normalizeBinding(st.binding) };
}

export async function getCloudApp(fetchApi: FetchFn): Promise<AppSummary> {
  return normalizeAppSummary((await fetchApi(`${CLOUD_PREFIX}/app`)) as AppSummary);
}

export async function putCloudBinding(
  fetchApi: FetchFn,
  body: {
    appResourceId?: string;
    databaseResourceId?: string;
    appId?: string;
    databaseId?: string;
    region?: string;
    displayName?: string;
    appName?: string;
  },
): Promise<CloudBinding> {
  const payload = {
    appResourceId: body.appResourceId || body.appId,
    databaseResourceId: body.databaseResourceId || body.databaseId,
    appId: body.appId || body.appResourceId,
    databaseId: body.databaseId || body.databaseResourceId,
    region: body.region,
    displayName: body.displayName || body.appName,
    appName: body.appName || body.displayName,
  };
  return normalizeBinding(
    (await fetchApi(`${CLOUD_PREFIX}/binding`, {
      method: "PUT",
      body: JSON.stringify(payload),
    })) as CloudBinding,
  ) as CloudBinding;
}

export async function scaleCloudApp(
  fetchApi: FetchFn,
  body: {
    apiInstanceCount?: number;
    apiSizeClass?: string;
    apiInstanceSizeSlug?: string;
    workerInstanceCount?: number;
    workerSizeClass?: string;
    workerInstanceSizeSlug?: string;
  },
): Promise<AppSummary> {
  const payload = {
    apiInstanceCount: body.apiInstanceCount,
    apiSizeClass: body.apiSizeClass || body.apiInstanceSizeSlug,
    apiInstanceSizeSlug: body.apiInstanceSizeSlug || body.apiSizeClass,
    workerInstanceCount: body.workerInstanceCount,
    workerSizeClass: body.workerSizeClass || body.workerInstanceSizeSlug,
    workerInstanceSizeSlug: body.workerInstanceSizeSlug || body.workerSizeClass,
  };
  return normalizeAppSummary(
    (await fetchApi(`${CLOUD_PREFIX}/app/scale`, {
      method: "PATCH",
      body: JSON.stringify(payload),
    })) as AppSummary,
  );
}

export async function resizeCloudDatabase(
  fetchApi: FetchFn,
  body: { sizeClass?: string; size?: string; numNodes?: number },
): Promise<unknown> {
  return fetchApi(`${CLOUD_PREFIX}/database/resize`, {
    method: "PATCH",
    body: JSON.stringify({
      sizeClass: body.sizeClass || body.size,
      size: body.size || body.sizeClass,
      numNodes: body.numNodes,
    }),
  });
}

export async function listCloudEnvironments(fetchApi: FetchFn): Promise<EnvironmentsPayload> {
  return (await fetchApi(`${CLOUD_PREFIX}/environments`)) as EnvironmentsPayload;
}

export async function provisionCloudEnvironment(
  fetchApi: FetchFn,
  body: Record<string, unknown>,
): Promise<unknown> {
  return fetchApi(`${CLOUD_PREFIX}/environments`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export const DO_CONSOLE_APPS = "https://cloud.digitalocean.com/apps";
