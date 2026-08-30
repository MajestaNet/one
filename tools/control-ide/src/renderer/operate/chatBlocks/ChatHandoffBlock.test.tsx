import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ChatHandoffBlock } from "./ChatHandoffBlock";

afterEach(() => cleanup());

describe("ChatHandoffBlock", () => {
  it("renders record strip and stages mutations without What to do", async () => {
    const user = userEvent.setup();
    const onStage = vi.fn();
    render(
      <ChatHandoffBlock
        handoff={{
          source: "run",
          objectApiName: "Account",
          rationale: "Top accounts",
          recordIds: ["a1", "a2"],
          proposedMutations: [{ op: "update", object: "Account", id: "a1", data: { Name: "Acme" } }],
          suggestions: [{ id: "s1", label: "Show matching records", action: "open_object" }],
        }}
        onStageMutations={onStage}
      />,
    );
    expect(screen.getByTestId("chat-record-strip").textContent).toMatch(/a1/);
    expect(screen.queryByTestId("crm-what-to-do")).toBeNull();
    // Suggestion chips removed — no "Open …" / "Show matching records" bar.
    expect(screen.queryByRole("button", { name: /Show matching records/i })).toBeNull();
    expect(screen.queryByTestId("chat-suggestion-chips")).toBeNull();
    await user.click(screen.getByTestId("chat-stage-mutations"));
    expect(onStage).toHaveBeenCalledWith(1);
    expect(screen.getByText(/Staged for review/i)).toBeTruthy();
  });

  it("shows mutation field keys from data when present", () => {
    render(
      <ChatHandoffBlock
        handoff={{
          source: "run",
          objectApiName: "Account",
          recordIds: ["a1"],
          proposedMutations: [{ op: "update", object: "Account", id: "a1", data: { Name: "Acme", Type: "Customer" } }],
        }}
      />,
    );
    expect(screen.getByTestId("chat-mutation-review").textContent).toMatch(/Name/);
    expect(screen.getByTestId("chat-mutation-review").textContent).toMatch(/Type/);
  });

  it("renders nothing for suggestion-only handoffs", () => {
    const { container } = render(
      <ChatHandoffBlock
        handoff={{
          source: "run",
          objectApiName: "Account",
          rationale: "sdsdds",
          suggestions: [{ id: "show-matches", label: "Show matching records", action: "open_object" }],
        }}
      />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("opens Query for the handoff object when requested", async () => {
    const user = userEvent.setup();
    const onOpenInQuery = vi.fn();
    render(
      <ChatHandoffBlock
        handoff={{
          source: "run",
          objectApiName: "Account",
          recordIds: ["a1"],
        }}
        onOpenInQuery={onOpenInQuery}
      />,
    );
    await user.click(screen.getByTestId("chat-open-in-query"));
    expect(onOpenInQuery).toHaveBeenCalledWith("Account");
  });

  it("pins the working set to Run and stages proposal payloads through callbacks", async () => {
    const user = userEvent.setup();
    const handoff = {
      source: "tool_result" as const,
      objectApiName: "Account",
      recordIds: ["a1"],
      proposedMutations: [
        { op: "update" as const, object: "Account", id: "a1", data: { Name: "Acme" } },
      ],
    };
    const onPinToGraph = vi.fn().mockResolvedValue(undefined);
    const onStageProposal = vi.fn().mockResolvedValue(undefined);
    render(
      <ChatHandoffBlock
        handoff={handoff}
        onPinToGraph={onPinToGraph}
        onStageProposal={onStageProposal}
      />,
    );

    await user.click(screen.getByTestId("chat-pin-to-graph"));
    expect(onPinToGraph).toHaveBeenCalledWith(handoff);
    expect(screen.getByText("Pinned to my graph")).toBeTruthy();
    await user.click(screen.getByTestId("chat-stage-mutations"));
    expect(onStageProposal).toHaveBeenCalledWith(handoff);
    expect(screen.getByText(/Staged for review/i)).toBeTruthy();
  });
});
