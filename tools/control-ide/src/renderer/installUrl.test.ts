import { describe, expect, it } from "vitest";
import { assertApiBaseUrl, baseUrlOrigin, checkInstallBaseUrl, isLoopbackHost } from "./installUrl";

describe("isLoopbackHost", () => {
  it("recognises common loopback names", () => {
    expect(isLoopbackHost("localhost")).toBe(true);
    expect(isLoopbackHost("127.0.0.1")).toBe(true);
    expect(isLoopbackHost("[::1]")).toBe(true);
    expect(isLoopbackHost("api.example")).toBe(false);
  });
});

describe("assertApiBaseUrl", () => {
  it("accepts https and loopback http", () => {
    expect(assertApiBaseUrl("https://one.example/")).toBe("https://one.example");
    expect(assertApiBaseUrl("http://localhost:8080/")).toBe("http://localhost:8080");
  });

  it("rejects embedded credentials, query strings, and non-http schemes", () => {
    expect(() => assertApiBaseUrl("https://user:pass@one.example")).toThrow(/credentials/);
    expect(() => assertApiBaseUrl("https://one.example/?x=1")).toThrow(/query/);
    expect(() => assertApiBaseUrl("file:///etc/passwd")).toThrow(/http/);
  });

  it("rejects non-loopback http unless explicitly acknowledged", () => {
    expect(() => assertApiBaseUrl("http://api.example")).toThrow(/https/);
    expect(assertApiBaseUrl("http://api.example", { allowInsecureHttp: true })).toBe("http://api.example");
  });
});

describe("checkInstallBaseUrl", () => {
  it("asks for acknowledgement before allowing plaintext remote HTTP", () => {
    const blocked = checkInstallBaseUrl("http://api.example");
    expect(blocked.ok).toBe(false);
    if (!blocked.ok) {
      expect(blocked.needsInsecureAck).toBe(true);
      expect(blocked.error).toMatch(/plaintext HTTP/);
    }

    const allowed = checkInstallBaseUrl("http://api.example", { allowInsecureHttp: true });
    expect(allowed).toEqual({ ok: true, url: "http://api.example", insecure: true });
  });

  it("accepts loopback http without acknowledgement", () => {
    expect(checkInstallBaseUrl("http://localhost:8080")).toEqual({
      ok: true,
      url: "http://localhost:8080",
      insecure: false,
    });
  });
});

describe("baseUrlOrigin", () => {
  it("returns the origin or empty string", () => {
    expect(baseUrlOrigin("https://one.example:8443/v1")).toBe("https://one.example:8443");
    expect(baseUrlOrigin("not a url")).toBe("");
  });
});
