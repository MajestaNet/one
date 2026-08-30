import { test } from "node:test";
import assert from "node:assert/strict";
import {
  CLIENT_PREFERRED_API_REVISION,
  OneAPIError,
  PREFERRED_API_REVISION,
  createOneClient,
  probeVersion,
  type FetchLike,
} from "./index.js";

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

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function clientWithFetch(fetchImpl: FetchLike) {
  return createOneClient({
    baseUrl: "http://install.example",
    getAccessToken: () => "tok",
    fetch: fetchImpl,
  });
}

test("PREFERRED_API_REVISION is 1", () => {
  assert.equal(PREFERRED_API_REVISION, 1);
  assert.equal(CLIENT_PREFERRED_API_REVISION, 1);
});

test("query posts { object } not objectApiName and pins One-API-Revision 1", async () => {
  let url = "";
  let init: RequestInit | undefined;
  const client = clientWithFetch(async (input, requestInit) => {
    url = String(input);
    init = requestInit;
    return jsonResponse(200, { records: [], totalSize: 0, done: true });
  });

  await client.query({ object: "Account", select: ["Name"], limit: 25 });

  assert.match(url, /\/client\/v1\/query$/);
  assert.equal(init?.method, "POST");
  assert.equal(header(init, "One-API-Revision"), "1");
  assert.equal(header(init, "Authorization"), "Bearer tok");
  const body = JSON.parse(String(init?.body)) as Record<string, unknown>;
  assert.equal(body.object, "Account");
  assert.deepEqual(body.select, ["Name"]);
  assert.equal(body.limit, 25);
  assert.equal("objectApiName" in body, false);
  assert.equal("fields" in body, false);
  assert.equal("filter" in body, false);
  assert.equal("offset" in body, false);
});

test("getRecord uses /sobjects/{object}/{id}", async () => {
  let url = "";
  const client = clientWithFetch(async (input) => {
    url = String(input);
    return jsonResponse(200, { Id: "001", Name: "Acme" });
  });

  const rec = await client.getRecord("Account", "001abc");
  assert.match(url, /\/client\/v1\/sobjects\/Account\/001abc$/);
  assert.equal(url.includes("/records/"), false);
  assert.equal(rec.Id, "001");
});

test("create / update / delete hit /sobjects and delete accepts 204", async () => {
  const calls: { url: string; method?: string }[] = [];
  const client = clientWithFetch(async (input, init) => {
    const url = String(input);
    calls.push({ url, method: init?.method });
    if (init?.method === "DELETE") return new Response(null, { status: 204 });
    if (init?.method === "POST") return jsonResponse(201, { Id: "n1", Name: "New" });
    return jsonResponse(200, { Id: "n1", Name: "Patched" });
  });

  await client.createRecord("Account", { Name: "New" });
  await client.updateRecord("Account", "n1", { Name: "Patched" });
  await client.deleteRecord("Account", "n1");

  assert.equal(calls[0]?.method, "POST");
  assert.match(calls[0]!.url, /\/sobjects\/Account$/);
  assert.equal(calls[1]?.method, "PATCH");
  assert.match(calls[1]!.url, /\/sobjects\/Account\/n1$/);
  assert.equal(calls[2]?.method, "DELETE");
  assert.match(calls[2]!.url, /\/sobjects\/Account\/n1$/);
});

test("describe and describeObject use GET /describe", async () => {
  const urls: string[] = [];
  const client = clientWithFetch(async (input) => {
    urls.push(String(input));
    return jsonResponse(200, { objects: [] });
  });
  await client.describe();
  await client.describeObject("Account");
  assert.match(urls[0]!, /\/client\/v1\/describe$/);
  assert.match(urls[1]!, /\/client\/v1\/describe\/Account$/);
});

test("400 API_REVISION_UNSUPPORTED throws OneAPIError with code and cta", async () => {
  const client = clientWithFetch(async () =>
    jsonResponse(400, {
      error: "API_REVISION_UNSUPPORTED",
      message: "API revision is outside the supported window for this install",
      cta: "Upgrade the install product image (/ops/v1) or lower the client pin",
      pin: 99,
      min: 1,
      current: 1,
    }),
  );

  await assert.rejects(
    () => client.query({ object: "Account" }),
    (err: unknown) => {
      assert.ok(err instanceof OneAPIError);
      assert.equal(err.status, 400);
      assert.equal(err.code, "API_REVISION_UNSUPPORTED");
      assert.match(err.message, /outside the supported window/);
      assert.match(String(err.cta), /Upgrade the install/);
      return true;
    },
  );
});

test("probeVersion GETs /version without One-API-Revision", async () => {
  let url = "";
  let init: RequestInit | undefined;
  const version = await probeVersion("http://install.example", {
    fetch: async (input, requestInit) => {
      url = String(input);
      init = requestInit;
      return jsonResponse(200, {
        productVersion: "0.14.0",
        apiRevision: { min: 1, current: 1, recommended: 1 },
      });
    },
  });
  assert.match(url, /\/version$/);
  assert.equal(url.includes("/client/"), false);
  assert.equal(header(init, "One-API-Revision"), undefined);
  assert.equal(version.apiRevision?.current, 1);
});
