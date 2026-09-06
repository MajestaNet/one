/**
 * Open a vetted URL in the operator's OS browser (CIDE-01).
 *
 * On macOS, `shell.openExternal` for loopback `http://` can hand the URL back to
 * Electron (Launch Services picks the running app), which looks like a second IDE
 * and never reaches Safari/Chrome. `/usr/bin/open` with a single argv URL avoids that.
 */

export type OpenInOsBrowserDeps = {
  platform: string;
  openExternal: (url: string) => Promise<void>;
  /** macOS: `execFile("/usr/bin/open", [url])` — no shell, so query `&` stays in the URL. */
  openWithSystemOpener: (url: string) => Promise<void>;
};

export async function openInOsBrowser(url: string, deps: OpenInOsBrowserDeps): Promise<void> {
  if (deps.platform === "darwin") {
    await deps.openWithSystemOpener(url);
    return;
  }
  await deps.openExternal(url);
}
