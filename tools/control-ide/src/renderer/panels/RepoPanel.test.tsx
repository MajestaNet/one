import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AppBridge } from "../App";
import { RepoPanel } from "./RepoPanel";

afterEach(() => {
  cleanup();
  Reflect.deleteProperty(window, "one");
});

function bridge(session: AppBridge["session"] = { baseUrl: "http://x", token: "t" }): AppBridge {
  return {
    session,
    setSession: vi.fn().mockResolvedValue(undefined),
    fetch: vi.fn().mockResolvedValue({ customerId: "acme", customerRepoUrl: "https://git.example/acme.git" }),
  };
}

describe("RepoPanel", () => {
  it("requires a local folder before initialize", async () => {
    render(<RepoPanel bridge={bridge()} />);
    expect((screen.getByTestId("repo-initialize-remote") as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByTestId("repo-pull-org") as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByTestId("repo-path-display").textContent).toMatch(/No folder selected/i);
  });

  it("chooses a local folder via Electron dialog and saves path", async () => {
    const user = userEvent.setup();
    const setSession = vi.fn().mockResolvedValue(undefined);
    const chooseLocalFolder = vi.fn().mockResolvedValue({ ok: true, path: "/data/picked" });
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      chooseLocalFolder,
      gitStatus: vi.fn(),
      listTree: vi.fn(),
      readText: vi.fn(),
      writeText: vi.fn(),
    };
    const b = bridge();
    b.setSession = setSession;
    render(<RepoPanel bridge={b} />);
    await user.click(screen.getByTestId("repo-browse"));
    await waitFor(() => expect(chooseLocalFolder).toHaveBeenCalled());
    expect(setSession).toHaveBeenCalledWith(expect.objectContaining({ repoPath: "/data/picked" }));
    expect(screen.getByTestId("repo-path-display").textContent).toMatch(/\/data\/picked/);
  });

  it("initializes remote, persists URL, and clones into chosen folder", async () => {
    const user = userEvent.setup();
    const setSession = vi.fn().mockResolvedValue(undefined);
    const gitClone = vi.fn().mockResolvedValue({ ok: true, path: "/data/customer" });
    const fetch = vi.fn().mockImplementation(async (path: string) => {
      if (path === "/deploy/v1/environment") {
        return { customerId: "acme", customerRepoUrl: "https://git.example/acme.git" };
      }
      if (path === "/deploy/v1/packages/initialize-repo") {
        return { customerRepoUrl: "https://git.example/acme.git", commitSha: "abcdef0123456789" };
      }
      return {};
    });
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      gitClone,
      gitStatus: vi.fn(),
      listTree: vi.fn(),
      readText: vi.fn(),
      writeText: vi.fn(),
    };
    const b = bridge({ baseUrl: "http://x", token: "t", repoPath: "/data/customer" });
    b.setSession = setSession;
    b.fetch = fetch;
    render(<RepoPanel bridge={b} />);
    await waitFor(() => expect(screen.getByTestId("repo-pull-org").textContent).toMatch(/acme/i));
    await user.click(screen.getByTestId("repo-initialize-remote"));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/deploy/v1/packages/initialize-repo",
        expect.objectContaining({ method: "POST" }),
      ),
    );
    await waitFor(() =>
      expect(gitClone).toHaveBeenCalledWith("https://git.example/acme.git", "/data/customer"),
    );
    expect(await screen.findByTestId("repo-info")).toBeTruthy();
    expect(screen.getByTestId("repo-info").textContent).toMatch(/Remote initialized/i);
  });

  it("pulls from org into local folder", async () => {
    const user = userEvent.setup();
    const setSession = vi.fn().mockResolvedValue(undefined);
    const importExportZip = vi.fn().mockResolvedValue({ ok: true, path: "/data/customer" });
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      importExportZip,
      gitStatus: vi.fn().mockResolvedValue({ ok: true, branch: "main", status: "" }),
      listTree: vi.fn(),
      readText: vi.fn(),
      writeText: vi.fn(),
    };
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      arrayBuffer: async () => {
        // minimal PK zip header bytes
        return new Uint8Array([0x50, 0x4b, 0x03, 0x04, 0x00]).buffer;
      },
      text: async () => "",
    });
    vi.stubGlobal("fetch", fetchMock);
    const b = bridge({
      baseUrl: "http://x",
      token: "t",
      repoPath: "/data/customer",
    });
    b.setSession = setSession;
    render(<RepoPanel bridge={b} />);
    await waitFor(() => expect(screen.getByTestId("repo-pull-org").textContent).toMatch(/acme/i));
    await user.click(screen.getByTestId("repo-pull-org"));
    await waitFor(() => expect(importExportZip).toHaveBeenCalled());
    expect(await screen.findByTestId("repo-info")).toBeTruthy();
    expect(screen.getByTestId("repo-info").textContent).toMatch(/Pulled from org acme/i);
    vi.unstubAllGlobals();
  });

  it("opens folder in the system editor", async () => {
    const user = userEvent.setup();
    const openInEditor = vi.fn().mockResolvedValue({ ok: true, editor: "code" });
    const registerRepoRoot = vi.fn().mockResolvedValue({ ok: true, path: "/data/customer" });
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      openInEditor,
      registerRepoRoot,
      gitStatus: vi.fn(),
      listTree: vi.fn(),
      readText: vi.fn(),
      writeText: vi.fn(),
    };
    render(
      <RepoPanel bridge={bridge({ baseUrl: "http://x", token: "t", repoPath: "/data/customer" })} />,
    );
    await user.click(screen.getByTestId("repo-open-editor"));
    await waitFor(() => expect(openInEditor).toHaveBeenCalledWith("/data/customer", "auto"));
    expect(screen.getByTestId("repo-editor-note")).toBeTruthy();
  });

  it("shows connect empty state without session", () => {
    render(<RepoPanel bridge={bridge(null)} />);
    expect(screen.getByText(/Connect an environment/i)).toBeTruthy();
  });

  it("does not render branch/commit sections", () => {
    render(<RepoPanel bridge={bridge({ baseUrl: "http://x", token: "t", repoPath: "/data/t" })} />);
    expect(screen.queryByTestId("repo-section-branch")).toBeNull();
    expect(screen.queryByTestId("repo-section-commit")).toBeNull();
    expect(screen.queryByTestId("repo-commit")).toBeNull();
  });
});
