import { afterEach, describe, expect, it, vi } from "vitest";
import { apiFetch } from "./api";

function jsonResponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => (typeof body === "string" ? body : JSON.stringify(body)),
  };
}

describe("apiFetch", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("sends Authorization bearer and returns JSON", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      text: async () => JSON.stringify({ id: "u1" }),
    });
    vi.stubGlobal("fetch", fetchMock);

    const body = await apiFetch("http://localhost:8080/", "tok", "/client/v1/me");
    expect(body).toEqual({ id: "u1" });
    expect(fetchMock).toHaveBeenCalledOnce();
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://localhost:8080/client/v1/me");
    expect(new Headers(init.headers).get("Authorization")).toBe("Bearer tok");
  });

  it("sends One-API-Revision when apiRevisionPin set", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      text: async () => "{}",
    });
    vi.stubGlobal("fetch", fetchMock);

    await apiFetch("https://example.com", "t", "/client/v1/me", {}, { apiRevisionPin: 12 });
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect(new Headers(init.headers).get("One-API-Revision")).toBe("12");
  });

  it("sets Content-Type for JSON body when missing", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      text: async () => "{}",
    });
    vi.stubGlobal("fetch", fetchMock);

    await apiFetch("https://example.com", "t", "/client/v1/query", {
      method: "POST",
      body: JSON.stringify({ object: "Account" }),
    });
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect(new Headers(init.headers).get("Content-Type")).toBe("application/json");
  });

  it("throws with status and path on non-OK, redacting JWT-shaped bodies", async () => {
    const jwt = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signaturepart";
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
        text: async () => JSON.stringify({ error: "unauthorized", access_token: jwt }),
      }),
    );

    let message = "";
    try {
      await apiFetch("https://example.com", "bad", "/client/v1/me");
    } catch (e) {
      message = String(e);
    }
    expect(message).toMatch(/401 \/client\/v1\/me/);
    expect(message).toMatch(/\[redacted/);
    expect(message).not.toContain(jwt);
  });

  it("refuses non-loopback http unless allowInsecureHttp is set", async () => {
    await expect(apiFetch("http://api.example", "t", "/client/v1/me")).rejects.toThrow(/https/);
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, text: async () => "{}" });
    vi.stubGlobal("fetch", fetchMock);
    await apiFetch("http://api.example", "t", "/client/v1/me", {}, { allowInsecureHttp: true });
    expect(fetchMock).toHaveBeenCalled();
  });

  it("sends X-One-Device-Id when deviceId is provided", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      text: async () => "{}",
    });
    vi.stubGlobal("fetch", fetchMock);

    await apiFetch("https://example.com", "t", "/client/v1/me", {}, { deviceId: "dev-1" });
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect(new Headers(init.headers).get("X-One-Device-Id")).toBe("dev-1");
  });

  it("returns raw text when body is not JSON", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        text: async () => "plain",
      }),
    );
    await expect(apiFetch("https://example.com", "t", "/health")).resolves.toBe("plain");
  });

  it("on 401 refreshes once then retries the original request", async () => {
    const onRotated = vi.fn().mockResolvedValue(undefined);
    const fetchMock = vi.fn().mockImplementation(async (url: string, init?: RequestInit) => {
      const path = String(url);
      if (path.includes("/auth/v1/token")) {
        return jsonResponse({
          access_token: "new-jwt",
          refresh_token: "new-rt",
          expires_in: 3600,
        });
      }
      const auth = new Headers(init?.headers).get("Authorization") ?? "";
      if (auth.includes("expired")) {
        return jsonResponse({ error: "expired" }, 401);
      }
      return jsonResponse({ id: "u1" });
    });
    vi.stubGlobal("fetch", fetchMock);

    const body = await apiFetch("http://localhost:8080", "expired", "/client/v1/me", {}, {
      refreshToken: "rt-old",
      onRotated,
    });
    expect(body).toEqual({ id: "u1" });
    expect(onRotated).toHaveBeenCalledWith({
      accessToken: "new-jwt",
      refreshToken: "new-rt",
      expiresIn: 3600,
    });
    const meCalls = fetchMock.mock.calls.filter((c) => String(c[0]).includes("/client/v1/me"));
    const tokenCalls = fetchMock.mock.calls.filter((c) => String(c[0]).includes("/auth/v1/token"));
    expect(meCalls).toHaveLength(2);
    expect(tokenCalls).toHaveLength(1);
    expect(new Headers((tokenCalls[0]![1] as RequestInit).headers).get("Content-Type")).toBe(
      "application/x-www-form-urlencoded",
    );
    expect(String((tokenCalls[0]![1] as RequestInit).body)).toContain("grant_type=refresh_token");
    expect(String((tokenCalls[0]![1] as RequestInit).body)).toContain("refresh_token=rt-old");
    expect(String((tokenCalls[0]![1] as RequestInit).body)).toContain(
      `client_id=${encodeURIComponent("one.controlIde")}`,
    );
    expect(new Headers((meCalls[1]![1] as RequestInit).headers).get("Authorization")).toBe(
      "Bearer new-jwt",
    );
  });

  it("shares one refresh POST across two parallel 401s", async () => {
    let release!: () => void;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    const fetchMock = vi.fn().mockImplementation(async (url: string, init?: RequestInit) => {
      const path = String(url);
      if (path.includes("/auth/v1/token")) {
        await gate;
        return jsonResponse({
          access_token: "new-jwt",
          refresh_token: "new-rt",
          expires_in: 3600,
        });
      }
      const auth = new Headers(init?.headers).get("Authorization") ?? "";
      if (auth.includes("expired")) {
        return jsonResponse({ error: "expired" }, 401);
      }
      return jsonResponse({ ok: true });
    });
    vi.stubGlobal("fetch", fetchMock);

    const a = apiFetch("http://localhost:8080", "expired", "/client/v1/me", {}, {
      refreshToken: "rt-shared",
    });
    const b = apiFetch("http://localhost:8080", "expired", "/client/v1/objects", {}, {
      refreshToken: "rt-shared",
    });
    await vi.waitFor(() => {
      const paths = fetchMock.mock.calls.map((c) => String(c[0]));
      expect(paths.some((p) => p.includes("/client/v1/me"))).toBe(true);
      expect(paths.some((p) => p.includes("/client/v1/objects"))).toBe(true);
    });
    await Promise.resolve();
    await Promise.resolve();
    await vi.waitFor(() => {
      expect(fetchMock.mock.calls.some((c) => String(c[0]).includes("/auth/v1/token"))).toBe(true);
    });
    await Promise.resolve();
    release();
    await Promise.all([a, b]);
    const tokenCalls = fetchMock.mock.calls.filter((c) => String(c[0]).includes("/auth/v1/token"));
    expect(tokenCalls).toHaveLength(1);
  });

  it("throws without looping when refresh fails", async () => {
    const fetchMock = vi.fn().mockImplementation(async (url: string) => {
      if (String(url).includes("/auth/v1/token")) {
        return jsonResponse({ error: "INVALID_GRANT" }, 401);
      }
      return jsonResponse({ error: "expired" }, 401);
    });
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      apiFetch("http://localhost:8080", "expired", "/client/v1/me", {}, { refreshToken: "rt-dead" }),
    ).rejects.toThrow(/401 \/auth\/v1\/token/);
    const tokenCalls = fetchMock.mock.calls.filter((c) => String(c[0]).includes("/auth/v1/token"));
    const meCalls = fetchMock.mock.calls.filter((c) => String(c[0]).includes("/client/v1/me"));
    expect(tokenCalls).toHaveLength(1);
    expect(meCalls).toHaveLength(1);
  });

  it("does not refresh when skipRefresh is set", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ error: "expired" }, 401));
    vi.stubGlobal("fetch", fetchMock);
    await expect(
      apiFetch("http://localhost:8080", "expired", "/client/v1/me", {}, {
        refreshToken: "rt-old",
        skipRefresh: true,
      }),
    ).rejects.toThrow(/401 \/client\/v1\/me/);
    expect(fetchMock.mock.calls.some((c) => String(c[0]).includes("/auth/v1/token"))).toBe(false);
  });
});
