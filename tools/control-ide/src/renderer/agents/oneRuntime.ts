/**
 * Majesta One ↔ assistant-ui bridge: map StreamMessage transcript to ExternalStoreRuntime.
 * Agent execution stays on Go `/client/v1/agents/runs` (ADR-010) — this is UI-only.
 */
import { useCallback, useRef } from "react";
import {
  useExternalStoreRuntime,
  type AppendMessage,
  type ThreadMessageLike,
} from "@assistant-ui/react";
import type { StreamMessage, StreamRole } from "../workspace/types";

export function streamRoleToAui(role: StreamRole): ThreadMessageLike["role"] {
  if (role === "human") return "user";
  if (role === "system") return "system";
  return "assistant";
}

export function convertStreamMessage(message: StreamMessage): ThreadMessageLike {
  return {
    role: streamRoleToAui(message.role),
    id: message.id,
    createdAt: message.createdAt ? new Date(message.createdAt) : undefined,
    content: [{ type: "text", text: message.body }],
    metadata: {
      custom: { one: message },
    },
  };
}

export function extractAppendText(message: AppendMessage): string {
  const content = message.content as readonly { type: string; text?: string }[];
  return content
    .filter((part) => part.type === "text" && typeof part.text === "string")
    .map((part) => part.text ?? "")
    .join("")
    .trim();
}

export function useOneAgentRuntime(opts: {
  messages: StreamMessage[];
  isRunning?: boolean;
  isDisabled?: boolean;
  isSendDisabled?: boolean;
  onSend: (text: string) => void;
}) {
  const onSendRef = useRef(opts.onSend);
  onSendRef.current = opts.onSend;

  const onNew = useCallback(async (message: AppendMessage) => {
    const text = extractAppendText(message);
    if (!text) return;
    onSendRef.current(text);
  }, []);

  return useExternalStoreRuntime({
    messages: opts.messages,
    isRunning: Boolean(opts.isRunning),
    isDisabled: Boolean(opts.isDisabled),
    isSendDisabled: Boolean(opts.isSendDisabled),
    convertMessage: convertStreamMessage,
    onNew,
  });
}
