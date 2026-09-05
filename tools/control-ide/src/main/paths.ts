import fs from "node:fs";
import os from "node:os";
import path from "node:path";

/** Resolve `rel` under `root` and reject path escape attempts. */
export function resolveUnderRoot(root: string, rel: string): string {
  const base = path.resolve(root);
  const full = path.resolve(base, rel);
  const relToBase = path.relative(base, full);
  if (relToBase.startsWith("..") || path.isAbsolute(relToBase)) {
    throw new Error("path escape");
  }
  return full;
}

/**
 * Directories a customer repo may never live in. `resolveUnderRoot` only constrains the
 * relative half of a path; the root itself needs its own policy or the renderer can name
 * `/` and read anything the user can (CIDE-04).
 */
const DENIED_ROOT_PREFIXES = [
  "/etc",
  "/usr",
  "/bin",
  "/sbin",
  "/lib",
  "/boot",
  "/dev",
  "/proc",
  "/sys",
  "/var",
  "/System",
  "/Library",
  "C:\\Windows",
  "C:\\Program Files",
] as const;

/** Files that mark a directory as a Majesta One customer repo. */
const REPO_MARKERS = [".git", "one.yaml", "one.yml"] as const;

export type RootPolicyOptions = {
  /** Extra directories to refuse (e.g. Electron `userData`). */
  denyDirs?: string[];
  /** Overridable for tests. */
  homeDir?: string;
};

function normalizeForCompare(p: string): string {
  return process.platform === "win32" ? p.toLowerCase() : p;
}

function isWithin(child: string, parent: string): boolean {
  const rel = path.relative(normalizeForCompare(parent), normalizeForCompare(child));
  return rel === "" || (!rel.startsWith("..") && !path.isAbsolute(rel));
}

/**
 * Shared checks for any directory the renderer names: absolute, not a filesystem root, not
 * the home directory itself, and not inside a system or app-data location.
 */
function assertUsableDirectory(candidate: unknown, options: RootPolicyOptions = {}): string {
  const raw = typeof candidate === "string" ? candidate.trim() : "";
  if (!raw) throw new Error("Repo path is required");
  if (raw.includes("\0")) throw new Error("Repo path contains a NUL byte");

  const resolved = path.resolve(raw);
  if (resolved === path.parse(resolved).root) {
    throw new Error("Repo path must not be a filesystem root");
  }

  const home = options.homeDir ?? os.homedir();
  if (home && normalizeForCompare(resolved) === normalizeForCompare(path.resolve(home))) {
    throw new Error("Repo path must not be the home directory itself");
  }

  for (const denied of [...DENIED_ROOT_PREFIXES, ...(options.denyDirs ?? [])]) {
    if (isWithin(resolved, path.resolve(denied))) {
      throw new Error(`Repo path is not allowed under ${denied}`);
    }
  }

  return resolved;
}

/**
 * Validate a directory the IDE may treat as a customer repo root. Must already exist and
 * carry a repo marker, so a renderer cannot promote an arbitrary directory.
 */
export function assertRegisterableRoot(candidate: unknown, options: RootPolicyOptions = {}): string {
  const resolved = assertUsableDirectory(candidate, options);

  let stat: fs.Stats;
  try {
    stat = fs.statSync(resolved);
  } catch {
    throw new Error(`Repo path does not exist: ${resolved}`);
  }
  if (!stat.isDirectory()) throw new Error(`Repo path is not a directory: ${resolved}`);

  const hasMarker = REPO_MARKERS.some((marker) => fs.existsSync(path.join(resolved, marker)));
  if (!hasMarker) {
    throw new Error(`Not a Majesta One customer repo (expected .git or one.yaml): ${resolved}`);
  }

  return fs.realpathSync(resolved);
}

/**
 * Validate a directory the user picked as a local workspace folder. Unlike
 * assertRegisterableRoot, the folder may be empty (no .git / one.yaml yet).
 */
