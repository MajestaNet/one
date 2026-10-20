// Majesta One automation guest bootstrap (ADR-014 / ADR-034).
// Host speaks NDJSON on stdin/stdout. The Node permission model denies net, fs
// write, child processes, workers, and addons. Only one:automation is resolvable
// from customer code.

import * as userMod from "./user_entry.ts";
import { emit, finish, loadTrigger, rpc } from "./guest_io.ts";

const trigger = loadTrigger();

const ctx = {
  trigger,
  createRecord: (args: unknown) => rpc("createRecord", args),
  updateRecord: (args: unknown) => rpc("updateRecord", args),
  getRecord: (args: unknown) => rpc("getRecord", args),
  deleteRecord: (args: unknown) => rpc("deleteRecord", args),
  query: (args: unknown) => rpc("query", args),
  http: (args: unknown) => rpc("http", args),
  connector: (args: unknown) => rpc("connector", args),
  invokeAction: (args: unknown) => rpc("invokeAction", args),
  log: (...a: unknown[]) => emit({ kind: "log", message: a.map(String).join(" ") }),
};

try {
  const run = (userMod as { default?: unknown }).default;
  if (typeof run !== "function") {
    throw new Error("automation must export default async function run(ctx)");
  }
  const result = await (run as (c: typeof ctx) => Promise<unknown>)(ctx);
  emit({ kind: "result", result: result ?? { ok: true } });
  // Exit explicitly: the stdin reader keeps the event loop alive otherwise.
  finish(0);
} catch (e) {
  const err = e as { message?: string };
  emit({ kind: "error", error: String(err?.message ?? e) });
  finish(1);
}
