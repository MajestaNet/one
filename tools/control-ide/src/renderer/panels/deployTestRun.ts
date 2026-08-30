/** Honest interpretation of POST /deploy/v1/tests/runs (BP-066 WS0). HTTP 200 is not a pass. */

export type TestRunVerdict = "passed" | "failed" | "pending";

export type FetchFn = (path: string, init?: RequestInit) => Promise<unknown>;

type JsonRecord = Record<string, unknown>;

function asRecord(value: unknown): JsonRecord | null {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as JsonRecord) : null;
}

function nestedRun(body: JsonRecord): JsonRecord {
  return asRecord(body.run) ?? body;
}

function numberOrZero(value: unknown): number {
  const n = Number(value);
  return Number.isFinite(n) ? n : 0;
}

/**
 * Suite outcome from a test-run JSON body. Unknown 200 payloads stay pending —
 * never treat HTTP success alone as Passed.
 */
export function testRunVerdict(body: unknown): TestRunVerdict {
  const row = asRecord(body);
  if (!row) return "pending";
  const run = nestedRun(row);
  const summary = asRecord(run.summary) ?? asRecord(row.summary);
  const status = String(run.status ?? row.status ?? "").toLowerCase();
  const failedCount = numberOrZero(summary?.failed ?? run.failed ?? row.failed);
  const ok = run.ok ?? row.ok;

  if (ok === false || status === "failed" || failedCount > 0) return "failed";
  if (status === "passed" || ok === true) return "passed";
  if (
    status === "queued" ||
    status === "running" ||
    status === "accepted" ||
    row.accepted === true ||
    row.mode === "async"
  ) {
    return "pending";
  }
  return "pending";
}

export function testRunJobId(body: unknown): string {
  const row = asRecord(body);
  if (!row) return "";
  const id = row.jobId ?? row.workId;
  return typeof id === "string" ? id.trim() : "";
}

export function testRunPollPath(body: unknown): string {
  const row = asRecord(body);
  const poll = row && typeof row.poll === "string" ? row.poll.trim() : "";
  if (poll.startsWith("/")) return poll;
  const jobId = testRunJobId(body);
  return jobId ? `/deploy/v1/work/${encodeURIComponent(jobId)}` : "";
}

export function testRunId(body: unknown): string {
  const row = asRecord(body);
  if (!row) return "";
  const run = nestedRun(row);
  const id = run.id ?? row.runId;
  return typeof id === "string" ? id.trim() : "";
}

type WorkStatus = {
  status?: string;
  lastError?: string | null;
  result?: unknown;
};

async function sleep(ms: number): Promise<void> {
  await new Promise((r) => setTimeout(r, ms));
}

export async function pollDeployWork(
  fetchFn: FetchFn,
  path: string,
  opts: { intervalMs?: number; maxAttempts?: number } = {},
): Promise<unknown> {
  const intervalMs = opts.intervalMs ?? 400;
  const maxAttempts = opts.maxAttempts ?? 45;
  let last: WorkStatus | null = null;
  for (let i = 0; i < maxAttempts; i++) {
    const raw = (await fetchFn(path)) as WorkStatus;
    last = raw;
    const status = String(raw?.status ?? "").toLowerCase();
    if (status === "completed") {
      if (raw.result != null && raw.result !== "") return raw.result;
      return raw;
    }
    if (status === "failed") {
      const err = raw.lastError?.trim();
      throw new Error(err || "Customer test work failed");
    }
    await sleep(intervalMs);
  }
  throw new Error(
    `Customer test work timed out while ${last?.status ?? "queued"} — not a passed suite`,
  );
}

export async function pollTestRun(
  fetchFn: FetchFn,
  id: string,
  opts: { intervalMs?: number; maxAttempts?: number } = {},
): Promise<unknown> {
  const intervalMs = opts.intervalMs ?? 400;
  const maxAttempts = opts.maxAttempts ?? 45;
  const path = `/deploy/v1/tests/runs/${encodeURIComponent(id)}`;
  let last: unknown = null;
  for (let i = 0; i < maxAttempts; i++) {
    last = await fetchFn(path);
    const verdict = testRunVerdict(last);
    if (verdict !== "pending") return last;
    await sleep(intervalMs);
  }
  const lastVerdict = testRunVerdict(last);
  throw new Error(
    `Customer test run timed out before reporting passed or failed (last ${lastVerdict})`,
  );
}

export type ResolvedTestRun = {
  body: unknown;
  verdict: TestRunVerdict;
};

/**
 * Follow async work / run ids when present, then return the honest verdict.
 * A body with no passed/failed evidence stays pending (caller must not mark Passed).
 */
export async function resolveCustomerTestRun(
  fetchFn: FetchFn,
  body: unknown,
  opts: { intervalMs?: number; maxAttempts?: number } = {},
): Promise<ResolvedTestRun> {
  let current = body;
  const pollPath = testRunPollPath(current);
  if (pollPath) {
    current = await pollDeployWork(fetchFn, pollPath, opts);
  }
  let verdict = testRunVerdict(current);
  if (verdict === "pending" && !pollPath) {
    const id = testRunId(current) || testRunId(body);
    const row = asRecord(current) ?? asRecord(body);
    const status = String(
      (row && nestedRun(row).status) || row?.status || "",
    ).toLowerCase();
    const asyncRun =
      Boolean(id) &&
      (status === "queued" || status === "running" || row?.mode === "async");
    if (asyncRun && id) {
      current = await pollTestRun(fetchFn, id, opts);
      verdict = testRunVerdict(current);
    }
  }
  return { body: current, verdict };
}
