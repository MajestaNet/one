import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import {
  RepoRootRegistry,
  assertCreatableRepoDir,
  assertImportExportDest,
  assertRegisterableRoot,
  assertSelectableLocalDir,
  copyDirRecursive,
  isCommitAllowlisted,
  listFilesTree,
  listYamlTree,
  readTextUnderRoot,
  resolveCustomerRepoTemplate,
  resolveUnderRoot,
  writeTextUnderRoot,
} from "./paths";

describe("resolveUnderRoot", () => {
  it("resolves a relative path under root", () => {
    const root = path.resolve("/tmp/one-root");
    expect(resolveUnderRoot(root, "metadata/Account.yaml")).toBe(path.join(root, "metadata/Account.yaml"));
  });

  it("rejects .. escape", () => {
    expect(() => resolveUnderRoot("/tmp/one-root", "../etc/passwd")).toThrow(/path escape/);
  });

  it("rejects absolute escape", () => {
    expect(() => resolveUnderRoot("/tmp/one-root", "/etc/passwd")).toThrow(/path escape/);
  });
});

describe("listYamlTree / read / write", () => {
  const roots: string[] = [];

  afterEach(() => {
    for (const r of roots.splice(0)) {
      fs.rmSync(r, { recursive: true, force: true });
    }
  });

  function tmpRoot(): string {
    const r = fs.mkdtempSync(path.join(os.tmpdir(), "control-ide-"));
    roots.push(r);
    return r;
  }

  it("lists yaml files and ignores non-yaml", () => {
    const root = tmpRoot();
    fs.mkdirSync(path.join(root, "metadata", "objects"), { recursive: true });
    fs.writeFileSync(path.join(root, "metadata", "objects", "Account.yaml"), "apiName: Account\n");
    fs.writeFileSync(path.join(root, "metadata", "objects", "Contact.yml"), "apiName: Contact\n");
    fs.writeFileSync(path.join(root, "metadata", "objects", "notes.txt"), "skip\n");
    expect(listYamlTree(root)).toEqual(["metadata/objects/Account.yaml", "metadata/objects/Contact.yml"]);
  });

  it("returns empty list when metadata missing", () => {
    expect(listYamlTree(tmpRoot())).toEqual([]);
  });

  it("reads and writes text under root", () => {
    const root = tmpRoot();
    writeTextUnderRoot(root, "metadata/objects/Account.yaml", "apiName: Account\n");
    expect(readTextUnderRoot(root, "metadata/objects/Account.yaml")).toBe("apiName: Account\n");
  });

  it("write rejects path escape", () => {
    const root = tmpRoot();
    expect(() => writeTextUnderRoot(root, "../evil.yaml", "x")).toThrow(/path escape/);
  });

  it("lists ts under src/automations", () => {
    const root = tmpRoot();
    fs.mkdirSync(path.join(root, "src", "automations"), { recursive: true });
    fs.writeFileSync(path.join(root, "src", "automations", "a.ts"), "export default async function run() {}");
    expect(listFilesTree(root, "src/automations")).toEqual(["src/automations/a.ts"]);
  });

  it("allowlists commit paths", () => {
    expect(isCommitAllowlisted("metadata/objects/X.yaml")).toBe(true);
    expect(isCommitAllowlisted("src/automations/x.ts")).toBe(true);
    expect(isCommitAllowlisted("one.yaml")).toBe(true);
    expect(isCommitAllowlisted("secrets.env")).toBe(false);
  });

  it("selects empty local folders and import destinations", () => {
    const root = tmpRoot();
    expect(assertSelectableLocalDir(root)).toBe(fs.realpathSync(root));
    expect(assertImportExportDest(root)).toBe(root); // empty → creatable
    fs.writeFileSync(path.join(root, "one.yaml"), "customerId: x\n");
    expect(assertRegisterableRoot(root)).toBe(fs.realpathSync(root));
    expect(assertImportExportDest(root)).toBe(fs.realpathSync(root));
  });

  it("rejects writes under .one/baseline", () => {
    const root = tmpRoot();
    expect(() =>
      writeTextUnderRoot(root, ".one/baseline/objects/Account.yaml", "apiName: Account\n"),
    ).toThrow(/Read-only/);
  });

  it("copies a sample tree into an empty destination", () => {
    const src = tmpRoot();
    const dest = path.join(os.tmpdir(), `control-ide-dest-${Date.now()}`);
    roots.push(dest);
    fs.mkdirSync(path.join(src, "metadata", "objects"), { recursive: true });
    fs.writeFileSync(path.join(src, "one.yaml"), "customerId: demo\n");
    fs.writeFileSync(path.join(src, "metadata", "objects", "Referral__c.yaml"), "apiName: Referral__c\n");
    fs.mkdirSync(path.join(src, ".git"), { recursive: true });
    fs.writeFileSync(path.join(src, ".git", "config"), "skip\n");
    copyDirRecursive(src, dest);
    expect(fs.readFileSync(path.join(dest, "one.yaml"), "utf8")).toBe("customerId: demo\n");
    expect(fs.existsSync(path.join(dest, "metadata", "objects", "Referral__c.yaml"))).toBe(true);
    expect(fs.existsSync(path.join(dest, ".git"))).toBe(false);
  });

  it("rejects copy into a non-empty destination", () => {
    const src = tmpRoot();
    const dest = tmpRoot();
    fs.writeFileSync(path.join(src, "one.yaml"), "x\n");
    fs.writeFileSync(path.join(dest, "existing.txt"), "y\n");
    expect(() => copyDirRecursive(src, dest)).toThrow(/not empty/);
  });

  it("resolves the monorepo customer-repo template when present", () => {
    const found = resolveCustomerRepoTemplate();
    expect(found).toBeTruthy();
    expect(fs.existsSync(path.join(found!, "one.yaml"))).toBe(true);
  });

  it("honors ONE_CUSTOMER_REPO_TEMPLATE env override", () => {
    const src = tmpRoot();
    fs.writeFileSync(path.join(src, "one.yaml"), "customerId: env\n");
    const prev = process.env.ONE_CUSTOMER_REPO_TEMPLATE;
    process.env.ONE_CUSTOMER_REPO_TEMPLATE = src;
    try {
      expect(resolveCustomerRepoTemplate()).toBe(path.resolve(src));
    } finally {
      if (prev === undefined) delete process.env.ONE_CUSTOMER_REPO_TEMPLATE;
      else process.env.ONE_CUSTOMER_REPO_TEMPLATE = prev;
    }
  });
});

