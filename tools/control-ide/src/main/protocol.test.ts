import { describe, expect, it } from "vitest";
import { extractProtocolUrl, isAppProtocolUrl } from "./protocol";

describe("extractProtocolUrl", () => {
  it("returns the first one-control deep link from argv", () => {
    expect(
      extractProtocolUrl(
        ["/path/to/electron", "one-control://oauth?code=1&state=s", "--flag"],
        "one-control",
      ),
    ).toBe("one-control://oauth?code=1&state=s");
  });

  it("ignores unrelated argv entries", () => {
    expect(extractProtocolUrl(["electron", "https://example.com", "--foo"], "one-control")).toBeUndefined();
    expect(extractProtocolUrl([], "one-control")).toBeUndefined();
  });
});

describe("isAppProtocolUrl", () => {
  it("matches only the configured scheme", () => {
    expect(isAppProtocolUrl("one-control://x", "one-control")).toBe(true);
    expect(isAppProtocolUrl("https://example.com", "one-control")).toBe(false);
    expect(isAppProtocolUrl("", "one-control")).toBe(false);
  });
});
