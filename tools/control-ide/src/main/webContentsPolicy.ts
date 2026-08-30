/**
 * Pure decisions for renderer window-open / navigation policy (CIDE-01, CIDE-06).
 * Electron event wiring (`setWindowOpenHandler`, `will-navigate`) stays in main.
 */
import { isSafeExternalUrl, isTrustedFrameUrl, type FrameTrust } from "./security.js";

export type WindowOpenDecision = {
  /** Always deny in-app windows — never hand a child the preload bridge. */
  action: "deny";
  /** When set, the main process should open this URL in the OS browser. */
  openExternally?: string;
};

/** Deny every renderer-initiated window; route vetted URLs through `shell.openExternal`. */
export function decideWindowOpen(url: string): WindowOpenDecision {
  if (isSafeExternalUrl(url)) {
    return { action: "deny", openExternally: url };
  }
  return { action: "deny" };
}

/** Allow in-app navigation only to the app's own renderer document. */
export function shouldAllowNavigation(url: string, trust: FrameTrust): boolean {
  return isTrustedFrameUrl(url, trust);
}

/**
 * Decide whether to prevent a navigation and optionally hand the URL to the OS browser.
 * Returning `prevent: false` means the navigation may proceed in-app.
 */
export function decideNavigation(
  url: string,
  trust: FrameTrust,
): { prevent: boolean; openExternally?: string } {
  if (shouldAllowNavigation(url, trust)) {
    return { prevent: false };
  }
  if (isSafeExternalUrl(url)) {
    return { prevent: true, openExternally: url };
  }
  return { prevent: true };
}
