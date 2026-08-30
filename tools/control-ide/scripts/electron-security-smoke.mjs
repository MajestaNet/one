#!/usr/bin/env node
/**
 * Launches the packaged renderer in the real Electron shell and asserts the trust-boundary
 * controls from docs/architecture/control-ide-security-audit.md hold end to end:
 *
 *   CIDE-01  renderer-initiated windows are denied (no child target gets the preload bridge)
 *   CIDE-03  the strict CSP is active and does not block the app's own bundle
 *   CIDE-04  fs IPC refuses a root the renderer never registered
 *   CIDE-05  git clone refuses an argument-injection URL
 *
 * Drives the app over the Chrome DevTools Protocol so no test hooks exist in product code.
 * Run after `npm run build`: `npm run smoke:electron`.
 */
import { spawn } from "node:child_process";
import { once } from "node:events";
import { existsSync, mkdtempSync, rmSync } from "node:fs";
import { createRequire } from "node:module";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const appDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const port = 9333 + (process.pid % 200);
const results = [];
const electronOutput = [];
const smokeProfileDir = mkdtempSync(path.join(os.tmpdir(), "one-electron-smoke-"));
let child;

/**
 * Electron 43+ no longer runs install.js from a package postinstall — the binary is downloaded
 * lazily when `require("electron")` resolves the executable path. Resolve through that path so
 * CI `npm ci` trees (no dist/electron yet) still work.
 */
function resolveElectronBinary() {
  const bin = require("electron");
  if (!bin || !existsSync(bin)) {
    throw new Error(`Electron binary missing after install (${bin || "undefined"})`);
  }
  return bin;
}

function record(name, passed, detail = "") {
  results.push({ name, passed, detail });
  const mark = passed ? "PASS" : "FAIL";
  console.log(`${mark}  ${name}${detail ? ` — ${detail}` : ""}`);
}

async function waitForRendererTarget(timeoutMs = 45_000) {
  const deadline = Date.now() + timeoutMs;
  let lastTargets = [];
  while (Date.now() < deadline) {
    try {
      const res = await fetch(`http://127.0.0.1:${port}/json/list`, { signal: AbortSignal.timeout(2_000) });
      const targets = await res.json();
      lastTargets = targets;
      // BrowserWindow is briefly exposed as about:blank before loadFile completes.
      // Connecting during that navigation can leave the CDP session attached to a
      // discarded renderer, so wait for the actual built app document.
      const page = targets.find(
        (t) =>
          t.type === "page" &&
          t.webSocketDebuggerUrl &&
          typeof t.url === "string" &&
          t.url.includes("/dist/index.html"),
      );
      if (page) return page;
    } catch {
      /* devtools endpoint not up yet */
    }
    await new Promise((r) => setTimeout(r, 500));
  }
  const observed = lastTargets.map((t) => `${t.type}:${t.url || "<blank>"}`).join(", ");
  throw new Error(`Electron renderer target never appeared${observed ? ` (observed ${observed})` : ""}`);
}

async function pageTargetCount() {
  const res = await fetch(`http://127.0.0.1:${port}/json/list`, { signal: AbortSignal.timeout(2_000) });
  return (await res.json()).filter((t) => t.type === "page").length;
}

class Cdp {
  #socket;
  #next = 1;
  #pending = new Map();
  logs = [];

  constructor(socket) {
    this.#socket = socket;
    socket.addEventListener("message", (event) => {
      const msg = JSON.parse(event.data);
      if (msg.id && this.#pending.has(msg.id)) {
        const { resolve, reject } = this.#pending.get(msg.id);
        this.#pending.delete(msg.id);
        if (msg.error) reject(new Error(msg.error.message));
        else resolve(msg.result);
        return;
      }
      if (msg.method === "Log.entryAdded") this.logs.push(msg.params.entry);
      if (msg.method === "Runtime.consoleAPICalled") {
        this.logs.push({ source: "console", text: (msg.params.args ?? []).map((a) => a.value).join(" ") });
      }
    });
  }

  static async connect(url) {
    const socket = new WebSocket(url);
    await once(socket, "open");
    return new Cdp(socket);
  }

