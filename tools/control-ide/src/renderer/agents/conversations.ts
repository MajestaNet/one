/** Client API helpers for /client/v1/agents/conversations (Agentic Run uplift). */

import type { FetchFn } from "./runs";
import type { AgentChat, AppSection, StreamMessage } from "../workspace/types";
import type { ContextExcerpt } from "../workspace/contextExcerpt";

export type ConversationMessageRow = {
  id: string;
  role: string;
  body: string;
  parts?: unknown;
  runId?: string;
  createdAt?: string;
};

export type AgentConversation = {
  id: string;
  playbookApiName?: string;
  mode: string;
  title: string;
  createdAt?: string;
  updatedAt?: string;
  messages?: ConversationMessageRow[];
};

export async function listAgentConversations(fetchFn: FetchFn): Promise<AgentConversation[]> {
  const row = (await fetchFn("/client/v1/agents/conversations")) as { conversations?: AgentConversation[] };
  return row.conversations ?? [];
}

export async function createAgentConversation(
  fetchFn: FetchFn,
  body: { playbookApiName?: string; mode: AppSection; title: string },
): Promise<AgentConversation> {
  return (await fetchFn("/client/v1/agents/conversations", {
    method: "POST",
    body: JSON.stringify(body),
  })) as AgentConversation;
}

export async function getAgentConversation(fetchFn: FetchFn, id: string): Promise<AgentConversation> {
  return (await fetchFn(`/client/v1/agents/conversations/${encodeURIComponent(id)}`)) as AgentConversation;
}

export async function appendConversationMessages(
  fetchFn: FetchFn,
  conversationId: string,
  messages: Array<{
    role: string;
    body: string;
    parts?: unknown;
    runId?: string;
  }>,
): Promise<ConversationMessageRow[]> {
  const row = (await fetchFn(
    `/client/v1/agents/conversations/${encodeURIComponent(conversationId)}/messages`,
    {
      method: "POST",
      body: JSON.stringify({ messages }),
    },
  )) as { messages?: ConversationMessageRow[] };
  return row.messages ?? [];
}

function partsFromMessage(message: StreamMessage): unknown[] {
  const parts: unknown[] = [];
  if (message.contextExcerpts?.length) {
    parts.push({ type: "contextExcerpts", excerpts: message.contextExcerpts });
  }
  if (message.boardHandoff) parts.push({ type: "boardHandoff", handoff: message.boardHandoff });
  if (message.toolHandoff) parts.push({ type: "toolHandoff", handoff: message.toolHandoff });
  return parts;
}

export function streamMessageFromConversationRow(row: ConversationMessageRow): StreamMessage {
  let contextExcerpts: ContextExcerpt[] | undefined;
  let boardHandoff: StreamMessage["boardHandoff"];
  let toolHandoff: StreamMessage["toolHandoff"];
  if (Array.isArray(row.parts)) {
    for (const part of row.parts) {
      if (!part || typeof part !== "object") continue;
      const p = part as Record<string, unknown>;
      if (p.type === "contextExcerpts" && Array.isArray(p.excerpts)) {
        contextExcerpts = p.excerpts as ContextExcerpt[];
      }
      if (p.type === "boardHandoff") boardHandoff = p.handoff as StreamMessage["boardHandoff"];
      if (p.type === "toolHandoff") toolHandoff = p.handoff as StreamMessage["toolHandoff"];
    }
  }
  const role =
    row.role === "human" || row.role === "user"
      ? "human"
      : row.role === "system"
        ? "system"
        : row.role === "approval"
          ? "approval"
          : row.role === "tool"
            ? "tool"
            : "agent";
  return {
    id: row.id,
    role,
    body: row.body,
    runId: row.runId,
    createdAt: row.createdAt,
    contextExcerpts,
    boardHandoff,
    toolHandoff,
  };
}

/** Ensure a server conversation exists for this chat tile; returns conversation id. */
export async function syncConversationFromChat(
  fetchFn: FetchFn,
  chat: AgentChat,
  mode: AppSection,
): Promise<string> {
  if (chat.conversationId) return chat.conversationId;
  const created = await createAgentConversation(fetchFn, {
    playbookApiName: chat.agentName || undefined,
    mode: chat.modes[0] ?? mode,
    title: chat.title,
  });
  return created.id;
}

export async function persistStreamMessages(
  fetchFn: FetchFn,
  conversationId: string,
  messages: StreamMessage[],
): Promise<void> {
  if (!messages.length) return;
  await appendConversationMessages(
    fetchFn,
    conversationId,
    messages.map((m) => ({
      role: m.role === "human" ? "human" : m.role,
      body: m.body,
      parts: partsFromMessage(m),
      runId: m.runId,
    })),
  );
}

export type ConversationTurn = { role: "user" | "assistant"; content: string };

const MAX_CONVERSATION_TURNS = 32;
const MAX_TURN_CHARS = 4000;

/** Prior transcript turns for POST /agents/runs `input.conversation` (excludes the current goal). */
export function conversationTurnsFromMessages(messages: StreamMessage[]): ConversationTurn[] {
  const turns: ConversationTurn[] = [];
  for (const message of messages) {
    const body = message.body?.trim();
    if (!body || message.runStatus === "running") continue;
    if (message.role === "human") {
      turns.push({ role: "user", content: body.slice(0, MAX_TURN_CHARS) });
    } else if (message.role === "agent") {
      turns.push({ role: "assistant", content: body.slice(0, MAX_TURN_CHARS) });
    }
  }
  if (turns.length > MAX_CONVERSATION_TURNS) {
    return turns.slice(turns.length - MAX_CONVERSATION_TURNS);
  }
  return turns;
}
