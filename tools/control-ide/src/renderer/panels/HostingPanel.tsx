import { useEffect, useState } from "react";
import type { AppBridge } from "../App";
import { DigitalOceanCloudSection } from "../govern/DigitalOceanCloudSection";
import { EmptyState, PanelHeader, StatusBadge, ToolSurface } from "../ui";
import { IconEnv } from "../icons/Icons";

type AvailableUpgrades = {
  currentVersion?: string;
  rollerMode?: string;
  publicURL?: string;
  platformSmokeSuite?: string;
  optionalCustomerSuite?: string;
  notes?: string;
};

function canReadOps(session: AppBridge["session"]): boolean {
  if (!session) return false;
  if (session.isAdmin) return true;
  const scopes = session.scopes ?? [];
  return scopes.includes("ops") || scopes.includes("admin");
}

/**
 * Settings → Hosting: day-2 cloud admin for the active install (Deploy /cloud/*).
 * Relocated from Ship/Govern Environments (BP-051).
 */
export function HostingPanel({ bridge }: { bridge: AppBridge }) {
  const [env, setEnv] = useState<Record<string, unknown> | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [upgrades, setUpgrades] = useState<AvailableUpgrades | null>(null);
  const [upgradesErr, setUpgradesErr] = useState("");
  const connected = Boolean(bridge.session?.baseUrl && bridge.session?.token);
  const opsOk = canReadOps(bridge.session);

  const load = async () => {
    if (!connected) {
      setEnv(null);
      setUpgrades(null);
      setUpgradesErr("");
      return;
    }
    setErr("");
    setBusy(true);
    try {
      const row = (await bridge.fetch("/deploy/v1/environment")) as Record<string, unknown>;
      setEnv(row);
    } catch (e) {
      setErr(String(e));
      setEnv(null);
    } finally {
      setBusy(false);
    }
    if (!opsOk) {
      setUpgrades(null);
      setUpgradesErr("");
      return;
    }
    try {
      const avail = (await bridge.fetch("/ops/v1/upgrades/available")) as AvailableUpgrades;
      setUpgrades(avail);
      setUpgradesErr("");
    } catch (e) {
      setUpgrades(null);
      const msg = String(e);
      if (/503|UNAVAILABLE|ops engine not configured/i.test(msg)) {
        setUpgradesErr("Ops upgrades are not configured on this install.");
      } else if (/403|FORBIDDEN/i.test(msg)) {
        setUpgradesErr("This session cannot read Ops upgrades.");
      } else {
        setUpgradesErr(msg);
      }
    }
  };

  useEffect(() => {
    if (connected) void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- refresh when session connect flips
  }, [connected, bridge.session?.activeInstallId, opsOk]);

  return (
    <ToolSurface className="hosting-panel" testId="hosting-panel">
      <PanelHeader
        title="Hosting"
        subtitle="Scale, bind, and provision peer environments for this install via Deploy cloud APIs."
        actions={
          connected ? (
            <button type="button" className="secondary" disabled={busy} onClick={() => void load()} data-testid="hosting-refresh">
              {busy ? "Refreshing…" : "Refresh"}
            </button>
          ) : null
        }
      />
      {err ? <p className="err">{err}</p> : null}
      {!connected ? (
        <EmptyState
          icon={<IconEnv size={28} />}
          title="Connect to manage hosting"
          description="Open Settings → Environments to authenticate, then return here to administer the cloud host for the active install."
        />
      ) : (
        <>
          <div className="env-card" data-testid="hosting-upgrades">
            <PanelHeader
              title="Available upgrades"
              subtitle="GET /ops/v1/upgrades/available when this session has Ops scope. Confirm rolls stay on Ops HTTP — this panel is read-only."
              actions={
                opsOk ? (
                  <StatusBadge tone="accent">ops</StatusBadge>
                ) : (
                  <StatusBadge tone="neutral">needs ops</StatusBadge>
                )
              }
            />
            {!opsOk ? (
              <p className="muted">Connect with ops or admin scope to read available upgrades.</p>
            ) : upgradesErr ? (
              <p className="muted" data-testid="hosting-upgrades-empty">
                {upgradesErr}
              </p>
            ) : upgrades ? (
              <ul className="agents-harness-preview" data-testid="hosting-upgrades-body">
                <li>
                  <strong>Current version:</strong> {upgrades.currentVersion || "—"}
                </li>
                <li>
                  <strong>Roller:</strong> {upgrades.rollerMode || "—"}
                </li>
                {upgrades.notes ? (
                  <li>
                    <strong>Notes:</strong> {upgrades.notes}
                  </li>
                ) : null}
              </ul>
            ) : (
              <p className="muted">No upgrade metadata yet.</p>
            )}
          </div>
          <DigitalOceanCloudSection bridge={bridge} env={env} />
        </>
      )}
    </ToolSurface>
  );
}
