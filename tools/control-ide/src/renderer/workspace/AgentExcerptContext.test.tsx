import { describe, expect, it } from "vitest";
import { renderHook } from "@testing-library/react";
import { AgentExcerptProvider, useAgentExcerptBridge } from "./AgentExcerptContext";

describe("AgentExcerptContext", () => {
  it("provides excerpt bridge to children", () => {
    const bridge = { addExcerptToOpenChat: () => {} };
    const { result } = renderHook(() => useAgentExcerptBridge(), {
      wrapper: ({ children }) => <AgentExcerptProvider bridge={bridge}>{children}</AgentExcerptProvider>,
    });
    expect(result.current?.addExcerptToOpenChat).toBe(bridge.addExcerptToOpenChat);
  });
});
