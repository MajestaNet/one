import { describe, expect, it, vi } from "vitest";
import { createExperience, listExperiences, patchExperience } from "./experiences";

describe("listExperiences", () => {
  it("returns experiences array from metadata payload", async () => {
    const fetchApi = vi.fn().mockResolvedValue({
      experiences: [{ apiName: "portal", label: "Portal" }],
    });
    await expect(listExperiences(fetchApi)).resolves.toEqual([{ apiName: "portal", label: "Portal" }]);
    expect(fetchApi).toHaveBeenCalledWith("/metadata/v1/experiences");
  });

  it("returns empty list when payload omits experiences", async () => {
    const fetchApi = vi.fn().mockResolvedValue({});
    await expect(listExperiences(fetchApi)).resolves.toEqual([]);
  });

  it("creates and patches via Metadata write routes", async () => {
    const fetchApi = vi
      .fn()
      .mockResolvedValueOnce({ apiName: "portal", label: "Portal" })
      .mockResolvedValueOnce({ apiName: "portal", label: "Portal v2" });
    await createExperience(fetchApi, { apiName: "portal", label: "Portal" });
    expect(fetchApi).toHaveBeenCalledWith(
      "/metadata/v1/experiences",
      expect.objectContaining({ method: "POST" }),
    );
    await patchExperience(fetchApi, "portal", { label: "Portal v2" });
    expect(fetchApi).toHaveBeenCalledWith(
      "/metadata/v1/experiences/portal",
      expect.objectContaining({ method: "PATCH" }),
    );
  });
});
