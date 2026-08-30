import { describe, expect, it } from "vitest";
import { formatError, redactSensitive } from "./errors";

describe("redactSensitive", () => {
  it("strips JWT-shaped and bearer material", () => {
    const jwt = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signaturepart";
    expect(redactSensitive(`token=${jwt}`)).not.toContain(jwt);
    expect(redactSensitive(`token=${jwt}`)).toMatch(/\[redacted/);
    expect(redactSensitive(`Authorization: Bearer ${jwt}`)).not.toContain(jwt);
    expect(redactSensitive(`Authorization: Bearer ${jwt}`)).toMatch(/Bearer \[redacted/i);
    expect(redactSensitive(`{"access_token":"${jwt}"}`)).not.toContain(jwt);
    expect(redactSensitive(`{"access_token":"${jwt}"}`)).toMatch(/\[redacted/);
    expect(redactSensitive(`{"refresh_token":"rt-secret-value"}`)).not.toContain("rt-secret-value");
    expect(redactSensitive(`{"refresh_token":"rt-secret-value"}`)).toMatch(/\[redacted/);
  });

  it("truncates long bodies", () => {
    const out = redactSensitive("x".repeat(1000));
    expect(out.length).toBeLessThan(500);
    expect(out).toMatch(/truncated/);
  });
});

describe("formatError", () => {
  it("formats Errors and plain values through the redactor", () => {
    const jwt = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig";
    const out = formatError(new Error(`Bearer ${jwt}`));
    expect(out).not.toContain(jwt);
    expect(out).toMatch(/Bearer \[redacted/i);
    expect(formatError("plain")).toBe("plain");
  });
});
