/** Thin Client API helpers for Roles. */

export type ApiFetch = (path: string, init?: RequestInit) => Promise<unknown>;

export type Role = {
  id?: string;
  apiName: string;
  label?: string;
  isSystem?: boolean;
  scopes?: string[];
  [key: string]: unknown;
};

export type CreateRoleInput = {
  apiName: string;
  label?: string;
  scopes: string[];
};

export type PatchRoleInput = {
  label?: string;
  scopes?: string[];
};

export async function listRoles(fetchApi: ApiFetch): Promise<Role[]> {
  const res = (await fetchApi("/client/v1/roles")) as { roles?: Role[] };
  return res.roles ?? [];
}

export async function getRole(fetchApi: ApiFetch, apiName: string): Promise<Role> {
  return (await fetchApi(`/client/v1/roles/${encodeURIComponent(apiName)}`)) as Role;
}

export async function createRole(fetchApi: ApiFetch, body: CreateRoleInput): Promise<Role> {
  return (await fetchApi("/client/v1/roles", {
    method: "POST",
    body: JSON.stringify(body),
  })) as Role;
}

export async function patchRole(
  fetchApi: ApiFetch,
  apiName: string,
  body: PatchRoleInput,
): Promise<Role> {
  return (await fetchApi(`/client/v1/roles/${encodeURIComponent(apiName)}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  })) as Role;
}

export async function deleteRole(
  fetchApi: ApiFetch,
  apiName: string,
  force = false,
): Promise<{ ok?: boolean }> {
  const q = force ? "?force=true" : "";
  return (await fetchApi(`/client/v1/roles/${encodeURIComponent(apiName)}${q}`, {
    method: "DELETE",
  })) as { ok?: boolean };
}
