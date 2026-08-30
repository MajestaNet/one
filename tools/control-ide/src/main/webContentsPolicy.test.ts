import { describe, expect, it } from "vitest";
import { decideNavigation, decideWindowOpen, shouldAllowNavigation } from "./webContentsPolicy";

const trust = {
  appIndexUrl: "file:///app/dist/index.html",
  devServerUrl: "http://127.0.0.1:5173/",
};

describe("decideWindowOpen", () => {
  it("always denies in-app windows and routes safe URLs externally", () => {
    expect(decideWindowOpen("https://login.example/oauth")).toEqual({
      action: "deny",
      openExternally: "https://login.example/oauth",
    });
    expect(decideWindowOpen("http://localhost:8080/auth/v1/login")).toEqual({
      action: "deny",
      openExternally: "http://localhost:8080/auth/v1/login",
    });
    expect(decideWindowOpen("file:///etc/passwd")).toEqual({ action: "deny" });
    expect(decideWindowOpen("https://user:pass@evil.example/")).toEqual({ action: "deny" });
  });
});

describe("decideNavigation / shouldAllowNavigation", () => {
  it("allows only the app renderer document", () => {
    expect(shouldAllowNavigation("file:///app/dist/index.html", trust)).toBe(true);
    expect(shouldAllowNavigation("http://127.0.0.1:5173/", trust)).toBe(true);
    expect(shouldAllowNavigation("https://evil.example/", trust)).toBe(false);
  });

  it("prevents foreign navigation and opens safe URLs externally", () => {
    expect(decideNavigation("file:///app/dist/index.html", trust)).toEqual({ prevent: false });
    expect(decideNavigation("https://login.example/", trust)).toEqual({
      prevent: true,
      openExternally: "https://login.example/",
    });
    expect(decideNavigation("file:///tmp/x", trust)).toEqual({ prevent: true });
  });
});
