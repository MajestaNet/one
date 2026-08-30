import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AppBridge } from "../App";
import { PackagesPanel } from "./PackagesPanel";
import { upsertEnvironment } from "../session";

afterEach(() => {
  cleanup();
  Reflect.deleteProperty(window, "one");
  vi.restoreAllMocks();
});

function bridge(fetchImpl?: AppBridge["fetch"]): AppBridge {
  const session = upsertEnvironment(null, {
    installId: "dev",
    installRole: "test",
    baseUrl: "http://api",
    token: "jwt",
  });
  return {
    session,
    setSession: vi.fn().mockResolvedValue(undefined),
    fetch: fetchImpl ?? vi.fn(),
  };
}

const CATALOG = {
  packages: [
    {
      name: "core",
      label: "Core",
      optional: false,
      enabled: true,
      objectApiNames: ["Account", "Contact"],
    },
    {
      name: "notes",
      label: "Notes",
      description: "Note object",
      optional: true,
      enabled: false,
      dependsOn: ["core"],
      objectApiNames: ["Note"],
      version: "1.0.0",
    },
    {
      name: "sales",
      label: "Sales",
      optional: true,
      enabled: false,
      dependsOn: ["core", "catalog"],
      objectApiNames: ["Opportunity", "Quote"],
    },
    {
      name: "catalog",
      label: "Catalog",
      optional: true,
      enabled: false,
      dependsOn: ["core"],
      objectApiNames: ["Product"],
    },
  ],
};

describe("PackagesPanel", () => {
  it("lists packages and enables notes when deps are met", async () => {
    const user = userEvent.setup();
    const fetch = vi
      .fn()
      .mockResolvedValueOnce(CATALOG)
      .mockResolvedValueOnce({})
      .mockResolvedValueOnce({
        packages: CATALOG.packages.map((p) =>
          p.name === "notes" ? { ...p, enabled: true } : p,
        ),
      });
    render(<PackagesPanel bridge={bridge(fetch)} />);
    expect(await screen.findByTestId("pkg-list")).toBeTruthy();
    expect(screen.getByTestId("pkg-search")).toBeTruthy();
    expect(screen.getByRole("button", { name: /Refresh/i })).toBeTruthy();
    const toolbar = screen.getByTestId("tool-toolbar");
    const toolbarButtons = within(toolbar).getAllByRole("button");
    expect(toolbarButtons[0].textContent).toMatch(/Refresh/i);
    expect(within(toolbar).getByTestId("pkg-search")).toBeTruthy();
    const rows = [...screen.getByTestId("pkg-list").querySelectorAll("tbody tr")];
    expect(rows.map((row) => row.getAttribute("data-testid"))).toEqual([
      "pkg-row-catalog",
      "pkg-row-core",
      "pkg-row-notes",
      "pkg-row-sales",
    ]);
    expect(screen.getByTestId("pkg-row-notes")).toBeTruthy();
    expect(screen.getByText("Note")).toBeTruthy();
    await user.click(screen.getByTestId("pkg-enable-notes"));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/metadata/v1/packages/notes/enable",
        expect.objectContaining({ method: "POST" }),
      ),
    );
    await waitFor(() => expect(screen.getByTestId("pkg-disable-notes")).toBeTruthy());
  });

  it("blocks enable when dependencies are missing", async () => {
    const fetch = vi.fn().mockResolvedValue(CATALOG);
    render(<PackagesPanel bridge={bridge(fetch)} />);
    await screen.findByTestId("pkg-row-sales");
    expect(screen.queryByTestId("pkg-enable-sales")).toBeNull();
    expect(within(screen.getByTestId("pkg-row-sales")).getByText(/Needs dependencies/i)).toBeTruthy();
  });

  it("shows connect empty state without session token", () => {
    render(
      <PackagesPanel bridge={{ session: null, setSession: vi.fn(), fetch: vi.fn() }} />,
    );
    expect(screen.getByText(/Connect an environment/i)).toBeTruthy();
  });
});
