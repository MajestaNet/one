import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { StreamMessageBubble } from "./StreamMessageBubble";
import type { StreamMessage } from "./types";

afterEach(() => cleanup());

describe("StreamMessageBubble", () => {
  it("renders record strip for handoffs with Ids and omits Show matching records CTA", () => {
    const message: StreamMessage = {
      id: "m1",
      role: "agent",
      body: "Ranked accounts ready",
      agentLabel: "QueryAssistant",
      createdAt: "2026-07-23T12:00:00.000Z",
      runStatus: "completed",
      boardHandoff: {
        source: "run",
        objectApiName: "Account",
        recordIds: ["a1"],
      },
    };
    render(<StreamMessageBubble message={message} expandHandoff onOpenBoard={vi.fn()} />);
    expect(screen.queryByText("QueryAssistant")).toBeNull();
    expect(screen.queryByText(/^You$/i)).toBeNull();
    expect(screen.getByTestId("stream-run-status").textContent).toMatch(/completed/);
    expect(screen.getByTestId("chat-handoff-block")).toBeTruthy();
    expect(screen.getByTestId("chat-record-strip").textContent).toMatch(/a1/);
    expect(screen.queryByTestId("open-board-from-run")).toBeNull();
    expect(screen.queryByTestId("crm-what-to-do")).toBeNull();
    expect(screen.queryByText(/Show matching records/i)).toBeNull();
  });

  it("omits You chip on human messages", () => {
    const message: StreamMessage = {
      id: "h1",
      role: "human",
      body: "Find Acme",
      createdAt: "2026-07-23T12:00:00.000Z",
    };
    render(<StreamMessageBubble message={message} />);
    expect(screen.getByText("Find Acme")).toBeTruthy();
    expect(screen.queryByText(/^You$/i)).toBeNull();
    expect(screen.queryByText(/QueryAssistant/i)).toBeNull();
  });

  it("does not render suggestion-only handoff chrome", () => {
    const message: StreamMessage = {
      id: "m2",
      role: "agent",
      body: "Suggestion only",
      boardHandoff: {
        source: "run",
        objectApiName: "Account",
        rationale: "sdsdds",
        suggestions: [{ id: "show-matches", label: "Show matching records", action: "open_object" }],
      },
    };
    render(<StreamMessageBubble message={message} expandHandoff onOpenBoard={vi.fn()} />);
    expect(screen.queryByTestId("chat-handoff-block")).toBeNull();
    expect(screen.queryByTestId("crm-what-to-do")).toBeNull();
    expect(screen.queryByText(/Show matching records/i)).toBeNull();
  });

  it("renders Run Tool reference card with open action", async () => {
    const user = userEvent.setup();
    const onOpenSessionTool = vi.fn();
    const message: StreamMessage = {
      id: "tr1",
      role: "agent",
      body: "Tool ready",
      toolHandoff: {
        source: "tool_result",
        toolId: "run-abc",
        toolTitle: "Pipeline tool",
        rationale: "Composed from your prompt",
      },
    };
    render(<StreamMessageBubble message={message} onOpenSessionTool={onOpenSessionTool} />);
    expect(screen.getByTestId("chat-tool-ref")).toBeTruthy();
    await user.click(screen.getByTestId("chat-open-tool"));
    expect(onOpenSessionTool).toHaveBeenCalledWith("run-abc");
  });

  it("shows collapsible tool steps and inline approve", async () => {
    const user = userEvent.setup();
    const onApprove = vi.fn();
    const message: StreamMessage = {
      id: "t1",
      role: "approval",
      body: "Needs approval",
      runId: "run-9",
      runStatus: "awaiting_approval",
      steps: [
        { id: "s1", label: "query", state: "pending" },
        { id: "s2", label: "update", state: "pending" },
      ],
    };
    render(<StreamMessageBubble message={message} onApprove={onApprove} />);
    expect(screen.getByTestId("stream-steps")).toBeTruthy();
    expect(screen.queryByRole("button", { name: /Show 2 steps/i })).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /Show 2 steps/i }));
    expect(screen.getByTestId("stream-steps-list").textContent).toMatch(/query/);
    await user.click(screen.getByRole("button", { name: /Hide 2 steps/i }));
    expect(screen.queryByTestId("stream-steps-list")).toBeNull();
    expect(screen.getByTestId("tool-approval-card").textContent).toMatch(/query/);
    await user.click(screen.getByTestId("approve-run-inline"));
    expect(onApprove).toHaveBeenCalledWith("run-9");
  });

  it("renders safe rich markdown for agent responses", () => {
    render(
      <StreamMessageBubble
        message={{
          id: "md1",
          role: "agent",
          body: "## Summary\n\n- **Acme** needs review\n- Run `query` next",
        }}
      />,
    );
    expect(screen.getByRole("heading", { name: "Summary" })).toBeTruthy();
    expect(screen.getByText("Acme").tagName).toBe("STRONG");
    expect(screen.getByText("query").tagName).toBe("CODE");
  });

  it("shows thinking inside an empty running assistant bubble", () => {
    render(
      <StreamMessageBubble
        message={{
          id: "s1",
          role: "agent",
          body: "",
          runStatus: "running",
        }}
      />,
    );
    expect(screen.getByTestId("chat-typing").textContent).toMatch(/Thinking/);
    expect(screen.queryByTestId("stream-run-status")).toBeNull();
  });

  it("keeps streamed text in the same bubble without a thinking row", () => {
    render(
      <StreamMessageBubble
        message={{
          id: "s2",
          role: "agent",
          body: "Hello from the agent",
          runStatus: "running",
        }}
      />,
    );
    expect(screen.getByText("Hello from the agent")).toBeTruthy();
    expect(screen.queryByTestId("chat-typing")).toBeNull();
    expect(screen.getByTestId("stream-bubble-s2").querySelector(".stream-body-plain.is-streaming")).toBeTruthy();
    expect(screen.getByTestId("stream-bubble-s2").querySelector(".stream-markdown")).toBeNull();
  });
});
