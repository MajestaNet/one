import { describe, expect, it } from "vitest";
import { enabledPackageNames, fetchEnabledPackages, tabsForPackages } from "./packages";

describe("operate packages", () => {
  it("gates Opportunity/Case/Quote by enabled packages", () => {
    const enabled = enabledPackageNames([
      { name: "core", enabled: true },
      { name: "sales", enabled: true },
      { name: "service", enabled: false },
    ]);
    const tabs = tabsForPackages(enabled, true);
    expect(tabs.map((t) => t.objectApiName)).toEqual(["Account", "Contact", "Opportunity", "Quote"]);
    expect(enabled.has("sales")).toBe(true);
  });

  it("offline tabs keep classic three", () => {
    expect(tabsForPackages(null, false).map((t) => t.objectApiName)).toEqual([
      "Account",
      "Contact",
      "Opportunity",
    ]);
  });

  it("fetchEnabledPackages reads enabled set and falls back on error", async () => {
    const ok = await fetchEnabledPackages(async () => ({
      packages: [{ name: "service", enabled: true }],
    }));
    expect(ok.has("service")).toBe(true);
    expect(ok.has("core")).toBe(true);
    const fallback = await fetchEnabledPackages(async () => {
      throw new Error("no metadata");
    });
    expect(fallback.has("sales")).toBe(true);
  });
});
