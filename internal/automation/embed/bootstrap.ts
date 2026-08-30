// Majesta One automation guest bootstrap (ADR-014 Phase 4).
// Host speaks NDJSON on stdin/stdout; guest has no network or npm.

import * as userMod from "./user_entry.ts";

const enc = new TextEncoder();

function emit(obj: unknown): void {
  Deno.stdout.writeSync(enc.encode(JSON.stringify(obj) + "\n"));
}

type Pending = {
  resolve: (v: unknown) => void;
  reject: (e: Error) => void;
};

const pending = new Map<number, Pending>();
let nextId = 1;
let buf = "";
let readerStarted = false;

function startReader(): void {
  if (readerStarted) return;
  readerStarted = true;
  const decoder = new TextDecoder();
  const reader = Deno.stdin.readable.getReader();
  (async () => {
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      let idx: number;
      while ((idx = buf.indexOf("\n")) >= 0) {
        const line = buf.slice(0, idx).trim();
        buf = buf.slice(idx + 1);
        if (!line) continue;
        let msg: {
          kind?: string;
          id?: number;
          result?: unknown;
          error?: string;
        };
        try {
          msg = JSON.parse(line);
        } catch {
          continue;
        }
        if (msg.kind !== "rpcResult" || typeof msg.id !== "number") continue;
        const p = pending.get(msg.id);
        if (!p) continue;
        pending.delete(msg.id);
        if (msg.error) p.reject(new Error(String(msg.error)));
        else p.resolve(msg.result);
      }
    }
  })().catch((e) => {
    emit({ kind: "error", error: `stdin reader failed: ${String(e)}` });
  });
}

function rpc(method: string, args: unknown): Promise<unknown> {
  startReader();
  const id = nextId++;
  return new Promise((resolve, reject) => {
    pending.set(id, { resolve, reject });
    emit({ kind: "rpc", id, method, args });
  });
}

const trigger = JSON.parse(await Deno.readTextFile(new URL("./trigger.json", import.meta.url)));

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
  Deno.exit(0);
} catch (e) {
  const err = e as { message?: string };
  emit({ kind: "error", error: String(err?.message ?? e) });
  Deno.exit(1);
}
