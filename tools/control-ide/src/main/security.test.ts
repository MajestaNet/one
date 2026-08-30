import { describe, expect, it } from "vitest";
import {
  GIT_CLONE_SAFE_CONFIG,
  GIT_CLONE_SAFE_ENV,
  assertChangeBranchName,
  assertSafeCloneUrl,
  assertSafeGitArg,
  buildCsp,
  isSafeExternalUrl,
  isTrustedFrameUrl,
} from "./security";

describe("buildCsp", () => {
  it("locks the production renderer to its own scripts", () => {
    const csp = buildCsp("production");
    expect(csp).toContain("default-src 'none'");
    expect(csp).toContain("script-src 'self'");
    expect(csp).not.toContain("unsafe-eval");
    expect(csp).toContain("frame-src 'none'");
    expect(csp).toContain("object-src 'none'");
    expect(csp).toContain("base-uri 'none'");
    expect(csp).toContain("form-action 'none'");
  });

  it("keeps the pieces Monaco needs", () => {
    const csp = buildCsp("production");
    expect(csp).toContain("style-src 'self' 'unsafe-inline'");
    expect(csp).toContain("worker-src 'self' blob:");
  });

  it("allows HMR only in development", () => {
    const dev = buildCsp("development", "http://localhost:5173");
    expect(dev).toContain("'unsafe-eval'");
    expect(dev).toContain("ws://localhost:*");
    expect(buildCsp("production")).not.toContain("ws://");
  });

  it("allows the loopback install API in both modes", () => {
    for (const mode of ["production", "development"] as const) {
      expect(buildCsp(mode)).toContain("http://localhost:*");
    }
  });
});

describe("isTrustedFrameUrl", () => {
  const appIndexUrl = "file:///opt/app/dist/index.html";

  it("accepts the packaged renderer entry point", () => {
    expect(isTrustedFrameUrl(appIndexUrl, { appIndexUrl })).toBe(true);
    expect(isTrustedFrameUrl(`${appIndexUrl}#/build`, { appIndexUrl })).toBe(true);
    expect(isTrustedFrameUrl(`${appIndexUrl}?mode=ship`, { appIndexUrl })).toBe(true);
  });

  it("accepts any path on the dev server origin", () => {
    const devServerUrl = "http://localhost:5173";
    expect(isTrustedFrameUrl("http://localhost:5173/index.html", { devServerUrl })).toBe(true);
    expect(isTrustedFrameUrl("http://localhost:5174/index.html", { devServerUrl })).toBe(false);
  });

  it("rejects remote origins and other local files", () => {
    expect(isTrustedFrameUrl("https://attacker.example/login", { appIndexUrl })).toBe(false);
    expect(isTrustedFrameUrl("file:///etc/passwd", { appIndexUrl })).toBe(false);
    expect(isTrustedFrameUrl("", { appIndexUrl })).toBe(false);
    expect(isTrustedFrameUrl(appIndexUrl, {})).toBe(false);
  });
});

describe("isSafeExternalUrl", () => {
  it("allows https and loopback http", () => {
    expect(isSafeExternalUrl("https://one.example/auth/v1/login")).toBe(true);
    expect(isSafeExternalUrl("http://localhost:8080/auth/v1/login")).toBe(true);
    expect(isSafeExternalUrl("http://127.0.0.1:8080/auth/v1/login")).toBe(true);
  });

  it("refuses other schemes, remote http, and embedded credentials", () => {
    expect(isSafeExternalUrl("http://attacker.example/login")).toBe(false);
    expect(isSafeExternalUrl("file:///etc/passwd")).toBe(false);
    expect(isSafeExternalUrl("javascript:alert(1)")).toBe(false);
    expect(isSafeExternalUrl("one-control://oauth/callback?code=x")).toBe(false);
    expect(isSafeExternalUrl("https://user:pass@one.example/")).toBe(false);
    expect(isSafeExternalUrl("not a url")).toBe(false);
  });
});

describe("assertSafeGitArg", () => {
  it("returns trimmed operands", () => {
    expect(assertSafeGitArg("  change/add-field  ", "Branch")).toBe("change/add-field");
  });

  it("refuses anything git would read as an option", () => {
    expect(() => assertSafeGitArg("--upload-pack=touch /tmp/pwn", "Clone URL")).toThrow(/must not start with/);
    expect(() => assertSafeGitArg("-c", "Branch")).toThrow(/must not start with/);
  });

  it("refuses empty and control characters", () => {
    expect(() => assertSafeGitArg("", "Branch")).toThrow(/required/);
    expect(() => assertSafeGitArg("main\nrm -rf /", "Branch")).toThrow(/control character/);
    expect(() => assertSafeGitArg("main\0", "Branch")).toThrow(/control character/);
  });
});

describe("assertChangeBranchName", () => {
  it("requires the change/ prefix after the option-shaped reject", () => {
    expect(assertChangeBranchName("change/add-field")).toBe("change/add-field");
    expect(() => assertChangeBranchName("main")).toThrow(/must start with change\//);
    expect(() => assertChangeBranchName("--upload-pack=x")).toThrow(/must not start with/);
  });
});

describe("assertSafeCloneUrl", () => {
  it("accepts the supported transports", () => {
    expect(assertSafeCloneUrl("https://github.com/acme/one.git")).toContain("https://");
    expect(assertSafeCloneUrl("ssh://git@github.com/acme/one.git")).toContain("ssh://");
    expect(assertSafeCloneUrl("git@github.com:acme/one.git")).toContain("git@");
  });

  it("refuses the argument-injection payload that reaches execFile", () => {
    // Reproduced against git 2.39: `git clone --upload-pack=<cmd> <path>` runs <cmd>.
    expect(() => assertSafeCloneUrl("--upload-pack=touch /tmp/pwn;git-upload-pack")).toThrow();
  });

  it("refuses remote helpers and local paths", () => {
    expect(() => assertSafeCloneUrl("ext::sh -c 'touch /tmp/pwn'")).toThrow(/https:\/\//);
    expect(() => assertSafeCloneUrl("file:///tmp/evil-repo")).toThrow(/https:\/\//);
    expect(() => assertSafeCloneUrl("/tmp/evil-repo")).toThrow(/https:\/\//);
    expect(() => assertSafeCloneUrl("http://insecure.example/repo.git")).toThrow(/https:\/\//);
  });
});

describe("git hardening flags", () => {
  it("disables the transports and hooks an untrusted remote could abuse", () => {
    const config = GIT_CLONE_SAFE_CONFIG.join(" ");
    expect(config).toContain("protocol.ext.allow=never");
    expect(config).toContain("protocol.file.allow=never");
    expect(config).toContain("core.hooksPath=/dev/null");
    expect(GIT_CLONE_SAFE_ENV.GIT_ALLOW_PROTOCOL).toBe("https:ssh");
    expect(GIT_CLONE_SAFE_ENV.GIT_TERMINAL_PROMPT).toBe("0");
  });
});
