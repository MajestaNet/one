/** Thin Client API helpers for /client/v1/agents/runs (ADR-006, BP-016). */

export type AgentRunStatus =
  | "queued"
  | "awaiting_approval"
  | "running"
  | "completed"
  | "dry_run_complete"
  | "failed"
  | string;

export type AgentRun = {
  id: string;
  status: AgentRunStatus;
  goal?: string;
  playbookApiName?: string;
  input?: Record<string, unknown>;
  output?: Record<string, unknown> | string | null;
  error?: string;
  dryRun?: boolean;
  createdAt?: string;
  completedAt?: string;
};

export type CreateAgentRunBody = {
  goal?: string;
  playbookApiName?: string;
  input?: Record<string, unknown>;
  dryRun?: boolean;
  approved?: boolean;
  /** When true, server streams SSE tokens (BP-052). */
  stream?: boolean;
  /** Link run to a persisted IDE conversation thread. */
  conversationId?: string;
};

export type FetchFn = (path: string, init?: RequestInit) => Promise<unknown>;

export type StreamHandlers = {
  onRun?: (payload: { id?: string; status?: string }) => void;
  onToken?: (payload: { delta?: string }) => void;
  onDone?: (payload: { id?: string; status?: string; output?: unknown }) => void;
  onError?: (payload: { code?: string; error?: string }) => void;
};

/** Shown when a stream create/approve still returns JSON park/queue instead of SSE tokens. */
export const STREAM_PARKED_HINT =
  "This install parked the run for approval instead of streaming a reply. Open Settings → Inference and use Test chat to validate the model. If Test chat streams, restart the Majesta One API (`make api`) so Operate chat skips pre-LLM approval.";

function agentStreamHeaders(token: string): HeadersInit {
  return {
    Authorization: `Bearer ${token}`,
    "Content-Type": "application/json",
    Accept: "text/event-stream",
  };
}

async function consumeAgentRunResponse(
  res: Response,
  handlers: StreamHandlers,
  onParked?: (run: AgentRun) => Promise<AgentRun>,
): Promise<AgentRun> {
  const contentType = res.headers.get("content-type") ?? "";
  if (contentType.includes("text/event-stream")) {
    return readAgentRunSSE(res, handlers);
  }
  const run = (await res.json()) as AgentRun;
  if (run.status === "awaiting_approval" && run.id && onParked) {
    return onParked(run);
  }
  if (isTerminalRunStatus(run.status)) {
    handlers.onRun?.({ id: run.id, status: run.status });
    handlers.onDone?.({ id: run.id, status: run.status, output: run.output });
    return run;
  }
  throw new Error(STREAM_PARKED_HINT);
}

/** Parse SSE from POST /agents/runs or POST /agents/runs/{id}/approve. */
export async function readAgentRunSSE(res: Response, handlers: StreamHandlers): Promise<AgentRun> {
  if (!res.body) {
    throw new Error("stream failed: empty body");
  }
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buf = "";
  let event = "message";
  let run: AgentRun = { id: "", status: "running" };
  let streamError = "";
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });
    const parts = buf.split("\n");
    buf = parts.pop() ?? "";
    for (const line of parts) {
      if (line.startsWith("event:")) {
        event = line.slice(6).trim();
        continue;
      }
      if (!line.startsWith("data:")) continue;
      const raw = line.slice(5).trim();
      let payload: Record<string, unknown>;
      try {
        payload = JSON.parse(raw) as Record<string, unknown>;
      } catch {
        continue;
      }
      if (event === "run") {
        const next = payload as { id?: string; status?: string };
        if (next.id) run.id = next.id;
        if (next.status) run.status = next.status;
        handlers.onRun?.(next);
      } else if (event === "token") handlers.onToken?.(payload as { delta?: string });
      else if (event === "done") {
        const next = payload as { id?: string; status?: string; output?: unknown };
        if (next.id) run.id = next.id;
        if (next.status) run.status = next.status;
        if ("output" in next) run.output = next.output as AgentRun["output"];
        handlers.onDone?.(next);
      } else if (event === "error") {
        const next = payload as { code?: string; error?: string };
        streamError = next.error || next.code || "Agent stream failed";
        run = { ...run, status: "failed", error: streamError };
        handlers.onError?.(next);
      }
    }
  }
  if (streamError) throw new Error(streamError);
  return run;
}

/** POST /agents/runs with stream:true using raw fetch (needs baseUrl + token). */
export async function createAgentRunStream(
  baseUrl: string,
  token: string,
  body: CreateAgentRunBody,
  handlers: StreamHandlers,
  signal?: AbortSignal,
): Promise<AgentRun> {
  return postAgentRunStream(baseUrl, token, body, handlers, signal, false);
}

async function postAgentRunStream(
  baseUrl: string,
  token: string,
  body: CreateAgentRunBody,
  handlers: StreamHandlers,
  signal: AbortSignal | undefined,
  generationRetry: boolean,
): Promise<AgentRun> {
  const url = `${baseUrl.replace(/\/$/, "")}/client/v1/agents/runs`;
  const res = await fetch(url, {
    method: "POST",
    headers: agentStreamHeaders(token),
    body: JSON.stringify({ ...body, stream: true }),
    signal,
  });
  if ((!res.ok && res.status !== 202) || !res.body) {
    const text = await res.text().catch(() => "");
    throw new Error(text || `stream failed HTTP ${res.status}`);
  }
  return consumeAgentRunResponse(res, handlers, async (parked) => {
    try {
      return await approveAgentRunStream(baseUrl, token, parked.id, handlers, signal);
    } catch (err) {
      // Older APIs park stream+approved:false before the LLM. Retry once as generation-only.
      // Hosted MCP tool loop is shipped (BP-006). Distinct write-park is awaiting_tool_approval (WS1).
      if (generationRetry || body.approved) throw err;
      return postAgentRunStream(
        baseUrl,
        token,
        { ...body, approved: true },
        handlers,
        signal,
        true,
      );
    }
  });
}

