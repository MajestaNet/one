import { afterEach, describe, expect, it, vi } from "vitest";
import {
  accessTokenNearingExpiry,
  refreshAccessToken,
  revokeRefreshToken,
} from "./refreshSession";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("accessTokenNearingExpiry", () => {
  it("is true when expiry is missing or within 60s", () => {
    expect(accessTokenNearingExpiry(undefined, 1_000_000)).toBe(true);
    expect(accessTokenNearingExpiry(1_000_000 + 30_000, 1_000_000)).toBe(true);
    expect(accessTokenNearingExpiry(1_000_000 + 60_000, 1_000_000)).toBe(true);
    expect(accessTokenNearingExpiry(1_000_000 + 60_001, 1_000_000)).toBe(false);
  });
});

describe("refreshAccessToken", () => {
  it("posts form-encoded refresh_token grant", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      text: async () =>
        JSON.stringify({
          access_token: "jwt",
          refresh_token: "rt-new",
          expires_in: 3600,
          refresh_expires_in: 2592000,
        }),
    });
    vi.stubGlobal("fetch", fetchMock);
    const out = await refreshAccessToken("http://localhost:8080/", "rt-old");
    expect(out).toEqual({
      accessToken: "jwt",
      refreshToken: "rt-new",
      expiresIn: 3600,
      refreshExpiresIn: 2592000,
    });
    expect(fetchMock.mock.calls[0]![0]).toBe("http://localhost:8080/auth/v1/token");
    expect(String((fetchMock.mock.calls[0]![1] as RequestInit).body)).toContain(
      "grant_type=refresh_token",
    );
  });
});

describe("revokeRefreshToken", () => {
  it("posts token_type_hint refresh_token and swallows network errors", async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200, text: async () => "" });
    vi.stubGlobal("fetch", fetchMock);
    await revokeRefreshToken("http://localhost:8080", "rt-old");
    expect(fetchMock.mock.calls[0]![0]).toBe("http://localhost:8080/auth/v1/revoke");
    expect(JSON.parse(String((fetchMock.mock.calls[0]![1] as RequestInit).body))).toEqual({
      token: "rt-old",
      token_type_hint: "refresh_token",
    });

    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("offline")));
    await expect(revokeRefreshToken("http://localhost:8080", "rt-old")).resolves.toBeUndefined();
  });
});
