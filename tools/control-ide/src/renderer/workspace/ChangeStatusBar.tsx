import { CheckDot } from "../ui";
import { statusLabel, type ChangeStatus, type CheckItem } from "./types";

const STATE_ORDER: Record<CheckItem["state"], number> = {
  running: 0,
  failed: 1,
  pending: 2,
  passed: 3,
};

export function ChangeStatusBar({
  status,
  checks,
  showChecks = true,
}: {
  status: ChangeStatus;
  checks: CheckItem[];
  /** When false (e.g. home launcher / auth / Account), hide the check strip. */
  showChecks?: boolean;
}) {
  const running = checks.filter((c) => c.state === "running").length;
  const passed = checks.filter((c) => c.state === "passed").length;
  const pending = checks.filter((c) => c.state === "pending").length;
  const failed = checks.filter((c) => c.state === "failed").length;

  const ordered = [...checks].sort(
    (a, b) => STATE_ORDER[a.state] - STATE_ORDER[b.state] || a.label.localeCompare(b.label),
  );

  const tone =
    status === "ready" || status === "applied" || status === "promoted"
      ? "success"
      : status === "running"
        ? "accent"
        : status === "needs_review"
          ? "warn"
          : "neutral";

  return (
    <footer className="change-status-bar" aria-label="Change status" data-testid="change-status-bar">
      <div className="change-status-left">
        <span className={`change-badge status-badge tone-${tone}`}>{statusLabel(status)}</span>
        {showChecks ? (
          <span className="muted change-status-summary">
            Checks {passed}/{checks.length}
            {running > 0 ? ` · ${running} running` : ""}
            {pending > 0 ? ` · ${pending} pending` : ""}
            {failed > 0 ? ` · ${failed} failed` : ""}
          </span>
        ) : null}
      </div>
      {showChecks ? (
        <ul className="change-checks" data-testid="change-checks">
          {ordered.map((c) => (
            <li key={c.id} className={`change-check-chip state-${c.state}`}>
              <CheckDot state={c.state} />
              <span className="change-check-label">{c.label}</span>
              {c.duration ? <span className="muted check-dur">{c.duration}</span> : null}
            </li>
          ))}
        </ul>
      ) : null}
    </footer>
  );
}
