/**
 * Preload bridge surface (CIDE-06 / ADR-012). Channel names and the typed API map live here
 * so Vitest can lock the IPC contract without loading Electron's `contextBridge`.
 */

/** Every channel the renderer may invoke through `window.one`. */
export const IPC_CHANNELS = {
  sessionGet: "session:get",
  sessionSet: "session:set",
  sessionEncryptionAvailable: "session:encryptionAvailable",
  repoRegisterRoot: "repo:registerRoot",
  repoChooseRoot: "repo:chooseRoot",
  repoChooseLocalFolder: "repo:chooseLocalFolder",
  shellOpenExternal: "shell:openExternal",
  gitStatus: "git:status",
  gitClone: "git:clone",
  gitPull: "git:pull",
  gitCreateBranch: "git:createBranch",
  gitPush: "git:push",
  gitCommit: "git:commit",
  repoInitSample: "repo:initSample",
  repoImportExportZip: "repo:importExportZip",
  editorOpen: "editor:open",
  repoExportZip: "repo:exportZip",
  fsListTree: "fs:listTree",
  fsReadText: "fs:readText",
  fsWriteText: "fs:writeText",
  updatesStatus: "updates:status",
  updatesCheck: "updates:check",
  updatesInstall: "updates:install",
  oauthCallback: "oauth:callback",
} as const;

export type IpcChannel = (typeof IPC_CHANNELS)[keyof typeof IPC_CHANNELS];

export type InvokeFn = (channel: string, ...args: unknown[]) => Promise<unknown>;

export type OAuthListenFn = (
  channel: string,
  listener: (event: unknown, url: string) => void,
) => void;

export type OAuthUnlistenFn = (
  channel: string,
  listener: (event: unknown, url: string) => void,
) => void;

/** Build the object exposed as `window.one`. */
export function buildOneApi(
  invoke: InvokeFn,
  on: OAuthListenFn,
  removeListener: OAuthUnlistenFn,
) {
  return {
    getSession: () => invoke(IPC_CHANNELS.sessionGet),
    setSession: (s: unknown) => invoke(IPC_CHANNELS.sessionSet, s),
    isSessionEncryptionAvailable: () => invoke(IPC_CHANNELS.sessionEncryptionAvailable),
    registerRepoRoot: (repoPath: string) => invoke(IPC_CHANNELS.repoRegisterRoot, repoPath),
    chooseRepoRoot: () => invoke(IPC_CHANNELS.repoChooseRoot),
    chooseLocalFolder: () => invoke(IPC_CHANNELS.repoChooseLocalFolder),
    openExternal: (url: string) => invoke(IPC_CHANNELS.shellOpenExternal, url),
    gitStatus: (repoPath: string) => invoke(IPC_CHANNELS.gitStatus, repoPath),
    gitClone: (url: string, destPath: string) => invoke(IPC_CHANNELS.gitClone, url, destPath),
    gitPull: (repoPath: string) => invoke(IPC_CHANNELS.gitPull, repoPath),
    gitCreateBranch: (repoPath: string, branch: string) =>
      invoke(IPC_CHANNELS.gitCreateBranch, repoPath, branch),
    gitPush: (repoPath: string) => invoke(IPC_CHANNELS.gitPush, repoPath),
    gitCommit: (repoPath: string, message: string) =>
      invoke(IPC_CHANNELS.gitCommit, repoPath, message),
    initSampleRepo: (destPath: string) => invoke(IPC_CHANNELS.repoInitSample, destPath),
    importExportZip: (destPath: string, base64: string) =>
      invoke(IPC_CHANNELS.repoImportExportZip, destPath, base64),
    openInEditor: (repoPath: string, editor: string = "auto") =>
      invoke(IPC_CHANNELS.editorOpen, repoPath, editor),
    exportRepoZip: (repoPath: string, paths?: string[]) =>
      invoke(IPC_CHANNELS.repoExportZip, repoPath, paths ?? []),
    listTree: (root: string, rel?: string) => invoke(IPC_CHANNELS.fsListTree, root, rel),
    readText: (root: string, rel: string) => invoke(IPC_CHANNELS.fsReadText, root, rel),
    writeText: (root: string, rel: string, content: string) =>
      invoke(IPC_CHANNELS.fsWriteText, root, rel, content),
    getUpdateStatus: () => invoke(IPC_CHANNELS.updatesStatus),
    checkForUpdates: () => invoke(IPC_CHANNELS.updatesCheck),
    installUpdate: () => invoke(IPC_CHANNELS.updatesInstall),
    onOAuthCallback: (handler: (url: string) => void): (() => void) => {
      const listener = (_event: unknown, url: string) => handler(url);
      on(IPC_CHANNELS.oauthCallback, listener);
      return () => {
        removeListener(IPC_CHANNELS.oauthCallback, listener);
      };
    },
  };
}

/** Stable list of invoke channels (excludes the push-only oauth callback). */
export function listedInvokeChannels(): string[] {
  return Object.values(IPC_CHANNELS).filter((c) => c !== IPC_CHANNELS.oauthCallback);
}
