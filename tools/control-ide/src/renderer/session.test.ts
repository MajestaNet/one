import { afterEach, describe, expect, it } from "vitest";
import {
  connectionFromActor,
  initialsFromName,
  isUnauthorizedError,
  mergePeerHints,
  normalizeSession,
  sessionIdentity,
  switchActiveEnvironment,
  upsertEnvironment,
  withActiveMirrors,
  withActorIdentity,
  withRotatedTokens,
} from "./session";

afterEach(() => {
  /* pure helpers */
});

describe("normalizeSession", () => {
  it("migrates legacy flat sessions", () => {
    const s = normalizeSession({
      baseUrl: "http://localhost:8080",
      token: "jwt",
      repoPath: "/tmp/repo",
      scopes: ["client", "metadata"],
    });
    expect(s).toBeTruthy();
    expect(s!.environments).toHaveLength(1);
    expect(s!.baseUrl).toBe("http://localhost:8080");
    expect(s!.token).toBe("jwt");
    expect(s!.repoPath).toBe("/tmp/repo");
    expect(s!.activeInstallId).toBeTruthy();
  });

  it("keeps multi-env sessions and syncs mirrors", () => {
    const s = normalizeSession({
      activeInstallId: "b",
      environments: [
        { installId: "a", installRole: "test", baseUrl: "http://a", token: "ta" },
        { installId: "b", installRole: "staging", baseUrl: "http://b", token: "tb", scopes: ["deploy"] },
      ],
    });
    expect(s!.baseUrl).toBe("http://b");
    expect(s!.token).toBe("tb");
    expect(s!.scopes).toEqual(["deploy"]);
  });

  it("returns null for empty input", () => {
    expect(normalizeSession(null)).toBeNull();
    expect(normalizeSession({})).toBeNull();
  });

  it("preserves refreshToken and accessExpiresAt on env and top-level mirrors", () => {
    const s = normalizeSession({
      activeInstallId: "b",
      environments: [
        { installId: "a", installRole: "test", baseUrl: "http://a", token: "ta" },
        {
          installId: "b",
          installRole: "staging",
          baseUrl: "http://b",
          token: "tb",
          refreshToken: "rt-b",
          accessExpiresAt: 1_700_000_000_000,
        },
      ],
    });
    expect(s!.refreshToken).toBe("rt-b");
    expect(s!.accessExpiresAt).toBe(1_700_000_000_000);
    expect(s!.environments[1]!.refreshToken).toBe("rt-b");
    expect(s!.environments[0]!.refreshToken).toBeUndefined();
  });

  it("migrates refresh fields from a legacy flat session", () => {
    const s = normalizeSession({
      baseUrl: "http://localhost:8080",
      token: "jwt",
      refreshToken: "rt-flat",
      accessExpiresAt: 1_800_000_000_000,
    });
    expect(s!.refreshToken).toBe("rt-flat");
    expect(s!.accessExpiresAt).toBe(1_800_000_000_000);
    expect(s!.environments[0]!.refreshToken).toBe("rt-flat");
  });

  it("preserves allowInsecureHttp when the operator accepted plaintext HTTP", () => {
    const s = normalizeSession({
      baseUrl: "http://api.example",
      token: "jwt",
      allowInsecureHttp: true,
    });
    expect(s?.allowInsecureHttp).toBe(true);
    const cleared = normalizeSession({
      baseUrl: "http://api.example",
      token: "jwt",
      allowInsecureHttp: false,
    });
    expect(cleared?.allowInsecureHttp).toBeUndefined();
  });
});

describe("upsertEnvironment / switch", () => {
  it("adds and switches environments", () => {
    let s = upsertEnvironment(null, {
      installId: "dev",
      installRole: "test",
      baseUrl: "http://dev",
      token: "t1",
    });
    s = upsertEnvironment(s, {
      installId: "uat",
      installRole: "staging",
      baseUrl: "http://uat",
      token: "t2",
    });
    expect(s.environments).toHaveLength(2);
    expect(s.activeInstallId).toBe("uat");
    s = switchActiveEnvironment(s, "dev");
    expect(s.baseUrl).toBe("http://dev");
    expect(s.token).toBe("t1");
  });

  it("preserves refresh fields through upsert", () => {
    const s = upsertEnvironment(null, {
      installId: "dev",
      installRole: "test",
      baseUrl: "http://dev",
      token: "t1",
      refreshToken: "rt-1",
      accessExpiresAt: 1_700_000_000_000,
    });
    expect(s.refreshToken).toBe("rt-1");
    expect(s.accessExpiresAt).toBe(1_700_000_000_000);
    const next = upsertEnvironment(s, {
      installId: "dev",
      installRole: "test",
      baseUrl: "http://dev",
      token: "t2",
      refreshToken: "rt-2",
      accessExpiresAt: 1_800_000_000_000,
    });
    expect(next.token).toBe("t2");
    expect(next.refreshToken).toBe("rt-2");
    expect(next.accessExpiresAt).toBe(1_800_000_000_000);
  });

  it("merges peer hints without tokens as disconnected", () => {
    const base = upsertEnvironment(null, {
      installId: "dev",
      installRole: "test",
      baseUrl: "http://dev",
      token: "t1",
    });
    const merged = mergePeerHints(base, [
      { installId: "prod", installRole: "prod", baseUrl: "http://prod", active: true },
    ]);
    expect(merged.environments).toHaveLength(2);
    const prod = merged.environments.find((e) => e.installId === "prod")!;
    expect(prod.token).toBe("");
  });
});

