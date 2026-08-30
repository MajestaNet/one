/** Parse a git status --porcelain line into a repo-relative path. */
export function parsePorcelainPath(line: string): string | null {
  const raw = line.trimEnd();
  if (!raw) return null;
  // Rename: "R  old -> new" or "RM old -> new"
  const rename = raw.match(/^[A-Z? ]{1,2}\s+(.+?)\s+->\s+(.+)$/);
  if (rename) return rename[2].trim();
  // Standard: " M path" / "?? path" (status is first two columns)
  if (raw.length < 4) return null;
  return raw.slice(3).trim() || null;
}

/** True when a relative path is editable metadata YAML in the Metadata panel. */
export function isMetadataYaml(rel: string): boolean {
  const n = rel.replace(/\\/g, "/");
  return n.startsWith("metadata/") && /\.ya?ml$/i.test(n);
}

/** Automation YAML or guest TypeScript under the customer repo. */
export function isAutomationPath(rel: string): boolean {
  const n = rel.replace(/\\/g, "/");
  if (n.startsWith("metadata/automations/") && /\.ya?ml$/i.test(n)) return true;
  if (n.startsWith("src/automations/") && /\.tsx?$/i.test(n)) return true;
  if (n.startsWith("tests/automations/") && /\.tsx?$/i.test(n)) return true;
  return false;
}
