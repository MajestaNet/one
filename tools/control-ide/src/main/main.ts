import { app, BrowserWindow, dialog, ipcMain, safeStorage, shell } from "electron";
import path from "node:path";
import fs from "node:fs";
import { fileURLToPath, pathToFileURL } from "node:url";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import electronUpdater from "electron-updater";
import {
  RepoRootRegistry,
  assertCreatableRepoDir,
  assertImportExportDest,
  assertSelectableLocalDir,
  copyDirRecursive,
  isCommitAllowlisted,
  listFilesTree,
  listYamlTree,
  readTextUnderRoot,
  resolveCustomerRepoTemplate,
  writeTextUnderRoot,
} from "./paths.js";
import { createTrustedHandle } from "./ipcTrust.js";
import { extractProtocolUrl, isAppProtocolUrl } from "./protocol.js";
import {
  GIT_CLONE_SAFE_CONFIG,
  GIT_CLONE_SAFE_ENV,
  GIT_SAFE_ENV,
  assertChangeBranchName,
  assertSafeCloneUrl,
  buildCsp,
  isSafeExternalUrl,
  type FrameTrust,
} from "./security.js";
import { readSessionFile, writeSessionFile, type WriteSessionResult } from "./sessionStore.js";
import {
  configureUpdater,
  disabledStatus,
  gateDisabledReason,
  resolveFeedUrl,
  shouldEnableUpdates,
  type UpdateStatus,
  type UpdaterLike,
} from "./updates.js";
import { decideNavigation, decideWindowOpen } from "./webContentsPolicy.js";

const { autoUpdater } = electronUpdater;

const execFileAsync = promisify(execFile);
const __dirname = path.dirname(fileURLToPath(import.meta.url));

const SESSION_FILE = () => path.join(app.getPath("userData"), "session.bin");
const SESSION_KEY_FILE = () => path.join(app.getPath("userData"), "session.key");

const RENDERER_INDEX = path.join(__dirname, "../dist/index.html");

/** Frames allowed to send IPC and to be navigated to (CIDE-06). */
const frameTrust: FrameTrust = {
  devServerUrl: process.env.VITE_DEV_SERVER_URL,
  appIndexUrl: pathToFileURL(RENDERER_INDEX).href,
};

let repoRoots: RepoRootRegistry;

function repoRootRegistry(): RepoRootRegistry {
  if (!repoRoots) {
    repoRoots = new RepoRootRegistry({
      denyDirs: [app.getPath("userData"), app.getPath("appData")],
    });
  }
  return repoRoots;
}

type EnvConnection = {
  installId: string;
  installRole: string;
  baseUrl: string;
  token: string;
  scopes?: string[];
  isAdmin?: boolean;
  systemPermissions?: string[];
  label?: string;
};

type Session = {
  activeInstallId?: string;
  environments?: EnvConnection[];
  baseUrl: string;
  token: string;
  repoPath?: string;
  scopes?: string[];
  isAdmin?: boolean;
  customerRepoUrl?: string;
  deviceId?: string;
  systemPermissions?: string[];
  /** Set when the operator explicitly accepted plaintext HTTP for a non-loopback install. */
  allowInsecureHttp?: boolean;
};

let updateStatus: UpdateStatus = disabledStatus("Updates not initialized.");

function setUpdateStatus(next: UpdateStatus) {
  updateStatus = next;
}

const sessionCrypto = {
  isEncryptionAvailable: () => safeStorage.isEncryptionAvailable(),
  encryptString: (plain: string) => safeStorage.encryptString(plain),
  decryptString: (cipher: Buffer) => safeStorage.decryptString(cipher),
};

function sessionPersistOpts() {
  return { localKeyPath: SESSION_KEY_FILE() };
}

function loadSession(): Session | null {
  try {
    const raw = readSessionFile(SESSION_FILE(), sessionCrypto, sessionPersistOpts());
    if (!raw) return null;
    return JSON.parse(raw) as Session;
  } catch {
    return null;
  }
}

function saveSession(s: Session | null): WriteSessionResult {
  if (s === null) {
    return writeSessionFile(SESSION_FILE(), null, sessionCrypto, sessionPersistOpts());
  }
  // CIDE-10: never write cleartext. Prefer OS safeStorage; otherwise AES-GCM with a
  // 0600 local key so a restart restores the session. Memory-only
  // (ephemeral) is the last resort when even the local key cannot be written.
  const result = writeSessionFile(SESSION_FILE(), JSON.stringify(s), sessionCrypto, sessionPersistOpts());
  if (result.ok) return result;
  return { ok: true, ephemeral: true };
}

