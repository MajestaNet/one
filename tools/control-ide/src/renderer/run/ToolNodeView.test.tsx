import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ToolNodeView } from "./ToolNodeView";
import { automationApiNameFromChip, parseActionChips, type ToolNode } from "./types";

const actionChipNode: ToolNode = {
  id: "actions",
  kind: "actionChipGroup",
  title: "Next steps",
  props: {
    actions: [
      { label: "Email champion", prompt: "Draft a follow-up email to the Acme champion" },
      {
        label: "Enrich accounts",
        type: "automationRun",
        automationApiName: "Demo_Enrich_Accounts",
        input: { source: "run-demo-tool" },
      },
    ],
  },
};

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("actionChipGroup automationRun", () => {
  it("parses automationRun chips with optional input", () => {
    const chips = parseActionChips([
      { label: "Run job", type: "automationRun", automationApiName: "Demo__c", input: { a: 1 } },
    ]);
    expect(chips[0].automationApiName).toBe("Demo__c");
    expect(chips[0].input).toEqual({ a: 1 });
    expect(automationApiNameFromChip(chips[0])).toBe("Demo__c");
  });

  it("invokes automation under caller JWT", async () => {
    const user = userEvent.setup();
    const onInvokeAutomation = vi.fn();
    const onEnqueuePrompt = vi.fn();
    render(
      <ToolNodeView
        node={actionChipNode}
        onInvokeAutomation={onInvokeAutomation}
        onEnqueuePrompt={onEnqueuePrompt}
      />,
    );
    await user.click(screen.getByTestId("canvas-action-chip-actions-Enrich accounts"));
    expect(onInvokeAutomation).toHaveBeenCalledWith(
      expect.objectContaining({
        label: "Enrich accounts",
        automationApiName: "Demo_Enrich_Accounts",
      }),
      actionChipNode,
    );
    expect(onEnqueuePrompt).not.toHaveBeenCalled();
  });

  it("prompt chips still enqueue agent prompts", async () => {
    const user = userEvent.setup();
    const onEnqueue = vi.fn();
    render(<ToolNodeView node={actionChipNode} onEnqueuePrompt={onEnqueue} />);
    await user.click(screen.getByTestId("canvas-action-chip-actions-Email champion"));
    expect(onEnqueue).toHaveBeenCalledWith(
      "Draft a follow-up email to the Acme champion",
      actionChipNode,
    );
  });
});
