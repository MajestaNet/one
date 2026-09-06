import { afterEach, describe, expect, it } from "vitest";
import {
  handleLoopbackRequest,
  LOOPBACK_OAUTH_PATH,
  LOOPBACK_OAUTH_PORT,
  LOOPBACK_REDIRECT_URI,
  startOAuthLoopback,
} from "./oauthLoopback";

describe("handleLoopbackRequest", () => {
  const base = {
    method: "GET",
    host: `127.0.0.1:${LOOPBACK_OAUTH_PORT}`,
    port: LOOPBACK_OAUTH_PORT,
  };

  it("forwards a complete callback and returns the success page", () => {
    const out = handleLoopbackRequest({
      ...base,
      url: `${LOOPBACK_OAUTH_PATH}?code=abc&state=xyz`,
    });
    expect(out.status).toBe(200);
    expect(out.callbackUrl).toBe("http://127.0.0.1:5173/oauth/callback?code=abc&state=xyz");
    expect(out.body).toMatch(/return to Control IDE/i);
  });

  it("rejects missing code or state without broadcasting", () => {
    expect(handleLoopbackRequest({ ...base, url: `${LOOPBACK_OAUTH_PATH}?code=abc` }).callbackUrl).toBeUndefined();
    expect(handleLoopbackRequest({ ...base, url: `${LOOPBACK_OAUTH_PATH}?state=xyz` }).status).toBe(400);
  });

  it("rejects a spoofed Host header and unknown paths", () => {
    expect(
      handleLoopbackRequest({
        ...base,
        host: "evil.example:5173",
        url: `${LOOPBACK_OAUTH_PATH}?code=a&state=b`,
      }).status,
    ).toBe(404);
    expect(handleLoopbackRequest({ ...base, url: "/secret?code=a&state=b" }).status).toBe(404);
  });

  it("rejects non-GET methods", () => {
    expect(handleLoopbackRequest({ ...base, method: "POST", url: LOOPBACK_OAUTH_PATH }).status).toBe(405);
  });
});

describe("startOAuthLoopback", () => {
  afterEach(async () => {
    /* closed in each test */
  });

  it("binds loopback and delivers the callback URL", async () => {
    const received: string[] = [];
    const server = await startOAuthLoopback((url) => received.push(url), 0);
    try {
      const addr = new URL(server.redirectUri);
      expect(addr.hostname).toBe("127.0.0.1");
      const res = await fetch(`${server.redirectUri}?code=tok&state=st`);
      expect(res.status).toBe(200);
      expect(received).toEqual([`${server.redirectUri}?code=tok&state=st`]);
    } finally {
      await server.close();
    }
  });
});

describe("LOOPBACK_REDIRECT_URI", () => {
  it("matches the seeded Control IDE callback", () => {
    expect(LOOPBACK_REDIRECT_URI).toBe("http://127.0.0.1:5173/oauth/callback");
  });
});
