import { describe, expect, it } from "vitest";
import {
  extractActorScopes,
  hasCapability,
  hasScope,
  canOpenSettings,
  modesForScopes,
  toolsForMode,
  toolsForSettings,
  IDE_CAPABILITY_GROUPS,
} from "./scopes";

describe("scopes", () => {
  it("normalizes actor scopes from /me", () => {
    expect(extractActorScopes({ scopes: ["client", "metadata"], isAdmin: false })).toEqual({
      scopes: ["client", "metadata"],
      isAdmin: false,
    });
    expect(extractActorScopes({ scope: "client+deploy", is_admin: true }).scopes).toContain("client");
  });

  it("matches family scopes exactly (no substring)", () => {
    expect(hasScope(["client"], "client")).toBe(true);
    expect(hasScope(["metadata"], "client")).toBe(false);
    expect(hasScope(["clientmetadata"], "client")).toBe(false);
    expect(hasScope(["admin"], "deploy")).toBe(true);
  });

  it("gates modes by family scopes when caps unknown", () => {
    expect(modesForScopes(["client"])).toEqual(["operate", "govern"]);
    expect(modesForScopes(["metadata"])).toContain("build");
    expect(modesForScopes(["deploy"])).toContain("build");
    expect(modesForScopes(undefined)).toEqual(["operate", "build", "govern"]);
  });

  it("aliases legacy ide.run and ide.ship caps", () => {
    expect(hasCapability(["ide.run"], "ide.operate")).toBe(true);
    expect(hasCapability(["ide.ship"], "ide.build")).toBe(true);
    expect(hasCapability(["ide.ship.env"], "ide.settings.env")).toBe(true);
    expect(hasCapability(["ide.govern.env"], "ide.settings.env")).toBe(true);
  });

  it("fail-closes modes when ide.* caps are loaded", () => {
    expect(
      modesForScopes(["client", "metadata", "deploy"], {
        systemPermissions: ["ide.build", "ide.operate.query"],
        isAdmin: false,
      }),
    ).toEqual(["build"]);
    expect(
      modesForScopes(["client"], {
        systemPermissions: ["ide.run", "ide.run.tools"],
        isAdmin: false,
      }),
    ).toEqual(["operate"]);
    expect(
      modesForScopes(["client"], {
        systemPermissions: ["identity.users"],
        isAdmin: false,
      }),
    ).toEqual([]);
  });

  it("filters workspace tools with ide.* when caps known", () => {
    expect(toolsForMode("operate", ["client"])).toEqual(["runGraph", "objectHome"]);
    expect(
      toolsForMode("operate", ["client"], {
        systemPermissions: ["ide.run"],
        isAdmin: false,
      }),
    ).toEqual([]);
    expect(
      toolsForMode("operate", ["client"], {
        systemPermissions: ["ide.run", "ide.run.tools"],
        isAdmin: false,
      }),
    ).toEqual(["runGraph", "objectHome"]);
    expect(
      toolsForMode("build", ["client"], {
        systemPermissions: ["ide.build", "ide.operate.query", "ide.operate.explorer"],
        isAdmin: false,
      }),
    ).toEqual(["query", "explorer"]);
    expect(
      toolsForMode("build", ["client"], {
        systemPermissions: [
          "ide.build",
          "ide.operate.query",
          "ide.operate.monitor",
          "ide.operate.explorer",
          "debug.read",
        ],
        isAdmin: false,
      }),
    ).toEqual(["query", "monitor", "explorer"]);
    expect(toolsForMode("build", ["client"])).toEqual([
      "objects",
      "packages",
      "agentSpecs",
      "tools",
      "automations",
      "repo",
      "query",
      "monitor",
      "explorer",
    ]);
    expect(
      toolsForMode("build", ["deploy"], {
        systemPermissions: ["ide.ship", "ide.ship.deploy"],
        isAdmin: false,
      }),
    ).toEqual(["deploy"]);
    expect(
      toolsForMode("build", ["metadata"], {
        systemPermissions: [
          "ide.build",
          "ide.build.objects",
          "ide.build.packages",
          "ide.build.agentSpecs",
          "ide.build.tools",
          "ide.build.repo",
        ],
        isAdmin: false,
      }),
    ).toEqual(["objects", "packages", "agentSpecs", "tools", "repo"]);
    expect(
      toolsForMode("build", ["metadata"], {
        systemPermissions: ["ide.build", "metadata.build"],
        isAdmin: false,
      }),
    ).toEqual(["automations"]);
    expect(
      toolsForMode("build", ["metadata"], {
        systemPermissions: [
          "ide.build",
          "ide.build.objects",
          "ide.build.packages",
          "ide.build.agentSpecs",
          "ide.build.tools",
          "ide.build.repo",
          "metadata.build",
        ],
        isAdmin: false,
      }),
    ).toEqual(["objects", "packages", "agentSpecs", "tools", "automations", "repo"]);
  });

  it("does not list Environments on the Govern rail", () => {
    expect(toolsForMode("govern", undefined)).not.toContain("env");
    expect(toolsForMode("govern", ["client", "deploy"])).not.toContain("env");
    expect(
      toolsForMode("govern", ["client"], {
        systemPermissions: ["ide.govern", "ide.govern.env"],
        isAdmin: false,
      }),
    ).not.toContain("env");
  });

  it("lists Environments only under Settings tools in the capability catalog", () => {
    const governIds = IDE_CAPABILITY_GROUPS.find((g) => g.label === "Govern tools")?.caps.map((c) => c.id) ?? [];
    const settingsIds =
      IDE_CAPABILITY_GROUPS.find((g) => g.label === "Settings tools")?.caps.map((c) => c.id) ?? [];
    expect(governIds).not.toContain("ide.govern.env");
    expect(settingsIds).toContain("ide.settings.env");
  });

  it("filters settings tools including environments", () => {
    expect(
      toolsForSettings(["deploy"], {
        systemPermissions: ["ide.settings", "ide.settings.env"],
        isAdmin: false,
      }),
    ).toContain("env");
    expect(
      toolsForSettings(["deploy"], {
        systemPermissions: ["ide.settings", "ide.ship.env"],
        isAdmin: false,
      }),
    ).toContain("env");
    expect(
      toolsForSettings(["client"], {
        systemPermissions: ["ide.govern.env"],
        isAdmin: false,
      }),
    ).toContain("env");
  });

  it("canOpenSettings respects ide.settings cap", () => {
    expect(canOpenSettings(["client"], { systemPermissions: ["ide.settings"], isAdmin: false })).toBe(
      true,
    );
    expect(canOpenSettings(["client"], { systemPermissions: ["ide.operate"], isAdmin: false })).toBe(
      false,
    );
    expect(
      canOpenSettings(["client"], { systemPermissions: ["ide.settings.env"], isAdmin: false }),
    ).toBe(true);
    expect(
      canOpenSettings(["client"], { systemPermissions: ["ide.govern.env"], isAdmin: false }),
    ).toBe(true);
  });
});
