import { describe, expect, it, vi } from "vitest";
import {
  configureUpdater,
  disabledStatus,
  gateDisabledReason,
  resolveFeedUrl,
  shouldEnableUpdates,
  validateFeedUrl,
  type UpdaterLike,
} from "./updates";

describe("validateFeedUrl", () => {
  it("requires https and an allowlisted host", () => {
    expect(validateFeedUrl("https://cdn.example/ide/", "cdn.example")).toBe("https://cdn.example/ide");
    expect(() => validateFeedUrl("http://cdn.example/ide/", "cdn.example")).toThrow(/https/);
    expect(() => validateFeedUrl("https://cdn.example/ide/", "")).toThrow(/ALLOWLIST/);
    expect(() => validateFeedUrl("https://evil.example/ide/", "cdn.example")).toThrow(/not in UPDATE_FEED_HOST_ALLOWLIST/);
    expect(() => validateFeedUrl("https://user:pass@cdn.example/ide/", "cdn.example")).toThrow(/credentials/);
  });
});

describe("resolveFeedUrl", () => {
  it("returns undefined when unset", () => {
    expect(resolveFeedUrl({} as NodeJS.ProcessEnv)).toEqual({ url: undefined });
    expect(resolveFeedUrl({ UPDATE_FEED_URL: "  " } as NodeJS.ProcessEnv)).toEqual({ url: undefined });
  });

  it("returns a validated URL when allowlisted", () => {
    expect(
      resolveFeedUrl({
        UPDATE_FEED_URL: " https://cdn.example/ide/ ",
        UPDATE_FEED_HOST_ALLOWLIST: "cdn.example,updates.example",
      } as NodeJS.ProcessEnv),
    ).toEqual({ url: "https://cdn.example/ide" });
  });

  it("returns an error rather than a URL when validation fails", () => {
    const resolved = resolveFeedUrl({
      UPDATE_FEED_URL: "https://cdn.example/ide/",
    } as NodeJS.ProcessEnv);
    expect(resolved.url).toBeUndefined();
    expect(resolved.error).toMatch(/ALLOWLIST/);
  });
});

describe("shouldEnableUpdates", () => {
  it("requires packaged + feed URL and no feed error", () => {
    expect(shouldEnableUpdates({ packaged: false, feedUrl: "https://x" })).toBe(false);
    expect(shouldEnableUpdates({ packaged: true, feedUrl: undefined })).toBe(false);
    expect(shouldEnableUpdates({ packaged: true, feedUrl: "https://x", feedError: "nope" })).toBe(false);
    expect(shouldEnableUpdates({ packaged: true, feedUrl: "https://x" })).toBe(true);
  });
});

describe("gateDisabledReason", () => {
  it("prefers a concrete feed validation error", () => {
    expect(gateDisabledReason({ packaged: false, feedUrl: "https://x" })).toMatch(/packaged/i);
    expect(
      gateDisabledReason({ packaged: true, feedUrl: undefined, feedError: "host not allowlisted" }),
    ).toBe("host not allowlisted");
    expect(gateDisabledReason({ packaged: true, feedUrl: undefined })).toMatch(/UPDATE_FEED_URL/);
  });
});

describe("disabledStatus", () => {
  it("builds a disabled status payload", () => {
    expect(disabledStatus("nope")).toEqual({ state: "disabled", message: "nope" });
  });
});

describe("configureUpdater", () => {
  it("sets the generic feed URL and keeps autoDownload off until signed releases", () => {
    const updater: UpdaterLike = {
      autoDownload: true,
      setFeedURL: vi.fn(),
      checkForUpdates: vi.fn(),
      quitAndInstall: vi.fn(),
      on: vi.fn(),
    };
    configureUpdater(updater, "https://cdn.example/ide/");
    expect(updater.autoDownload).toBe(false);
    expect(updater.setFeedURL).toHaveBeenCalledWith({
      provider: "generic",
      url: "https://cdn.example/ide",
    });
  });
});
