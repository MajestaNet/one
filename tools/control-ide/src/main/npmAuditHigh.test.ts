import { describe, expect, it, vi } from "vitest";
import {
  classifyNpmAuditOutput,
  decideGate,
  evaluateOsvFindings,
  ghsaSeverityIsHigh,
  lockfileToQueries,
  packageNameFromLockPath,
  runAuditHigh,
} from "../../scripts/npm-audit-high.mjs";

describe("classifyNpmAuditOutput", () => {
  it("honours a real npm audit report with high findings", () => {
    const result = classifyNpmAuditOutput({
      exitCode: 1,
      stdout: JSON.stringify({
        auditReportVersion: 2,
        metadata: { vulnerabilities: { info: 0, low: 1, moderate: 2, high: 1, critical: 0, total: 4 } },
      }),
    });
    expect(result).toEqual({ kind: "audited", high: 1, critical: 0, total: 4 });
  });

  it("treats a clean report as audited with zero high/critical", () => {
    const result = classifyNpmAuditOutput({
      exitCode: 0,
      stdout: JSON.stringify({
        metadata: { vulnerabilities: { info: 0, low: 1, moderate: 2, high: 0, critical: 0, total: 3 } },
      }),
    });
    expect(result.kind).toBe("audited");
    expect(result).toMatchObject({ high: 0, critical: 0 });
  });

  it("classifies the retired /quick 400 as unreachable, not a finding", () => {
    const result = classifyNpmAuditOutput({
      exitCode: 1,
      stderr: [
        "npm warn audit 400 Bad Request - POST https://registry.npmjs.org/-/npm/v1/security/audits/quick",
        '{ "statusCode": 400, "error": "Bad Request", "message": "Invalid package tree, run  npm install  to rebuild your package-lock.json" }',
        "npm error audit endpoint returned an error",
      ].join("\n"),
    });
    expect(result.kind).toBe("unreachable");
    expect(result.reason).toMatch(/400|package tree|endpoint/i);
  });

  it("classifies a 503 on the audit endpoint as unreachable", () => {
    const result = classifyNpmAuditOutput({
      exitCode: 1,
      stderr: "npm warn audit 503 Service Unavailable - POST https://registry.npmjs.org/-/npm/v1/security/audits/quick",
    });
    expect(result.kind).toBe("unreachable");
  });

  it("classifies a timeout as unreachable", () => {
    expect(classifyNpmAuditOutput({ timedOut: true }).kind).toBe("unreachable");
  });

  it("fails closed on unreadable output even when npm exits 0", () => {
    const result = classifyNpmAuditOutput({ exitCode: 0, stdout: "not-json" });
    expect(result.kind).toBe("unreadable");
  });
});

describe("lockfileToQueries", () => {
  it("extracts unique name@version pairs from a v3 lockfile", () => {
    const queries = lockfileToQueries({
      packages: {
        "": { name: "@one/control-ide", version: "0.1.0" },
        "node_modules/fast-uri": { version: "4.1.4" },
        "node_modules/ajv/node_modules/fast-uri": { version: "4.1.4" },
        "node_modules/@xmldom/xmldom": { version: "0.8.14" },
      },
    });
    expect(queries).toEqual([
      { package: { name: "fast-uri", ecosystem: "npm" }, version: "4.1.4" },
      { package: { name: "@xmldom/xmldom", ecosystem: "npm" }, version: "0.8.14" },
    ]);
  });

  it("reads scoped names from nested node_modules paths", () => {
    expect(packageNameFromLockPath("node_modules/foo/node_modules/@scope/pkg", undefined)).toBe("@scope/pkg");
  });
});

