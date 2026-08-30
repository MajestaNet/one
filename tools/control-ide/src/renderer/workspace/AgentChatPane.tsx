import { useCallback, useMemo, useState } from "react";
import {
  AssistantRuntimeProvider,
  ComposerPrimitive,
  MessagePrimitive,
  ThreadPrimitive,
  getExternalStoreMessages,
  useAuiState,
} from "@assistant-ui/react";
import type { BoardHandoff } from "../operate/types";
import { IconSend } from "../icons/Icons";
import { Spinner } from "../ui";
import { useOneAgentRuntime } from "../agents/oneRuntime";
import { StreamMessageBubble } from "./StreamMessageBubble";
import { plainTextFromMessageContent } from "./messageFallback";
import type { AgentChat, StreamMessage, TileId } from "./types";
import { AGENT_CHAT_MIME, isAgentBound } from "./types";
import {
  isContextExcerptDrag,
  readExcerptFromDataTransfer,
  type ContextExcerpt,
} from "./contextExcerpt";

function OneStoreMessage({
  agentName,
  onOpenBoard,
  onOpenTile,
  onOpenSessionTool,
  onApprove,
  approveBusy,
  onStageMutations,
  onStageProposal,
  onPinToGraph,
  onOpenInQuery,
  expandHandoff,
}: {
  agentName: string;
  onOpenBoard?: (handoff: BoardHandoff) => void;
  onOpenTile?: (id: TileId) => void;
  onOpenSessionTool?: (toolId: string) => void;
  onApprove?: (runId: string) => void;
  approveBusy?: boolean;
  onStageMutations?: (count: number) => void;
  onStageProposal?: (handoff: BoardHandoff) => Promise<void>;
  onPinToGraph?: (handoff: BoardHandoff) => Promise<void>;
  onOpenInQuery?: (objectApiName: string) => void;
  expandHandoff?: boolean;
}) {
  const auiMessage = useAuiState((s) => s.message);
  const fromStore = getExternalStoreMessages<StreamMessage>(auiMessage)[0];
  const fromCustom = (auiMessage.metadata?.custom as { one?: StreamMessage } | undefined)?.one;
  const streamMsg = fromStore ?? fromCustom;
  if (!streamMsg) {
    const text = plainTextFromMessageContent(auiMessage.content);
    return (
      <MessagePrimitive.Root className="aui-msg-fallback">
        <p className="stream-body">{text}</p>
      </MessagePrimitive.Root>
    );
  }
  return (
    <MessagePrimitive.Root className="aui-msg-root" data-role={streamMsg.role}>
      <StreamMessageBubble
        message={streamMsg}
        agentName={agentName}
        onOpenBoard={onOpenBoard}
        onOpenTile={onOpenTile}
        onOpenSessionTool={onOpenSessionTool}
        onApprove={onApprove}
        approveBusy={approveBusy}
        onStageMutations={onStageMutations}
        onStageProposal={onStageProposal}
        onPinToGraph={onPinToGraph}
        onOpenInQuery={onOpenInQuery}
        expandHandoff={expandHandoff}
      />
    </MessagePrimitive.Root>
  );
}

