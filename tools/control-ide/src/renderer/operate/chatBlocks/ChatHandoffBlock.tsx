import { useState } from "react";
import type { BoardHandoff, ProposedMutation } from "../types";
import { Button } from "../../ui";

/** Show the top N field keys from mutation data (not full dump). */
function mutationFieldKeys(m: ProposedMutation): string[] {
  if (!m.data) return [];
  return Object.keys(m.data).slice(0, 5);
}

/** Inline BoardHandoff working set inside the Operate chat transcript. */
export function ChatHandoffBlock({
  handoff,
  onStageMutations,
  onStageProposal,
  onPinToGraph,
  onDismiss,
  onOpenInQuery,
}: {
  handoff: BoardHandoff;
  onStageMutations?: (count: number) => void;
  onStageProposal?: (handoff: BoardHandoff) => Promise<void>;
  onPinToGraph?: (handoff: BoardHandoff) => Promise<void>;
  onDismiss?: () => void;
  /** Open the Query panel pre-seeded to this object. */
  onOpenInQuery?: (objectApiName: string) => void;
}) {
  const [staged, setStaged] = useState(false);
  const [pinned, setPinned] = useState(false);
  const [busy, setBusy] = useState<"stage" | "pin" | null>(null);
  const [error, setError] = useState("");
  const ids = handoff.recordIds ?? [];
  const mutations = handoff.proposedMutations ?? [];
  const objectLabel = handoff.objectApiName ?? "Records";

  const stage = async () => {
    if (!mutations.length) return;
    setBusy("stage");
    setError("");
    try {
      await onStageProposal?.(handoff);
      setStaged(true);
      onStageMutations?.(mutations.length);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setBusy(null);
    }
  };

  const pin = async () => {
    if (!ids.length || !handoff.objectApiName || !onPinToGraph) return;
    setBusy("pin");
    setError("");
    try {
      await onPinToGraph(handoff);
      setPinned(true);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setBusy(null);
    }
  };

  // Skip empty suggestion-only handoffs (unwired What-to-do / Show matching records).
  if (ids.length === 0 && mutations.length === 0) {
    return null;
  }

  return (
    <div className="chat-handoff-block" data-testid="chat-handoff-block">
      <div className="chat-handoff-header">
        <div>
          <p className="chat-handoff-kicker">{objectLabel}</p>
          {handoff.rationale ? <p className="muted chat-handoff-rationale">{handoff.rationale}</p> : null}
        </div>
        <div className="chat-handoff-header-actions">
          {handoff.objectApiName && onOpenInQuery ? (
            <Button
              variant="ghost"
              data-testid="chat-open-in-query"
              onClick={() => onOpenInQuery(handoff.objectApiName!)}
            >
              Open in Query
            </Button>
          ) : null}
          {ids.length > 0 && handoff.objectApiName && onPinToGraph ? (
            <Button
              variant="secondary"
              busy={busy === "pin"}
              disabled={busy !== null || pinned}
              data-testid="chat-pin-to-graph"
              onClick={() => void pin()}
            >
              {pinned ? "Pinned to my graph" : "Pin to my graph"}
            </Button>
          ) : null}
          {onDismiss ? (
            <Button variant="ghost" onClick={onDismiss} data-testid="chat-handoff-dismiss">
              Dismiss
            </Button>
          ) : null}
        </div>
      </div>

      {ids.length > 0 ? (
        <ul className="chat-record-strip" data-testid="chat-record-strip">
          {ids.slice(0, 12).map((id) => (
            <li key={id} className="chat-record-chip">
              {handoff.objectApiName ? (
                <span className="chat-record-object muted">{handoff.objectApiName} · </span>
              ) : null}
              <span className="chat-record-id">{id.length > 12 ? `${id.slice(0, 12)}…` : id}</span>
            </li>
          ))}
          {ids.length > 12 ? (
            <li className="muted chat-record-more">+{ids.length - 12} more</li>
          ) : null}
        </ul>
      ) : null}

      {mutations.length > 0 ? (
        <div className="chat-mutation-review" data-testid="chat-mutation-review">
          <p className="chat-handoff-kicker">Proposed updates</p>
          <ul className="chat-mutation-list">
            {mutations.map((m, i) => {
              const keys = mutationFieldKeys(m);
              return (
                <li key={`${m.op}-${m.object}-${m.id ?? i}`}>
                  <span className="chat-mutation-op">{m.op}</span> {m.object}
                  {m.id ? <span className="muted"> · {m.id}</span> : null}
                  {keys.length > 0 ? (
                    <span className="muted chat-mutation-fields"> [{keys.join(", ")}]</span>
                  ) : null}
                </li>
              );
            })}
          </ul>
          <Button
            variant="primary"
            busy={busy === "stage"}
            disabled={staged || busy !== null}
            onClick={() => void stage()}
            data-testid="chat-stage-mutations"
          >
            {staged ? "Staged for review" : `Stage ${mutations.length} mutation(s)`}
          </Button>
        </div>
      ) : null}
      {error ? <p className="error" role="alert">{error}</p> : null}
    </div>
  );
}
