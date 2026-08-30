/** Thin Client API helpers for Govern integrations admin. */

export type ApiFetch = (path: string, init?: RequestInit) => Promise<unknown>;

export type Integration = {
  id?: string;
  apiName: string;
  label?: string;
  description?: string;
  principalId?: string;
  clientKind?: string;
  oauthFlows?: string[];
  callbackUrls?: string[];
  logoutUrls?: string[];
  allowedScopesHint?: string[];
  allowedCidrs?: string[];
  pkceRequired?: boolean;
  ownership?: string;
  packageName?: string;
  isActive?: boolean;
  hasOneSecret?: boolean;
  createdAt?: string;
  updatedAt?: string;
  oneClientSecret?: string;
  clientId?: string;
  [key: string]: unknown;
};

export type CreateIntegrationInput = {
  apiName: string;
  label?: string;
  description?: string;
  clientKind?: string;
  oauthFlows: string[];
  callbackUrls?: string[];
  logoutUrls?: string[];
  allowedScopesHint?: string[];
  pkceRequired?: boolean;
  roleApiNames?: string[];
  principalEmail?: string;
  principalName?: string;
};

export type PatchIntegrationInput = {
  label?: string;
  description?: string;
  oauthFlows?: string[];
  callbackUrls?: string[];
  logoutUrls?: string[];
  allowedScopesHint?: string[];
  allowedCidrs?: string[];
  pkceRequired?: boolean;
  isActive?: boolean;
};

export async function listIntegrations(fetchApi: ApiFetch): Promise<Integration[]> {
  const res = (await fetchApi("/client/v1/integrations")) as { items?: Integration[] };
  return res.items ?? [];
}

export async function getIntegration(fetchApi: ApiFetch, apiName: string): Promise<Integration> {
  return (await fetchApi(`/client/v1/integrations/${encodeURIComponent(apiName)}`)) as Integration;
}

export async function createIntegration(
  fetchApi: ApiFetch,
  body: CreateIntegrationInput,
): Promise<Integration> {
  return (await fetchApi("/client/v1/integrations", {
    method: "POST",
    body: JSON.stringify(body),
  })) as Integration;
}

export async function patchIntegration(
  fetchApi: ApiFetch,
  apiName: string,
  body: PatchIntegrationInput,
): Promise<Integration> {
  return (await fetchApi(`/client/v1/integrations/${encodeURIComponent(apiName)}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  })) as Integration;
}

export async function deleteIntegration(
  fetchApi: ApiFetch,
  apiName: string,
): Promise<{ ok?: boolean; apiName?: string }> {
  return (await fetchApi(`/client/v1/integrations/${encodeURIComponent(apiName)}`, {
    method: "DELETE",
  })) as { ok?: boolean; apiName?: string };
}

export async function rotateIntegrationSecrets(
  fetchApi: ApiFetch,
  apiName: string,
): Promise<Integration> {
  return (await fetchApi(`/client/v1/integrations/${encodeURIComponent(apiName)}/secrets/rotate`, {
    method: "POST",
    body: JSON.stringify({}),
  })) as Integration;
}

export async function revealIntegrationSecrets(
  fetchApi: ApiFetch,
  apiName: string,
): Promise<{ oneClientSecret?: string }> {
  return (await fetchApi(`/client/v1/integrations/${encodeURIComponent(apiName)}/secrets/reveal`, {
    method: "POST",
    body: JSON.stringify({}),
  })) as { oneClientSecret?: string };
}