export function AgentChatPane({
  chat,
  onSend,
  onOpenBoard,
  onOpenTile,
  onOpenSessionTool,
  onApprove,
  onAttachAgent,
  onStageMutations,
  onStageProposal,
  onPinToGraph,
  onOpenInQuery,
  busy = false,
  approveBusy = false,
  acceptAgentDrop = false,
  pendingExcerpts = [],
  onPendingExcerptsChange,
}: {
  chat: AgentChat;
  onClose?: () => void;
  onSend: (chatId: string, text: string, excerpts?: ContextExcerpt[]) => void;
  onOpenBoard?: (handoff: BoardHandoff) => void;
  onOpenTile?: (id: TileId) => void;
  onOpenSessionTool?: (toolId: string) => void;
  onApprove?: (runId: string) => void;
  onAttachAgent?: (agentId: string) => void;
  onStageMutations?: (count: number) => void;
  onStageProposal?: (handoff: BoardHandoff) => Promise<void>;
  onPinToGraph?: (handoff: BoardHandoff) => Promise<void>;
  onOpenInQuery?: (objectApiName: string) => void;
  busy?: boolean;
  approveBusy?: boolean;
  acceptAgentDrop?: boolean;
  pendingExcerpts?: ContextExcerpt[];
  onPendingExcerptsChange?: (excerpts: ContextExcerpt[]) => void;
}) {
  const [dragOver, setDragOver] = useState(false);
  const bound = isAgentBound(chat);

  const addPendingExcerpt = useCallback(
    (excerpt: ContextExcerpt) => {
      if (!onPendingExcerptsChange) return;
      onPendingExcerptsChange([...pendingExcerpts, excerpt]);
    },
    [onPendingExcerptsChange, pendingExcerpts],
  );

  const removePendingExcerpt = useCallback(
    (id: string) => {
      if (!onPendingExcerptsChange) return;
      onPendingExcerptsChange(pendingExcerpts.filter((e) => e.id !== id));
    },
    [onPendingExcerptsChange, pendingExcerpts],
  );

  const handleSend = useCallback(
    (text: string) => {
      const excerpts = pendingExcerpts.length > 0 ? [...pendingExcerpts] : undefined;
      if (excerpts && onPendingExcerptsChange) onPendingExcerptsChange([]);
      onSend(chat.id, text, excerpts);
    },
    [chat.id, onPendingExcerptsChange, onSend, pendingExcerpts],
  );

  const runtime = useOneAgentRuntime({
    messages: chat.messages,
    isRunning: busy,
    isDisabled: false,
    isSendDisabled: !bound || busy,
    onSend: handleSend,
  });

  const pendingApproval = chat.messages.find(
    (m: StreamMessage) => m.role === "approval" && m.runStatus === "awaiting_approval" && m.runId,
  );

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      setDragOver(false);
      if (isContextExcerptDrag(e.dataTransfer)) {
        const excerpt = readExcerptFromDataTransfer(e.dataTransfer);
        if (excerpt) addPendingExcerpt(excerpt);
        return;
      }
      if (!acceptAgentDrop || !onAttachAgent) return;
      const raw = e.dataTransfer.getData(AGENT_CHAT_MIME) || e.dataTransfer.getData("text/plain");
      try {
        const parsed = JSON.parse(raw) as { id?: string };
        if (parsed.id) onAttachAgent(parsed.id);
      } catch {
        /* ignore */
      }
    },
    [acceptAgentDrop, addPendingExcerpt, onAttachAgent],
  );

  const expandHandoff = Boolean(chat.primary);
  const messageComponents = useMemo(
    () => {
      const Message = () => (
        <OneStoreMessage
          agentName={chat.agentName}
          onOpenBoard={onOpenBoard}
          onOpenTile={onOpenTile}
          onOpenSessionTool={onOpenSessionTool}
          onApprove={onApprove}
          approveBusy={approveBusy}
          onStageMutations={onStageMutations}
          onStageProposal={onStageProposal}
          onPinToGraph={onPinToGraph}
          onOpenInQuery={onOpenInQuery}
          expandHandoff={expandHandoff}
        />
      );
      return {
        UserMessage: Message,
        AssistantMessage: Message,
        SystemMessage: Message,
      };
    },
    [
      approveBusy,
      chat.agentName,
      expandHandoff,
      onApprove,
      onOpenBoard,
      onOpenInQuery,
      onOpenSessionTool,
      onOpenTile,
      onPinToGraph,
      onStageMutations,
      onStageProposal,
    ],
  );
  const streamingInPlace = chat.messages.some(
    (m) => (m.role === "agent" || m.role === "approval") && m.runStatus === "running",
  );

  return (
    <article
      className={`agent-chat-pane aui-chat-pane ${dragOver ? "agent-drop-active" : ""} ${chat.primary ? "is-primary" : ""}`}
      data-testid={`agent-chat-pane-${chat.id}`}
      onDragOver={(e) => {
        const excerptDrag = isContextExcerptDrag(e.dataTransfer);
        const agentDrag =
          acceptAgentDrop &&
          onAttachAgent &&
          ([...e.dataTransfer.types].includes(AGENT_CHAT_MIME) || e.dataTransfer.types.includes("text/plain"));
        if (!excerptDrag && !agentDrag) return;
        e.preventDefault();
        e.dataTransfer.dropEffect = "copy";
        setDragOver(true);
      }}
      onDragLeave={() => setDragOver(false)}
      onDrop={handleDrop}
    >
      <header className="agent-chat-pane-header">
        <div className="agent-chat-pane-heading">
          <h3 className="agent-chat-title">{chat.agentName || chat.title || "Agent"}</h3>
          {bound ? (
            <p className="muted agent-chat-playbook" data-testid="agent-chat-bound">
              1:1 chat
            </p>
          ) : (
            <p className="muted agent-chat-playbook" data-testid="agent-chat-unbound">
              Choose an agent from the dock
            </p>
          )}
        </div>
        {busy ? (
          <p className="agent-chat-lifecycle" data-testid="chat-run-lifecycle">
            <Spinner size={12} /> Running…
          </p>
        ) : pendingApproval ? (
          <p className="agent-chat-lifecycle muted" data-testid="chat-run-lifecycle">
            Awaiting approval
          </p>
        ) : null}
      </header>

      <AssistantRuntimeProvider runtime={runtime}>
        <ThreadPrimitive.Root className="aui-thread-root" data-testid={`agent-chat-messages-${chat.id}`}>
          <ThreadPrimitive.Viewport className="aui-thread-viewport agent-chat-pane-messages">
            <ThreadPrimitive.Messages components={messageComponents} />
            {busy && !streamingInPlace ? (
              <div className="stream-typing" data-testid="chat-typing" aria-live="polite">
                <span className="stream-typing-dot" />
                <span className="stream-typing-dot" />
                <span className="stream-typing-dot" />
                <span className="muted">Thinking</span>
              </div>
            ) : null}
          </ThreadPrimitive.Viewport>

          {pendingExcerpts.length > 0 ? (
            <div className="agent-chat-pending-excerpts" data-testid="agent-chat-pending-excerpts">
              {pendingExcerpts.map((ex) => (
                <span key={ex.id} className="agent-chat-excerpt-chip" data-testid={`pending-excerpt-${ex.id}`}>
                  <span className="agent-chat-excerpt-chip-label">{ex.label}</span>
                  <button
                    type="button"
                    className="agent-chat-excerpt-chip-remove"
                    aria-label="Remove context"
                    onClick={() => removePendingExcerpt(ex.id)}
                  >
                    ×
                  </button>
                </span>
              ))}
            </div>
          ) : null}

          <ComposerPrimitive.Root
            className="agent-chat-pane-composer aui-composer aui-composer-inline"
            data-testid={`agent-chat-composer-${chat.id}`}
          >
            <ComposerPrimitive.Input
              className="aui-composer-input"
              rows={1}
              placeholder={bound ? `Send follow-up to ${chat.agentName}…` : "Choose an agent to start asking…"}
              disabled={!bound || busy}
              submitMode="enter"
            />
            <ComposerPrimitive.Send
              className="btn btn-primary aui-composer-send"
              disabled={!bound || busy}
              aria-label="Send"
            >
              <IconSend size={14} />
            </ComposerPrimitive.Send>
          </ComposerPrimitive.Root>
        </ThreadPrimitive.Root>
      </AssistantRuntimeProvider>
    </article>
  );
}
