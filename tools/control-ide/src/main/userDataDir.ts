/**
 * Parse an explicit Electron `--user-data-dir` from process argv.
 * Chromium may inject a default later; we only treat a flag present on argv
 * as an isolated profile (S-E dual-IDE recipe).
 */

export function parseUserDataDirFlag(argv: readonly string[]): string | undefined {
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === "--user-data-dir" || arg === "--userDataDir") {
      const next = argv[i + 1];
      if (next && !next.startsWith("-")) return next;
      return undefined;
    }
    if (arg.startsWith("--user-data-dir=")) {
      const value = arg.slice("--user-data-dir=".length).trim();
      return value || undefined;
    }
  }
  return undefined;
}

/** Default profile keeps the single-instance lock for one-control:// OAuth. Isolated dirs skip it. */
export function shouldTakeSingleInstanceLock(argv: readonly string[]): boolean {
  return parseUserDataDirFlag(argv) == null;
}
