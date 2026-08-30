import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { RunToolPanel } from "./RunToolPanel";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("RunToolPanel AuthZ-honest open", () => {
  it("resolves dataBindings via Client query and never paints baked rows", async () => {
    const fetchFn = vi.fn(async (path: string) => {
      if (path === "/client/v1/tools/Live_Tool") {
        return {
          apiName: "Live_Tool",
          label: "Live Tool",
          layout: {
            mode: "sections",
            sections: [{ id: "s", nodeIds: ["table"] }],
          },
          dataBindings: [{ id: "b1", objectApiName: "Account", query: { limit: 5 } }],
          nodes: [
            {
              id: "table",
              kind: "recordTable",
              bindingId: "b1",
              title: "Accounts",
              props: {
                columns: [{ key: "Name", label: "Name" }],
                rows: [{ Name: "BAKED_SECRET" }],
              },
            },
          ],
        };
      }
      if (path === "/client/v1/query") {
        return { records: [{ id: "a1", Name: "Visible Acme" }] };
      }
      throw new Error(`unexpected ${path}`);
    });

    render(<RunToolPanel apiName="Live_Tool" label="Live Tool" fetchFn={fetchFn} />);

    await waitFor(() => expect(screen.queryByTestId("run-tool-loading")).toBeNull());
    expect(screen.queryByText("BAKED_SECRET")).toBeNull();
    expect(screen.getByTestId("run-table-table")).toBeTruthy();
    expect(fetchFn).toHaveBeenCalledWith("/client/v1/query", expect.any(Object));
    const queryCall = fetchFn.mock.calls.find((c) => c[0] === "/client/v1/query");
    expect(queryCall?.[1]).toEqual(
      expect.objectContaining({
        method: "POST",
        body: expect.stringContaining("Account"),
      }),
    );
  });

  it("shows binding warnings when Client query fails and keeps empty table", async () => {
    const fetchFn = vi.fn(async (path: string) => {
      if (path.startsWith("/client/v1/tools/")) {
        return {
          apiName: "Denied_Tool",
          label: "Denied",
          layout: { mode: "sections", sections: [{ id: "s", nodeIds: ["table"] }] },
          dataBindings: [{ id: "b1", objectApiName: "Account", query: {} }],
          nodes: [
            {
              id: "table",
              kind: "recordTable",
              bindingId: "b1",
              props: {
                columns: [{ key: "Name", label: "Name" }],
                rows: [{ Name: "BAKED" }],
              },
            },
          ],
        };
      }
      throw new Error("forbidden");
    });

    render(<RunToolPanel apiName="Denied_Tool" label="Denied" fetchFn={fetchFn} />);
    await waitFor(() => expect(screen.getByTestId("run-tool-binding-warnings")).toBeTruthy());
    expect(screen.queryByText("BAKED")).toBeNull();
  });

  it("renders open-only ToolSpecs read-only and suppresses interaction callbacks", async () => {
    const user = userEvent.setup();
    const onAskAgent = vi.fn();
    const fetchFn = vi.fn(async (path: string) => {
      if (path === "/client/v1/tools/Read_Only_Tool") {
        return {
          apiName: "Read_Only_Tool",
          label: "Read only",
          permissions: {
            canOpen: true,
            canInteract: false,
            canModify: false,
            canPublish: false,
          },
          layout: { mode: "sections", sections: [{ id: "s", nodeIds: ["actions"] }] },
          nodes: [
            {
              id: "actions",
              kind: "actionChipGroup",
              props: { actions: [{ label: "Ask agent", prompt: "Summarize this Tool" }] },
            },
          ],
        };
      }
      throw new Error(`unexpected ${path}`);
    });

    render(
      <RunToolPanel
        apiName="Read_Only_Tool"
        label="Read only"
        fetchFn={fetchFn}
        onAskAgent={onAskAgent}
      />,
    );

    await waitFor(() => expect(screen.getByTestId("run-tool-read-only")).toBeTruthy());
    await user.click(screen.getByTestId("canvas-action-chip-actions-Ask agent"));
    expect(onAskAgent).not.toHaveBeenCalled();
    expect(fetchFn).toHaveBeenCalledTimes(1);
  });
});

describe("RunToolErrorBoundary", () => {
  it("isolates render failures without taking down the host", async () => {
    const user = userEvent.setup();
    const Boom = () => {
      throw new Error("boom-node");
    };
    const { RunToolErrorBoundary } = await import("./RunToolErrorBoundary");
    render(
      <div>
        <p data-testid="shell-alive">shell</p>
        <RunToolErrorBoundary label="Bad">
          <Boom />
        </RunToolErrorBoundary>
      </div>,
    );
    expect(screen.getByTestId("shell-alive")).toBeTruthy();
    expect(screen.getByTestId("run-tool-error-boundary")).toBeTruthy();
    expect(screen.getByText(/boom-node/)).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /try again/i }));
  });
});
