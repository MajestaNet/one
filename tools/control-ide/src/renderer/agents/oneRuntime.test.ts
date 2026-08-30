import { describe, expect, it } from "vitest";
import {
  convertStreamMessage,
  extractAppendText,
  streamRoleToAui,
} from "./oneRuntime";
import type { StreamMessage } from "../workspace/types";
import type { AppendMessage } from "@assistant-ui/react";

describe("oneRuntime", () => {
  it("maps Majesta One roles to assistant-ui roles", () => {
    expect(streamRoleToAui("human")).toBe("user");
    expect(streamRoleToAui("system")).toBe("system");
    expect(streamRoleToAui("agent")).toBe("assistant");
    expect(streamRoleToAui("tool")).toBe("assistant");
    expect(streamRoleToAui("approval")).toBe("assistant");
  });

  it("converts StreamMessage with one metadata", () => {
    const msg: StreamMessage = {
      id: "m1",
      role: "agent",
      body: "Hello",
      agentLabel: "QueryAssistant",
    };
    const like = convertStreamMessage(msg);
    expect(like.role).toBe("assistant");
    expect(like.id).toBe("m1");
    expect(like.content).toEqual([{ type: "text", text: "Hello" }]);
    expect(like.metadata?.custom).toEqual({ one: msg });
  });

  it("extracts text from AppendMessage parts", () => {
    const append = {
      role: "user",
      content: [
        { type: "text", text: "Rank " },
        { type: "text", text: "accounts" },
      ],
    } as AppendMessage;
    expect(extractAppendText(append)).toBe("Rank accounts");
  });
});
