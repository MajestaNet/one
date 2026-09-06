import { describe, expect, it } from "vitest";
import {
  extractProtocolUrl,
  isAppProtocolUrl,
  protocolClientLaunch,
  shouldRegisterProtocolClient,
} from "./protocol";

describe("extractProtocolUrl", () => {
  it("returns the first one-control deep link from argv", () => {
    expect(
      extractProtocolUrl(
        ["/path/to/electron", "one-control://oauth?code=1&state=s", "--flag"],
        "one-control",
      ),
    ).toBe("one-control://oauth?code=1&state=s");
  });

  it("ignores unrelated argv entries", () => {
    expect(extractProtocolUrl(["electron", "https://example.com", "--foo"], "one-control")).toBeUndefined();
    expect(extractProtocolUrl([], "one-control")).toBeUndefined();
  });
});

describe("isAppProtocolUrl", () => {
  it("matches only the configured scheme", () => {
    expect(isAppProtocolUrl("one-control://x", "one-control")).toBe(true);
    expect(isAppProtocolUrl("https://example.com", "one-control")).toBe(false);
    expect(isAppProtocolUrl("", "one-control")).toBe(false);
  });
});

describe("shouldRegisterProtocolClient", () => {
  it("skips unpackaged macOS so Electron.app is not the one-control handler", () => {
    expect(shouldRegisterProtocolClient({ packaged: false, defaultApp: true, platform: "darwin" })).toBe(
      false,
    );
    expect(shouldRegisterProtocolClient({ packaged: false, defaultApp: false, platform: "darwin" })).toBe(
      false,
    );
  });

  it("registers packaged Mac and unpackaged Windows/Linux", () => {
    expect(shouldRegisterProtocolClient({ packaged: true, defaultApp: false, platform: "darwin" })).toBe(true);
    expect(shouldRegisterProtocolClient({ packaged: false, defaultApp: true, platform: "win32" })).toBe(true);
    expect(shouldRegisterProtocolClient({ packaged: false, defaultApp: true, platform: "linux" })).toBe(true);
  });
});

describe("protocolClientLaunch", () => {
  it("passes execPath + script for unpackaged Windows/Linux", () => {
    expect(
      protocolClientLaunch({
        packaged: false,
        defaultApp: true,
        platform: "win32",
        execPath: "/e/Electron",
        argv: ["/e/Electron", "/app/main.js"],
      }),
    ).toEqual({ execPath: "/e/Electron", args: ["/app/main.js"] });
  });

  it("uses the default client for packaged apps", () => {
    expect(
      protocolClientLaunch({
        packaged: true,
        defaultApp: false,
        platform: "darwin",
        execPath: "/Apps/Control.app/Contents/MacOS/Control",
        argv: ["/Apps/Control.app/Contents/MacOS/Control"],
      }),
    ).toEqual({});
  });
});
