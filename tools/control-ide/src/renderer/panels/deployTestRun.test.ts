import { describe, expect, it, vi } from "vitest";
import {
  resolveCustomerTestRun,
  testRunJobId,
  testRunVerdict,
} from "./deployTestRun";

describe("testRunVerdict", () => {
  it("does not treat HTTP-shaped success without a suite status as passed", () => {
    expect(testRunVerdict({ runId: "r1" })).toBe("pending");
    expect(testRunVerdict({})).toBe("pending");
    expect(testRunVerdict(null)).toBe("pending");
  });

  it("marks failed from status, ok:false, or summary.failed", () => {
    expect(testRunVerdict({ status: "failed" })).toBe("failed");
    expect(testRunVerdict({ ok: false })).toBe("failed");
    expect(testRunVerdict({ run: { status: "failed" } })).toBe("failed");
    expect(testRunVerdict({ summary: { failed: 2, passed: 0 } })).toBe("failed");
  });

  it("marks passed only when the suite passed", () => {
    expect(testRunVerdict({ status: "passed" })).toBe("passed");
    expect(testRunVerdict({ run: { status: "passed" } })).toBe("passed");
    expect(testRunVerdict({ ok: true })).toBe("passed");
  });
});

describe("resolveCustomerTestRun", () => {
  it("polls /deploy/v1/work/{id} when jobId is returned", async () => {
    const fetchFn = vi
      .fn()
      .mockResolvedValueOnce({ jobId: "job-1", status: "running" })
      .mockResolvedValueOnce({
        jobId: "job-1",
        status: "completed",
        result: { run: { status: "passed" } },
      });
    const resolved = await resolveCustomerTestRun(
      fetchFn,
      { jobId: "job-1", status: "queued", accepted: true },
      { intervalMs: 1, maxAttempts: 5 },
    );
    expect(resolved.verdict).toBe("passed");
    expect(fetchFn).toHaveBeenCalledWith("/deploy/v1/work/job-1");
  });

  it("does not pass a failed work result", async () => {
    const fetchFn = vi.fn().mockResolvedValue({
      jobId: "job-2",
      status: "completed",
      result: { status: "failed" },
    });
    const resolved = await resolveCustomerTestRun(fetchFn, { jobId: "job-2" }, { intervalMs: 1 });
    expect(resolved.verdict).toBe("failed");
  });
});

describe("testRunJobId", () => {
  it("reads jobId or workId", () => {
    expect(testRunJobId({ jobId: "j1" })).toBe("j1");
    expect(testRunJobId({ workId: "w1" })).toBe("w1");
    expect(testRunJobId({})).toBe("");
  });
});
