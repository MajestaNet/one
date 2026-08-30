import { Button } from "../ui";
import type { ToolAutomationStatus } from "./useRunToolActions";

export function ToolAutomationStatusBanner({
  status,
  onDismiss,
}: {
  status: ToolAutomationStatus;
  onDismiss?: () => void;
}) {
  const tone =
    status.phase === "error" ? "error" : status.phase === "running" ? "muted" : "panel-warn";
  return (
    <div
      className={`run-automation-status ${tone}`}
      data-testid="run-automation-status"
      data-phase={status.phase}
      role="status"
    >
      <div>
        <p className="run-automation-status-title">{status.apiName || "Automation"}</p>
        <p className="run-automation-status-message">{status.message}</p>
        {status.run?.id ? (
          <p className="muted mono run-automation-status-id">Run {status.run.id}</p>
        ) : null}
      </div>
      {onDismiss && status.phase !== "running" ? (
        <Button variant="ghost" data-testid="run-automation-dismiss" onClick={onDismiss}>
          Dismiss
        </Button>
      ) : null}
    </div>
  );
}
