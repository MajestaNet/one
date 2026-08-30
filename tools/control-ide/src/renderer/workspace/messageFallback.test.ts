import { describe, expect, it } from "vitest";
import { plainTextFromMessageContent } from "./messageFallback";

describe("plainTextFromMessageContent", () => {
  it("joins text parts and ignores non-text content (CIDE-16)", () => {
    expect(
      plainTextFromMessageContent([
        { type: "text", text: "Hello " },
        { type: "image", image: "https://evil.example/x.png" },
        { type: "text", text: "world" },
      ]),
    ).toBe("Hello world");
  });

  it("returns empty for missing or malformed content", () => {
    expect(plainTextFromMessageContent(undefined)).toBe("");
    expect(plainTextFromMessageContent(null)).toBe("");
    expect(plainTextFromMessageContent("raw")).toBe("");
    expect(plainTextFromMessageContent([{ type: "text" }])).toBe("");
  });
});