describe("connectionFromActor", () => {
  it("prefers install identity from environment payload", () => {
    const c = connectionFromActor(
      "http://api/",
      "tok",
      { scopes: ["client"], isAdmin: true },
      { installId: "acme-test", installRole: "test" },
    );
    expect(c.installId).toBe("acme-test");
    expect(c.installRole).toBe("test");
    expect(c.baseUrl).toBe("http://api");
    expect(c.isAdmin).toBe(true);
  });

  it("copies display name and email from /me", () => {
    const c = connectionFromActor("http://api/", "tok", {
      displayName: "Ada Lovelace",
      email: "ada@example.com",
      principalType: "user",
      scopes: ["client"],
    });
    expect(c.displayName).toBe("Ada Lovelace");
    expect(c.email).toBe("ada@example.com");
    expect(c.principalType).toBe("user");
  });

  it("copies refresh token metadata when provided", () => {
    const c = connectionFromActor(
      "http://api/",
      "tok",
      { displayName: "Ada" },
      null,
      { refreshToken: "rt-1", expiresIn: 3600, now: 1_000_000 },
    );
    expect(c.refreshToken).toBe("rt-1");
    expect(c.accessExpiresAt).toBe(1_000_000 + 3600 * 1000);
  });
});

describe("withActiveMirrors", () => {
  it("falls back to first env when active missing", () => {
    const s = withActiveMirrors({
      activeInstallId: "missing",
      environments: [{ installId: "x", installRole: "r", baseUrl: "http://x", token: "t" }],
      baseUrl: "",
      token: "",
    });
    expect(s.activeInstallId).toBe("x");
    expect(s.baseUrl).toBe("http://x");
  });
});

describe("sessionIdentity", () => {
  it("builds initials and username from display name", () => {
    const s = upsertEnvironment(null, {
      installId: "dev",
      installRole: "test",
      baseUrl: "http://dev",
      token: "t1",
      displayName: "Ada Lovelace",
      email: "ada@example.com",
    });
    expect(sessionIdentity(s)).toEqual({
      displayName: "Ada Lovelace",
      initials: "AL",
      email: "ada@example.com",
    });
  });

  it("falls back to email and Signed in", () => {
    expect(initialsFromName("ada.lovelace@example.com")).toBe("AL");
    expect(initialsFromName("")).toBe("?");
    const emailOnly = upsertEnvironment(null, {
      installId: "dev",
      installRole: "test",
      baseUrl: "http://dev",
      token: "t1",
      email: "ada@example.com",
    });
    expect(sessionIdentity(emailOnly)?.displayName).toBe("ada@example.com");
    expect(sessionIdentity(null)).toBeNull();
  });

  it("never includes JWT or refresh token strings in chip identity", () => {
    const s = upsertEnvironment(null, {
      installId: "dev",
      installRole: "test",
      baseUrl: "http://dev",
      token: "super-secret-jwt",
      refreshToken: "super-secret-rt",
      displayName: "Ada Lovelace",
      email: "ada@example.com",
    });
    const identity = sessionIdentity(s);
    expect(identity).toEqual({
      displayName: "Ada Lovelace",
      initials: "AL",
      email: "ada@example.com",
    });
    expect(JSON.stringify(identity)).not.toContain("super-secret");
  });

  it("merges /me onto the active env and detects 401", () => {
    const s = upsertEnvironment(null, {
      installId: "dev",
      installRole: "test",
      baseUrl: "http://dev",
      token: "t1",
    });
    const next = withActorIdentity(s, { displayName: "Kai", email: "kai@ex.com", principalType: "user" });
    expect(next.displayName).toBe("Kai");
    expect(next.email).toBe("kai@ex.com");
    expect(isUnauthorizedError(new Error("401 /client/v1/me: expired"))).toBe(true);
    expect(isUnauthorizedError(new Error("Failed to fetch"))).toBe(false);
  });
});

describe("withRotatedTokens", () => {
  it("replaces access JWT and refresh token on the active env", () => {
    const s = upsertEnvironment(null, {
      installId: "dev",
      installRole: "test",
      baseUrl: "http://dev",
      token: "old-jwt",
      refreshToken: "old-rt",
      accessExpiresAt: 1,
    });
    const next = withRotatedTokens(
      s,
      { accessToken: "new-jwt", refreshToken: "new-rt", expiresIn: 3600 },
      1_000_000,
    );
    expect(next.token).toBe("new-jwt");
    expect(next.refreshToken).toBe("new-rt");
    expect(next.accessExpiresAt).toBe(1_000_000 + 3600 * 1000);
    expect(next.environments[0]!.token).toBe("new-jwt");
    expect(next.environments[0]!.refreshToken).toBe("new-rt");
  });
});
