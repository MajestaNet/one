import { useEffect, useMemo, useState } from "react";
import type { AppBridge } from "../App";
import { envDisplayName } from "../session";
import { loadRepoEnvironments, orderStagesByRepoEnv } from "../repoEnvironments";
import type { ChangeStatus, CheckItem } from "../workspace/types";
import { Button, FileDrop, PanelHeader, StatusBadge, ToolSurface } from "../ui";
import { resolveCustomerTestRun } from "./deployTestRun";

type StepState = "idle" | "running" | "passed" | "failed";

function stepToCheck(state: StepState): CheckItem["state"] {
  if (state === "running") return "running";
  if (state === "passed") return "passed";
  if (state === "failed") return "failed";
  return "pending";
}

const DEFAULT_SUITE = "CreateAccountFromContact";

type ValidateLocalResponse = {
  bundleId?: string;
  checksum?: string;
  ok?: boolean;
  diff?: { counts?: { add?: number; change?: number; remove?: number; baseline?: number } };
  validation?: { ok?: boolean; issues?: unknown[] };
};

export function DeployPanel({
  bridge,
  onPipelineChange,
  onMoreChanges,
}: {
  bridge: AppBridge;
  onPipelineChange?: (checks: CheckItem[], status: ChangeStatus) => void;
  onMoreChanges?: () => void;
}) {
  const [log, setLog] = useState("");
  const [err, setErr] = useState("");
  const [bundleId, setBundleId] = useState("");
  const [checksum, setChecksum] = useState("");
  const [validateOk, setValidateOk] = useState(false);
  const [suiteApiName, setSuiteApiName] = useState(DEFAULT_SUITE);
  const [packState, setPackState] = useState<StepState>("idle");
  const [validateState, setValidateState] = useState<StepState>("idle");
  const [testsState, setTestsState] = useState<StepState>("idle");
  const [deployState, setDeployState] = useState<StepState>("idle");
  const [fileName, setFileName] = useState("");
  const [repoDirty, setRepoDirty] = useState(false);
  const [repoChecked, setRepoChecked] = useState(false);
  const [activeChange, setActiveChange] = useState("");
  const [selectivePaths, setSelectivePaths] = useState("");
  const [diffCounts, setDiffCounts] = useState<{ add: number; change: number; remove: number; baseline: number } | null>(null);
  const [validationIssueCount, setValidationIssueCount] = useState(0);
  const [repoEnvOrder, setRepoEnvOrder] = useState<
    { alias: string; installId: string; installRole: string; baseUrl: string }[]
  >([]);

  const append = (msg: string) => setLog((prev) => `${prev}${prev ? "\n" : ""}${msg}`);

  const tone = (s: StepState) =>
    s === "passed" ? "success" : s === "running" ? "accent" : s === "failed" ? "danger" : "neutral";

  const label = (s: StepState) =>
    s === "idle" ? "Idle" : s === "running" ? "Running" : s === "passed" ? "Passed" : "Failed";

  const stages = useMemo(() => {
    const envs = bridge.session?.environments ?? [];
    return orderStagesByRepoEnv(envs, repoEnvOrder);
  }, [bridge.session?.environments, repoEnvOrder]);

  const activeId = bridge.session?.activeInstallId;
  const repoPath = bridge.session?.repoPath;
  const canDeploy =
    validateOk &&
    Boolean(bundleId) &&
    Boolean(checksum) &&
    validateState === "passed" &&
    testsState === "passed";

  const packPathList = () =>
    selectivePaths
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean);

  useEffect(() => {
    if (!onPipelineChange) return;
    const checks: CheckItem[] = [
      { id: "pack", label: "Pack", state: stepToCheck(packState) },
      { id: "validate", label: "Validate vs org", state: stepToCheck(validateState) },
      { id: "tests", label: "Customer tests", state: stepToCheck(testsState) },
      { id: "deploy", label: "Deploy to org", state: stepToCheck(deployState) },
    ];
    let status: ChangeStatus = "draft";
    if (deployState === "passed") status = "promoted";
    else if ([packState, validateState, testsState, deployState].includes("failed")) status = "needs_review";
    else if ([packState, validateState, testsState, deployState].includes("running")) status = "running";
    else if (canDeploy) status = "ready";
    else if (packState !== "idle" || validateState !== "idle") status = "running";
    onPipelineChange(checks, status);
  }, [packState, validateState, testsState, deployState, canDeploy, onPipelineChange]);

  useEffect(() => {
    let cancelled = false;
    const refreshRepo = async () => {
      if (!repoPath || !window.one?.gitStatus) {
        setRepoChecked(false);
        setRepoDirty(false);
        setActiveChange("");
        return;
      }
      try {
        const res = await window.one.gitStatus(repoPath);
        if (cancelled) return;
        setRepoChecked(Boolean(res.ok));
        setRepoDirty(Boolean(res.ok && (res.status || "").trim()));
        const br = (res.branch || "").trim();
        setActiveChange(br.startsWith("change/") ? br.slice("change/".length) : "");
      } catch {
        if (!cancelled) {
          setRepoChecked(false);
          setRepoDirty(false);
          setActiveChange("");
        }
      }
    };
    const detectSuite = async () => {
      if (!repoPath || !window.one?.listTree) return;
      try {
        const files = await window.one.listTree(repoPath, "tests");
        if (cancelled) return;
        if (files.some((f) => /CreateAccountFromContact\.ya?ml$/i.test(f))) {
          setSuiteApiName(DEFAULT_SUITE);
        }
      } catch {
        /* keep default */
      }
    };
    const loadEnvs = async () => {
      if (!repoPath || !window.one?.listTree || !window.one?.readText) {
        setRepoEnvOrder([]);
        return;
      }
      try {
        const envs = await loadRepoEnvironments(repoPath, window.one);
        if (!cancelled) setRepoEnvOrder(envs);
      } catch {
        if (!cancelled) setRepoEnvOrder([]);
      }
    };
    void refreshRepo();
    void detectSuite();
    void loadEnvs();
    return () => {
      cancelled = true;
    };
  }, [repoPath]);

  const invalidateDeployGate = () => {
    setValidateOk(false);
    setChecksum("");
    setDiffCounts(null);
    setValidationIssueCount(0);
    setTestsState("idle");
    setDeployState("idle");
  };

  const applyValidateResult = (body: ValidateLocalResponse) => {
    if (body.bundleId) setBundleId(body.bundleId);
    if (body.checksum) setChecksum(body.checksum);
    const ok = Boolean(body.ok ?? body.validation?.ok);
    const counts = body.diff?.counts;
    setDiffCounts(
      counts
        ? {
            add: Number(counts.add ?? 0),
            change: Number(counts.change ?? 0),
            remove: Number(counts.remove ?? 0),
            baseline: Number(counts.baseline ?? 0),
          }
        : null,
    );
    setValidationIssueCount(body.validation?.issues?.length ?? 0);
    setValidateOk(ok);
    append(JSON.stringify(body, null, 2));
    if (ok) {
      setValidateState("passed");
      setPackState("passed");
    } else {
      setValidateState("failed");
    }
  };

  const postValidateLocalZip = async (bytes: ArrayBuffer, labelName: string) => {
    if (!bridge.session) throw new Error("Connect first");
    const url = `${bridge.session.baseUrl.replace(/\/$/, "")}/deploy/v1/packages/validate-local?label=${encodeURIComponent(labelName)}`;
    const res = await fetch(url, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${bridge.session.token}`,
        "Content-Type": "application/zip",
      },
      body: bytes,
    });
    const text = await res.text();
    let body: unknown = text;
    try {
      body = JSON.parse(text);
    } catch {
      /* keep */
    }
    if (!res.ok) throw new Error(typeof body === "string" ? body : JSON.stringify(body));
    applyValidateResult(body as ValidateLocalResponse);
  };

  const validateVsOrg = async () => {
    setErr("");
    invalidateDeployGate();
    if (!bridge.session) {
      setErr("Connect first");
      return;
    }
    setValidateState("running");
    setPackState("running");
    try {
      const root = bridge.session.repoPath;
      if (root && window.one?.exportRepoZip) {
        if (window.one.gitStatus) {
          const st = await window.one.gitStatus(root);
          if (st.ok && (st.status || "").trim()) {
            setRepoDirty(true);
            setRepoChecked(true);
            throw new Error("Working tree is dirty — commit in Build → Repo before validating HEAD.");
          }
          setRepoDirty(false);
          setRepoChecked(true);
        }
        const res = await window.one.exportRepoZip(root, packPathList());
        if (!res.ok || !res.base64) throw new Error(res.error ?? "export failed");
        const binary = Uint8Array.from(atob(res.base64), (c) => c.charCodeAt(0));
        await postValidateLocalZip(binary.buffer, `validate-HEAD-${Date.now()}.zip`);
        return;
      }
      if (bundleId) {
        const body = (await bridge.fetch("/deploy/v1/packages/validate-local", {
          method: "POST",
          body: JSON.stringify({ bundleId }),
        })) as ValidateLocalResponse;
        applyValidateResult(body);
        return;
      }
      throw new Error("Set a local repo in Build → Repo (Electron), or pack a bundle id first (Advanced).");
    } catch (e) {
      setErr(String(e));
      setValidateState("failed");
      setPackState("failed");
      setValidateOk(false);
    }
  };

  const createFromSnapshot = async () => {
    setErr("");
    invalidateDeployGate();
    if (!bridge.session) {
      setErr("Connect first");
      return;
    }
    setPackState("running");
    try {
      const body = await bridge.fetch("/deploy/v1/bundles", {
        method: "POST",
        body: JSON.stringify({ label: `ide-${Date.now()}` }),
      });
      const row = body as { id?: string; checksum?: string };
      if (row.id) setBundleId(row.id);
      if (row.checksum) setChecksum(row.checksum);
      append(JSON.stringify(body, null, 2));
      setPackState("passed");
    } catch (e) {
      setErr(String(e));
      setPackState("failed");
    }
  };

  const packZip = async (file: File | null) => {
    setErr("");
    invalidateDeployGate();
    if (!file || !bridge.session) {
      setErr("Choose a one/v1 zip and connect first");
      return;
    }
    setFileName(file.name);
    setValidateState("running");
    setPackState("running");
    try {
      const buf = await file.arrayBuffer();
      await postValidateLocalZip(buf, file.name);
    } catch (e) {
      setErr(String(e));
      setValidateState("failed");
      setPackState("failed");
    }
  };

  const runTests = async () => {
    setErr("");
    const suite = suiteApiName.trim();
    if (!suite) {
      setErr("Suite API name is required");
      return;
    }
    setTestsState("running");
    try {
      const report = await bridge.fetch("/deploy/v1/tests/runs", {
        method: "POST",
        body: JSON.stringify({ suiteApiName: suite }),
      });
      append(JSON.stringify(report, null, 2));
      const resolved = await resolveCustomerTestRun(bridge.fetch, report);
      if (resolved.body !== report) {
        append(JSON.stringify(resolved.body, null, 2));
      }
      if (resolved.verdict === "passed") {
        setTestsState("passed");
        return;
      }
      setTestsState("failed");
      if (resolved.verdict === "pending") {
        setErr("Test run did not report a passed suite (HTTP success is not a pass).");
      }
    } catch (e) {
      setErr(String(e));
      setTestsState("failed");
    }
  };

  const deployToOrg = async () => {
    setErr("");
    if (!canDeploy) {
      setErr("Deploy requires a green Validate vs org for the current pack checksum.");
      return;
    }
    setDeployState("running");
    try {
      const promo = await bridge.fetch("/deploy/v1/promotions", {
        method: "POST",
        body: JSON.stringify({ bundleId, dryRun: false }),
      });
      append(JSON.stringify(promo, null, 2));
      setDeployState("passed");
    } catch (e) {
      setErr(String(e));
      setDeployState("failed");
    }
  };

  return (
    <ToolSurface testId="deploy-panel">
      <PanelHeader
        title="Ship — repo → org"
        subtitle="Always Validate vs org first (diff + gates). Deploy to org applies the same pack checksum to the connected install. Multi-env = switch org and repeat from the same Git SHA."
      />

      {activeChange ? (
        <p className="muted" data-testid="ship-active-change">
          Active change: <code>change/{activeChange}</code>
        </p>
      ) : null}

      {stages.length > 0 ? (
        <ul className="stage-strip" data-testid="ship-stage-strip">
          {stages.map((e) => (
            <li
              key={e.installId}
              className={`stage-chip ${e.installId === activeId ? "active" : ""} ${e.token ? "" : "disconnected"}`}
            >
              <div className="stage-chip-role">{envDisplayName(e)}</div>
              <div className="muted stage-chip-id">{e.installId}</div>
              <StatusBadge tone={e.installId === activeId ? "accent" : "neutral"}>
                {e.installId === activeId ? "Connected org" : "Other env"}
              </StatusBadge>
            </li>
          ))}
        </ul>
      ) : (
        <p className="muted">Connect and refresh Environments to load installs. Stage order prefers <code>environments/*.yaml</code> in the local repo.</p>
      )}

      <div className="pipeline-steps" data-testid="pipeline-steps">
        <div className="pipeline-step">
          <p className="pipeline-step-title">1. Pack</p>
          <StatusBadge tone={tone(packState)}>{label(packState)}</StatusBadge>
        </div>
        <div className="pipeline-step">
          <p className="pipeline-step-title">2. Validate vs org</p>
          <StatusBadge tone={tone(validateState)}>{label(validateState)}</StatusBadge>
        </div>
        <div className="pipeline-step" data-testid="tests-step">
          <p className="pipeline-step-title">3. Tests</p>
          <StatusBadge tone={tone(testsState)}>{label(testsState)}</StatusBadge>
        </div>
        <div className="pipeline-step">
          <p className="pipeline-step-title">4. Deploy to org</p>
          <StatusBadge tone={tone(deployState)}>{label(deployState)}</StatusBadge>
        </div>
      </div>

      {validateState !== "idle" ? (
        <section className={`release-readiness ${validateOk ? "is-ready" : "is-blocked"}`} data-testid="release-readiness">
          <div>
            <p className="eyebrow">Release evidence</p>
            <h3>{validateOk ? "Validation passed" : validateState === "running" ? "Validation running" : "Validation blocked"}</h3>
            <p className="muted">
              {diffCounts
                ? `${diffCounts.add} add · ${diffCounts.change} change · ${diffCounts.remove} remove · ${diffCounts.baseline} baseline`
                : "Waiting for a structured diff from the connected org."}
            </p>
          </div>
          <div className="release-evidence-badges">
            <StatusBadge tone={validationIssueCount > 0 ? "danger" : validateOk ? "success" : "neutral"}>
              {validationIssueCount} issues
            </StatusBadge>
            {checksum ? <code title={checksum}>{checksum.slice(0, 12)}</code> : null}
          </div>
        </section>
      ) : null}

      <section className="govern-section">
        <h3 className="section-title">Validate vs org</h3>
        {repoPath ? (
          <p className="muted" data-testid="pack-repo-hint">
            {repoChecked && repoDirty
              ? "Working tree dirty — commit in Build → Repo before validating HEAD."
              : `Packs HEAD from ${repoPath}, then compares + validates against the connected install.`}
          </p>
        ) : (
          <p className="muted">Set a local repo in Build → Repo, or use Advanced (bundle id / zip).</p>
        )}
        <div className="row">
          <Button
            variant="primary"
            busy={validateState === "running"}
            onClick={() => void validateVsOrg()}
            data-testid="validate-vs-org"
            disabled={Boolean(repoChecked && repoDirty)}
          >
            Validate vs org
          </Button>
          <Button variant="secondary" busy={testsState === "running"} onClick={() => void runTests()} data-testid="run-tests">
            Run tests
          </Button>
        </div>
        <label style={{ display: "block", marginTop: "0.75rem" }}>
          Selective paths (optional)
          <textarea
            value={selectivePaths}
            onChange={(e) => {
              setSelectivePaths(e.target.value);
              invalidateDeployGate();
            }}
            rows={2}
            placeholder={"Leave empty for full tree.\nExample: metadata/objects/Referral__c.yaml"}
            aria-label="Selective pack paths"
            data-testid="selective-paths"
            style={{ width: "100%", fontFamily: "var(--font-mono, monospace)", fontSize: "0.85rem" }}
          />
        </label>
        <div className="row">
          <label>
            Bundle id
            <input value={bundleId} onChange={(e) => { setBundleId(e.target.value); invalidateDeployGate(); }} data-testid="bundle-id" />
          </label>
          <label>
            Checksum
            <input value={checksum} readOnly aria-label="Pack checksum" data-testid="pack-checksum" />
          </label>
          <label>
            Test suite
            <input
              value={suiteApiName}
              onChange={(e) => setSuiteApiName(e.target.value)}
              aria-label="Test suite API name"
              data-testid="deploy-suite"
            />
          </label>
        </div>
      </section>

      <section className="govern-section" data-testid="ship-decision">
        <h3 className="section-title">Deploy to org</h3>
        <p className="muted">
          Enabled only after a green validate and customer test run for this checksum. Applies the repo pack on the{" "}
          <strong>connected</strong> install — not peer-to-peer.
        </p>
        <div className="row">
          <Button variant="secondary" onClick={() => onMoreChanges?.()} data-testid="more-changes">
            More changes (Build)
          </Button>
          <Button
            variant="primary"
            busy={deployState === "running"}
            onClick={() => void deployToOrg()}
            data-testid="deploy-to-org"
            disabled={!canDeploy}
            title={canDeploy ? "Apply validated pack to connected org" : "Run Validate vs org and customer tests first"}
          >
            Deploy to org
          </Button>
        </div>
      </section>

      <details className="govern-advanced" data-testid="deploy-advanced">
        <summary>Advanced — zip / snapshot</summary>
        <p className="muted">
          Day-to-day path is commit → Validate vs org → Deploy to org. Zip upload is for CI/air-gap.
          Multi-env: switch the connected org and validate/deploy the same Git SHA — there is no
          install→install bundle push.
        </p>
        <FileDrop
          label={fileName ? `Selected: ${fileName}` : "Drop a one zip, or browse"}
          onFile={(f) => void packZip(f)}
        />
        <div className="row" style={{ marginTop: "0.75rem" }}>
          <Button variant="ghost" busy={packState === "running"} onClick={() => void createFromSnapshot()} data-testid="bundle-from-snapshot">
            Bundle from install snapshot
          </Button>
        </div>
      </details>

      {err && <p className="err">{err}</p>}
      {log && (
        <details className="details-block">
          <summary>Technical details</summary>
          <pre className="log">{log}</pre>
        </details>
      )}
    </ToolSurface>
  );
}
