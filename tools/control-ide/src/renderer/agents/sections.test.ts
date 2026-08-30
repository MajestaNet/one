import { describe, expect, it } from "vitest";
import {
  dockSectionForPrimarySection,
  modesFromPrimarySection,
  PRIMARY_SECTIONS,
  sectionLabel,
} from "./sections";

describe("sections", () => {
  it("lists four launcher primary sections", () => {
    expect(PRIMARY_SECTIONS).toEqual(["operate", "build", "govern", "settings"]);
  });

  it("maps primarySection onto dock homes with legacy aliases", () => {
    expect(modesFromPrimarySection("build")).toEqual(["build"]);
    expect(modesFromPrimarySection("settings")).toEqual(["settings"]);
    expect(modesFromPrimarySection("run")).toEqual(["operate"]);
    expect(modesFromPrimarySection("ship")).toEqual(["build"]);
    expect(modesFromPrimarySection("operate", "harness.operate.query")).toEqual(["build"]);
    expect(modesFromPrimarySection("operate", "harness.run.tools")).toEqual(["operate"]);
    expect(modesFromPrimarySection("operate")).toEqual(["build"]);
    expect(modesFromPrimarySection("")).toEqual(["operate"]);
    expect(modesFromPrimarySection("nope")).toEqual(["operate"]);
    expect(modesFromPrimarySection("  SHIP  ")).toEqual(["build"]);
  });

  it("labels settings launcher tile", () => {
    expect(sectionLabel("settings")).toBe("Settings");
    expect(dockSectionForPrimarySection("ship")).toBe("build");
  });
});
