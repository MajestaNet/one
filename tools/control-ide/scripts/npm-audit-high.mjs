#!/usr/bin/env node
/**
 * Control IDE high+ advisory gate (CIDE-14 / BP-026).
 *
 * `npm audit` still talks to `/-/npm/v1/security/advisories/bulk` and, on a
 * timeout or malformed gzip body, falls back to the retired
 * `/-/npm/v1/security/audits/quick` endpoint. That fallback returns 400
 * "Invalid package tree" or 503 after several minutes — the same exit code 1
 * as a real high finding — so CI cannot tell the two apart.
 *
 * This script:
 *   1. Tries `npm audit --json` with a short timeout.
 *   2. If npm returns a real report, fails on high/critical counts.
 *   3. If npm's audit API is unreachable, retired, or unreadable, scans the
 *      lockfile against the OSV API using GHSA `database_specific.severity`
 *      (the same severity npm uses) and fails on high+.
 *   4. Fails closed if neither source can be read.
 */
import { spawn } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

export const NPM_AUDIT_TIMEOUT_MS = 25_000;
export const OSV_QUERY_URL = "https://api.osv.dev/v1/querybatch";
export const OSV_VULN_URL = "https://api.osv.dev/v1/vulns";
const HIGH_SEVERITIES = new Set(["HIGH", "CRITICAL"]);

export function classifyNpmAuditOutput({ stdout = "", stderr = "", exitCode = null, timedOut = false } = {}) {
  if (timedOut) {
    return { kind: "unreachable", reason: "npm audit timed out" };
  }
  const text = `${stdout}\n${stderr}`;
  let parsed = null;
  const jsonSlice = extractJsonObject(stdout) || extractJsonObject(stderr);
  if (jsonSlice) {
    try {
      parsed = JSON.parse(jsonSlice);
    } catch {
      parsed = null;
    }
  }
  if (parsed && parsed.metadata && parsed.metadata.vulnerabilities) {
    const counts = parsed.metadata.vulnerabilities;
    return {
      kind: "audited",
      high: Number(counts.high) || 0,
      critical: Number(counts.critical) || 0,
      total: Number(counts.total) || 0,
    };
  }
  if (isNpmAuditTransportFailure(parsed, text, exitCode)) {
    return { kind: "unreachable", reason: npmUnreachableReason(parsed, text) };
  }
  return {
    kind: "unreadable",
    reason: `npm audit returned no report (exit ${exitCode ?? "unknown"})`,
  };
}

export function extractJsonObject(text) {
  if (!text) return null;
  const start = text.indexOf("{");
  const end = text.lastIndexOf("}");
  if (start < 0 || end <= start) return null;
  return text.slice(start, end + 1);
}

export function isNpmAuditTransportFailure(parsed, text, exitCode) {
  const blob = `${text}`.toLowerCase();
  if (parsed && (parsed.statusCode === 400 || parsed.statusCode === 410 || parsed.statusCode === 503)) {
    return true;
  }
  if (parsed && parsed.error && !parsed.metadata) {
    return true;
  }
  if (
    blob.includes("audit endpoint returned an error") ||
    blob.includes("invalid package tree") ||
    blob.includes("security/audits/quick") ||
    blob.includes("invalid json response body") ||
    blob.includes("this endpoint is being retired") ||
    blob.includes("service unavailable")
  ) {
    return true;
  }
  return exitCode === 1 && blob.includes("audit endpoint");
}

function npmUnreachableReason(parsed, text) {
  if (parsed?.statusCode) {
    return `npm audit endpoint HTTP ${parsed.statusCode}`;
  }
  if (/invalid package tree/i.test(text)) return "npm audit rejected the package tree (retired /quick endpoint)";
  if (/invalid json response body/i.test(text)) return "npm audit bulk response was not JSON";
  if (/timed? ?out|aborted/i.test(text)) return "npm audit timed out";
  return "npm audit endpoint unreachable";
}

export function lockfileToQueries(lock) {
  const queries = [];
  const seen = new Set();
  for (const [pkgPath, meta] of Object.entries(lock?.packages || {})) {
    if (!meta || typeof meta.version !== "string" || !pkgPath) continue;
    const name = packageNameFromLockPath(pkgPath, meta.name);
    if (!name) continue;
    const key = `${name}@${meta.version}`;
    if (seen.has(key)) continue;
    seen.add(key);
    queries.push({ package: { name, ecosystem: "npm" }, version: meta.version });
  }
  return queries;
}

export function packageNameFromLockPath(pkgPath, explicitName) {
  if (explicitName && pkgPath !== "") return explicitName;
  const marker = "node_modules/";
  const idx = pkgPath.lastIndexOf(marker);
  if (idx < 0) return null;
  return pkgPath.slice(idx + marker.length) || null;
}

export function ghsaSeverityIsHigh(detail) {
  const raw = detail?.database_specific?.severity;
  return typeof raw === "string" && HIGH_SEVERITIES.has(raw.toUpperCase());
}

export function evaluateOsvFindings(queries, detailsById, results) {
  const findings = [];
  for (let i = 0; i < queries.length; i += 1) {
    const vulns = results[i]?.vulns || [];
    for (const vuln of vulns) {
      const detail = detailsById.get(vuln.id);
      if (!detail || !ghsaSeverityIsHigh(detail)) continue;
      findings.push({
        id: vuln.id,
        severity: detail.database_specific.severity,
        name: queries[i].package.name,
        version: queries[i].version,
        summary: (detail.summary || "").split("\n")[0],
      });
    }
  }
  return findings;
}