/**
 * Renderer-initiated windows must never receive the preload bridge. Deny every one and send
 * vetted URLs to the OS browser instead (CIDE-01). Navigation confined to the app origin (CIDE-06).
 */
function hardenWebContents(win: BrowserWindow) {
  win.webContents.setWindowOpenHandler(({ url }) => {
    const decision = decideWindowOpen(url);
    if (decision.openExternally) void shell.openExternal(decision.openExternally);
    return { action: decision.action };
  });

  win.webContents.on("will-navigate", (event, url) => {
    const decision = decideNavigation(url, frameTrust);
    if (decision.prevent) {
      event.preventDefault();
      if (decision.openExternally) void shell.openExternal(decision.openExternally);
    }
  });

  win.webContents.on("will-attach-webview", (event) => {
    event.preventDefault();
  });

  win.webContents.on("will-frame-navigate", (event) => {
    const decision = decideNavigation(event.url, frameTrust);
    if (decision.prevent) event.preventDefault();
  });
}

/** CSP for http(s)-served documents; `file://` documents rely on the meta tag (CIDE-03). */
function applyContentSecurityPolicy(win: BrowserWindow) {
  const csp = buildCsp(
    process.env.VITE_DEV_SERVER_URL ? "development" : "production",
    process.env.VITE_DEV_SERVER_URL,
  );
  const contentSession = win.webContents.session;

  contentSession.webRequest.onHeadersReceived((details, callback) => {
    callback({
      responseHeaders: {
        ...details.responseHeaders,
        "Content-Security-Policy": [csp],
      },
    });
  });

  // The renderer needs no device or media permissions; refuse the lot.
  contentSession.setPermissionRequestHandler((_wc, _permission, callback) => callback(false));
  contentSession.setPermissionCheckHandler(() => false);
}

function createWindow() {
  const win = BrowserWindow.getAllWindows()[0];
  if (win) {
    win.focus();
    return;
  }
  const next = new BrowserWindow({
    width: 1280,
    height: 840,
    backgroundColor: "#0b0f14",
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
      webviewTag: false,
    },
  });

  applyContentSecurityPolicy(next);
  hardenWebContents(next);

  if (process.env.VITE_DEV_SERVER_URL) {
    void next.loadURL(process.env.VITE_DEV_SERVER_URL);
  } else {
    void next.loadFile(RENDERER_INDEX);
  }
}

function wireUpdates() {
  const resolved = resolveFeedUrl();
  const gate = {
    packaged: app.isPackaged,
    feedUrl: resolved.url,
    feedError: "error" in resolved ? resolved.error : undefined,
  };
  if (!shouldEnableUpdates(gate)) {
    setUpdateStatus(disabledStatus(gateDisabledReason(gate)));
    return;
  }

  configureUpdater(autoUpdater as unknown as UpdaterLike, resolved.url!);
  setUpdateStatus({
    state: "idle",
    message: "Ready to check for updates (download requires confirmation until signed releases land).",
  });

  autoUpdater.on("checking-for-update", () => {
    setUpdateStatus({ state: "checking", message: "Checking for updates…" });
  });
  autoUpdater.on("update-available", (info: { version?: string }) => {
    setUpdateStatus({
      state: "available",
      message: "Update available; downloading…",
      version: info?.version,
    });
  });
  autoUpdater.on("update-not-available", () => {
    setUpdateStatus({ state: "not-available", message: "You are on the latest version." });
  });
  autoUpdater.on("download-progress", (p: { percent?: number }) => {
    setUpdateStatus({
      state: "downloading",
      message: "Downloading update…",
      progress: p?.percent,
    });
  });
  autoUpdater.on("update-downloaded", (info: { version?: string }) => {
    setUpdateStatus({
      state: "downloaded",
      message: "Update ready — restart to install.",
      version: info?.version,
    });
  });
  autoUpdater.on("error", (err: Error) => {
    setUpdateStatus({ state: "error", message: err?.message || String(err) });
  });
}

async function runGit(args: string[], env: Record<string, string> = GIT_SAFE_ENV) {
  return execFileAsync("git", args, {
    shell: false,
    env: { ...process.env, ...env },
  });
}

