import { afterEach, describe, expect, it, vi } from "vitest";
import { openExternalUrl } from "./external";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  delete (window as { one?: unknown }).one;
});

describe("openExternalUrl", () => {
  it("routes through the Electron bridge, never window.open", async () => {
    const openExternal = vi.fn().mockResolvedValue({ ok: true });
    const open = vi.fn();
    vi.stubGlobal("open", open);
    (window as unknown as { one: unknown }).one = { openExternal };

    await openExternalUrl("https://one.example/auth/v1/login");

    expect(openExternal).toHaveBeenCalledWith("https://one.example/auth/v1/login");
    expect(open).not.toHaveBeenCalled();
  });

  it("surfaces a refusal from the main process", async () => {
    (window as unknown as { one: unknown }).one = {
      openExternal: vi.fn().mockResolvedValue({ ok: false, error: "Refused to open a non-https URL" }),
    };

    await expect(openExternalUrl("http://attacker.example")).rejects.toThrow(/Refused to open/);
  });

  it("falls back to window.open only in the browser preview", async () => {
    const open = vi.fn().mockReturnValue({});
    vi.stubGlobal("open", open);

    await openExternalUrl("https://one.example/auth/v1/login");

    expect(open).toHaveBeenCalledWith("https://one.example/auth/v1/login", "_blank", "noopener,noreferrer");
  });

  it("reports a blocked pop-up in the browser preview", async () => {
    vi.stubGlobal("open", vi.fn().mockReturnValue(null));
    await expect(openExternalUrl("https://one.example")).rejects.toThrow(/Pop|pop/);
  });
});
