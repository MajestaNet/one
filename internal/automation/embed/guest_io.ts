// Shared NDJSON stdio for the Node guest (ADR-034). Platform file: may import node:fs.
// Customer entries must not import this module; they receive ctx from the bootstrap.

import { readFileSync, writeSync } from "node:fs";

const enc = new TextEncoder();

export function emit(obj: unknown): void {
  writeSync(1, enc.encode(JSON.stringify(obj) + "\n"));
}

type Pending = {
  resolve: (v: unknown) => void;
  reject: (e: Error) => void;
};

const pending = new Map<number, Pending>();
let nextId = 1;
let buf = "";
let readerStarted = false;

function handleLine(line: string): void {
  if (!line) return;
  let msg: {
    kind?: string;
    id?: number;
    result?: unknown;
    error?: string;
  };
  try {
    msg = JSON.parse(line);
  } catch {
    return;
  }
  if (msg.kind !== "rpcResult" || typeof msg.id !== "number") return;
  const p = pending.get(msg.id);
  if (!p) return;
  pending.delete(msg.id);
  if (msg.error) p.reject(new Error(String(msg.error)));
  else p.resolve(msg.result);
}

function startReader(): void {
  if (readerStarted) return;
  readerStarted = true;
  const decoder = new TextDecoder();
  process.stdin.on("data", (chunk: Uint8Array | string) => {
    buf += typeof chunk === "string" ? chunk : decoder.decode(chunk);
    let idx: number;
    while ((idx = buf.indexOf("\n")) >= 0) {
      const line = buf.slice(0, idx).trim();
      buf = buf.slice(idx + 1);
      handleLine(line);
    }
  });
  process.stdin.on("error", (e: Error) => {
    emit({ kind: "error", error: `stdin reader failed: ${String(e)}` });
  });
}

export function rpc(method: string, args: unknown): Promise<unknown> {
  startReader();
  const id = nextId++;
  return new Promise((resolve, reject) => {
    pending.set(id, { resolve, reject });
    emit({ kind: "rpc", id, method, args });
  });
}

export function loadTrigger(): unknown {
  return JSON.parse(readFileSync(new URL("./trigger.json", import.meta.url), "utf8"));
}

export function finish(code: number): void {
  process.exit(code);
}
