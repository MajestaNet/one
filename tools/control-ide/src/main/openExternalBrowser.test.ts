import { describe, expect, it, vi } from "vitest";
import { openInOsBrowser } from "./openExternalBrowser";

describe("openInOsBrowser", () => {
  it("uses the macOS opener so loopback http is not handed back to Electron", async () => {
    const openExternal = vi.fn();
    const openWithSystemOpener = vi.fn().mockResolvedValue(undefined);
    await openInOsBrowser("http://localhost:8080/auth/v1/login?x=1&y=2", {
      platform: "darwin",
      openExternal,
      openWithSystemOpener,
    });
    expect(openWithSystemOpener).toHaveBeenCalledWith("http://localhost:8080/auth/v1/login?x=1&y=2");
    expect(openExternal).not.toHaveBeenCalled();
  });

  it("uses shell.openExternal on other platforms", async () => {
    const openExternal = vi.fn().mockResolvedValue(undefined);
    const openWithSystemOpener = vi.fn();
    await openInOsBrowser("https://one.example/login", {
      platform: "linux",
      openExternal,
      openWithSystemOpener,
    });
    expect(openExternal).toHaveBeenCalledWith("https://one.example/login");
    expect(openWithSystemOpener).not.toHaveBeenCalled();
  });
});