/** Settings → Inference probe: stream a goal-only run (approved so older APIs still generate). */
export async function createInferenceTestChat(
  baseUrl: string,
  token: string,
  prompt: string,
  handlers: StreamHandlers,
  signal?: AbortSignal,
): Promise<AgentRun> {
  const goal = prompt.trim() || "Say hello in one short sentence.";
  return createAgentRunStream(
    baseUrl,
    token,
    {
      goal,
      approved: true,
      dryRun: false,
      input: { oneInferenceTest: true },
    },
    handlers,
    signal,
  );
}

export async function createAgentRun(fetchFn: FetchFn, body: CreateAgentRunBody): Promise<AgentRun> {
  return (await fetchFn("/client/v1/agents/runs", {
    method: "POST",
    body: JSON.stringify(body),
  })) as AgentRun;
}

export async function getAgentRun(fetchFn: FetchFn, id: string): Promise<AgentRun> {
  return (await fetchFn(`/client/v1/agents/runs/${encodeURIComponent(id)}`)) as AgentRun;
}

/** JSON approve (worker queue). Prefer approveAgentRunStream for chat. */
export async function approveAgentRun(fetchFn: FetchFn, id: string): Promise<AgentRun> {
  return (await fetchFn(`/client/v1/agents/runs/${encodeURIComponent(id)}/approve`, {
    method: "POST",
    body: "{}",
  })) as AgentRun;
}

/** POST /agents/runs/{id}/approve with SSE so generation continues in-process. */
export async function approveAgentRunStream(
  baseUrl: string,
  token: string,
  id: string,
  handlers: StreamHandlers,
  signal?: AbortSignal,
): Promise<AgentRun> {
  const url = `${baseUrl.replace(/\/$/, "")}/client/v1/agents/runs/${encodeURIComponent(id)}/approve`;
  const res = await fetch(url, {
    method: "POST",
    headers: agentStreamHeaders(token),
    body: JSON.stringify({ stream: true }),
    signal,
  });
  if ((!res.ok && res.status !== 202) || !res.body) {
    const text = await res.text().catch(() => "");
    throw new Error(text || `approve stream failed HTTP ${res.status}`);
  }
  return consumeAgentRunResponse(res, handlers);
}

const TERMINAL = new Set(["completed", "dry_run_complete", "failed"]);

export function isTerminalRunStatus(status: string): boolean {
  return TERMINAL.has(status);
}

/** Poll until terminal or awaiting_approval (caller may approve). Throws if still queued/running. */
export async function pollAgentRun(
  fetchFn: FetchFn,
  id: string,
  opts: { intervalMs?: number; maxAttempts?: number; signal?: AbortSignal } = {},
): Promise<AgentRun> {
  const intervalMs = opts.intervalMs ?? 800;
  const maxAttempts = opts.maxAttempts ?? 40;
  let last: AgentRun | null = null;
  for (let i = 0; i < maxAttempts; i++) {
    if (opts.signal?.aborted) throw new Error("poll aborted");
    last = await getAgentRun(fetchFn, id);
    if (isTerminalRunStatus(last.status) || last.status === "awaiting_approval") return last;
    await new Promise((r) => setTimeout(r, intervalMs));
  }
  throw new Error(
    `Agent run ${id} timed out while ${last?.status ?? "queued"} — not a completed reply`,
  );
}

export function summarizeRunOutput(run: AgentRun): string {
  if (run.error) return `Failed: ${run.error}`;
  if (run.status === "awaiting_approval") {
    return `Run ${run.id} awaits approval before tools execute.`;
  }
  if (run.status === "queued" || run.status === "running") {
    return `Agent run still ${run.status}${run.goal ? ` · ${run.goal}` : ""}`;
  }
  const out = run.output;
  if (out && typeof out === "object") {
    const summary = (out as { summary?: string }).summary;
    if (summary) return summary;
    const tools = (out as { toolsPlanned?: string[] }).toolsPlanned;
    if (tools?.length) return `Completed · planned tools: ${tools.join(", ")}`;
    return `Run ${run.status}: ${JSON.stringify(out).slice(0, 280)}`;
  }
  if (typeof out === "string" && out.trim()) return out;
  return `Agent run ${run.status}${run.goal ? ` · ${run.goal}` : ""}`;
}

export type PlaybookRow = {
  apiName?: string;
  label?: string;
  goalTemplate?: string;
  instructions?: string;
  requireApproval?: boolean;
  active?: boolean;
  primarySection?: string;
  harnessId?: string;
};

export async function listPlaybooks(fetchFn: FetchFn): Promise<PlaybookRow[]> {
  const row = (await fetchFn("/client/v1/agents/playbooks")) as { playbooks?: PlaybookRow[] };
  return row.playbooks ?? [];
}