describe("OSV severity + gate decision", () => {
  it("treats GHSA HIGH/CRITICAL as blocking and MODERATE as not", () => {
    expect(ghsaSeverityIsHigh({ database_specific: { severity: "HIGH" } })).toBe(true);
    expect(ghsaSeverityIsHigh({ database_specific: { severity: "critical" } })).toBe(true);
    expect(ghsaSeverityIsHigh({ database_specific: { severity: "MODERATE" } })).toBe(false);
  });

  it("collects high findings against the queried versions", () => {
    const queries = [{ package: { name: "fast-uri", ecosystem: "npm" }, version: "4.1.2" }];
    const details = new Map([
      ["GHSA-5jgf-p345-68v8", { database_specific: { severity: "HIGH" }, summary: "host confusion" }],
    ]);
    const findings = evaluateOsvFindings(queries, details, [{ vulns: [{ id: "GHSA-5jgf-p345-68v8" }] }]);
    expect(findings).toEqual([
      {
        id: "GHSA-5jgf-p345-68v8",
        severity: "HIGH",
        name: "fast-uri",
        version: "4.1.2",
        summary: "host confusion",
      },
    ]);
  });

  it("blocks on npm high counts and ignores OSV in that case", () => {
    const decision = decideGate({ kind: "audited", high: 1, critical: 0 }, { kind: "audited", findings: [] });
    expect(decision.ok).toBe(false);
    expect(decision.source).toBe("npm");
  });

  it("passes a clean npm report", () => {
    expect(decideGate({ kind: "audited", high: 0, critical: 0 }, null).ok).toBe(true);
  });

  it("falls back to OSV when npm is unreachable and still blocks high+", () => {
    const decision = decideGate(
      { kind: "unreachable", reason: "HTTP 400" },
      { kind: "audited", findings: [{ id: "GHSA-x", severity: "HIGH", name: "fast-uri", version: "4.1.2" }] },
    );
    expect(decision.ok).toBe(false);
    expect(decision.source).toBe("osv");
  });

  it("passes when npm is unreachable and OSV is clean", () => {
    const decision = decideGate({ kind: "unreachable", reason: "timeout" }, { kind: "audited", findings: [] });
    expect(decision.ok).toBe(true);
    expect(decision.source).toBe("osv");
  });

  it("fails closed when neither source produced a report", () => {
    const decision = decideGate({ kind: "unreachable", reason: "timeout" }, { kind: "unreadable", reason: "OSV 503" });
    expect(decision.ok).toBe(false);
    expect(decision.source).toBe("unreadable");
  });
});

describe("runAuditHigh", () => {
  it("does not call OSV when npm returns a clean report", async () => {
    const osvQuery = vi.fn();
    const code = await runAuditHigh({
      npmAudit: async () => ({
        exitCode: 0,
        stdout: JSON.stringify({ metadata: { vulnerabilities: { high: 0, critical: 0, total: 0 } } }),
      }),
      osvQuery,
      log: { log: vi.fn(), warn: vi.fn(), error: vi.fn() },
    });
    expect(code).toBe(0);
    expect(osvQuery).not.toHaveBeenCalled();
  });

  it("uses OSV when npm's retired endpoint 400s and fails on a high GHSA", async () => {
    const log = { log: vi.fn(), warn: vi.fn(), error: vi.fn() };
    const code = await runAuditHigh({
      lockfilePath: "package-lock.json",
      npmAudit: async () => ({
        exitCode: 1,
        stderr: "npm error audit endpoint returned an error\nInvalid package tree",
      }),
      readLockfile: () => ({ packages: { "node_modules/fast-uri": { version: "4.1.2" } } }),
      osvQuery: async () => [{ vulns: [{ id: "GHSA-5jgf-p345-68v8" }] }],
      osvDetails: async () =>
        new Map([["GHSA-5jgf-p345-68v8", { database_specific: { severity: "HIGH" }, summary: "host confusion" }]]),
      log,
    });
    expect(code).toBe(1);
    expect(log.warn).toHaveBeenCalled();
    expect(String(log.error.mock.calls.flat().join("\n"))).toMatch(/GHSA-5jgf-p345-68v8/);
  });

  it("passes the OSV fallback when the lockfile has no high findings", async () => {
    const code = await runAuditHigh({
      lockfilePath: "package-lock.json",
      npmAudit: async () => ({ timedOut: true, stdout: "", stderr: "" }),
      readLockfile: () => ({ packages: { "node_modules/fast-uri": { version: "4.1.4" } } }),
      osvQuery: async () => [{}],
      osvDetails: async () => new Map(),
      log: { log: vi.fn(), warn: vi.fn(), error: vi.fn() },
    });
    expect(code).toBe(0);
  });
});