export function assertSelectableLocalDir(candidate: unknown, options: RootPolicyOptions = {}): string {
  const resolved = assertUsableDirectory(candidate, options);
  let stat: fs.Stats;
  try {
    stat = fs.statSync(resolved);
  } catch {
    throw new Error(`Path does not exist: ${resolved}`);
  }
  if (!stat.isDirectory()) throw new Error(`Path is not a directory: ${resolved}`);
  return fs.realpathSync(resolved);
}

/**
 * Destination for install export unpack: existing customer repo (overwrite) or empty/creatable dir.
 */
export function assertImportExportDest(candidate: unknown, options: RootPolicyOptions = {}): string {
  try {
    return assertRegisterableRoot(candidate, options);
  } catch {
    return assertCreatableRepoDir(candidate, options);
  }
}

/**
 * Validate a destination for clone / sample init. The directory need not exist yet, so the
 * repo-marker check does not apply — but the system-location policy still does.
 */
export function assertCreatableRepoDir(candidate: unknown, options: RootPolicyOptions = {}): string {
  const resolved = assertUsableDirectory(candidate, options);
  if (fs.existsSync(resolved)) {
    if (!fs.statSync(resolved).isDirectory()) {
      throw new Error(`Destination is not a directory: ${resolved}`);
    }
    if (fs.readdirSync(resolved).length > 0) {
      throw new Error(`Destination not empty: ${resolved}`);
    }
  }
  return resolved;
}

/**
 * In-memory allowlist of repo roots this session may touch. Roots enter it only through
 * `assertRegisterableRoot`, so `fs:*` and `git:*` handlers can trust the root they are
 * given without trusting the renderer that sent it.
 */
export class RepoRootRegistry {
  private readonly roots = new Set<string>();

  constructor(private readonly options: RootPolicyOptions = {}) {}

  /** Register a root, returning its canonical path. Throws if the policy rejects it. */
  register(candidate: unknown): string {
    const resolved = assertRegisterableRoot(candidate, this.options);
    this.roots.add(normalizeForCompare(resolved));
    return resolved;
  }

  /** Register without throwing — for restoring a persisted session at startup. */
  tryRegister(candidate: unknown): string | null {
    try {
      return this.register(candidate);
    } catch {
      return null;
    }
  }

  /** Resolve a root the renderer named, or throw if it was never registered. */
  require(candidate: unknown): string {
    const raw = typeof candidate === "string" ? candidate.trim() : "";
    if (!raw) throw new Error("Repo path is required");
    const resolved = path.resolve(raw);
    const real = fs.existsSync(resolved) ? fs.realpathSync(resolved) : resolved;
    for (const known of [resolved, real]) {
      if (this.roots.has(normalizeForCompare(known))) return real;
    }
    throw new Error(
      `Repo path is not registered for this session: ${resolved}. Save or choose the repo path first.`,
    );
  }

  list(): string[] {
    return [...this.roots];
  }

  clear(): void {
    this.roots.clear();
  }
}

const YAML_EXT = /\.ya?ml$/i;
const DEFAULT_SOURCE_EXT = /\.(ya?ml|ts|tsx|js|json)$/i;

/** Paths safe to stage for customer-repo commits. */
export const COMMIT_ALLOW_PREFIXES = [
  "metadata/",
  "src/automations/",
  "tests/",
  "environments/",
  "changes/",
  ".one/",
] as const;

export const COMMIT_ALLOW_FILES = new Set(["one.yaml", "one.yml", "README.md"]);

