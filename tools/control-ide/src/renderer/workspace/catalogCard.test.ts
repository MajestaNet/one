import { describe, expect, it } from "vitest";
import { catalogCardCopy, isDuplicateCatalogTitle } from "./catalogCard";

describe("catalogCardCopy", () => {
  it("drops a kicker that repeats the title", () => {
    expect(catalogCardCopy("Objects", "objects")).toEqual({ title: "Objects" });
    expect(catalogCardCopy("Query assistant", "QueryAssistant")).toEqual({
      title: "Query assistant",
    });
    expect(catalogCardCopy("Metadata builder", "MetadataBuilder")).toEqual({
      title: "Metadata builder",
    });
  });

  it("keeps a distinct apiName kicker", () => {
    expect(catalogCardCopy("Accounts", "Account__c")).toEqual({
      title: "Accounts",
      kicker: "Account__c",
    });
  });

  it("treats empty secondary as absent", () => {
    expect(isDuplicateCatalogTitle("Agents", "")).toBe(true);
    expect(catalogCardCopy("Agents")).toEqual({ title: "Agents" });
  });
});
