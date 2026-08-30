import { describe, expect, it, vi } from "vitest";
import { IPC_CHANNELS, buildOneApi, listedInvokeChannels } from "./oneApi";

describe("oneApi", () => {
  it("exposes a stable invoke channel set for the preload bridge", () => {
    const channels = listedInvokeChannels();
    expect(channels).toContain("session:get");
    expect(channels).toContain("shell:openExternal");
    expect(channels).toContain("fs:readText");
    expect(channels).toContain("git:clone");
    expect(channels).toContain("git:pull");
    expect(channels).toContain("repo:importExportZip");
    expect(channels).toContain("updates:check");
    expect(channels).not.toContain("oauth:callback");
    expect(new Set(channels).size).toBe(channels.length);
  });

  it("routes every bridge method to the expected IPC channel", async () => {
    const invoke = vi.fn().mockResolvedValue({ ok: true });
    const on = vi.fn();
    const removeListener = vi.fn();
    const api = buildOneApi(invoke, on, removeListener);

    await api.getSession();
    await api.setSession(null);
    await api.isSessionEncryptionAvailable();
    await api.registerRepoRoot("/tmp/repo");
    await api.chooseRepoRoot();
    await api.chooseLocalFolder();
    await api.openExternal("https://example.com");
    await api.gitStatus("/tmp/repo");
    await api.gitClone("https://example.com/r.git", "/tmp/dest");
    await api.gitPull("/tmp/repo");
    await api.gitCreateBranch("/tmp/repo", "change/x");
    await api.gitPush("/tmp/repo");
    await api.gitCommit("/tmp/repo", "msg");
    await api.initSampleRepo("/tmp/dest");
    await api.importExportZip("/tmp/dest", "UEsDBBQ=");
    await api.openInEditor("/tmp/repo", "code");
    await api.exportRepoZip("/tmp/repo");
    await api.listTree("/tmp/repo", "metadata");
    await api.readText("/tmp/repo", "a.yaml");
    await api.writeText("/tmp/repo", "a.yaml", "x");
    await api.getUpdateStatus();
    await api.checkForUpdates();
    await api.installUpdate();

    const invoked = invoke.mock.calls.map((c) => c[0] as string);
    expect(invoked).toEqual(listedInvokeChannels());
    expect(invoke.mock.calls.find((c) => c[0] === IPC_CHANNELS.shellOpenExternal)?.[1]).toBe(
      "https://example.com",
    );
  });

  it("subscribes and unsubscribes the oauth callback channel", () => {
    const invoke = vi.fn();
    const on = vi.fn();
    const removeListener = vi.fn();
    const api = buildOneApi(invoke, on, removeListener);
    const handler = vi.fn();
    const unsubscribe = api.onOAuthCallback(handler);
    expect(on).toHaveBeenCalledWith(IPC_CHANNELS.oauthCallback, expect.any(Function));
    const listener = on.mock.calls[0][1] as (event: unknown, url: string) => void;
    listener({}, "one-control://oauth?code=1&state=s");
    expect(handler).toHaveBeenCalledWith("one-control://oauth?code=1&state=s");
    unsubscribe();
    expect(removeListener).toHaveBeenCalledWith(IPC_CHANNELS.oauthCallback, listener);
  });
});
