import { useEffect, useState } from "react";
import type { AppBridge } from "../App";
import { DigitalOceanCloudSection } from "../govern/DigitalOceanCloudSection";
import { EmptyState, PanelHeader, ToolSurface } from "../ui";
import { IconEnv } from "../icons/Icons";

/**
 * Settings → Hosting: day-2 cloud admin for the active install (Deploy /cloud/*).
 * Relocated from Ship/Govern Environments (BP-051).
 */
export function HostingPanel({ bridge }: { bridge: AppBridge }) {
  const [env, setEnv] = useState<Record<string, unknown> | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const connected = Boolean(bridge.session?.baseUrl && bridge.session?.token);

  const load = async () => {
    if (!connected) {
      setEnv(null);
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
  };

  useEffect(() => {
    if (connected) void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- refresh when session connect flips
  }, [connected, bridge.session?.activeInstallId]);

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
        <DigitalOceanCloudSection bridge={bridge} env={env} />
      )}
    </ToolSurface>
  );
}