  send(method, params = {}) {
    const id = this.#next++;
    return new Promise((resolve, reject) => {
      this.#pending.set(id, { resolve, reject });
      this.#socket.send(JSON.stringify({ id, method, params }));
      setTimeout(() => {
        if (this.#pending.delete(id)) reject(new Error(`${method} timed out`));
      }, 20_000);
    });
  }

  /** Evaluate in the renderer and return the JSON-ish value. */
  async evaluate(expression) {
    const res = await this.send("Runtime.evaluate", {
      expression,
      awaitPromise: true,
      returnByValue: true,
      userGesture: true,
    });
    if (res.exceptionDetails) {
      return { __threw: res.exceptionDetails.exception?.description ?? "exception" };
    }
    return res.result.value;
  }

  close() {
    this.#socket.close();
  }
}

async function main() {
  if (!existsSync(path.join(appDir, "dist", "index.html"))) {
    throw new Error("dist/index.html missing — run `npm run build` first");
  }

  const electronBin = resolveElectronBinary();
  const useXvfb = !process.env.DISPLAY && process.platform === "linux";
  const bin = useXvfb ? "xvfb-run" : electronBin;
  const electronArgs = [
    `--remote-debugging-port=${port}`,
    `--user-data-dir=${smokeProfileDir}`,
    "--no-sandbox",
    ".",
  ];
  const args = useXvfb
    ? ["-a", electronBin, ...electronArgs]
    : electronArgs;

  // detached so the whole process group (xvfb-run + electron) can be torn down at the end.
  child = spawn(bin, args, { cwd: appDir, stdio: ["ignore", "pipe", "pipe"], detached: true });
  child.stderr.on("data", (d) => electronOutput.push(String(d)));
  child.stdout.on("data", (d) => electronOutput.push(String(d)));

  const target = await waitForRendererTarget();
  const cdp = await Cdp.connect(target.webSocketDebuggerUrl);
  await cdp.send("Log.enable");
  await cdp.send("Runtime.enable");

  // Give React a moment to mount so we can tell a CSP block from a slow boot.
  await new Promise((r) => setTimeout(r, 3000));

  const mounted = await cdp.evaluate("document.querySelector('#root')?.childElementCount ?? 0");
  record("CIDE-03 strict CSP does not block the app's own bundle", Number(mounted) > 0, `#root children: ${mounted}`);

  const cspHeader = await cdp.evaluate(
    "document.querySelector('meta[http-equiv=\"Content-Security-Policy\"]')?.content ?? ''",
  );
  record(
    "CIDE-03 CSP is present and denies script from other origins",
    typeof cspHeader === "string" && cspHeader.includes("script-src 'self'") && cspHeader.includes("default-src 'none'"),
    cspHeader ? "script-src 'self'" : "missing",
  );

  const bridge = await cdp.evaluate("typeof window.one");
  record("preload bridge is present in the app frame", bridge === "object", `typeof: ${bridge}`);

  const before = await pageTargetCount();
  await cdp.evaluate("(() => { try { window.open('https://example.com/', '_blank'); } catch (e) { return String(e); } })()");
  await new Promise((r) => setTimeout(r, 1500));
  const after = await pageTargetCount();
  record(
    "CIDE-01 window.open from the renderer creates no child window",
    after === before,
    `page targets ${before} -> ${after}`,
  );

  const readEtc = await cdp.evaluate(
    "window.one.readText('/etc', 'passwd').then(() => 'READ_SUCCEEDED', (e) => 'refused: ' + String(e && e.message || e))",
  );
  record(
    "CIDE-04 fs:readText refuses an unregistered root",
    typeof readEtc === "string" && readEtc.startsWith("refused:"),
    String(readEtc).slice(0, 120),
  );

  const writeHome = await cdp.evaluate(
    "window.one.writeText('/', 'tmp/one-pwned', 'x').then(() => 'WRITE_SUCCEEDED', (e) => 'refused: ' + String(e && e.message || e))",
  );
  record(
    "CIDE-04 fs:writeText refuses a filesystem root",
    typeof writeHome === "string" && writeHome.startsWith("refused:"),
    String(writeHome).slice(0, 120),
  );

  const clone = await cdp.evaluate(
    "window.one.gitClone('--upload-pack=touch /tmp/one-pwned;git-upload-pack', '/tmp/one-clone-smoke').then(r => JSON.stringify(r), e => 'threw: ' + String(e))",
  );
  const cloneRefused = typeof clone === "string" && clone.includes('"ok":false');
  record("CIDE-05 git:clone refuses an --upload-pack payload", cloneRefused, String(clone).slice(0, 140));

  const registerEtc = await cdp.evaluate(
    "window.one.registerRepoRoot('/etc').then(r => JSON.stringify(r), e => 'threw: ' + String(e))",
  );
  record(
    "CIDE-04 repo root registration refuses /etc",
    typeof registerEtc === "string" && registerEtc.includes('"ok":false'),
    String(registerEtc).slice(0, 140),
  );

  const external = await cdp.evaluate(
    "window.one.openExternal('file:///etc/passwd').then(r => JSON.stringify(r), e => 'threw: ' + String(e))",
  );
  record(
    "CIDE-01 shell:openExternal refuses a non-https scheme",
    typeof external === "string" && external.includes('"ok":false'),
    String(external).slice(0, 140),
  );

  const cspViolations = cdp.logs.filter((l) => /Content Security Policy/i.test(l.text ?? ""));
  record(
    "CSP reports no violation for the app's own resources",
    cspViolations.length === 0,
    cspViolations.length ? cspViolations[0].text.slice(0, 160) : "none",
  );

  cdp.close();

  const failed = results.filter((r) => !r.passed);
  console.log(`\n${results.length - failed.length}/${results.length} checks passed`);
  if (failed.length) {
    console.log("\n--- electron output ---\n" + electronOutput.join("").slice(-4000));
    process.exitCode = 1;
  }
}

main()
  .catch((err) => {
    console.error(`smoke harness error: ${err.message}`);
    const output = electronOutput.join("").slice(-4000);
    if (output) console.error("\n--- electron output ---\n" + output);
    process.exitCode = 1;
  })
  .finally(() => {
    if (child?.pid) {
      try {
        process.kill(-child.pid, "SIGKILL");
      } catch {
        try {
          child.kill("SIGKILL");
        } catch {
          /* already gone */
        }
      }
    }
    rmSync(smokeProfileDir, { recursive: true, force: true });
  });
