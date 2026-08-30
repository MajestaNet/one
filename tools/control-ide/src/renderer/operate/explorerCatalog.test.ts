import { describe, expect, it } from "vitest";
import {
  EXPLORER_ALL_PACKAGES,
  catalogDescribes,
  explorerPackageOptions,
  mergeExplorerObjects,
  normalizePackageCatalog,
} from "./explorerCatalog";

const CATALOG = {
  packages: [
    {
      name: "core",
      label: "Core",
      enabled: true,
      objectApiNames: ["Account", "Contact"],
      objects: [
        { apiName: "Account", label: "Account", fieldCount: 12, fields: [] },
        {
          apiName: "Contact",
          label: "Contact",
          fieldCount: 8,
          fields: [{ apiName: "AccountId", fieldType: "lookup", referenceTo: "Account" }],
        },
      ],
    },
    {
      name: "sales",
      label: "Sales",
      enabled: false,
      objectApiNames: ["Opportunity"],
      objects: [
        {
          apiName: "Opportunity",
          label: "Opportunity",
          fieldCount: 14,
          fields: [{ apiName: "AccountId", fieldType: "lookup", referenceTo: "Account" }],
        },
      ],
    },
  ],
};

describe("explorerCatalog", () => {
  it("normalizes package catalog payloads", () => {
    const rows = normalizePackageCatalog(CATALOG);
    expect(rows.map((p) => p.name)).toEqual(["core", "sales"]);
    expect(rows[1]?.enabled).toBe(false);
    expect(rows[1]?.objects?.[0]?.apiName).toBe("Opportunity");
  });

  it("lists not-enabled packages in the selector", () => {
    const installed = [{ apiName: "Account", packageName: "core" }];
    const opts = explorerPackageOptions(installed, normalizePackageCatalog(CATALOG));
    expect(opts).toEqual([
      { name: "core", label: "Core", enabled: true },
      { name: "sales", label: "Sales", enabled: false },
    ]);
  });

  it("keeps Enabled view installed-only and All packages includes catalog", () => {
    const installed = [{ apiName: "Account", label: "Account", packageName: "core" }];
    const catalog = normalizePackageCatalog(CATALOG);
    expect(mergeExplorerObjects(installed, catalog).map((o) => o.apiName)).toEqual(["Account"]);
    const all = mergeExplorerObjects(installed, catalog, { packageFilter: EXPLORER_ALL_PACKAGES });
    expect(all.map((o) => o.apiName).sort()).toEqual(["Account", "Opportunity"]);
    expect(all.find((o) => o.apiName === "Opportunity")?.enabled).toBe(false);
    expect(all.find((o) => o.apiName === "Account")?.enabled).toBe(true);
  });

  it("shows a not-enabled package when selected", () => {
    const installed = [{ apiName: "Account", packageName: "core" }];
    const catalog = normalizePackageCatalog(CATALOG);
    const sales = mergeExplorerObjects(installed, catalog, { packageFilter: "sales" });
    expect(sales).toEqual([
      expect.objectContaining({ apiName: "Opportunity", enabled: false, packageName: "sales" }),
    ]);
  });

  it("builds catalog describes with lookup edges", () => {
    const desc = catalogDescribes(normalizePackageCatalog(CATALOG));
    expect(desc.get("Opportunity")?.fields?.[0]).toMatchObject({
      apiName: "AccountId",
      referenceTo: "Account",
    });
  });
});
