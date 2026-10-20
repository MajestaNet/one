// Majesta One Node guest lockdown (ADR-034).
// Loaded with `node --import` before the bootstrap. In-process registerHooks
// (Node >= 22.15) so --permission can keep WorkerThreads denied.
// Customer code may import only one:automation. node:fs is limited to platform files.

import { registerHooks } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";

const workdir = fileURLToPath(new URL("./", import.meta.url));
const platformFile = /\/(guest_io|bootstrap|unit_bootstrap)\.ts$/;

const blockedBuiltins = new Set([
  "async_hooks",
  "child_process",
  "cluster",
  "dgram",
  "dns",
  "dns/promises",
  "fs",
  "fs/promises",
  "http",
  "http2",
  "https",
  "inspector",
  "module",
  "net",
  "os",
  "perf_hooks",
  "repl",
  "sqlite",
  "tls",
  "trace_events",
  "v8",
  "vm",
  "wasi",
  "worker_threads",
]);

function insideWorkdir(fileUrl) {
  let abs;
  try {
    abs = fileURLToPath(fileUrl);
  } catch {
    return false;
  }
  const rel = path.relative(workdir, abs);
  return rel === "" || (!rel.startsWith("..") && !path.isAbsolute(rel));
}

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "one:automation") {
      return {
        url: new URL("./one_automation.ts", import.meta.url).href,
        shortCircuit: true,
      };
    }
    const parent = context.parentURL || "";
    const platformFs =
      (specifier === "node:fs" || specifier === "node:fs/promises") &&
      platformFile.test(parent);
    if (platformFs) {
      return nextResolve(specifier, context);
    }
    // Node resolves the entrypoint as a file: URL. Allow only files inside the workdir.
    if (specifier.startsWith("file:")) {
      if (!insideWorkdir(specifier)) {
        throw new Error("forbidden import " + specifier);
      }
      return nextResolve(specifier, context);
    }
    if (
      specifier.startsWith("node:") ||
      specifier.startsWith("npm:") ||
      specifier.startsWith("jsr:") ||
      specifier.startsWith("http:") ||
      specifier.startsWith("https:") ||
      specifier.startsWith("data:")
    ) {
      throw new Error("forbidden import " + specifier);
    }
    if (!specifier.startsWith(".")) {
      throw new Error("forbidden import " + specifier);
    }
    const resolved = nextResolve(specifier, context);
    if (resolved && resolved.url && !insideWorkdir(resolved.url)) {
      throw new Error("forbidden import " + specifier);
    }
    return resolved;
  },
});

const denyNetwork = () => {
  throw new Error("guest network is disabled");
};

for (const name of ["fetch", "WebSocket", "EventSource"]) {
  try {
    const value = name === "fetch" ? denyNetwork : class DisabledSocket {
      constructor() {
        denyNetwork();
      }
    };
    Object.defineProperty(globalThis, name, {
      value,
      configurable: false,
      writable: false,
    });
  } catch {
    // Binding already sealed; the permission model and import hook still apply.
  }
}

if (typeof process.getBuiltinModule === "function") {
  const orig = process.getBuiltinModule.bind(process);
  process.getBuiltinModule = (id) => {
    const name = String(id).replace(/^node:/, "");
    if (blockedBuiltins.has(name)) {
      throw new Error("forbidden builtin " + id);
    }
    return orig(id);
  };
}

if (typeof process.dlopen === "function") {
  process.dlopen = () => {
    throw new Error("native addons are disabled");
  };
}
