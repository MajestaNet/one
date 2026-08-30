import { createContext, useContext } from "react";
import type { ContextExcerpt } from "./contextExcerpt";

export type AgentExcerptBridge = {
  addExcerptToOpenChat: (excerpt: ContextExcerpt) => void;
};

const AgentExcerptContext = createContext<AgentExcerptBridge | null>(null);

export function AgentExcerptProvider({
  bridge,
  children,
}: {
  bridge: AgentExcerptBridge;
  children: React.ReactNode;
}) {
  return <AgentExcerptContext.Provider value={bridge}>{children}</AgentExcerptContext.Provider>;
}

export function useAgentExcerptBridge(): AgentExcerptBridge | null {
  return useContext(AgentExcerptContext);
}
