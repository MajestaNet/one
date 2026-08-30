import { describe, expect, it, vi } from "vitest";
import { listExperiences } from "./experiences";

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
});
