/** Client self-service: password + devices (BP-066 WS3). */

export type ApiFetch = (path: string, init?: RequestInit) => Promise<unknown>;

export type DeviceRow = {
  id?: string;
  deviceId: string;
  label?: string;
  fingerprint?: string;
  revokedAt?: string | null;
  expiresAt?: string;
  createdAt?: string;
};

export async function listDevices(fetchApi: ApiFetch): Promise<DeviceRow[]> {
  const row = (await fetchApi("/client/v1/devices")) as { devices?: DeviceRow[] } | null | undefined;
  return row?.devices ?? [];
}

export async function revokeDevice(fetchApi: ApiFetch, deviceId: string): Promise<void> {
  await fetchApi(`/client/v1/devices/${encodeURIComponent(deviceId)}/revoke`, { method: "POST" });
}

export async function changeMyPassword(
  fetchApi: ApiFetch,
  currentPassword: string,
  newPassword: string,
): Promise<void> {
  await fetchApi("/client/v1/me/password", {
    method: "POST",
    body: JSON.stringify({ currentPassword, newPassword }),
  });
}