describe("repo root policy", () => {
  const cleanup: string[] = [];

  afterEach(() => {
    for (const r of cleanup.splice(0)) fs.rmSync(r, { recursive: true, force: true });
  });

  function customerRepo(): string {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), "control-ide-repo-"));
    cleanup.push(root);
    fs.writeFileSync(path.join(root, "one.yaml"), "customerId: demo\n");
    return root;
  }

  describe("assertRegisterableRoot", () => {
    it("accepts a directory carrying a customer repo marker", () => {
      const root = customerRepo();
      expect(assertRegisterableRoot(root)).toBe(fs.realpathSync(root));
    });

    it("accepts a git checkout without one.yaml", () => {
      const root = fs.mkdtempSync(path.join(os.tmpdir(), "control-ide-git-"));
      cleanup.push(root);
      fs.mkdirSync(path.join(root, ".git"));
      expect(assertRegisterableRoot(root)).toBe(fs.realpathSync(root));
    });

    it("refuses a filesystem root", () => {
      expect(() => assertRegisterableRoot("/")).toThrow(/filesystem root/);
    });

    it("refuses the home directory itself", () => {
      const home = customerRepo();
      expect(() => assertRegisterableRoot(home, { homeDir: home })).toThrow(/home directory/);
    });

    it("refuses system locations", () => {
      expect(() => assertRegisterableRoot("/etc")).toThrow(/not allowed under/);
      expect(() => assertRegisterableRoot("/etc/ssl/private")).toThrow(/not allowed under/);
    });

    it("refuses extra denied directories such as userData", () => {
      const root = customerRepo();
      expect(() => assertRegisterableRoot(root, { denyDirs: [path.dirname(root)] })).toThrow(
        /not allowed under/,
      );
    });

    it("refuses a directory that is not a customer repo", () => {
      const plain = fs.mkdtempSync(path.join(os.tmpdir(), "control-ide-plain-"));
      cleanup.push(plain);
      expect(() => assertRegisterableRoot(plain)).toThrow(/Not a Majesta One customer repo/);
    });

    it("refuses missing paths, empty input, and NUL bytes", () => {
      expect(() => assertRegisterableRoot(path.join(os.tmpdir(), "control-ide-absent"))).toThrow(
        /does not exist/,
      );
      expect(() => assertRegisterableRoot("")).toThrow(/required/);
      expect(() => assertRegisterableRoot("/tmp/x\0y")).toThrow(/NUL byte/);
    });
  });

  describe("assertCreatableRepoDir", () => {
    it("accepts a path that does not exist yet", () => {
      const dest = path.join(os.tmpdir(), `control-ide-new-${Date.now()}`);
      expect(assertCreatableRepoDir(dest)).toBe(path.resolve(dest));
    });

    it("accepts an existing empty directory", () => {
      const dest = fs.mkdtempSync(path.join(os.tmpdir(), "control-ide-empty-"));
      cleanup.push(dest);
      expect(assertCreatableRepoDir(dest)).toBe(path.resolve(dest));
    });

    it("refuses a non-empty directory and system locations", () => {
      const dest = customerRepo();
      expect(() => assertCreatableRepoDir(dest)).toThrow(/not empty/);
      expect(() => assertCreatableRepoDir("/etc/one")).toThrow(/not allowed under/);
      expect(() => assertCreatableRepoDir("/")).toThrow(/filesystem root/);
    });
  });

  describe("RepoRootRegistry", () => {
    it("only resolves roots that were registered", () => {
      const registry = new RepoRootRegistry();
      const root = customerRepo();
      expect(() => registry.require(root)).toThrow(/not registered/);
      registry.register(root);
      expect(registry.require(root)).toBe(fs.realpathSync(root));
    });

    it("refuses a sibling repo that was never registered", () => {
      const registry = new RepoRootRegistry();
      registry.register(customerRepo());
      expect(() => registry.require(customerRepo())).toThrow(/not registered/);
    });

    it("refuses the renderer naming a filesystem root even after a real registration", () => {
      const registry = new RepoRootRegistry();
      registry.register(customerRepo());
      expect(() => registry.require("/")).toThrow(/not registered/);
      expect(() => registry.require("/etc")).toThrow(/not registered/);
    });

    it("tryRegister swallows policy failures", () => {
      const registry = new RepoRootRegistry();
      expect(registry.tryRegister("/etc")).toBeNull();
      expect(registry.tryRegister(undefined)).toBeNull();
      expect(registry.list()).toEqual([]);
    });

    it("accepts a trailing-slash or unnormalized form of a registered root", () => {
      const registry = new RepoRootRegistry();
      const root = registry.register(customerRepo());
      expect(registry.require(`${root}/`)).toBe(root);
      expect(registry.require(path.join(root, "metadata", ".."))).toBe(root);
    });
  });
});
