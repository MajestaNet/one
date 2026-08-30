import { beforeAll, describe, expect, it } from "vitest";
import { apiFetch } from "./api";

/**
 * Live contract tests against a running Majesta One install.
 *
 * Env:
 *   ONE_API_URL  — default http://localhost:8080
 *   ONE_JWT      — Bearer token (preferred)
 *   ONE_API_KEY  — bootstrap API key used to mint JWT via /auth/v1/token
 *                      (e.g. .env API_KEYS entry like dev-admin-key)
 *
 * Skips when neither ONE_JWT nor ONE_API_KEY is set.
 */
const baseUrl = (process.env.ONE_API_URL ?? "http://localhost:8080").replace(/\/$/, "");

async function mintJwtFromApiKey(apiKey: string): Promise<string> {
  const res = await fetch(`${baseUrl}/auth/v1/token`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      grant_type: "client_credentials",
      client_secret: apiKey,
    }),
  });
  const json = (await res.json()) as { access_token?: string; error?: string };
  if (!res.ok || !json.access_token) {
    throw new Error(`token mint failed: ${res.status} ${JSON.stringify(json)}`);
  }
  return json.access_token;
}

describe("live API contracts (Control IDE client)", () => {
  let token = "";
  let available = false;

  beforeAll(async () => {
    if (process.env.ONE_JWT) {
      token = process.env.ONE_JWT;
      available = true;
      return;
    }
    const apiKey = process.env.ONE_API_KEY;
    if (!apiKey) {
      return;
    }
    try {
      const health = await fetch(`${baseUrl}/healthz`);
      if (!health.ok) return;
      token = await mintJwtFromApiKey(apiKey);
      available = true;
    } catch {
      available = false;
    }
  });

  it("skips cleanly when no live credentials", () => {
    if (!available) {
      console.info(
        "skip live API contracts: set ONE_JWT or ONE_API_KEY (and run make api)",
      );
    }
    expect(true).toBe(true);
  });

  it("GET /version advertises apiRevision", async ({ skip }) => {
    if (!available) skip();
    const res = await fetch(`${baseUrl}/version`);
    const body = (await res.json()) as Record<string, unknown>;
    expect(body).toHaveProperty("apiRevision");
    expect(body).toHaveProperty("httpApi");
  });

  it("GET /client/v1/me with Bearer JWT", async ({ skip }) => {
    if (!available) skip();
    const me = (await apiFetch(baseUrl, token, "/client/v1/me", {}, { apiRevisionPin: 1 })) as Record<
      string,
      unknown
    >;
    expect(me).toBeTruthy();
    expect(typeof me).toBe("object");
    expect(me).toHaveProperty("apiRevision");
  });

  it("GET /deploy/v1/environment", async ({ skip }) => {
    if (!available) skip();
    const env = (await apiFetch(baseUrl, token, "/deploy/v1/environment")) as Record<
      string,
      unknown
    >;
    expect(env).toBeTruthy();
    expect(env).toHaveProperty("installId");
  });

  it("GET /client/v1/describe/Account", async ({ skip }) => {
    if (!available) skip();
    const desc = (await apiFetch(baseUrl, token, "/client/v1/describe/Account")) as Record<
      string,
      unknown
    >;
    expect(desc).toBeTruthy();
  });

  it("POST /client/v1/query Account", async ({ skip }) => {
    if (!available) skip();
    const rows = await apiFetch(baseUrl, token, "/client/v1/query", {
      method: "POST",
      body: JSON.stringify({ object: "Account", limit: 1 }),
    });
    expect(rows).toBeTruthy();
  });
});
