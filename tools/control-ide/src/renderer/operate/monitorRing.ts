export const MONITOR_RING_MAX = 2000;
export const MONITOR_MESSAGE_MAX = 2048;

export type MonitorLogLevel = "debug" | "info" | "warn" | "error";

export type MonitorLogLine = {
  seq: number;
  level: MonitorLogLevel;
  message: string;
  at: string;
  runId?: string;
};

export type MonitorRingState = {
  lines: MonitorLogLine[];
  dropped: number;
  maxSeq: number;
};

export function createMonitorRing(): MonitorRingState {
  return { lines: [], dropped: 0, maxSeq: 0 };
}

export function trimMessage(message: string, max = MONITOR_MESSAGE_MAX): string {
  if (message.length <= max) return message;
  return `${message.slice(0, max)}…`;
}

export function normalizeLogLine(raw: unknown, fallbackSeq: number): MonitorLogLine | null {
  if (!raw || typeof raw !== "object") return null;
  const o = raw as Record<string, unknown>;
  const seq = Number(o.seq ?? o.Seq ?? fallbackSeq);
  if (!Number.isFinite(seq)) return null;
  const levelRaw = String(o.level ?? o.Level ?? "info").toLowerCase();
  const level: MonitorLogLevel =
    levelRaw === "debug" || levelRaw === "warn" || levelRaw === "error" ? levelRaw : "info";
  const message = trimMessage(String(o.message ?? o.Message ?? ""));
  const at = String(o.at ?? o.CreatedAt ?? o.createdAt ?? new Date().toISOString());
  const runId = o.runId ?? o.ExecutionRunId ?? o.executionRunId;
  return {
    seq,
    level,
    message,
    at,
    runId: runId != null ? String(runId) : undefined,
  };
}

/**
 * Append lines into a fixed ring buffer. Drops oldest on overflow.
 * Ignores duplicates / out-of-order seq ≤ maxSeq when seq is monotonic.
 */
export function appendMonitorLines(
  state: MonitorRingState,
  incoming: MonitorLogLine[],
  max = MONITOR_RING_MAX,
): MonitorRingState {
  if (!incoming.length) return state;
  const next = [...state.lines];
  let dropped = state.dropped;
  let maxSeq = state.maxSeq;
  for (const line of incoming) {
    if (line.seq <= maxSeq && next.some((l) => l.seq === line.seq)) continue;
    next.push(line);
    if (line.seq > maxSeq) maxSeq = line.seq;
  }
  next.sort((a, b) => a.seq - b.seq);
  if (next.length > max) {
    const overflow = next.length - max;
    dropped += overflow;
    next.splice(0, overflow);
  }
  return { lines: next, dropped, maxSeq };
}

/** Visible window for a virtualized list (no DOM for offscreen rows). */
export function windowLines(
  lines: MonitorLogLine[],
  scrollTop: number,
  viewportHeight: number,
  rowHeight: number,
): { start: number; end: number; offsetY: number } {
  const total = lines.length;
  if (total === 0 || rowHeight <= 0 || viewportHeight <= 0) {
    return { start: 0, end: 0, offsetY: 0 };
  }
  const start = Math.max(0, Math.floor(scrollTop / rowHeight) - 2);
  const visible = Math.ceil(viewportHeight / rowHeight) + 4;
  const end = Math.min(total, start + visible);
  return { start, end, offsetY: start * rowHeight };
}
