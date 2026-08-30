import { useState } from "react";
import { AgentChatPane } from "./AgentChatPane";
import { AGENT_CHAT_MIME, agentGridClass, type AgentChat } from "./types";
import { IconDropTarget } from "../icons/Icons";
import { EmptyState } from "../ui";

const MAX_AGENT_TILES = 4;

export function AgentsCanvas({
  catalog,
  openChats,
  onOpenChat,
  onCloseChat,
  onSendInChat,
}: {
  catalog: AgentChat[];
  openChats: AgentChat[];
  onOpenChat: (chat: AgentChat) => void;
  onCloseChat: (id: string) => void;
  onSendInChat: (chatId: string, text: string) => void;
}) {
  const [dragOver, setDragOver] = useState(false);
  const atCap = openChats.length >= MAX_AGENT_TILES;

  const acceptDrop = (raw: string) => {
    try {
      const parsed = JSON.parse(raw) as { id?: string };
      const chat = catalog.find((c) => c.id === parsed.id);
      if (chat) onOpenChat(chat);
    } catch {
      /* ignore bad payloads */
    }
  };

  return (
    <section
      className={`agents-canvas ${dragOver ? "drag-over" : ""}`}
      data-testid="agents-canvas"
      aria-label="Agents canvas"
      onDragOver={(e) => {
        if (![...e.dataTransfer.types].includes(AGENT_CHAT_MIME) && !e.dataTransfer.types.includes("text/plain")) {
          return;
        }
        e.preventDefault();
        e.dataTransfer.dropEffect = atCap ? "none" : "copy";
        setDragOver(true);
      }}
      onDragLeave={() => setDragOver(false)}
      onDrop={(e) => {
        e.preventDefault();
        setDragOver(false);
        if (atCap) return;
        const raw = e.dataTransfer.getData(AGENT_CHAT_MIME) || e.dataTransfer.getData("text/plain");
        if (raw) acceptDrop(raw);
      }}
    >
      {openChats.length === 0 ? (
        <div className="agents-empty" data-testid="agents-empty">
          <EmptyState
            icon={<IconDropTarget size={48} />}
            title="Agents workspace"
            description={`Drag an agent chat from the stream into this space. Up to ${MAX_AGENT_TILES} chats tile automatically — one fills the canvas; more arrange in a grid.`}
          />
        </div>
      ) : (
        <div className={agentGridClass(openChats.length)} data-testid="agent-grid">
          {openChats.map((chat) => (
            <AgentChatPane key={chat.id} chat={chat} onClose={() => onCloseChat(chat.id)} onSend={onSendInChat} />
          ))}
        </div>
      )}
      {atCap && <p className="agents-cap muted">Maximum of {MAX_AGENT_TILES} agent chats open.</p>}
    </section>
  );
}