async function trySpawnEditor(bin: string, repoPath: string): Promise<boolean> {
  try {
    await execFileAsync(bin, [repoPath], { shell: false });
    return true;
  } catch {
    return false;
  }
}

const PROTOCOL = "one-control";

function broadcastOAuthCallback(url: string) {
  for (const win of BrowserWindow.getAllWindows()) {
    win.webContents.send("oauth:callback", url);
    if (win.isMinimized()) win.restore();
    win.focus();
  }
}

const gotLock = app.requestSingleInstanceLock();
if (!gotLock) {
  app.quit();
} else {
  app.on("second-instance", (_event, argv) => {
    const url = extractProtocolUrl(argv, PROTOCOL);
    if (url) broadcastOAuthCallback(url);
    const win = BrowserWindow.getAllWindows()[0];
    if (win) {
      if (win.isMinimized()) win.restore();
      win.focus();
    }
  });
}

if (process.defaultApp) {
  if (process.argv.length >= 2) {
    app.setAsDefaultProtocolClient(PROTOCOL, process.execPath, [path.resolve(process.argv[1])]);
  }
} else {
  app.setAsDefaultProtocolClient(PROTOCOL);
}

app.on("open-url", (event, url) => {
  event.preventDefault();
  if (isAppProtocolUrl(url, PROTOCOL)) {
    broadcastOAuthCallback(url);
  }
});

/** Register an IPC handler that validates its sender before doing any work (CIDE-06). */
const handle = createTrustedHandle(
  (channel, listener) => {
    ipcMain.handle(channel, (event, ...args) => listener(event, ...args));
  },
  frameTrust,
);

type IpcResult<T = object> = ({ ok: true } & T) | { ok: false; error: string };

function failed(err: unknown): { ok: false; error: string } {
  return { ok: false, error: err instanceof Error ? err.message : String(err) };
}

