import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ToolAutomationStatusBanner } from "./ToolAutomationStatus";

afterEach(() => {
  cleanup();
});

describe("ToolAutomationStatusBanner", () => {
  it("renders running status without dismiss", () => {
    render(
      <ToolAutomationStatusBanner
        status={{ chipKey: "n1:Run", apiName: "sync_accounts", phase: "running", message: "Running sync_accounts…" }}
        onDismiss={vi.fn()}
      />,
    );
    const banner = screen.getByTestId("run-automation-status");
    expect(banner.getAttribute("data-phase")).toBe("running");
    expect(screen.getByText("sync_accounts")).toBeTruthy();
    expect(screen.queryByTestId("run-automation-dismiss")).toBeNull();
  });

  it("renders done status with dismiss and run id", async () => {
    const user = userEvent.setup();
    const onDismiss = vi.fn();
    render(
      <ToolAutomationStatusBanner
        status={{
          chipKey: "n1:Run",
          apiName: "sync_accounts",
          phase: "done",
          message: "Completed",
          run: { id: "run-42", status: "complete" },
        }}
        onDismiss={onDismiss}
      />,
    );
    expect(screen.getByText(/Run run-42/i)).toBeTruthy();
    await user.click(screen.getByTestId("run-automation-dismiss"));
    expect(onDismiss).toHaveBeenCalled();
  });

  it("renders error status", () => {
    render(
      <ToolAutomationStatusBanner
        status={{ chipKey: "n1:Run", apiName: "", phase: "error", message: "automationRun chip requires automationApiName" }}
      />,
    );
    expect(screen.getByTestId("run-automation-status").getAttribute("data-phase")).toBe("error");
    expect(screen.getByText(/Automation/)).toBeTruthy();
  });
});
