/** Thin Client API helpers for Govern identity admin. No business rules. */

export type ApiFetch = (path: string, init?: RequestInit) => Promise<unknown>;

export type Principal = {
  id: string;
  email?: string;
  displayName?: string;
  principalType?: string;
  isActive?: boolean;
  canAuthenticate?: boolean;
  isAdmin?: boolean;
  userName?: string;
  externalId?: string;
  title?: string;
  department?: string;
  frozenAt?: string;
  frozenReason?: string;
  roleApiNames?: string[];
  permissionSetApiNames?: string[];
  createdAt?: string;
  updatedAt?: string;
  [key: string]: unknown;
};

export type CreatePrincipalInput = {
  email?: string;
  displayName?: string;
  userName?: string;
  principalType?: "user" | "service" | "agent";
  isAdmin?: boolean;
  roleApiName?: string;
  roleApiNames?: string[];
  permissionSetApiNames?: string[];
  title?: string;
  department?: string;
};

export type PatchPrincipalInput = {
  email?: string;
  displayName?: string;
  userName?: string;
  isActive?: boolean;
  isAdmin?: boolean;
  permissionSetApiNames?: string[];
  title?: string;
  department?: string;
};

export type Credential = {
  id: string;
  credentialKind?: string;
  label?: string;
  expiresAt?: string;
  revokedAt?: string;
  createdAt?: string;
};

function qs(params: Record<string, string | undefined>): string {
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v != null && v !== "") sp.set(k, v);
  }
  const s = sp.toString();
  return s ? `?${s}` : "";
}

export async function listPrincipals(
  fetchApi: ApiFetch,
  opts?: { principalType?: string; email?: string; isActive?: string },
): Promise<Principal[]> {
  const path = `/client/v1/principals${qs({
    principalType: opts?.principalType,
    email: opts?.email,
    isActive: opts?.isActive,
  })}`;
  const res = (await fetchApi(path)) as { principals?: Principal[] } | Principal[];
  if (Array.isArray(res)) return res;
  return res.principals ?? [];
}

export async function getPrincipal(fetchApi: ApiFetch, id: string): Promise<Principal> {
  return (await fetchApi(`/client/v1/principals/${encodeURIComponent(id)}`)) as Principal;
}

export async function createPrincipal(
  fetchApi: ApiFetch,
  body: CreatePrincipalInput,
): Promise<Principal> {
  return (await fetchApi("/client/v1/principals", {
    method: "POST",
    body: JSON.stringify(body),
  })) as Principal;
}

export async function patchPrincipal(
  fetchApi: ApiFetch,
  id: string,
  body: PatchPrincipalInput,
): Promise<Principal> {
  return (await fetchApi(`/client/v1/principals/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  })) as Principal;
}

export async function freezePrincipal(
  fetchApi: ApiFetch,
  id: string,
  reason: string,
): Promise<Principal> {
  return (await fetchApi(`/client/v1/principals/${encodeURIComponent(id)}/freeze`, {
    method: "POST",
    body: JSON.stringify({ reason }),
  })) as Principal;
}

export async function unfreezePrincipal(
  fetchApi: ApiFetch,
  id: string,
  reactivate = true,
): Promise<Principal> {
  return (await fetchApi(`/client/v1/principals/${encodeURIComponent(id)}/unfreeze`, {
    method: "POST",
    body: JSON.stringify({ reactivate }),
  })) as Principal;
}

export async function listCredentials(fetchApi: ApiFetch, id: string): Promise<Credential[]> {
  const res = (await fetchApi(
    `/client/v1/principals/${encodeURIComponent(id)}/credentials`,
  )) as { credentials?: Credential[] };
  return res.credentials ?? [];
}

export async function createCredential(
  fetchApi: ApiFetch,
  id: string,
  label: string,
): Promise<Credential & { clientSecret?: string }> {
  return (await fetchApi(`/client/v1/principals/${encodeURIComponent(id)}/credentials`, {
    method: "POST",
    body: JSON.stringify({ label }),
  })) as Credential & { clientSecret?: string };
}

export async function revokeCredential(
  fetchApi: ApiFetch,
  principalId: string,
  credId: string,
): Promise<{ ok?: boolean }> {
  return (await fetchApi(
    `/client/v1/principals/${encodeURIComponent(principalId)}/credentials/${encodeURIComponent(credId)}/revoke`,
    { method: "POST", body: JSON.stringify({}) },
  )) as { ok?: boolean };
}

/** Admin-set password for a user principal (no email reset — BP-038). */
export async function setPrincipalPassword(
  fetchApi: ApiFetch,
  id: string,
  password: string,
): Promise<{ ok?: boolean; userId?: string }> {
  return (await fetchApi(`/client/v1/principals/${encodeURIComponent(id)}/password`, {
    method: "POST",
    body: JSON.stringify({ password }),
  })) as { ok?: boolean; userId?: string };
}

export async function assignRole(
  fetchApi: ApiFetch,
  userId: string,
  roleApiName: string,
): Promise<{ ok?: boolean }> {
  return (await fetchApi("/client/v1/roles/assign", {
    method: "POST",
    body: JSON.stringify({ userId, roleApiName }),
  })) as { ok?: boolean };
}

export async function unassignRole(
  fetchApi: ApiFetch,
  userId: string,
  roleApiName: string,
): Promise<{ ok?: boolean }> {
  return (await fetchApi("/client/v1/roles/unassign", {
    method: "POST",
    body: JSON.stringify({ userId, roleApiName }),
  })) as { ok?: boolean };
}

export async function assignPermissionSet(
  fetchApi: ApiFetch,
  userId: string,
  permissionSetApiName: string,
): Promise<{ ok?: boolean }> {
  return (await fetchApi("/client/v1/permissions/assign", {
    method: "POST",
    body: JSON.stringify({ userId, permissionSetApiName }),
  })) as { ok?: boolean };
}

export async function unassignPermissionSet(
  fetchApi: ApiFetch,
  userId: string,
  permissionSetApiName: string,
): Promise<{ ok?: boolean }> {
  return (await fetchApi("/client/v1/permissions/unassign", {
    method: "POST",
    body: JSON.stringify({ userId, permissionSetApiName }),
  })) as { ok?: boolean };
}
