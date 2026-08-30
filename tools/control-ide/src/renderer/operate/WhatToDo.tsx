import type { BoardHandoffSuggestion } from "./types";
import { Button } from "../ui";

export function WhatToDo({
  open,
  rationale,
  suggestions,
  onDismiss,
  onAction,
}: {
  open: boolean;
  rationale?: string;
  suggestions: BoardHandoffSuggestion[];
  onDismiss: () => void;
  onAction: (action: string) => void;
}) {
  if (!open || suggestions.length === 0) return null;

  return (
    <div className="crm-what-to-do" data-testid="crm-what-to-do" role="dialog" aria-label="What to do">
      <div className="crm-what-to-do-header">
        <div>
          <p className="crm-detail-kicker">What to do</p>
          {rationale ? <p className="muted">{rationale}</p> : null}
        </div>
        <Button variant="ghost" onClick={onDismiss} data-testid="crm-what-to-do-dismiss">
          Dismiss
        </Button>
      </div>
      <div className="crm-what-to-do-actions">
        {suggestions.map((s) => (
          <Button key={s.id} variant="secondary" onClick={() => onAction(s.action)}>
            {s.label}
          </Button>
        ))}
      </div>
    </div>
  );
}
