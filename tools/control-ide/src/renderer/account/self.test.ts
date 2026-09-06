import { describe, expect, it, vi } from "vitest";
import { changeMyPassword, listDevices, revokeDevice } from "./self";

describe("account self-service helpers", () => {
  it("lists and revokes devices", async () => {
    const fetchApi = vi.fn(async (path: string) => {
      if (path === "/client/v1/devices") {
        return { devices: [{ deviceId: "dev-1", label: "Control IDE" }] };
      }
      return {};
    });
    expect(await listDevices(fetchApi)).toEqual([{ deviceId: "dev-1", label: "Control IDE" }]);
    await revokeDevice(fetchApi, "dev-1");
    expect(fetchApi).toHaveBeenCalledWith("/client/v1/devices/dev-1/revoke", { method: "POST" });
  });

  it("posts current and new password", async () => {
    const fetchApi = vi.fn().mockResolvedValue({});
    await changeMyPassword(fetchApi, "old", "new-secret");
    expect(fetchApi).toHaveBeenCalledWith(
      "/client/v1/me/password",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ currentPassword: "old", newPassword: "new-secret" }),
      }),
    );
  });
});
