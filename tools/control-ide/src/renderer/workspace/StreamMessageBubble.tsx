import { memo, useState, type ReactNode } from "react";
import ReactMarkdown from "react-markdown";
import { openExternalUrl } from "../external";
import type { BoardHandoff } from "../operate/types";
import { ChatHandoffBlock } from "../operate/chatBlocks/ChatHandoffBlock";
import { ChatContextExcerptBlock } from "../run/chatBlocks/ChatContextExcerptBlock";
import { ChatToolRefBlock } from "../run/chatBlocks/ChatToolRefBlock";
import { Button } from "../ui";
import { formatMessageTime, roleLabel } from "./messageModel";
import type { StreamMessage, TileId } from "./types";

const markdownComponents = {
  a: ({ href, children }: { href?: string; children?: ReactNode }) => (
    <a
      href={href}
      onClick={(event) => {
        event.preventDefault();
        if (href) void openExternalUrl(href);
      }}
    >
      {children}
    </a>
  ),
};

const StreamMarkdown = memo(function StreamMarkdown({ body }: { body: string }) {
  return <ReactMarkdown components={markdownComponents}>{body}</ReactMarkdown>;
});

function StreamThinking() {
  return (
    <div className="stream-typing" data-testid="chat-typing" aria-live="polite">
      <span className="stream-typing-dot" />
      <span className="stream-typing-dot" />
      <span className="stream-typing-dot" />
      <span className="muted">Thinking</span>
    </div>
  );
}

