import { afterEach, describe, expect, it, vi } from "vitest";
import {
  CONTROL_IDE_INTEGRATION,
  DEFAULT_REDIRECT_URI,
  OFFLINE_ACCESS_SCOPE,
  buildOneAuthorizeUrl,
  buildOneLoginUrl,
  clearPendingPkce,
  createPkcePair,
  exchangeOneAuthorizationCode,
  exchangeOneIdToken,
  parseOAuthCallbackUrl,
  statesMatch,
  storePendingPkce,
  takePendingPkce,
} from "./oauthPkce";

function stubSessionStorage() {
  const store: Record<string, string> = {};
  vi.stubGlobal("sessionStorage", {
    getItem: (k: string) => store[k] ?? null,
    setItem: (k: string, v: string) => {
      store[k] = v;
    },
    removeItem: (k: string) => {
      delete store[k];
    },
  });
  return store;
}

describe("oauthPkce", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("creates verifier/challenge pair", async () => {
    const a = await createPkcePair();
    expect(a.verifier.length).toBeGreaterThan(20);
    expect(a.challenge.length).toBeGreaterThan(20);
    expect(a.verifier).not.toEqual(a.challenge);
  });

  it("builds Majesta One login page URL", async () => {
    const { challenge } = await createPkcePair();
    const url = buildOneLoginUrl({
      baseUrl: "http://localhost:8080/",
      clientId: CONTROL_IDE_INTEGRATION,
      redirectUri: DEFAULT_REDIRECT_URI,
      codeChallenge: challenge,
      state: "abc",
    });
    expect(url).toContain("/auth/v1/login?");
    expect(url).not.toContain("provider=");
    expect(url).toContain("code_challenge_method=S256");
    expect(url).toContain(`client_id=${encodeURIComponent(CONTROL_IDE_INTEGRATION)}`);
    expect(url).toContain(`scope=${encodeURIComponent(OFFLINE_ACCESS_SCOPE)}`);
  });

  it("parses one-control callback URLs", () => {
    const parsed = parseOAuthCallbackUrl("one-control://oauth/callback?code=abc&state=xyz");
    expect(parsed).toEqual({ code: "abc", state: "xyz" });
  });

  it("rejects callbacks that omit state", () => {
    expect(parseOAuthCallbackUrl("one-control://oauth/callback?code=abc")).toBeNull();
    expect(parseOAuthCallbackUrl("one-control://oauth/callback?state=xyz")).toBeNull();
  });

  it("compares OAuth states", () => {
    expect(statesMatch("abc", "abc")).toBe(true);
    expect(statesMatch("abc", "abd")).toBe(false);
    expect(statesMatch(undefined, "abc")).toBe(false);
    expect(statesMatch("abc", "")).toBe(false);
  });

  it("takes a pending PKCE flow exactly once", () => {
    stubSessionStorage();
    storePendingPkce({
      verifier: "v",
      state: "s",
      baseUrl: "http://localhost:8080",
      clientId: CONTROL_IDE_INTEGRATION,
      redirectUri: DEFAULT_REDIRECT_URI,
    });
    expect(takePendingPkce()?.state).toBe("s");
    expect(takePendingPkce()).toBeNull();
    clearPendingPkce();
  });

  it("builds Majesta One authorize URL for Google", async () => {
    const { challenge } = await createPkcePair();
    const url = buildOneAuthorizeUrl({
      baseUrl: "http://localhost:8080/",
      provider: "google",
      clientId: CONTROL_IDE_INTEGRATION,
      redirectUri: DEFAULT_REDIRECT_URI,
      codeChallenge: challenge,
      state: "abc",
    });
    expect(url).toContain("/auth/v1/authorize?");
    expect(url).toContain("provider=google");
    expect(url).toContain("code_challenge_method=S256");
    expect(url).toContain(`client_id=${encodeURIComponent(CONTROL_IDE_INTEGRATION)}`);
    expect(url).toContain(`scope=${encodeURIComponent(OFFLINE_ACCESS_SCOPE)}`);
    expect(CONTROL_IDE_INTEGRATION).toBe("one.controlIde");
  });

  it("exchanges Majesta One authorization code for JWT", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        access_token: "one.jwt",
        expires_in: 3600,
        refresh_token: "rt-1",
        refresh_expires_in: 2592000,
      }),
    });
    vi.stubGlobal("fetch", fetchMock);
    const out = await exchangeOneAuthorizationCode({
      baseUrl: "http://localhost:8080/",
      clientId: CONTROL_IDE_INTEGRATION,
      redirectUri: DEFAULT_REDIRECT_URI,
      code: "auth-code",
      codeVerifier: "verifier",
    });
    expect(out).toEqual({
      access_token: "one.jwt",
      expires_in: 3600,
      refresh_token: "rt-1",
      refresh_expires_in: 2592000,
    });
    expect(fetchMock.mock.calls[0][0]).toBe("http://localhost:8080/auth/v1/token");
    expect(String((fetchMock.mock.calls[0]![1] as RequestInit).body)).toContain(
      `client_id=${encodeURIComponent(CONTROL_IDE_INTEGRATION)}`,
    );
    expect(String((fetchMock.mock.calls[0]![1] as RequestInit).body)).toContain(
      `scope=${encodeURIComponent(OFFLINE_ACCESS_SCOPE)}`,
    );
  });

  it("throws when authorization_code exchange fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        json: async () => ({ error: "invalid_grant" }),
      }),
    );
    await expect(
      exchangeOneAuthorizationCode({
        baseUrl: "http://localhost:8080",
        clientId: CONTROL_IDE_INTEGRATION,
        redirectUri: DEFAULT_REDIRECT_URI,
        code: "bad",
        codeVerifier: "v",
      }),
    ).rejects.toThrow(/invalid_grant/);
  });

  it("exchanges IdP ID token for Majesta One JWT", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        access_token: "one.jwt.here",
        expires_in: 3600,
        refresh_token: "rt-ex",
        refresh_expires_in: 2592000,
      }),
    });
    vi.stubGlobal("fetch", fetchMock);
    const out = await exchangeOneIdToken("http://localhost:8080/", "idp.id.token");
    expect(out.access_token).toBe("one.jwt.here");
    expect(out.refresh_token).toBe("rt-ex");
    expect(out.refresh_expires_in).toBe(2592000);
    expect(fetchMock).toHaveBeenCalled();
  });

  it("throws when Majesta One token exchange fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        json: async () => ({ error: "INVALID_TOKEN" }),
      }),
    );
    await expect(exchangeOneIdToken("http://localhost:8080", "bad")).rejects.toThrow(
      /INVALID_TOKEN/,
    );
  });
});
