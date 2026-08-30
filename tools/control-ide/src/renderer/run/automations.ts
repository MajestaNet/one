/** Client API helpers for automation invoke from Run Tools (ADR-021 Phase 6 / BP-047). */

import type { FetchFn } from "./tools";

export type AutomationRun = {
  id?: string;
  automationApiName?: string;
  status: string;
  execution?: string;
  input?: Record<string, unknown>;
  lastError?: string;
  createdAt?: string;
  completedAt?: string;
};

const TERMINAL = new Set(["completed", "failed", "cancelled", "dry_run_complete"]);

export function isTerminalAutomationStatus(status: string): boolean {
  return TERMINAL.has(status);
}

export async function createAutomationRun(
  fetchFn: FetchFn,
  apiName: string,
  input?: Record<string, unknown>,
): Promise<AutomationRun> {
  return (await fetchFn(`/client/v1/automations/${encodeURIComponent(apiName)}/runs`, {
    method: "POST",
    body: JSON.stringify({ input: input ?? {} }),
  })) as AutomationRun;
}

export async function getAutomationRun(fetchFn: FetchFn, id: string): Promise<AutomationRun> {
  return (await fetchFn(`/client/v1/automations/runs/${encodeURIComponent(id)}`)) as AutomationRun;
}

export async function pollAutomationRun(
  fetchFn: FetchFn,
  id: string,
  opts: { intervalMs?: number; maxAttempts?: number } = {},
): Promise<AutomationRun> {
  const intervalMs = opts.intervalMs ?? 600;
  const maxAttempts = opts.maxAttempts ?? 40;
  let last: AutomationRun = { id, status: "queued" };
  for (let i = 0; i < maxAttempts; i++) {
    last = await getAutomationRun(fetchFn, id);
    if (isTerminalAutomationStatus(last.status)) return last;
    await new Promise((r) => setTimeout(r, intervalMs));
  }
  return last;
}

/** Invoke automation; polls async jobs until terminal. Sync runs return immediately. */
export async function invokeAutomationRun(
  fetchFn: FetchFn,
  apiName: string,
  input?: Record<string, unknown>,
): Promise<AutomationRun> {
  const created = await createAutomationRun(fetchFn, apiName, input);
  if (isTerminalAutomationStatus(created.status) || created.execution === "sync") {
    return created;
  }
  if (!created.id) return created;
  return pollAutomationRun(fetchFn, created.id);
}

export function automationRunSummary(run: AutomationRun): string {
  const name = run.automationApiName ?? "automation";
  if (run.lastError) return `${name} failed: ${run.lastError}`;
  if (run.status === "completed") return `${name} completed`;
  return `${name} · ${run.status.replace(/_/g, " ")}`;
}
