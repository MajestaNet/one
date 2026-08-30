import { describe, expect, it } from "vitest";
import { maskToken } from "./tokenDisplay";

describe("maskToken", () => {
  it("masks short and long secrets without echoing the full value", () => {
    expect(maskToken("")).toBe("");
    expect(maskToken("short")).toBe("•••••");
    expect(maskToken("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig")).toMatch(
      /^eyJhbG…\.sig \(\d+ chars\)$/,
    );
    expect(maskToken("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig")).not.toContain("payload");
  });
});
