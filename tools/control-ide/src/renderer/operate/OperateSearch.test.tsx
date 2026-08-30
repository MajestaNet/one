import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { OperateSearch } from "./OperateSearch";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("OperateSearch", () => {
  it("types a query, shows grouped hits, and Enter opens the record", async () => {
    const user = userEvent.setup();
    const fetchFn = vi.fn(async () => ({
      query: "acme",
      hits: [
        { id: "a-1", object: "Account", title: "Acme SearchCo", subtitle: "415-555" },
        { id: "c-1", object: "Contact", title: "Jane Doe", subtitle: "jane@search.test" },
      ],
    }));
    const onOpenHit = vi.fn();
    render(<OperateSearch fetchFn={fetchFn} onOpenHit={onOpenHit} />);

    const input = screen.getByTestId("operate-global-search");
    await user.type(input, "acme");

    await waitFor(() => expect(fetchFn).toHaveBeenCalled());
    expect(fetchFn.mock.calls[0]?.[0]).toBe("/client/v1/search");
    expect(await screen.findByText("Acme SearchCo")).toBeTruthy();
    expect(screen.getByText("Jane Doe")).toBeTruthy();

    await user.keyboard("{Enter}");
    expect(onOpenHit).toHaveBeenCalledWith(
      expect.objectContaining({ id: "a-1", object: "Account" }),
    );
  });

  it("does not call search below min length and shows an empty 403/error honestly", async () => {
    const user = userEvent.setup();
    const fetchFn = vi.fn(async () => {
      throw new Error("403 forbidden");
    });
    render(<OperateSearch fetchFn={fetchFn} onOpenHit={vi.fn()} />);
    const input = screen.getByTestId("operate-global-search");
    await user.type(input, "a");
    await new Promise((r) => setTimeout(r, 250));
    expect(fetchFn).not.toHaveBeenCalled();
    expect(screen.getByText(/at least 2 characters/i)).toBeTruthy();

    await user.type(input, "cme");
    await waitFor(() => expect(fetchFn).toHaveBeenCalled());
    expect(screen.getByText(/403 forbidden/i)).toBeTruthy();
    expect(screen.queryByTestId("operate-search-hit-a-1")).toBeNull();
  });

  it("focuses the field on Ctrl/Cmd+K", async () => {
    const user = userEvent.setup();
    render(<OperateSearch fetchFn={vi.fn()} onOpenHit={vi.fn()} />);
    const input = screen.getByTestId("operate-global-search") as HTMLInputElement;
    expect(document.activeElement).not.toBe(input);
    await user.keyboard("{Control>}k{/Control}");
    expect(document.activeElement).toBe(input);
  });
});
