/**
 * Opening links outside the app.
 *
 * In Electron this must go through the main process: renderer-created windows inherit the
 * parent's `webPreferences`, preload included, so a `window.open` login page would hold the
 * `window.one` bridge (CIDE-01). The main process denies every renderer-initiated
 * window and hands vetted URLs to the OS browser.
 */
export async function openExternalUrl(url: string): Promise<void> {
  const openExternal = window.one?.openExternal;
  if (openExternal) {
    const res = await openExternal(url);
    if (!res.ok) throw new Error(res.error ?? "Failed to open the browser");
    return;
  }

  // Browser preview (`npm run dev:web`) has no Electron shell and no preload to leak.
  // eslint-disable-next-line no-restricted-properties -- browser-only fallback, no preload bridge exists here
  const opened = window.open(url, "_blank", "noopener,noreferrer");
  if (!opened) throw new Error("Popup blocked — allow pop-ups to complete sign-in");
}