export function decideGate(npmResult, osvResult) {
  if (npmResult?.kind === "audited") {
    const blocking = (npmResult.high || 0) + (npmResult.critical || 0);
    if (blocking > 0) {
      return {
        ok: false,
        source: "npm",
        message: `npm audit reported ${npmResult.high} high and ${npmResult.critical} critical`,
      };
    }
    return { ok: true, source: "npm", message: "npm audit reported no high/critical findings" };
  }
  if (osvResult?.kind === "audited") {
    if (osvResult.findings.length > 0) {
      return {
        ok: false,
        source: "osv",
        message: `OSV reported ${osvResult.findings.length} high/critical finding(s)`,
        findings: osvResult.findings,
      };
    }
    return { ok: true, source: "osv", message: "OSV reported no high/critical findings" };
  }
  return {
    ok: false,
    source: "unreadable",
    message: `advisory gate could not run (${npmResult?.reason || "npm unread"}; ${osvResult?.reason || "osv unread"})`,
  };
}

export function runNpmAuditProcess({
  cwd,
  timeoutMs = NPM_AUDIT_TIMEOUT_MS,
  spawnFn = spawn,
} = {}) {
  return new Promise((resolve) => {
    const child = spawnFn("npm", ["audit", "--json", "--audit-level=high"], {
      cwd,
      env: process.env,
      stdio: ["ignore", "pipe", "pipe"],
    });
    let stdout = "";
    let stderr = "";
    let settled = false;
    const finish = (result) => {
      if (settled) return;
      settled = true;
      resolve(result);
    };
    child.stdout?.on("data", (chunk) => {
      stdout += chunk;
    });
    child.stderr?.on("data", (chunk) => {
      stderr += chunk;
    });
    const timer = setTimeout(() => {
      child.kill("SIGTERM");
      finish({ timedOut: true, stdout, stderr, exitCode: null });
    }, timeoutMs);
    child.on("close", (code) => {
      clearTimeout(timer);
      finish({ timedOut: false, stdout, stderr, exitCode: code });
    });
    child.on("error", (err) => {
      clearTimeout(timer);
      finish({ timedOut: false, stdout, stderr: `${stderr}${err.message}`, exitCode: 1 });
    });
  });
}

export async function queryOsv(queries, { fetchFn = fetch, chunkSize = 100 } = {}) {
  const results = [];
  for (let i = 0; i < queries.length; i += chunkSize) {
    const batch = queries.slice(i, i + chunkSize);
    const json = await fetchJson(
      OSV_QUERY_URL,
      {
        method: "POST",
        headers: { "content-type": "application/json", accept: "application/json" },
        body: JSON.stringify({ queries: batch }),
      },
      fetchFn,
    );
    results.push(...(json.results || []));
  }
  return results;
}

export async function loadOsvDetails(ids, { fetchFn = fetch } = {}) {
  const detailsById = new Map();
  for (const id of ids) {
    const detail = await fetchJson(`${OSV_VULN_URL}/${encodeURIComponent(id)}`, {}, fetchFn);
    detailsById.set(id, detail);
  }
  return detailsById;
}

async function fetchJson(url, init, fetchFn, attempts = 3) {
  let lastErr;
  for (let i = 1; i <= attempts; i += 1) {
    try {
      const res = await fetchFn(url, { ...init, signal: AbortSignal.timeout(30_000) });
      const text = await res.text();
      if (!res.ok) {
        throw new Error(`HTTP ${res.status} ${text.slice(0, 180)}`);
      }
      return JSON.parse(text);
    } catch (err) {
      lastErr = err;
      await new Promise((r) => setTimeout(r, 250 * i));
    }
  }
  throw lastErr;
}

export async function runAuditHigh({
  cwd,
  lockfilePath,
  npmAudit = runNpmAuditProcess,
  osvQuery = queryOsv,
  osvDetails = loadOsvDetails,
  readLockfile = (file) => JSON.parse(readFileSync(file, "utf8")),
  log = console,
} = {}) {
  const npmRaw = await npmAudit({ cwd });
  const npmResult = classifyNpmAuditOutput(npmRaw);
  let osvResult = null;
  if (npmResult.kind !== "audited") {
    log.warn?.(`npm audit unavailable (${npmResult.reason}); falling back to OSV`);
    try {
      const lock = readLockfile(lockfilePath);
      const queries = lockfileToQueries(lock);
      const results = await osvQuery(queries);
      const ids = new Set();
      for (const row of results) {
        for (const vuln of row.vulns || []) ids.add(vuln.id);
      }
      const detailsById = await osvDetails([...ids]);
      osvResult = { kind: "audited", findings: evaluateOsvFindings(queries, detailsById, results) };
    } catch (err) {
      osvResult = { kind: "unreadable", reason: err instanceof Error ? err.message : String(err) };
    }
  }
  const decision = decideGate(npmResult, osvResult);
  if (decision.findings) {
    for (const finding of decision.findings) {
      log.error?.(
        `  [${finding.severity}] ${finding.name}@${finding.version} ${finding.id} ${finding.summary || ""}`.trim(),
      );
    }
  }
  if (decision.ok) {
    log.log?.(decision.message);
    return 0;
  }
  log.error?.(decision.message);
  return 1;
}

const thisFile = fileURLToPath(import.meta.url);
const invokedAsMain = process.argv[1] && pathToFileURL(path.resolve(process.argv[1])).href === import.meta.url;

if (invokedAsMain) {
  const appDir = path.resolve(path.dirname(thisFile), "..");
  runAuditHigh({
    cwd: appDir,
    lockfilePath: path.join(appDir, "package-lock.json"),
  })
    .then((code) => process.exit(code))
    .catch((err) => {
      console.error(err);
      process.exit(1);
    });
}
