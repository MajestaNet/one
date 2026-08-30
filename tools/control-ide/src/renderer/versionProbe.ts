import { parseApiRevisionWindow, parseVersionProbe } from "./compat";

export type InstallVersionProbe = {
  productVersion?: string;
  apiRevision: { min: number; current: number } | null;
  raw: Record<string, unknown>;
};

/** Unauthenticated GET /version — used before Connect auth. */
export async function probeInstallVersion(baseUrl: string): Promise<InstallVersionProbe> {
  const url = `${baseUrl.replace(/\/$/, "")}/version`;
  const res = await fetch(url);
  const text = await res.text();
  let body: Record<string, unknown> = {};
  if (text) {
    try {
      body = JSON.parse(text) as Record<string, unknown>;
    } catch {
      /* keep empty object */
    }
  }
  if (!res.ok) {
    throw new Error(`${res.status} /version: ${text}`);
  }
  const parsed = parseVersionProbe(body);
  return {
    productVersion: parsed.productVersion,
    apiRevision: parsed.apiRevision,
    raw: body,
  };
}

export function apiRevisionFromPayload(payload: Record<string, unknown>): { min: number; current: number } | null {
  return parseApiRevisionWindow(payload.apiRevision);
}
