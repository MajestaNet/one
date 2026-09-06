import { describe, expect, it, vi } from "vitest";
import {
  createSharingRule,
  enableSharing,
  getSharingSettings,
  listSharingObjects,
  listSharingRules,
  patchSharingObject,
} from "./sharing";

describe("sharing helpers", () => {
  it("loads settings, objects, and rules from Metadata sharing APIs", async () => {
    const fetchApi = vi
      .fn()
      .mockResolvedValueOnce({ recordSharingEnabled: false })
      .mockResolvedValueOnce({ recordSharingEnabled: true })
      .mockResolvedValueOnce({ objects: [{ objectApiName: "Account", defaultAccess: "public_read" }] })
      .mockResolvedValueOnce({ objectApiName: "Account", defaultAccess: "private" })
      .mockResolvedValueOnce({ rules: [{ apiName: "SalesTeam", accessLevel: "read" }] })
      .mockResolvedValueOnce({ apiName: "SalesTeam", label: "Sales" });

    await expect(getSharingSettings(fetchApi)).resolves.toEqual({ recordSharingEnabled: false });
    await expect(enableSharing(fetchApi)).resolves.toEqual({ recordSharingEnabled: true });
    expect(fetchApi).toHaveBeenNthCalledWith(
      2,
      "/metadata/v1/sharing/enable",
      expect.objectContaining({ method: "POST" }),
    );
    await expect(listSharingObjects(fetchApi)).resolves.toEqual([
      { objectApiName: "Account", defaultAccess: "public_read" },
    ]);
    await expect(patchSharingObject(fetchApi, "Account", { defaultAccess: "private" })).resolves.toMatchObject({
      defaultAccess: "private",
    });
    await expect(listSharingRules(fetchApi, "Account")).resolves.toEqual([
      { apiName: "SalesTeam", accessLevel: "read" },
    ]);
    await createSharingRule(fetchApi, "Account", {
      apiName: "SalesTeam",
      label: "Sales",
      sharedToDataRoleApiName: "Sales",
      criteria: { filters: [{ field: "OwnerId", op: "eq", value: "x" }] },
    });
    expect(fetchApi).toHaveBeenLastCalledWith(
      "/metadata/v1/sharing/objects/Account/rules",
      expect.objectContaining({ method: "POST" }),
    );
  });
});
