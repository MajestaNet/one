import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ChatContextExcerptBlock } from "./ChatContextExcerptBlock";
import { rowsToContextExcerpt } from "../../workspace/contextExcerpt";

describe("ChatContextExcerptBlock", () => {
  const excerpts = [
    rowsToContextExcerpt({
      rows: [{ Name: "Acme", id: "1" }],
      label: "1 Account",
      objectApiName: "Account",
    }),
  ];

  it("renders preview and expands on click", async () => {
    const user = userEvent.setup();
    render(<ChatContextExcerptBlock excerpts={excerpts} />);
    expect(screen.getByTestId("chat-context-excerpt-block")).toBeTruthy();
    expect(screen.getByText("1 Account")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /Expand/i }));
    expect(screen.getByTestId(`chat-context-excerpt-${excerpts[0].id}`)).toBeTruthy();
  });

  it("calls onDismiss when provided", async () => {
    const user = userEvent.setup();
    const onDismiss = vi.fn();
    render(<ChatContextExcerptBlock excerpts={excerpts} onDismiss={onDismiss} />);
    await user.click(screen.getByTestId("chat-context-excerpt-dismiss"));
    expect(onDismiss).toHaveBeenCalled();
  });
});
