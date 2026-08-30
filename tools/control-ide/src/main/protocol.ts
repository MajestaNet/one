/**
 * Custom-protocol helpers for the `one-control://` OAuth deep link (CIDE-11).
 * Pure argv parsing — cold-start / second-instance wiring stays in main.
 */

/** Return the first argv entry that is a URL for the given scheme (e.g. `one-control`). */
export function extractProtocolUrl(argv: readonly string[], protocol: string): string | undefined {
  const prefix = `${protocol}://`;
  return argv.find((a) => typeof a === "string" && a.startsWith(prefix));
}

/** True when `url` is a deep link for the app's custom protocol. */
export function isAppProtocolUrl(url: string, protocol: string): boolean {
  return String(url ?? "").startsWith(`${protocol}://`);
}
