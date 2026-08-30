import { describe, expect, it } from "vitest";
import { isAutomationPath, isMetadataYaml, parsePorcelainPath } from "./gitPorcelain";

describe("gitPorcelain", () => {
  it("parses modified and untracked paths", () => {
    expect(parsePorcelainPath(" M metadata/objects/A.yaml")).toBe("metadata/objects/A.yaml");
    expect(parsePorcelainPath("?? metadata/fields/A/B.yaml")).toBe("metadata/fields/A/B.yaml");
    expect(parsePorcelainPath("A  tests/Smoke.yaml")).toBe("tests/Smoke.yaml");
  });

  it("parses renames to the new path", () => {
    expect(parsePorcelainPath("R  metadata/old.yaml -> metadata/new.yaml")).toBe("metadata/new.yaml");
  });

  it("detects metadata yaml", () => {
    expect(isMetadataYaml("metadata/objects/Account.yaml")).toBe(true);
    expect(isMetadataYaml("metadata/x.yml")).toBe(true);
    expect(isMetadataYaml("tests/Smoke.yaml")).toBe(false);
    expect(isMetadataYaml("README.md")).toBe(false);
  });

  it("detects automation yaml and guest TypeScript paths", () => {
    expect(isAutomationPath("metadata/automations/CreateAccount_From_Contact.yaml")).toBe(true);
    expect(isAutomationPath("src/automations/create_account_from_contact.ts")).toBe(true);
    expect(isAutomationPath("tests/automations/create_account_from_contact_test.ts")).toBe(true);
    expect(isAutomationPath("metadata/objects/Referral__c.yaml")).toBe(false);
    expect(isAutomationPath("src/other/file.ts")).toBe(false);
  });
});
