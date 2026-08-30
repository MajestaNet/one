import { test } from "node:test";
import assert from "node:assert/strict";
import {
  PREFERRED_API_REVISION,
  buildAuthorizeUrl,
  exchangeAuthorizationCode,
  type OneAuthConfig,
} from "./index.js";

const base: OneAuthConfig = {
  baseUrl: "http://install.example",
  clientId: "exp.listView",
  redirectUri: "http://127.0.0.1:5174/oauth/callback",
};

function header(init: RequestInit | undefined, name: string): string | undefined {
  const h = init?.headers;
  if (!h || h instanceof Headers || Array.isArray(h)) {
    if (h instanceof Headers) return h.get(name) ?? undefined;
    return undefined;
  }
  const rec = h as Record<string, string>;
  const want = name.toLowerCase();
  for (const [k, v] of Object.entries(rec)) {
    if (k.toLowerCase() === want) return v;
  }
  return undefined;
}

test("PREFERRED_API_REVISION is 1", () => {
  assert.equal(PREFERRED_API_REVISION, 1);
});

test("buildAuthorizeUrl defaults to scope=client and S256", () => {
  const url = new URL(buildAuthorizeUrl(base, "st", "challenge"));
  assert.equal(url.pathname, "/auth/v1/authorize");
  assert.equal(url.searchParams.get("response_type"), "code");
  assert.equal(url.searchParams.get("client_id"), "exp.listView");
  assert.equal(url.searchParams.get("scope"), "client");
  assert.equal(url.searchParams.get("code_challenge"), "challenge");
  assert.equal(url.searchParams.get("code_challenge_method"), "S256");
  assert.equal(url.searchParams.get("state"), "st");
});

test("buildAuthorizeUrl throws when scopes include metadata", () => {
  assert.throws(
    () => buildAuthorizeUrl({ ...base, scopes: ["client", "metadata"] }, "st", "ch"),
    /metadata/,
  );
});

test("buildAuthorizeUrl throws for deploy, ops, and admin", () => {
  for (const scope of ["deploy", "ops", "admin"]) {
    assert.throws(
      () => buildAuthorizeUrl({ ...base, scopes: ["client", scope] }, "st", "ch"),
      new RegExp(scope),
    );
  }
});

test("offline_access is allowed on authorize (refresh is R2)", () => {
  const url = new URL(
    buildAuthorizeUrl({ ...base, scopes: ["client", "offline_access"] }, "st", "ch"),
  );
  assert.equal(url.searchParams.get("scope"), "client offline_access");
});

test("token POST sets One-API-Revision 1", async () => {
  let url = "";
  let init: RequestInit | undefined;
  const tok = await exchangeAuthorizationCode(
    {
      ...base,
      fetch: async (input, requestInit) => {
        url = String(input);
        init = requestInit;
        return new Response(JSON.stringify({ access_token: "jwt" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      },
    },
    "code-1",
    "verifier-1",
  );
  assert.match(url, /\/auth\/v1\/token$/);
  assert.equal(init?.method, "POST");
  assert.equal(header(init, "One-API-Revision"), "1");
  assert.equal(header(init, "Content-Type"), "application/x-www-form-urlencoded");
  assert.equal(tok.access_token, "jwt");
  const body = String(init?.body);
  assert.match(body, /grant_type=authorization_code/);
  assert.match(body, /code=code-1/);
  assert.match(body, /code_verifier=verifier-1/);
});
