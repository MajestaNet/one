import { afterEach, describe, expect, it, vi } from "vitest";
import {
  automationRunSummary,
  createAutomationRun,
  invokeAutomationRun,
  isTerminalAutomationStatus,
  pollAutomationRun,
} from "./automations";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("run/automations", () => {
  it("detects terminal automation statuses", () => {
    expect(isTerminalAutomationStatus("completed")).toBe(true);
    expect(isTerminalAutomationStatus("failed")).toBe(true);
    expect(isTerminalAutomationStatus("running")).toBe(false);
  });

  it("summarizes run outcomes", () => {
    expect(automationRunSummary({ status: "completed", automationApiName: "Demo" })).toMatch(/completed/);
    expect(
      automationRunSummary({ status: "failed", automationApiName: "Demo", lastError: "denied" }),
    ).toMatch(/denied/);
  });

  it("invokes sync automations without polling", async () => {
    const fetch = vi.fn().mockResolvedValue({
      automationApiName: "Sync__c",
      status: "completed",
      execution: "sync",
    });
    const run = await invokeAutomationRun(fetch, "Sync__c", { x: 1 });
    expect(run.status).toBe("completed");
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("polls async automation runs until terminal", async () => {
    const fetch = vi
      .fn()
      .mockResolvedValueOnce({
        id: "job-1",
        automationApiName: "Async__c",
        status: "queued",
        execution: "async",
      })
      .mockResolvedValueOnce({ id: "job-1", status: "running" })
      .mockResolvedValueOnce({ id: "job-1", status: "completed" });
    const run = await invokeAutomationRun(fetch, "Async__c");
    expect(run.status).toBe("completed");
    expect(fetch).toHaveBeenCalledWith("/client/v1/automations/Async__c/runs", expect.any(Object));
    expect(fetch).toHaveBeenCalledWith("/client/v1/automations/runs/job-1");
  });

  it("createAutomationRun posts caller input", async () => {
    const fetch = vi.fn().mockResolvedValue({ id: "j1", status: "queued" });
    await createAutomationRun(fetch, "A__c", { hello: "world" });
    expect(fetch).toHaveBeenCalledWith(
      "/client/v1/automations/A__c/runs",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ input: { hello: "world" } }),
      }),
    );
  });

  it("pollAutomationRun returns last status when attempts exhaust", async () => {
    const fetch = vi.fn().mockResolvedValue({ id: "job-2", status: "running" });
    const run = await pollAutomationRun(fetch, "job-2", { intervalMs: 1, maxAttempts: 2 });
    expect(run.status).toBe("running");
    expect(fetch).toHaveBeenCalledTimes(2);
  });
});
