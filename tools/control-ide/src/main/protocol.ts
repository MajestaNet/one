/**
 * Custom-protocol helpers for the `one-control://` OAuth deep link (CIDE-11).
 * Pure argv parsing — cold-start / second-instance wiring stays in main.
 */

export const PROTOCOL = "one-control";

/** Packaged-app / fallback PKCE redirect (also seeded on the install). */
export const CUSTOM_PROTOCOL_REDIRECT_URI = "one-control://oauth/callback";

/** Return the first argv entry that is a URL for the given scheme (e.g. `one-control`). */
export function extractProtocolUrl(argv: readonly string[], protocol: string): string | undefined {
  const prefix = `${protocol}://`;
  return argv.find((a) => typeof a === "string" && a.startsWith(prefix));
}

/** True when `url` is a deep link for the app's custom protocol. */
export function isAppProtocolUrl(url: string, protocol: string): boolean {
  return String(url ?? "").startsWith(`${protocol}://`);
}

export type ProtocolClientContext = {
  packaged: boolean;
  defaultApp: boolean;
  platform: string;
};

/**
 * Unpackaged macOS must not call `setAsDefaultProtocolClient`.
 * On darwin the `path`/`args` parameters are ignored, so the handler becomes Electron.app.
 * Completing PKCE then launches a second Electron (different userData) instead of focusing
 * the running Control IDE. Packaged builds declare the scheme in Info.plist instead.
 */
export function shouldRegisterProtocolClient(ctx: ProtocolClientContext): boolean {
  if (ctx.platform === "darwin" && !ctx.packaged) return false;
  return true;
}

export type ProtocolClientLaunch = {
  /** When set, pass as the executable + args (Windows/Linux unpackaged). */
  execPath?: string;
  args?: string[];
};

/** Args for `app.setAsDefaultProtocolClient` when registration is allowed. */
export function protocolClientLaunch(
  ctx: ProtocolClientContext & { execPath: string; argv: readonly string[] },
): ProtocolClientLaunch {
  if (ctx.defaultApp && ctx.argv.length >= 2) {
    return { execPath: ctx.execPath, args: [ctx.argv[1]!] };
  }
  return {};
}
