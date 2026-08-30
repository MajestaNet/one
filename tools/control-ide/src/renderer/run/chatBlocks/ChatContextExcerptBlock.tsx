import { useState } from "react";
import type { ContextExcerpt } from "../../workspace/contextExcerpt";
import { Button } from "../../ui";

export function ChatContextExcerptBlock({
  excerpts,
  onDismiss,
}: {
  excerpts: ContextExcerpt[];
  onDismiss?: () => void;
}) {
  const [expanded, setExpanded] = useState(false);
  if (excerpts.length === 0) return null;

  const preview = excerpts[0];
  const more = excerpts.length - 1;

  return (
    <div className="chat-context-excerpt-block" data-testid="chat-context-excerpt-block">
      <div className="chat-context-excerpt-header">
        <span className="chat-context-excerpt-badge">Context</span>
        <strong className="chat-context-excerpt-label">{preview.label}</strong>
        {more > 0 ? <span className="muted">+{more} more</span> : null}
        <div className="chat-context-excerpt-actions">
          <Button variant="ghost" onClick={() => setExpanded((o) => !o)}>
            {expanded ? "Collapse" : "Expand"}
          </Button>
          {onDismiss ? (
            <Button variant="ghost" onClick={onDismiss} data-testid="chat-context-excerpt-dismiss">
              Dismiss
            </Button>
          ) : null}
        </div>
      </div>
      {expanded ? (
        <div className="chat-context-excerpt-body">
          {excerpts.map((ex) => (
            <pre key={ex.id} className="chat-context-excerpt-pre mono" data-testid={`chat-context-excerpt-${ex.id}`}>
              {ex.text}
            </pre>
          ))}
        </div>
      ) : (
        <p className="muted chat-context-excerpt-preview">{preview.text.split("\n").slice(0, 3).join("\n")}</p>
      )}
    </div>
  );
}
