import { describe, expect, it, vi } from "vitest";
import { assertTrustedSender, createTrustedHandle } from "./ipcTrust";

const trust = {
  appIndexUrl: "file:///app/dist/index.html",
  devServerUrl: "http://localhost:5173/",
};

describe("assertTrustedSender", () => {
  it("allows the packaged app index and the Vite dev origin", () => {
    expect(() =>
      assertTrustedSender({ senderFrame: { url: "file:///app/dist/index.html" } }, trust),
    ).not.toThrow();
    expect(() =>
      assertTrustedSender({ senderFrame: { url: "http://localhost:5173/some-route" } }, trust),
    ).not.toThrow();
  });

  it("refuses a remote or empty frame URL", () => {
    expect(() =>
      assertTrustedSender({ senderFrame: { url: "https://evil.example/" } }, trust),
    ).toThrow(/untrusted frame/i);
    expect(() => assertTrustedSender({ senderFrame: { url: "" } }, trust)).toThrow(/unknown/i);
    expect(() => assertTrustedSender({}, trust)).toThrow(/untrusted frame/i);
  });
});

describe("createTrustedHandle", () => {
  it("validates the sender before invoking the handler", async () => {
    const registrations = new Map<string, (event: { senderFrame?: { url?: string } }, ...args: unknown[]) => unknown>();
    const handle = createTrustedHandle((channel, listener) => {
      registrations.set(channel, listener);
    }, trust);
    const inner = vi.fn().mockResolvedValue({ ok: true });
    handle("fs:readText", inner);

    const listener = registrations.get("fs:readText")!;
    expect(() => listener({ senderFrame: { url: "https://evil.example/" } }, "/")).toThrow(
      /untrusted frame/i,
    );
    expect(inner).not.toHaveBeenCalled();

    await expect(
      Promise.resolve(
        listener({ senderFrame: { url: "file:///app/dist/index.html" } }, "repo", "a.yaml"),
      ),
    ).resolves.toEqual({ ok: true });
    expect(inner).toHaveBeenCalledWith(
      { senderFrame: { url: "file:///app/dist/index.html" } },
      "repo",
      "a.yaml",
    );
  });
});
