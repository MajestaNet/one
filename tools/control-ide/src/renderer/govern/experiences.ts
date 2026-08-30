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