export const StreamMessageBubble = memo(function StreamMessageBubble({
  message,
  onApprove,
  approveBusy,
  onStageMutations,
  onStageProposal,
  onPinToGraph,
  onOpenInQuery,
  onOpenSessionTool,
  expandHandoff = false,
}: {
  message: StreamMessage;
  /** @deprecated Role chip omitted in 1:1 chats; kept for call-site compatibility. */
  agentName?: string;
  /** @deprecated Open-board CTA removed; kept optional for call-site compatibility. */
  onOpenTile?: (id: TileId) => void;
  /** @deprecated Open-board CTA removed; kept optional for call-site compatibility. */
  onOpenBoard?: (handoff: BoardHandoff) => void;
  onApprove?: (runId: string) => void;
  approveBusy?: boolean;
  onStageMutations?: (count: number) => void;
  onStageProposal?: (handoff: BoardHandoff) => Promise<void>;
  onPinToGraph?: (handoff: BoardHandoff) => Promise<void>;
  /** Open Query panel for the handoff object. */
  onOpenInQuery?: (objectApiName: string) => void;
  /** Open a session working Tool in Run mode. */
  onOpenSessionTool?: (toolId: string) => void;
  /** When true, render inline handoff block immediately (Operate primary chat). */
  expandHandoff?: boolean;
}) {
  const [stepsOpen, setStepsOpen] = useState(false);
  const handoff = message.boardHandoff;
  const toolHandoff = message.toolHandoff;
  const hasRenderableHandoff = Boolean(
    handoff &&
      ((handoff.recordIds?.length ?? 0) > 0 ||
        (handoff.proposedMutations?.length ?? 0) > 0),
  );
  const hasRenderableToolHandoff = Boolean(toolHandoff?.toolId);
  const excerpts = message.contextExcerpts ?? [];
  const hasExcerpts = excerpts.length > 0;
  // Auto-open record/mutation handoffs; suggestion-only handoffs are omitted.
  const [handoffOpen, setHandoffOpen] = useState(hasRenderableHandoff || expandHandoff);
  const [toolRefOpen, setToolRefOpen] = useState(hasRenderableToolHandoff || expandHandoff);
  const time = formatMessageTime(message.createdAt);
  const align = message.role === "human" ? "end" : "start";
  const showApprove =
    message.role === "approval" &&
    Boolean(message.runId) &&
    Boolean(onApprove) &&
    (message.runStatus === "awaiting_approval" || Boolean(message.pendingToolApply));
  // Bound 1:1 chats already show the agent in the pane header; human msgs are right-aligned.
  const showRoleChip = message.role === "system" || message.role === "tool";
  const hasMetaRight = Boolean(message.runStatus || time);
  const showMeta = showRoleChip || hasMetaRight;
  const isRunning = message.runStatus === "running";
  const showThinking = isRunning && !message.body;
  const pendingSteps = message.steps?.filter((s) => s.state === "pending") ?? [];
  const approvalTools =
    pendingSteps.length > 0
      ? pendingSteps.map((s) => s.label)
      : (message.toolsPlanned ?? []);

  return (
    <article
      className={`stream-bubble role-${message.role} align-${align}`}
      data-testid={`stream-bubble-${message.id}`}
      data-role={message.role}
    >
      {showMeta ? (
        <div className={`stream-bubble-meta ${showRoleChip ? "" : "meta-right-only"}`.trim()}>
          {showRoleChip ? <span className="stream-chip">{roleLabel(message.role)}</span> : null}
          <div className="stream-bubble-meta-right">
            {message.runStatus && !showThinking ? (
              <span className="stream-run-status" data-testid="stream-run-status">
                {message.runStatus.replace(/_/g, " ")}
              </span>
            ) : null}
            {time ? <span className="stream-time">{time}</span> : null}
          </div>
        </div>
      ) : null}

      <div
        className={`stream-body ${isRunning && message.body ? "is-streaming" : ""} ${
          isRunning ? "stream-body-plain" : "stream-markdown"
        }`}
      >
        {showThinking ? (
          <StreamThinking />
        ) : message.body ? (
          isRunning ? (
            message.body
          ) : (
            <StreamMarkdown body={message.body} />
          )
        ) : null}
      </div>

      {message.steps && message.steps.length > 0 ? (
        <div className="stream-steps" data-testid="stream-steps">
          <button
            type="button"
            className="stream-steps-toggle"
            aria-expanded={stepsOpen}
            onClick={() => setStepsOpen((o) => !o)}
          >
            {stepsOpen ? "Hide" : "Show"} {message.steps.length} step
            {message.steps.length === 1 ? "" : "s"}
          </button>
          {stepsOpen ? (
            <ol className="stream-steps-list" data-testid="stream-steps-list">
              {message.steps.map((s) => (
                <li key={s.id} className={`stream-step state-${s.state}`}>
                  <span className="stream-step-dot" aria-hidden />
                  <span className="stream-step-label">{s.label}</span>
                  <span className="stream-step-state">{s.state}</span>
                </li>
              ))}
            </ol>
          ) : null}
        </div>
      ) : message.toolsPlanned && message.toolsPlanned.length > 0 && message.role !== "tool" ? (
        <p className="muted stream-tools-inline" data-testid="stream-tools-inline">
          Tools: {message.toolsPlanned.join(", ")}
        </p>
      ) : null}

      {hasExcerpts ? (
        <ChatContextExcerptBlock excerpts={excerpts} />
      ) : null}

      {handoff && handoffOpen && hasRenderableHandoff ? (
        <ChatHandoffBlock
          handoff={handoff}
          onStageMutations={onStageMutations}
          onStageProposal={onStageProposal}
          onPinToGraph={onPinToGraph}
          onOpenInQuery={onOpenInQuery}
          onDismiss={() => setHandoffOpen(false)}
        />
      ) : null}

      {toolHandoff && toolRefOpen && hasRenderableToolHandoff ? (
        <ChatToolRefBlock
          handoff={toolHandoff}
          onOpenTool={onOpenSessionTool}
          onDismiss={() => setToolRefOpen(false)}
        />
      ) : null}

      <div className="stream-bubble-actions">
        {showApprove ? (
          <div className="stream-tool-approval" data-testid="tool-approval-card">
            <p className="stream-tool-approval-title">Approve tool use</p>
            <p className="stream-tool-approval-copy">
              The agent wants to run the actions below. Nothing is executed until you approve.
            </p>
            {approvalTools.length > 0 ? (
              <ul className="stream-tool-approval-list">
                {approvalTools.map((label) => (
                  <li key={label}>{label}</li>
                ))}
              </ul>
            ) : (
              <p className="stream-tool-approval-copy">Tools planned by this playbook.</p>
            )}
            <Button
              variant="primary"
              busy={approveBusy}
              data-testid="approve-run-inline"
              onClick={() => onApprove!(message.runId!)}
            >
              Approve tool use
            </Button>
          </div>
        ) : null}
      </div>
    </article>
  );
});
