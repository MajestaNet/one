import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AppBridge } from "../App";
import { ToolsPanel } from "./ToolsPanel";
import { upsertEnvironment } from "../session";

afterEach(() => {
  cleanup();
  Reflect.deleteProperty(window, "one");
  vi.restoreAllMocks();
});

function bridge(fetchImpl?: AppBridge["fetch"], repoPath?: string): AppBridge {
  let session = upsertEnvironment(null, {
    installId: "dev",
    installRole: "test",
    baseUrl: "http://api",
    token: "jwt",
  });
  if (repoPath) session = { ...session, repoPath };
  return {
    session,
    setSession: vi.fn().mockResolvedValue(undefined),
    fetch: fetchImpl ?? vi.fn(),
  };
}

describe("ToolsPanel", () => {
  it("lists ToolSpecs and opens detail", async () => {
    const user = userEvent.setup();
    const fetch = vi
      .fn()
      .mockResolvedValueOnce({
        tools: [
          {
            apiName: "Sales_Open_Pipeline",
            label: "Open pipeline",
            ownership: "managed",
            active: true,
            sortOrder: 10,
          },
        ],
      })
      .mockResolvedValueOnce({
        apiName: "Sales_Open_Pipeline",
        label: "Open pipeline",
        ownership: "managed",
        layout: { mode: "sections" },
        nodes: [{ id: "hdr", kind: "sectionHeader", props: {} }],
        dataBindings: [],
      });
    render(<ToolsPanel bridge={bridge(fetch)} />);
    expect(await screen.findByTestId("tool-spec-list")).toBeTruthy();
    expect(screen.getByTestId("tool-spec-search")).toBeTruthy();
    expect(screen.getByTestId("tool-spec-new")).toBeTruthy();
    const toolbar = screen.getByTestId("tool-toolbar");
    const toolbarButtons = within(toolbar).getAllByRole("button");
    expect(toolbarButtons[0].textContent).toMatch(/New ToolSpec/i);
    expect(toolbarButtons[1].textContent).toMatch(/Refresh/i);
    expect(within(toolbar).getByTestId("tool-spec-search")).toBeTruthy();
    expect(within(screen.getByTestId("tool-spec-list")).getByText("Open pipeline")).toBeTruthy();
    await user.click(screen.getByTestId("tool-spec-item-Sales_Open_Pipeline"));
    expect(await screen.findByTestId("tool-spec-preview")).toBeTruthy();
    expect(screen.getByText(/Managed/)).toBeTruthy();
  });

  it("lists tools alphabetically by label", async () => {
    const fetch = vi.fn().mockResolvedValue({
      tools: [
        { apiName: "Zebra_Board", label: "Zebra", ownership: "custom", sortOrder: 0 },
        { apiName: "Alpha_Board", label: "Alpha", ownership: "managed", sortOrder: 50 },
      ],
    });
    render(<ToolsPanel bridge={bridge(fetch)} />);
    const list = await screen.findByTestId("tool-spec-list");
    const rows = [...list.querySelectorAll("tbody tr")];
    expect(rows.map((row) => row.getAttribute("data-testid"))).toEqual([
      "tool-spec-item-Alpha_Board",
      "tool-spec-item-Zebra_Board",
    ]);
  });

  it("creates a ToolSpec and mirrors yaml", async () => {
    const user = userEvent.setup();
    const onToolsChanged = vi.fn();
    const writeText = vi.fn().mockResolvedValue(true);
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      gitStatus: vi.fn(),
      listTree: vi.fn(),
      readText: vi.fn(),
      writeText,
    };
    const fetch = vi
      .fn()
      .mockResolvedValueOnce({ tools: [] })
      .mockResolvedValueOnce({
        apiName: "My_Tool",
        label: "My tool",
        ownership: "custom",
        layout: { mode: "sections" },
        nodes: [],
        dataBindings: [],
      })
      .mockResolvedValueOnce({ tools: [{ apiName: "My_Tool", label: "My tool", ownership: "custom" }] })
      .mockResolvedValueOnce({
        apiName: "My_Tool",
        label: "My tool",
        ownership: "custom",
        layout: { mode: "sections" },
        nodes: [],
        dataBindings: [],
      });
    render(<ToolsPanel bridge={bridge(fetch, "/tmp/repo")} onToolsChanged={onToolsChanged} />);
    await user.click(await screen.findByTestId("tool-spec-new"));
    const form = await screen.findByTestId("tool-spec-new-form");
    const inputs = form.querySelectorAll("input");
    await user.type(inputs[0], "My tool");
    await user.type(screen.getByTestId("tool-spec-api-name"), "My_Tool");
    await user.click(screen.getByTestId("tool-spec-create"));
    await waitFor(() => expect(onToolsChanged).toHaveBeenCalled());
    expect(writeText).toHaveBeenCalledWith(
      "/tmp/repo",
      "metadata/tools/My_Tool.yaml",
      expect.stringContaining("apiVersion: one.tool/v1"),
    );
  });
});