export function isCommitAllowlisted(rel: string): boolean {
  const n = rel.replace(/\\/g, "/").replace(/^\.\//, "");
  if (COMMIT_ALLOW_FILES.has(n)) return true;
  return COMMIT_ALLOW_PREFIXES.some((p) => n.startsWith(p));
}

/** List YAML files under `root/rel` (default `metadata`), paths relative to `root`. */
export function listYamlTree(root: string, rel = "metadata"): string[] {
  return listFilesTree(root, rel, YAML_EXT);
}

/** List source/text files under `root/rel` matching `ext`. */
export function listFilesTree(root: string, rel = "metadata", ext: RegExp = DEFAULT_SOURCE_EXT): string[] {
  const base = resolveUnderRoot(root, rel);
  const out: string[] = [];
  const walk = (dir: string) => {
    if (!fs.existsSync(dir)) return;
    for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, ent.name);
      if (ent.isDirectory()) walk(full);
      else if (ext.test(ent.name)) out.push(path.relative(root, full).replace(/\\/g, "/"));
    }
  };
  walk(base);
  return out.sort();
}

export function readTextUnderRoot(root: string, rel: string): string {
  return fs.readFileSync(resolveUnderRoot(root, rel), "utf8");
}

export function writeTextUnderRoot(root: string, rel: string, content: string): void {
  const normalized = rel.replace(/\\/g, "/").replace(/^\.\//, "");
  if (normalized === ".one/baseline" || normalized.startsWith(".one/baseline/")) {
    throw new Error("Read-only: .one/baseline is managed reference metadata");
  }
  const full = resolveUnderRoot(root, rel);
  fs.mkdirSync(path.dirname(full), { recursive: true });
  fs.writeFileSync(full, content, "utf8");
}

/** Recursively copy a directory into dest (dest must be empty or nonexistent). */
export function copyDirRecursive(src: string, dest: string): void {
  const srcAbs = path.resolve(src);
  if (!fs.existsSync(srcAbs) || !fs.statSync(srcAbs).isDirectory()) {
    throw new Error(`Sample template not found: ${srcAbs}`);
  }
  if (fs.existsSync(dest) && fs.readdirSync(dest).length > 0) {
    throw new Error(`Destination not empty: ${dest}`);
  }
  fs.mkdirSync(dest, { recursive: true });
  const walk = (from: string, to: string) => {
    for (const ent of fs.readdirSync(from, { withFileTypes: true })) {
      if (ent.name === ".git") continue;
      const srcPath = path.join(from, ent.name);
      const destPath = path.join(to, ent.name);
      if (ent.isDirectory()) {
        fs.mkdirSync(destPath, { recursive: true });
        walk(srcPath, destPath);
      } else {
        fs.copyFileSync(srcPath, destPath);
      }
    }
  };
  walk(srcAbs, dest);
}

/** Resolve deploy/customer-repo-template from env or known monorepo locations. */
export function resolveCustomerRepoTemplate(): string | null {
  const fromEnv = process.env.ONE_CUSTOMER_REPO_TEMPLATE?.trim();
  if (fromEnv && fs.existsSync(path.join(fromEnv, "one.yaml"))) return path.resolve(fromEnv);

  const candidates = [
    path.resolve(process.cwd(), "deploy", "customer-repo-template"),
    path.resolve(process.cwd(), "..", "..", "deploy", "customer-repo-template"),
    path.resolve(process.cwd(), "..", "deploy", "customer-repo-template"),
  ];
  for (const c of candidates) {
    if (fs.existsSync(path.join(c, "one.yaml"))) return c;
  }
  return null;
}

/** Packaged/dev locations for the Majesta globe window icon (not the Electron default). */
export function resolveAppIconPath(
  fromDir: string,
  exists: (p: string) => boolean = (p) => {
    try {
      return fs.existsSync(p);
    } catch {
      return false;
    }
  },
): string | undefined {
  const candidates = [
    path.join(fromDir, "app-icon.png"),
    path.join(fromDir, "dist/app-icon.png"),
    path.join(fromDir, "../dist/app-icon.png"),
    path.join(fromDir, "../public/app-icon.png"),
    path.join(fromDir, "public/app-icon.png"),
    path.join(fromDir, "../assets/brand/app-icon.png"),
    path.join(fromDir, "assets/brand/app-icon.png"),
  ];
  return candidates.find((p) => exists(path.resolve(p)));
}

