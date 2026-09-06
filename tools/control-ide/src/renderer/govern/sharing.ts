/** Thin Metadata helpers for /metadata/v1/sharing (BP-066 WS3). */

export type ApiFetch = (path: string, init?: RequestInit) => Promise<unknown>;

export type SharingSettings = {
  recordSharingEnabled?: boolean;
  recordSharingEnabledAt?: string;
};

export type ObjectSharing = {
  objectApiName: string;
  defaultAccess?: string;
  sharingRulesEnabled?: boolean;
  updatedAt?: string;
};

export type SharingRule = {
  apiName: string;
  label?: string;
  active?: boolean;
  accessLevel?: string;
  sharedToDataRoleApiName?: string;
  criteria?: unknown;
  sortOrder?: number;
};

export async function getSharingSettings(fetchApi: ApiFetch): Promise<SharingSettings> {
  return (await fetchApi("/metadata/v1/sharing/settings")) as SharingSettings;
}

export async function enableSharing(fetchApi: ApiFetch): Promise<SharingSettings> {
  return (await fetchApi("/metadata/v1/sharing/enable", {
    method: "POST",
    body: JSON.stringify({ confirm: true }),
  })) as SharingSettings;
}

export async function listSharingObjects(fetchApi: ApiFetch): Promise<ObjectSharing[]> {
  const row = (await fetchApi("/metadata/v1/sharing/objects")) as { objects?: ObjectSharing[] };
  return row.objects ?? [];
}

export async function patchSharingObject(
  fetchApi: ApiFetch,
  objectApiName: string,
  body: { defaultAccess?: string; sharingRulesEnabled?: boolean },
): Promise<ObjectSharing> {
  return (await fetchApi(`/metadata/v1/sharing/objects/${encodeURIComponent(objectApiName)}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  })) as ObjectSharing;
}

export async function listSharingRules(fetchApi: ApiFetch, objectApiName: string): Promise<SharingRule[]> {
  const row = (await fetchApi(
    `/metadata/v1/sharing/objects/${encodeURIComponent(objectApiName)}/rules`,
  )) as { rules?: SharingRule[] };
  return row.rules ?? [];
}

export async function createSharingRule(
  fetchApi: ApiFetch,
  objectApiName: string,
  body: {
    apiName: string;
    label: string;
    sharedToDataRoleApiName: string;
    accessLevel?: string;
    active?: boolean;
    criteria: unknown;
  },
): Promise<SharingRule> {
  return (await fetchApi(`/metadata/v1/sharing/objects/${encodeURIComponent(objectApiName)}/rules`, {
    method: "POST",
    body: JSON.stringify(body),
  })) as SharingRule;
}

export async function deleteSharingRule(
  fetchApi: ApiFetch,
  objectApiName: string,
  ruleApiName: string,
): Promise<void> {
  await fetchApi(
    `/metadata/v1/sharing/objects/${encodeURIComponent(objectApiName)}/rules/${encodeURIComponent(ruleApiName)}`,
    { method: "DELETE" },
  );
}