app.whenReady().then(() => {
  const roots = repoRootRegistry();
  // Seed the allowlist from the persisted session so a restored repo path keeps working.
  roots.tryRegister(loadSession()?.repoPath);

  handle("session:get", () => loadSession());
  handle("session:encryptionAvailable", () => safeStorage.isEncryptionAvailable());
  handle("session:set", (_e, session: Session | null): WriteSessionResult => {
    const result = saveSession(session);
    if (result.ok && session?.repoPath) roots.tryRegister(session.repoPath);
    return result;
  });

  handle("repo:registerRoot", (_e, repoPath: string): IpcResult<{ path: string }> => {
    try {
      return { ok: true, path: roots.register(repoPath) };
    } catch (err) {
      return failed(err);
    }
  });

  handle(
    "repo:chooseRoot",
    async (event): Promise<IpcResult<{ path: string }> | { ok: false; canceled: true; error: string }> => {
      const win =
        BrowserWindow.fromWebContents(event.sender as Electron.WebContents) ??
        BrowserWindow.getAllWindows()[0];
      const result = win
        ? await dialog.showOpenDialog(win, { properties: ["openDirectory"] })
        : await dialog.showOpenDialog({ properties: ["openDirectory"] });
      if (result.canceled || result.filePaths.length === 0) {
        return { ok: false, canceled: true, error: "No directory selected" };
      }
      try {
        return { ok: true, path: roots.register(result.filePaths[0]) };
      } catch (err) {
        return failed(err);
      }
    },
  );

  /** Pick any usable local folder (empty OK) — for first-time clone / init destinations. */
  handle(
    "repo:chooseLocalFolder",
    async (event): Promise<IpcResult<{ path: string }> | { ok: false; canceled: true; error: string }> => {
      const win =
        BrowserWindow.fromWebContents(event.sender as Electron.WebContents) ??
        BrowserWindow.getAllWindows()[0];
      const result = win
        ? await dialog.showOpenDialog(win, {
            properties: ["openDirectory", "createDirectory"],
          })
        : await dialog.showOpenDialog({
            properties: ["openDirectory", "createDirectory"],
          });
      if (result.canceled || result.filePaths.length === 0) {
        return { ok: false, canceled: true, error: "No directory selected" };
      }
      try {
        const selected = assertSelectableLocalDir(result.filePaths[0], {
          denyDirs: [app.getPath("userData"), app.getPath("appData")],
        });
        // Register only when it already looks like a customer repo; empty folders stay session-only.
        const registered = roots.tryRegister(selected);
        return { ok: true, path: registered ?? selected };
      } catch (err) {
        return failed(err);
      }
    },
  );

  handle("shell:openExternal", async (_e, url: string): Promise<IpcResult> => {
    if (!isSafeExternalUrl(url)) {
      return { ok: false, error: "Refused to open a non-https (or non-loopback) URL" };
    }
    try {
      await shell.openExternal(url);
      return { ok: true };
    } catch (err) {
      return failed(err);
    }
  });

  handle("git:status", async (_e, repoPath: string) => {
    try {
      const root = roots.require(repoPath);
      const { stdout: branch } = await runGit(["-C", root, "rev-parse", "--abbrev-ref", "HEAD"]);
      const { stdout: status } = await runGit(["-C", root, "status", "--porcelain"]);
      return { ok: true, branch: branch.trim(), status: status.trim() };
    } catch (err) {
      return failed(err);
    }
  });

  handle("git:clone", async (_e, url: string, destPath: string) => {
    try {
      const cloneUrl = assertSafeCloneUrl(url);
      const dest = assertCreatableRepoDir(destPath, {
        denyDirs: [app.getPath("userData"), app.getPath("appData")],
      });
      fs.mkdirSync(path.dirname(dest), { recursive: true });
      await runGit(
        [...GIT_CLONE_SAFE_CONFIG, "clone", "--no-recurse-submodules", "--", cloneUrl, dest],
        GIT_CLONE_SAFE_ENV,
      );
      return { ok: true, path: roots.register(dest) };
    } catch (err) {
      return failed(err);
    }
  });

  handle("git:pull", async (_e, repoPath: string) => {
    try {
      const root = roots.require(repoPath);
      const { stdout: status } = await runGit(["-C", root, "status", "--porcelain"]);
      if (status.trim()) {
        return { ok: false, error: "Working tree dirty — commit or stash before pull" };
      }
      await runGit(["-C", root, "fetch", "origin"]);
      // Prefer main (ADR-012 default); fall back to master for older remotes.
      let target = "main";
      try {
        await runGit(["-C", root, "rev-parse", "--verify", "origin/main"]);
      } catch {
        target = "master";
        await runGit(["-C", root, "rev-parse", "--verify", "origin/master"]);
      }
      await runGit(["-C", root, "checkout", target]);
      await runGit(["-C", root, "pull", "--ff-only", "origin", target]);
      const { stdout: branch } = await runGit(["-C", root, "rev-parse", "--abbrev-ref", "HEAD"]);
      return { ok: true, branch: branch.trim() };
    } catch (err) {
      return failed(err);
    }
  });

  handle("git:createBranch", async (_e, repoPath: string, branch: string) => {
    try {
      const root = roots.require(repoPath);
      const name = assertChangeBranchName(branch);
      await runGit(["-C", root, "checkout", "-b", name]);
      return { ok: true, branch: name };
    } catch (err) {
      return failed(err);
    }
  });

  handle("git:push", async (_e, repoPath: string) => {
    try {
      const root = roots.require(repoPath);
      await runGit(["-C", root, "push", "-u", "origin", "HEAD"]);
      return { ok: true };
    } catch (err) {
      return failed(err);
    }
  });

  handle("git:commit", async (_e, repoPath: string, message: string) => {
    try {
      const root = roots.require(repoPath);
      const msg = String(message || "").trim();
      if (!msg) return { ok: false, error: "Commit message required" };
      const { stdout: status } = await runGit(["-C", root, "status", "--porcelain"]);
      const lines = status
        .split("\n")
        .map((l) => l.trimEnd())
        .filter((l) => l.trim());
      if (lines.length === 0) return { ok: false, error: "Nothing to commit (working tree clean)" };

      const toStage: string[] = [];
      for (const line of lines) {
        // Rename: "R  old -> new"
        const rename = line.match(/^[A-Z? ]{1,2}\s+(.+?)\s+->\s+(.+)$/);
        const rel = (rename ? rename[2] : line.length >= 4 ? line.slice(3) : "").trim().replace(/\\/g, "/");
        if (!rel) continue;
        if (!isCommitAllowlisted(rel)) {
          return { ok: false, error: `Path not allowlisted for commit: ${rel}` };
        }
        toStage.push(rel);
        if (rename) {
          const oldRel = rename[1].trim().replace(/\\/g, "/");
          if (oldRel && isCommitAllowlisted(oldRel)) toStage.push(oldRel);
        }
      }
      if (toStage.length === 0) return { ok: false, error: "No allowlisted paths to commit" };

      await runGit(["-C", root, "add", "--", ...toStage]);
      await runGit(["-C", root, "commit", "-m", msg]);
      const { stdout: branch } = await runGit(["-C", root, "rev-parse", "--abbrev-ref", "HEAD"]);
      return { ok: true, branch: branch.trim() };
    } catch (err) {
      return failed(err);
    }
  });

  handle("repo:initSample", async (_e, destPath: string) => {
    try {
      const dest = assertCreatableRepoDir(destPath, {
        denyDirs: [app.getPath("userData"), app.getPath("appData")],
      });
      const template = resolveCustomerRepoTemplate();
      if (!template) {
        return {
          ok: false,
          error:
            "Sample template not found. Set ONE_CUSTOMER_REPO_TEMPLATE or run from the Majesta One monorepo.",
        };
      }
      copyDirRecursive(template, dest);
      // Initialize git if needed so pack-from-HEAD works immediately.
      if (!fs.existsSync(path.join(dest, ".git"))) {
        await runGit(["-C", dest, "init"]);
        await runGit(["-C", dest, "add", "."]);
        await runGit(["-C", dest, "commit", "-m", "Initialize from Majesta One customer sample"]);
      }
      return { ok: true, path: roots.register(dest), template };
    } catch (err) {
      return failed(err);
    }
  });

  /** Unpack GET /deploy/v1/packages/export zip into empty or existing customer repo folder. */
  handle("repo:importExportZip", async (_e, destPath: string, base64: string) => {
    const tmp = path.join(app.getPath("temp"), `one-export-${Date.now()}.zip`);
    const unpackDir = path.join(app.getPath("temp"), `one-export-unpack-${Date.now()}`);
    try {
      const dest = assertImportExportDest(destPath, {
        denyDirs: [app.getPath("userData"), app.getPath("appData")],
      });
      const raw = Buffer.from(String(base64 || ""), "base64");
      if (raw.length < 4 || raw[0] !== 0x50 || raw[1] !== 0x4b) {
        return { ok: false, error: "Not a zip archive" };
      }
      fs.writeFileSync(tmp, raw);
      fs.mkdirSync(unpackDir, { recursive: true });
      try {
        await execFileAsync("unzip", ["-q", "-o", tmp, "-d", unpackDir], { shell: false });
      } catch {
        // Windows / missing unzip: try PowerShell Expand-Archive.
        await execFileAsync(
          "powershell",
          [
            "-NoProfile",
            "-Command",
            `Expand-Archive -LiteralPath '${tmp.replace(/'/g, "''")}' -DestinationPath '${unpackDir.replace(/'/g, "''")}' -Force`,
          ],
          { shell: false },
        );
      }

      let source = unpackDir;
      if (!fs.existsSync(path.join(source, "one.yaml"))) {
        const kids = fs.readdirSync(source).filter((n) => n !== ".git");
        if (kids.length === 1) {
          const nested = path.join(source, kids[0]);
          if (fs.existsSync(path.join(nested, "one.yaml"))) {
            source = nested;
          }
        }
      }
      if (!fs.existsSync(path.join(source, "one.yaml"))) {
        return { ok: false, error: "Export zip missing one.yaml" };
      }

      fs.mkdirSync(dest, { recursive: true });
      // Overlay export onto dest without wiping .git.
      const walkCopy = (from: string, to: string) => {
        for (const ent of fs.readdirSync(from, { withFileTypes: true })) {
          if (ent.name === ".git") continue;
          const srcPath = path.join(from, ent.name);
          const destPathInner = path.join(to, ent.name);
          if (ent.isDirectory()) {
            fs.mkdirSync(destPathInner, { recursive: true });
            walkCopy(srcPath, destPathInner);
          } else {
            fs.copyFileSync(srcPath, destPathInner);
          }
        }
      };
      walkCopy(source, dest);

      if (!fs.existsSync(path.join(dest, ".git"))) {
        await runGit(["-C", dest, "init"]);
        await runGit(["-C", dest, "add", "."]);
        await runGit(["-C", dest, "commit", "-m", "Import install export (one/v1)"]);
      }
      return { ok: true, path: roots.register(dest) };
    } catch (err) {
      return failed(err);
    } finally {
      try {
        fs.unlinkSync(tmp);
      } catch {
        /* ignore */
      }
      try {
        fs.rmSync(unpackDir, { recursive: true, force: true });
      } catch {
        /* ignore */
      }
    }
  });

  handle("editor:open", async (_e, repoPath: string, editor: string = "auto") => {
    try {
      const root = roots.require(repoPath);
      const order = editor && editor !== "auto" ? [editor] : ["code"];
      for (const bin of order) {
        if (await trySpawnEditor(bin, root)) {
          return { ok: true, editor: bin };
        }
      }
      // Fallback: open the repo directory in the OS file manager. Confined to a registered
      // root so `shell.openPath` cannot be used to launch an arbitrary file (CIDE-12).
      const result = await shell.openPath(root);
      if (result) return { ok: false, error: result || "Install an editor CLI on PATH" };
      return { ok: true, editor: "shell" };
    } catch (err) {
      return failed(err);
    }
  });

  /** Pack committed HEAD as zip for Deploy packages/pack (one/v1).
   * Optional `paths` are git pathspecs (plus one.yaml always included when selective). */
  handle("repo:exportZip", async (_e, repoPath: string, paths: string[] = []) => {
    const tmp = path.join(app.getPath("temp"), `one-pack-${Date.now()}.zip`);
    try {
      const root = roots.require(repoPath);
      const specs = Array.isArray(paths)
        ? paths.map((p) => String(p || "").trim()).filter(Boolean)
        : [];
      const args = ["-C", root, "archive", "--format=zip", "-o", tmp, "HEAD"];
      if (specs.length > 0) {
        if (!specs.includes("one.yaml") && !specs.includes("one.yml")) {
          args.push("one.yaml");
        }
        for (const s of specs) {
          // Reject path traversal / absolute paths.
          if (s.startsWith("/") || s.includes("..")) {
            return { ok: false, error: `Invalid pack path: ${s}` };
          }
          if (
            !(
              s === "one.yaml" ||
              s === "one.yml" ||
              s.startsWith("metadata/") ||
              s.startsWith("src/") ||
              s.startsWith("tests/")
            )
          ) {
            return { ok: false, error: `Pack path must be under metadata/, src/, or tests/: ${s}` };
          }
          args.push(s);
        }
      }
      await runGit(args);
      const buf = fs.readFileSync(tmp);
      fs.unlinkSync(tmp);
      return { ok: true, base64: buf.toString("base64") };
    } catch (err) {
      try {
        fs.unlinkSync(tmp);
      } catch {
        /* ignore */
      }
      return failed(err);
    }
  });

  handle("fs:listTree", async (_e, root: string, rel: string = "metadata") => {
    const base = roots.require(root);
    if (rel === "src/automations" || rel === "tests/automations" || rel.startsWith("src/") || rel.startsWith("tests/")) {
      return listFilesTree(base, rel);
    }
    return listYamlTree(base, rel);
  });

  handle("fs:readText", async (_e, root: string, rel: string) =>
    readTextUnderRoot(roots.require(root), rel),
  );

  handle("fs:writeText", async (_e, root: string, rel: string, content: string) => {
    writeTextUnderRoot(roots.require(root), rel, content);
    return true;
  });

  handle("updates:status", () => updateStatus);
  handle("updates:check", async () => {
    if (updateStatus.state === "disabled") return updateStatus;
    try {
      // autoDownload is off until signed releases (ADR-030 freeze); download only on an explicit check.
      const result = await autoUpdater.checkForUpdates();
      if (result && typeof (autoUpdater as { downloadUpdate?: () => Promise<unknown> }).downloadUpdate === "function") {
        await (autoUpdater as { downloadUpdate: () => Promise<unknown> }).downloadUpdate();
      }
    } catch (err) {
      setUpdateStatus({ state: "error", message: String(err) });
    }
    return updateStatus;
  });
  handle("updates:install", () => {
    if (updateStatus.state !== "downloaded") {
      return { ok: false, error: "No downloaded update to install." };
    }
    autoUpdater.quitAndInstall(false, true);
    return { ok: true };
  });

  wireUpdates();
  createWindow();
  const coldStartUrl = extractProtocolUrl(process.argv, PROTOCOL);
  if (coldStartUrl) {
    // Defer until the renderer has subscribed.
    setTimeout(() => broadcastOAuthCallback(coldStartUrl), 500);
  }
  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});
