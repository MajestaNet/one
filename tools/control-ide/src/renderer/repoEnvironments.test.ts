import { describe, expect, it } from "vitest";
import {
  loadRepoEnvironments,
  orderStagesByRepoEnv,
  rankInstallRole,
} from "./repoEnvironments";

describe("repoEnvironments", () => {
  it("ranks test before prod", () => {
    expect(rankInstallRole("test")).toBeLessThan(rankInstallRole("prod"));
    expect(rankInstallRole("unknown")).toBe(10);
  });

  it("orders session envs by repo environments.yaml", () => {
    const session = [
      { installId: "p", installRole: "prod", baseUrl: "https://p", token: "t" },
      { installId: "t", installRole: "test", baseUrl: "https://t", token: "t" },
    ];
    const repo = [
      { alias: "test", installId: "t", installRole: "test", baseUrl: "https://t" },
      { alias: "prod", installId: "p", installRole: "prod", baseUrl: "https://p" },
    ];
    const ordered = orderStagesByRepoEnv(session, repo);
    expect(ordered.map((e) => e.installId)).toEqual(["t", "p"]);
  });

  it("sorts session envs by role when repo env list is empty", () => {
    const session = [
      { installId: "p", installRole: "prod", label: "prod" },
      { installId: "t", installRole: "test", label: "test" },
    ];
    const ordered = orderStagesByRepoEnv(session, []);
    expect(ordered.map((e) => e.installId)).toEqual(["t", "p"]);
  });

  it("matches repo env by baseUrl and appends unmatched session envs", () => {
    const session = [
      { installId: "t", installRole: "test", baseUrl: "https://test.example/" },
      { installId: "x", installRole: "sandbox", baseUrl: "https://sandbox.example" },
    ];
    const repo = [
      { alias: "test", installId: "", installRole: "test", baseUrl: "https://test.example" },
    ];
    const ordered = orderStagesByRepoEnv(session, repo);
    expect(ordered.map((e) => e.installId)).toEqual(["t", "x"]);
  });

  it("loads environments/*.yaml via listTree + readText", async () => {
    const api = {
      listTree: async () => ["environments/test.yaml", "environments/prod.yaml", "environments/notes.txt"],
      readText: async (_root: string, rel: string) => {
        if (rel === "environments/test.yaml") {
          return "installId: t1\ninstallRole: test\nbaseUrl: https://test.example\n";
        }
        if (rel === "environments/prod.yaml") {
          return "installId: p1\ninstallRole: prod\nbaseUrl: https://prod.example\nalias: production\n";
        }
        throw new Error("missing");
      },
    };
    const envs = await loadRepoEnvironments("/repo", api);
    expect(envs).toHaveLength(2);
    expect(envs[0].alias).toBe("test");
    expect(envs[0].installId).toBe("t1");
    expect(envs[1].alias).toBe("production");
    expect(envs[1].installRole).toBe("prod");
  });

  it("returns empty list when listTree throws", async () => {
    const envs = await loadRepoEnvironments("/repo", {
      listTree: async () => {
        throw new Error("no environments");
      },
      readText: async () => "",
    });
    expect(envs).toEqual([]);
  });
});
