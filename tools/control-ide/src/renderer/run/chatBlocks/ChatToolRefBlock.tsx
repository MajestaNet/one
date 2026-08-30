import type { ToolHandoff } from "../toolHandoff";
import { Button } from "../../ui";

/** Inline Tool reference card in Run chat (ADR-021 Phase 4). */
export function ChatToolRefBlock({
  handoff,
  onOpenTool,
  onDismiss,
}: {
  handoff: ToolHandoff;
  onOpenTool?: (toolId: string) => void;
  onDismiss?: () => void;
}) {
  const title = handoff.toolTitle ?? handoff.toolSpecApiName ?? "Tool";

  return (
    <div className="chat-tool-ref" data-testid="chat-tool-ref">
      <div className="chat-tool-ref-header">
        <div>
          <p className="chat-handoff-kicker">Tool</p>
          <p className="chat-tool-ref-title">{title}</p>
          {handoff.rationale ? <p className="muted chat-handoff-rationale">{handoff.rationale}</p> : null}
        </div>
        {onDismiss ? (
          <Button variant="ghost" onClick={onDismiss} data-testid="chat-tool-ref-dismiss">
            Dismiss
          </Button>
        ) : null}
      </div>
      <div className="chat-tool-ref-actions">
        <Button
          variant="primary"
          data-testid="chat-open-tool"
          onClick={() => onOpenTool?.(handoff.toolId)}
        >
          Open Tool
        </Button>
        <span className="muted chat-tool-ref-id">{handoff.toolId}</span>
      </div>
    </div>
  );
}
