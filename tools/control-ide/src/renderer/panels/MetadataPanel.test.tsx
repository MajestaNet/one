import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AppBridge } from "../App";
import { MetadataPanel } from "./MetadataPanel";

afterEach(() => {
  cleanup();
  Reflect.deleteProperty(window, "one");
});

function bridge(repoPath?: string): AppBridge {
  return {
    session: repoPath ? { baseUrl: "http://x", token: "t", repoPath } : { baseUrl: "http://x", token: "t" },
    setSession: vi.fn().mockResolvedValue(undefined),
    fetch: vi.fn(),
  };
}

describe("MetadataPanel", () => {
  it("requires Electron + repo path to refresh", async () => {
    const user = userEvent.setup();
    render(<MetadataPanel bridge={bridge()} />);
    await user.click(screen.getByRole("button", { name: /Refresh tree/i }));
    expect(await screen.findByText(/Electron required/i)).toBeTruthy();
  });

  it("lists yaml files, opens one, and saves editor content", async () => {
    const user = userEvent.setup();
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      gitStatus: vi.fn(),
      listTree: vi.fn().mockResolvedValue(["metadata/objects/Account.yaml"]),
      readText: vi.fn().mockResolvedValue("apiName: Account\n"),
      writeText: vi.fn().mockResolvedValue(true),
    };

    render(<MetadataPanel bridge={bridge("/tmp/repo")} />);
    await user.click(screen.getByRole("button", { name: /Refresh tree/i }));
    expect(await screen.findByRole("button", { name: "metadata/objects/Account.yaml" })).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "metadata/objects/Account.yaml" }));
    await waitFor(() => expect(window.one.readText).toHaveBeenCalledWith("/tmp/repo", "metadata/objects/Account.yaml"));

    await user.click(screen.getByRole("button", { name: /Save file/i }));
    await waitFor(() =>
      expect(window.one.writeText).toHaveBeenCalledWith(
        "/tmp/repo",
        "metadata/objects/Account.yaml",
        "apiName: Account\n",
      ),
    );
  });

  it("surfaces read errors", async () => {
    const user = userEvent.setup();
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      gitStatus: vi.fn(),
      listTree: vi.fn().mockResolvedValue(["metadata/objects/Broken.yaml"]),
      readText: vi.fn().mockRejectedValue(new Error("ENOENT")),
      writeText: vi.fn(),
    };

    render(<MetadataPanel bridge={bridge("/tmp/repo")} />);
    await user.click(screen.getByRole("button", { name: /Refresh tree/i }));
    await user.click(await screen.findByRole("button", { name: "metadata/objects/Broken.yaml" }));
    expect(await screen.findByText(/ENOENT/i)).toBeTruthy();
  });

  it("surfaces write errors", async () => {
    const user = userEvent.setup();
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      gitStatus: vi.fn(),
      listTree: vi.fn().mockResolvedValue(["metadata/objects/Account.yaml"]),
      readText: vi.fn().mockResolvedValue("apiName: Account\n"),
      writeText: vi.fn().mockRejectedValue(new Error("EACCES")),
    };

    render(<MetadataPanel bridge={bridge("/tmp/repo")} />);
    await user.click(screen.getByRole("button", { name: /Refresh tree/i }));
    await user.click(await screen.findByRole("button", { name: "metadata/objects/Account.yaml" }));
    await waitFor(() => expect(window.one.readText).toHaveBeenCalled());
    await user.click(screen.getByRole("button", { name: /Save file/i }));
    expect(await screen.findByText(/EACCES/i)).toBeTruthy();
  });

  it("opens focusPath when provided", async () => {
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      gitStatus: vi.fn(),
      listTree: vi.fn().mockResolvedValue(["metadata/objects/Account.yaml"]),
      readText: vi.fn().mockResolvedValue("apiName: Account\n"),
      writeText: vi.fn(),
    };
    const onFocusConsumed = vi.fn();
    render(
      <MetadataPanel
        bridge={bridge("/tmp/repo")}
        focusPath="metadata/objects/Account.yaml"
        onFocusConsumed={onFocusConsumed}
      />,
    );
    await waitFor(() =>
      expect(window.one.readText).toHaveBeenCalledWith("/tmp/repo", "metadata/objects/Account.yaml"),
    );
    await waitFor(() => expect(onFocusConsumed).toHaveBeenCalled());
  });
});
