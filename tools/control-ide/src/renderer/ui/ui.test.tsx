import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Button, DataTable, EmptyState, FileDrop, KeyValueList, SearchField, StatusBadge, ToolSurface, ToolToolbar } from "./index";

afterEach(() => cleanup());

describe("ui primitives", () => {
  it("renders Button busy state", () => {
    render(
      <Button variant="primary" busy>
        Saving
      </Button>,
    );
    expect(screen.getByRole("button", { name: /Saving/i }).getAttribute("aria-busy")).toBe("true");
    expect((screen.getByRole("button", { name: /Saving/i }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("renders EmptyState", () => {
    render(<EmptyState title="Nothing here" description="Add an item." />);
    expect(screen.getByTestId("empty-state").textContent).toMatch(/Nothing here/);
  });

  it("renders DataTable rows and empty", () => {
    const { rerender } = render(
      <DataTable columns={[{ key: "name", label: "Name" }]} rows={[{ name: "Acme" }]} />,
    );
    expect(screen.getByText("Acme")).toBeTruthy();
    rerender(<DataTable columns={[{ key: "name", label: "Name" }]} rows={[]} emptyLabel="No rows" />);
    expect(screen.getByText("No rows")).toBeTruthy();
  });

  it("renders KeyValueList and StatusBadge", () => {
    render(
      <>
        <KeyValueList items={[{ label: "Customer", value: "acme" }]} />
        <StatusBadge tone="success">Ready</StatusBadge>
      </>,
    );
    expect(screen.getByText("Customer")).toBeTruthy();
    expect(screen.getByText("acme")).toBeTruthy();
    expect(screen.getByText("Ready").className).toMatch(/tone-success/);
  });

  it("FileDrop accepts a file", async () => {
    const user = userEvent.setup();
    const onFile = vi.fn();
    render(<FileDrop label="Pack zip upload" onFile={onFile} />);
    const input = screen.getByTestId("file-drop-input");
    const file = new File([new Uint8Array([1])], "t.zip", { type: "application/zip" });
    await user.upload(input, file);
    expect(onFile).toHaveBeenCalled();
  });

  it("renders ToolSurface with a standard search toolbar", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <ToolSurface
        title="Objects"
        subtitle="Shape metadata."
        toolbar={
          <ToolToolbar search={<SearchField value="" onChange={onChange} testId="surf-search" />} />
        }
      >
        <p>Body</p>
      </ToolSurface>,
    );
    expect(screen.getByRole("heading", { name: "Objects" })).toBeTruthy();
    expect(screen.getByTestId("tool-toolbar")).toBeTruthy();
    await user.type(screen.getByTestId("surf-search"), "acme");
    expect(onChange).toHaveBeenCalled();
  });

  it("keeps SearchField as a row, not a label", () => {
    render(<SearchField value="" onChange={() => undefined} testId="search-row" />);
    const input = screen.getByTestId("search-row");
    expect(input.closest("label")).toBeNull();
    expect(input.parentElement?.className).toMatch(/tool-search/);
  });
});
