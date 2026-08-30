/** Thin Metadata API helpers for permission set definitions. */

export type ApiFetch = (path: string, init?: RequestInit) => Promise<unknown>;

export type ToolAccessEntry = {
  apiName: string;
  canOpen: boolean;
  canInteract: boolean;
  canModify: boolean;
  canPublish: boolean;
};

export type ToolAccess = {
  allTools?: boolean;
  tools?: ToolAccessEntry[];
};

export type PermissionSet = {
  id?: string;
  apiName: string;
  label?: string;
  description?: string;
  isSystem?: boolean;
  systemPermissions?: string[];
  createdAt?: string;
  dataAccess?: {
    objectPermissions?: unknown[];
    fieldPermissions?: unknown[];
  };
  automationAccess?: unknown;
  toolAccess?: ToolAccess;
  [key: string]: unknown;
};

export type CreatePermissionSetInput = {
  apiName: string;
  label: string;
  description?: string;
  systemPermissions?: string[];
  objectPermissions?: unknown[];
  fieldPermissions?: unknown[];
  dataAccess?: unknown;
  automationAccess?: unknown;
  toolAccess?: ToolAccess;
  allAutomations?: boolean;
  allTools?: boolean;
};

export type PatchPermissionSetInput = {
  label?: string;
  description?: string;
  systemPermissions?: string[];
  systemPermissionsAdd?: string[];
  systemPermissionsRemove?: string[];
  dataAccess?: unknown;
  automationAccess?: unknown;
  toolAccess?: ToolAccess;
};

export async function listPermissionSets(
  fetchApi: ApiFetch,
  opts?: {
    includeDataAccess?: boolean;
    includeAutomationAccess?: boolean;
    includeToolAccess?: boolean;
  },
): Promise<PermissionSet[]> {
  const parts: string[] = [];
  if (opts?.includeDataAccess) parts.push("dataAccess");
  if (opts?.includeAutomationAccess) parts.push("automationAccess");
  if (opts?.includeToolAccess) parts.push("toolAccess");
  const q = parts.length ? `?include=${parts.join(",")}` : "";
  const res = (await fetchApi(`/metadata/v1/permissions/sets${q}`)) as {
    permissionSets?: PermissionSet[];
  };
  return res.permissionSets ?? [];
}

export async function getPermissionSet(
  fetchApi: ApiFetch,
  apiName: string,
): Promise<PermissionSet> {
  return (await fetchApi(
    `/metadata/v1/permissions/sets/${encodeURIComponent(apiName)}`,
  )) as PermissionSet;
}

export async function createPermissionSet(
  fetchApi: ApiFetch,
  body: CreatePermissionSetInput,
): Promise<PermissionSet> {
  return (await fetchApi("/metadata/v1/permissions/sets", {
    method: "POST",
    body: JSON.stringify(body),
  })) as PermissionSet;
}

export async function patchPermissionSet(
  fetchApi: ApiFetch,
  apiName: string,
  body: PatchPermissionSetInput,
): Promise<PermissionSet> {
  return (await fetchApi(`/metadata/v1/permissions/sets/${encodeURIComponent(apiName)}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  })) as PermissionSet;
}
