import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { AppBridge } from "../App";
import { Button, EmptyState, PanelHeader, Spinner, StatusBadge, ToolSurface } from "../ui";
import { IconMonitor } from "../icons/Icons";
import {
  MONITOR_RING_MAX,
  appendMonitorLines,
  createMonitorRing,
  normalizeLogLine,
  windowLines,
  type MonitorLogLevel,
  type MonitorLogLine,
  type MonitorRingState,
} from "./monitorRing";

const ROW_HEIGHT = 28;
const POLL_MS = 1500;

type TraceFlagRow = {
  id: string;
  tracedUserId: string;
  level: MonitorLogLevel;
  expiresAt: string;
  active: boolean;
};

type DebugAvailability = "unknown" | "ready" | "missing";

function levelRank(level: MonitorLogLevel): number {
  switch (level) {
    case "debug":
      return 0;
    case "info":
      return 1;
    case "warn":
      return 2;
    case "error":
      return 3;
  }
}

export function MonitorPanel({
  bridge,
  refreshKey = 0,
}: {
  bridge: AppBridge;
  refreshKey?: number;
}) {
  const connected = Boolean(bridge.session?.token && bridge.session?.baseUrl);
  const [availability, setAvailability] = useState<DebugAvailability>("unknown");
  const [flags, setFlags] = useState<TraceFlagRow[]>([]);
  const [users, setUsers] = useState<Array<{ id: string; label: string }>>([]);
  const [userId, setUserId] = useState("");
  const [level, setLevel] = useState<MonitorLogLevel>("info");
  const [durationMin, setDurationMin] = useState(30);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [paused, setPaused] = useState(false);
  const [minLevel, setMinLevel] = useState<MonitorLogLevel>("debug");
  const [ring, setRing] = useState<MonitorRingState>(() => createMonitorRing());
  const [scrollTop, setScrollTop] = useState(0);
  const [viewportH, setViewportH] = useState(320);
  const ringRef = useRef(ring);
  const sinceSeqRef = useRef(0);
  const pollTimer = useRef<ReturnType<typeof setInterval> | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const pendingBatch = useRef<MonitorLogLine[]>([]);
  const rafRef = useRef<number | null>(null);

  useEffect(() => {
    ringRef.current = ring;
    sinceSeqRef.current = ring.maxSeq;
  }, [ring]);

  const flushBatch = useCallback(() => {
    rafRef.current = null;
    if (!pendingBatch.current.length) return;
    const batch = pendingBatch.current;
    pendingBatch.current = [];
    setRing((prev) => appendMonitorLines(prev, batch, MONITOR_RING_MAX));
  }, []);

  const enqueueLines = useCallback(
    (lines: MonitorLogLine[]) => {
      if (!lines.length) return;
      pendingBatch.current.push(...lines);
      if (rafRef.current == null) {
        rafRef.current = requestAnimationFrame(flushBatch);
      }
    },
    [flushBatch],
  );

  const probeDebug = useCallback(async () => {
    if (!connected) {
      setAvailability("unknown");
      return;
    }
    setBusy(true);
    setErr("");
    try {
      // Debug HTTP (/client/v1/debug/*) is not registered. Probe BP-033 objects
      // via Metadata; stay on the missing empty-state until they exist.
      try {
        await bridge.fetch("/metadata/v1/objects/ExecutionLogEntry");
      } catch {
        setAvailability("missing");
        setFlags([]);
        return;
      }
      setAvailability("ready");
      try {
        const q = (await bridge.fetch("/client/v1/query", {
          method: "POST",
          body: JSON.stringify({
            object: "TraceFlag",
            limit: 20,
            filters: [{ field: "Active", op: "eq", value: true }],
          }),
        })) as { records?: Record<string, unknown>[] };
        setFlags(
          (q.records ?? []).map((r, i) => ({
            id: String(r.id ?? r.Id ?? i),
            tracedUserId: String(r.TracedUserId ?? r.tracedUserId ?? ""),
            level: (String(r.Level ?? r.level ?? "info").toLowerCase() as MonitorLogLevel) || "info",
            expiresAt: String(r.ExpiresAt ?? r.expiresAt ?? ""),
            active: Boolean(r.Active ?? r.active ?? true),
          })),
        );
      } catch {
        setFlags([]);
      }

      try {
        const uq = (await bridge.fetch("/client/v1/query", {
          method: "POST",
          body: JSON.stringify({ object: "User", limit: 50 }),
        })) as { records?: Record<string, unknown>[] };
        const list = (uq.records ?? []).map((r) => ({
          id: String(r.id ?? r.Id ?? ""),
          label: String(r.Name ?? r.name ?? r.Username ?? r.Email ?? r.id ?? r.Id ?? "User"),
        }));
        setUsers(list.filter((u) => u.id));
        if (list[0] && !userId) setUserId(list[0].id);
      } catch {
        setUsers([]);
      }
    } catch (e) {
      setErr(String(e));
      setAvailability("missing");
    } finally {
      setBusy(false);
    }
  }, [bridge, connected, userId]);

  useEffect(() => {
    setRing(createMonitorRing());
    sinceSeqRef.current = 0;
    void probeDebug();
  }, [probeDebug, refreshKey]);

  const pollLogs = useCallback(async () => {
    if (!connected || availability !== "ready" || paused) return;
    if (typeof document !== "undefined" && document.visibilityState === "hidden") return;
    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;
    try {
      const traced = flags[0]?.tracedUserId || userId;
      let raw: unknown;
      try {
        raw = await bridge.fetch("/client/v1/query", {
          method: "POST",
          body: JSON.stringify({
            object: "ExecutionLogEntry",
            limit: 200,
            filters: traced ? [{ field: "TracedUserId", op: "eq", value: traced }] : [],
          }),
          signal: ac.signal,
        });
      } catch {
        return;
      }
      if (ac.signal.aborted) return;
      const body = raw as { lines?: unknown[]; logs?: unknown[]; records?: unknown[] };
      const list = Array.isArray(body.lines)
        ? body.lines
        : Array.isArray(body.logs)
          ? body.logs
          : Array.isArray(body.records)
            ? body.records
            : [];
      const normalized = list
        .map((item, i) => normalizeLogLine(item, sinceSeqRef.current + i + 1))
        .filter((x): x is MonitorLogLine => x != null)
        .filter((l) => l.seq > sinceSeqRef.current);
      // Backpressure: if full page, keep only the newest half for the ring enqueue.
      const capped =
        normalized.length >= 200 ? normalized.slice(normalized.length - 100) : normalized;
      enqueueLines(capped);
    } catch {
      /* ignore poll errors while armed */
    }
  }, [availability, bridge, connected, enqueueLines, flags, paused, userId]);

  useEffect(() => {
    if (pollTimer.current) {
      clearInterval(pollTimer.current);
      pollTimer.current = null;
    }
    if (availability === "ready" && flags.some((f) => f.active) && !paused) {
      void pollLogs();
      pollTimer.current = setInterval(() => void pollLogs(), POLL_MS);
    }
    return () => {
      if (pollTimer.current) clearInterval(pollTimer.current);
      pollTimer.current = null;
    };
  }, [availability, flags, paused, pollLogs]);

  useEffect(() => {
    const onVis = () => {
      if (document.visibilityState === "visible") void pollLogs();
    };
    document.addEventListener("visibilitychange", onVis);
    return () => document.removeEventListener("visibilitychange", onVis);
  }, [pollLogs]);

  useEffect(() => {
    const el = listRef.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setViewportH(el.clientHeight || 320));
    ro.observe(el);
    setViewportH(el.clientHeight || 320);
    return () => ro.disconnect();
  }, [availability]);

  useEffect(
    () => () => {
      abortRef.current?.abort();
      if (rafRef.current != null) cancelAnimationFrame(rafRef.current);
      if (pollTimer.current) clearInterval(pollTimer.current);
    },
    [],
  );

  const startTrace = async () => {
    if (!userId) {
      setErr("Pick a user to trace.");
      return;
    }
    setBusy(true);
    setErr("");
    try {
      const expiresAt = new Date(Date.now() + durationMin * 60_000).toISOString();
      await bridge.fetch("/client/v1/sobjects/TraceFlag", {
        method: "POST",
        body: JSON.stringify({
          data: { TracedUserId: userId, Level: level, ExpiresAt: expiresAt, Active: true },
        }),
      });
      await probeDebug();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const stopTrace = async (id: string) => {
    setBusy(true);
    setErr("");
    try {
      await bridge.fetch(`/client/v1/sobjects/TraceFlag/${encodeURIComponent(id)}`, {
        method: "DELETE",
      });
      await probeDebug();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const filtered = useMemo(
    () => ring.lines.filter((l) => levelRank(l.level) >= levelRank(minLevel)),
    [ring.lines, minLevel],
  );
  const win = windowLines(filtered, scrollTop, viewportH, ROW_HEIGHT);
  const visible = filtered.slice(win.start, win.end);

  return (
    <ToolSurface className="operate-monitor-panel" testId="operate-monitor-panel">
      <PanelHeader
        title="Monitor"
        subtitle="Arm a user TraceFlag when ExecutionLogEntry exists on this install. Debug HTTP (/client/v1/debug/*) is not registered."
        actions={
          <Button variant="secondary" busy={busy} disabled={!connected} onClick={() => void probeDebug()}>
            Refresh
          </Button>
        }
      />

      {!connected ? (
        <EmptyState
          icon={<IconMonitor size={28} />}
          title="Connect to monitor"
          description="Connect an environment to arm TraceFlags against the active install."
        />
      ) : availability === "missing" ? (
        <EmptyState
          icon={<IconMonitor size={28} />}
          title="Trace requires install debug objects"
          description="BP-033 ExecutionRun / ExecutionLogEntry and TraceFlag APIs are not available on this install yet."
        />
      ) : (
        <>
          <div className="row operate-monitor-arm">
            <label>
              User
              <select
                data-testid="monitor-user-select"
                value={userId}
                onChange={(e) => setUserId(e.target.value)}
              >
                {users.length === 0 ? <option value="">No users loaded</option> : null}
                {users.map((u) => (
                  <option key={u.id} value={u.id}>
                    {u.label}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Level
              <select
                data-testid="monitor-level-select"
                value={level}
                onChange={(e) => setLevel(e.target.value as MonitorLogLevel)}
              >
                <option value="debug">debug</option>
                <option value="info">info</option>
                <option value="warn">warn</option>
                <option value="error">error</option>
              </select>
            </label>
            <label>
              Duration (min)
              <input
                data-testid="monitor-duration"
                type="number"
                min={5}
                max={120}
                value={durationMin}
                onChange={(e) => setDurationMin(Math.min(120, Math.max(5, Number(e.target.value) || 30)))}
              />
            </label>
            <Button variant="primary" busy={busy} onClick={() => void startTrace()}>
              Start trace
            </Button>
            {busy ? <Spinner /> : null}
          </div>

          <section className="operate-monitor-flags" aria-label="Active trace flags">
            <h3>Active flags</h3>
            {flags.length === 0 ? (
              <p className="muted">No active TraceFlags.</p>
            ) : (
              <ul className="operate-monitor-flag-list">
                {flags.map((f) => (
                  <li key={f.id} className="operate-monitor-flag">
                    <span className="mono">{f.tracedUserId}</span>
                    <StatusBadge tone="info">{f.level}</StatusBadge>
                    <span className="muted">expires {f.expiresAt || "—"}</span>
                    <Button variant="ghost" onClick={() => void stopTrace(f.id)}>
                      Stop
                    </Button>
                  </li>
                ))}
              </ul>
            )}
          </section>

          <div className="row operate-monitor-tail-toolbar">
            <Button
              variant="secondary"
              data-testid="monitor-pause"
              onClick={() => setPaused((p) => !p)}
            >
              {paused ? "Resume" : "Pause"}
            </Button>
            <label>
              Min level
              <select
                value={minLevel}
                onChange={(e) => setMinLevel(e.target.value as MonitorLogLevel)}
              >
                <option value="debug">debug</option>
                <option value="info">info</option>
                <option value="warn">warn</option>
                <option value="error">error</option>
              </select>
            </label>
            <Button
              variant="ghost"
              onClick={() => {
                setRing(createMonitorRing());
                sinceSeqRef.current = 0;
              }}
            >
              Clear buffer
            </Button>
            {ring.dropped > 0 ? (
              <span className="operate-monitor-dropped" data-testid="monitor-dropped">
                {ring.dropped} dropped
              </span>
            ) : null}
            <span className="muted">
              {filtered.length}/{MONITOR_RING_MAX} lines
            </span>
          </div>

          <div
            className="operate-monitor-tail"
            data-testid="monitor-tail"
            ref={listRef}
            onScroll={(e) => setScrollTop((e.target as HTMLDivElement).scrollTop)}
          >
            <div style={{ height: filtered.length * ROW_HEIGHT, position: "relative" }}>
              <div style={{ transform: `translateY(${win.offsetY}px)` }}>
                {visible.map((line) => (
                  <div
                    key={line.seq}
                    className={`operate-monitor-line level-${line.level}`}
                    style={{ height: ROW_HEIGHT }}
                    data-testid="monitor-line"
                  >
                    <span className="mono muted">{line.seq}</span>
                    <span className={`operate-monitor-level level-${line.level}`}>{line.level}</span>
                    <span className="operate-monitor-msg">{line.message}</span>
                    <span className="muted operate-monitor-at">{line.at}</span>
                  </div>
                ))}
              </div>
            </div>
            {filtered.length === 0 ? (
              <p className="muted operate-monitor-empty">Waiting for log lines…</p>
            ) : null}
          </div>
        </>
      )}
      {err ? <p className="err">{err}</p> : null}
    </ToolSurface>
  );
}

/** Test helper — append synthetic lines through the same ring path. */
export function appendSyntheticMonitorLines(
  state: MonitorRingState,
  count: number,
  startSeq = 1,
): MonitorRingState {
  const lines: MonitorLogLine[] = [];
  for (let i = 0; i < count; i++) {
    lines.push({
      seq: startSeq + i,
      level: "info",
      message: `synthetic-${startSeq + i}`,
      at: new Date().toISOString(),
    });
  }
  return appendMonitorLines(state, lines, MONITOR_RING_MAX);
}
