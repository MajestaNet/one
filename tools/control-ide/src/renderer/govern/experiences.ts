/** Metadata API helpers for Govern Client Experiences list. */

import type { ApiFetch } from "./integrations";

export type Experience = {
  apiName: string;
  label?: string;
  description?: string;
  homeUrl?: string;
  connectedAppApiName?: string;
  allowedOrigins?: string[];
  active?: boolean;
  ownership?: string;
  packageName?: string;
};

export async function listExperiences(fetchApi: ApiFetch): Promise<Experience[]> {
  const res = (await fetchApi("/metadata/v1/experiences")) as { experiences?: Experience[] };
  return res.experiences ?? [];
}

export async function createExperience(
  fetchApi: ApiFetch,
  body: {
    apiName: string;
    label: string;
    homeUrl?: string;
    connectedAppApiName?: string;
    allowedOrigins?: string[];
    active?: boolean;
  },
): Promise<Experience> {
  return (await fetchApi("/metadata/v1/experiences", {
    method: "POST",
    body: JSON.stringify(body),
  })) as Experience;
}

export async function patchExperience(
  fetchApi: ApiFetch,
  apiName: string,
  body: Partial<Pick<Experience, "label" | "homeUrl" | "connectedAppApiName" | "active">> & {
    allowedOrigins?: string[];
    description?: string;
  },
): Promise<Experience> {
  return (await fetchApi(`/metadata/v1/experiences/${encodeURIComponent(apiName)}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  })) as Experience;
}

export async function deleteExperience(fetchApi: ApiFetch, apiName: string): Promise<void> {
  await fetchApi(`/metadata/v1/experiences/${encodeURIComponent(apiName)}`, { method: "DELETE" });
}
