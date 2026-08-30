import { describe, expect, it } from "vitest";
import {
  streamMessageFromConversationRow,
  conversationTurnsFromMessages,
  type ConversationMessageRow,
} from "./conversations";

describe("conversations", () => {
  it("streamMessageFromConversationRow maps excerpts and roles", () => {
    const row: ConversationMessageRow = {
      id: "m1",
      role: "human",
      body: "Summarize",
      parts: [
        {
          type: "contextExcerpts",
          excerpts: [{ id: "e1", label: "rows", text: "a|b", mime: "x", source: "tool_rows" }],
        },
      ],
      createdAt: "2026-01-01T00:00:00Z",
    };
    const msg = streamMessageFromConversationRow(row);
    expect(msg.role).toBe("human");
    expect(msg.body).toBe("Summarize");
    expect(msg.contextExcerpts?.length).toBe(1);
  });

  it("maps user role to human", () => {
    const msg = streamMessageFromConversationRow({ id: "m2", role: "user", body: "hi" });
    expect(msg.role).toBe("human");
  });

  it("conversationTurnsFromMessages keeps prior user/assistant turns", () => {
    const turns = conversationTurnsFromMessages([
      { id: "s", role: "system", body: "Choose an agent" },
      { id: "h1", role: "human", body: "Find Acme" },
      { id: "a1", role: "agent", body: "Acme is an Account.", runStatus: "completed" },
      { id: "run", role: "agent", body: "partial", runStatus: "running" },
    ]);
    expect(turns).toEqual([
      { role: "user", content: "Find Acme" },
      { role: "assistant", content: "Acme is an Account." },
    ]);
  });
});
