import { describe, expect, it } from "vitest";
import { envKindFromLabels } from "./envKind";

describe("envKindFromLabels", () => {
  it("maps installRole conventions without a new API", () => {
    expect(envKindFromLabels("demo", "")).toBe("demo");
    expect(envKindFromLabels("mock-oidc", "lab")).toBe("mock");
    expect(envKindFromLabels("production", "")).toBe("prod");
    expect(envKindFromLabels("staging", "west")).toBeNull();
  });
});
