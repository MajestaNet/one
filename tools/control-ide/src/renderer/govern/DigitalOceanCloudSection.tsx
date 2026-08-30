import { useCallback, useEffect, useState } from "react";
import type { AppBridge } from "../App";
import { Button, EmptyState, KeyValueList, PanelHeader } from "../ui";
import { IconEnv } from "../icons/Icons";
import {
  DO_CONSOLE_APPS,
  cloudEnabled,
  getCloudApp,
  getCloudStatus,
  listCloudEnvironments,
  provisionCloudEnvironment,
  putCloudBinding,
  resizeCloudDatabase,
  scaleCloudApp,
  type AppSummary,
  type CloudStatus,
  type EnvironmentsPayload,
} from "./cloud";

const SIZE_OPTIONS = [
  "small",
  "medium",
  "large",
  "apps-s-1vcpu-0.5gb",
  "apps-s-1vcpu-1gb",
  "apps-s-1vcpu-1gb-fixed",
  "apps-s-1vcpu-2gb",
];

const DB_SIZE_OPTIONS = ["small", "medium", "large", "db-s-1vcpu-1gb", "db-s-1vcpu-2gb", "db-s-2vcpu-4gb"];

/**
 * Path A day-2: scale App Platform, resize Managed Postgres, provision peer envs.
 * Calls host-free Deploy `/deploy/v1/cloud/*` (JWT client only — never DO APIs directly).
 */
export function DigitalOceanCloudSection({
  bridge,
  env,
}: {
  bridge: AppBridge;
  env: Record<string, unknown> | null;
}) {
  const cloudOn = cloudEnabled(env);
  const canMutate = Boolean(bridge.session?.isAdmin);
  const [status, setStatus] = useState<CloudStatus | null>(null);
  const [app, setApp] = useState<AppSummary | null>(null);
  const [envs, setEnvs] = useState<EnvironmentsPayload | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const [bindForm, setBindForm] = useState({ appId: "", databaseId: "", region: "nyc" });
  const [scaleForm, setScaleForm] = useState({
    apiInstanceCount: "1",
    apiInstanceSizeSlug: "apps-s-1vcpu-1gb",
    workerInstanceCount: "1",
    workerInstanceSizeSlug: "apps-s-1vcpu-1gb",
  });
  const [dbForm, setDbForm] = useState({ size: "db-s-1vcpu-1gb", numNodes: "1" });
  const [provForm, setProvForm] = useState({
    installId: "",
    installRole: "dev",
    region: "nyc",
    apiKeys: "",
    authJwtSigningKey: "",
    apiInstanceSizeSlug: "apps-s-1vcpu-0.5gb",
    workerInstanceSizeSlug: "apps-s-1vcpu-0.5gb",
    databaseSize: "db-s-1vcpu-1gb",
  });

  const refresh = useCallback(async () => {
    if (!bridge.session?.baseUrl || !cloudOn) {
      setStatus(null);
      setApp(null);
      setEnvs(null);
      return;
    }
    setErr("");
    setBusy(true);
    try {
      const st = await getCloudStatus(bridge.fetch);
      setStatus(st);
      if (st.binding?.appId || st.binding?.appResourceId) {
        setBindForm((f) => ({
          ...f,
          appId: st.binding?.appId || st.binding?.appResourceId || f.appId,
          databaseId: st.binding?.databaseId || st.binding?.databaseResourceId || f.databaseId,
          region: st.binding?.region || f.region,
        }));
      }
      try {
        const a = await getCloudApp(bridge.fetch);
        setApp(a);
        if (a.apiInstanceSizeSlug || a.apiSizeClass) {
          setScaleForm((f) => ({
            ...f,
            apiInstanceCount: String(a.apiInstanceCount ?? a.apiInstances ?? 1),
            apiInstanceSizeSlug: a.apiInstanceSizeSlug || a.apiSizeClass || f.apiInstanceSizeSlug,
            workerInstanceCount: String(a.workerInstanceCount ?? a.workerInstances ?? 1),
            workerInstanceSizeSlug: a.workerInstanceSizeSlug || a.workerSizeClass || f.workerInstanceSizeSlug,
          }));
        }
      } catch {
        setApp(null);
      }
      try {
        setEnvs(await listCloudEnvironments(bridge.fetch));
      } catch {
        setEnvs(null);
      }
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  }, [bridge.fetch, bridge.session?.baseUrl, cloudOn]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  if (!bridge.session?.baseUrl) return null;

  if (!cloudOn) {
    return (
      <section className="govern-section" data-testid="do-cloud-section">
        <PanelHeader
          title="Hosting"
          subtitle="Path A day-2 manage / scale / provision via Deploy cloud APIs."
        />
        <EmptyState
          icon={<IconEnv size={28} />}
          title="Cloud hosting not configured"
          description="Set the host API token on this install (for DigitalOcean: DIGITALOCEAN_API_TOKEN, and optionally APP_ID / DATABASE_ID), then refresh. Until then, use the provider console."
          action={
            <a className="btn secondary" href={DO_CONSOLE_APPS} target="_blank" rel="noreferrer">
              Open DO Apps console
            </a>
          }
        />
      </section>
    );
  }

  const run = async (fn: () => Promise<unknown>) => {
    setErr("");
    setBusy(true);
    try {
      await fn();
      await refresh();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const statusItems = [
    { label: "Token", value: status?.configured ? "configured" : "missing" },
    {
      label: "Reachable",
      value: status?.reachable == null ? "—" : status.reachable ? "yes" : "no",
    },
    { label: "Bound app", value: status?.binding?.appId || status?.binding?.appResourceId || "—" },
    { label: "Bound database", value: status?.binding?.databaseId || status?.binding?.databaseResourceId || "—" },
    { label: "Cloud host", value: status?.host || (typeof env?.cloudHost === "string" ? env.cloudHost : "—") },
  ];

  return (
    <section className="govern-section" data-testid="do-cloud-section">
      <PanelHeader
        title="Hosting"
        subtitle="Scale this app, resize Managed Postgres, or provision a peer env (new App + DB). Requires deploy scope; mutations need admin."
        actions={
          <Button variant="secondary" busy={busy} onClick={() => void refresh()}>
            Refresh hosting status
          </Button>
        }
      />
      {err ? <p className="err">{err}</p> : null}
      {!canMutate ? (
        <p className="muted">Connected without admin — status is read-only. Mutations need +admin.</p>
      ) : null}

      <div className="env-card" data-testid="do-cloud-status">
        <KeyValueList items={statusItems} />
        {app?.publicUrl ? (
          <p className="muted" style={{ marginTop: "0.5rem" }}>
            Public URL:{" "}
            <a href={app.publicUrl} target="_blank" rel="noreferrer">
              {app.publicUrl}
            </a>
          </p>
        ) : null}
        <p className="muted">
          <a href={DO_CONSOLE_APPS} target="_blank" rel="noreferrer">
            DigitalOcean Apps console
          </a>
        </p>
      </div>

      <div className="env-card" data-testid="do-cloud-bind">
        <h3 className="h3">Binding</h3>
        <div className="row">
          <label>
            App ID
            <input
              value={bindForm.appId}
              onChange={(e) => setBindForm((f) => ({ ...f, appId: e.target.value }))}
              data-testid="do-bind-app-id"
            />
          </label>
          <label>
            Database ID
            <input
              value={bindForm.databaseId}
              onChange={(e) => setBindForm((f) => ({ ...f, databaseId: e.target.value }))}
              data-testid="do-bind-db-id"
            />
          </label>
          <label>
            Region
            <input
              value={bindForm.region}
              onChange={(e) => setBindForm((f) => ({ ...f, region: e.target.value }))}
            />
          </label>
        </div>
        <Button
          variant="secondary"
          busy={busy}
          disabled={!canMutate || !bindForm.appId.trim() || !bindForm.databaseId.trim()}
          onClick={() =>
            void run(() =>
              putCloudBinding(bridge.fetch, {
                appId: bindForm.appId.trim(),
                databaseId: bindForm.databaseId.trim(),
                region: bindForm.region.trim() || undefined,
              }),
            )
          }
          data-testid="do-bind-save"
        >
          Save binding
        </Button>
      </div>

      <div className="env-card" data-testid="do-cloud-scale">
        <h3 className="h3">Scale app</h3>
        <div className="row">
          <label>
            API instances
            <input
              value={scaleForm.apiInstanceCount}
              onChange={(e) => setScaleForm((f) => ({ ...f, apiInstanceCount: e.target.value }))}
              data-testid="do-scale-api-count"
            />
          </label>
          <label>
            API size
            <select
              value={scaleForm.apiInstanceSizeSlug}
              onChange={(e) => setScaleForm((f) => ({ ...f, apiInstanceSizeSlug: e.target.value }))}
              data-testid="do-scale-api-size"
            >
              {SIZE_OPTIONS.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </label>
        </div>
        <div className="row">
          <label>
            Worker instances
            <input
              value={scaleForm.workerInstanceCount}
              onChange={(e) => setScaleForm((f) => ({ ...f, workerInstanceCount: e.target.value }))}
            />
          </label>
          <label>
            Worker size
            <select
              value={scaleForm.workerInstanceSizeSlug}
              onChange={(e) => setScaleForm((f) => ({ ...f, workerInstanceSizeSlug: e.target.value }))}
            >
              {SIZE_OPTIONS.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </label>
        </div>
        <Button
          variant="primary"
          busy={busy}
          disabled={!canMutate}
          onClick={() =>
            void run(() =>
              scaleCloudApp(bridge.fetch, {
                apiInstanceCount: Number(scaleForm.apiInstanceCount) || undefined,
                apiInstanceSizeSlug: scaleForm.apiInstanceSizeSlug,
                workerInstanceCount: Number(scaleForm.workerInstanceCount) || undefined,
                workerInstanceSizeSlug: scaleForm.workerInstanceSizeSlug,
              }),
            )
          }
          data-testid="do-scale-save"
        >
          Apply scale
        </Button>
      </div>

      <div className="env-card" data-testid="do-cloud-resize-db">
        <h3 className="h3">Resize database</h3>
        <div className="row">
          <label>
            Size
            <select
              value={dbForm.size}
              onChange={(e) => setDbForm((f) => ({ ...f, size: e.target.value }))}
              data-testid="do-db-size"
            >
              {DB_SIZE_OPTIONS.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </label>
          <label>
            Nodes
            <input
              value={dbForm.numNodes}
              onChange={(e) => setDbForm((f) => ({ ...f, numNodes: e.target.value }))}
            />
          </label>
        </div>
        <Button
          variant="primary"
          busy={busy}
          disabled={!canMutate}
          onClick={() =>
            void run(() =>
              resizeCloudDatabase(bridge.fetch, {
                size: dbForm.size,
                numNodes: Number(dbForm.numNodes) || 1,
              }),
            )
          }
          data-testid="do-db-resize"
        >
          Resize database
        </Button>
      </div>

      <div className="env-card" data-testid="do-cloud-provision">
        <h3 className="h3">Create environment</h3>
        <p className="muted">
          Provisions another App Platform app + Managed Postgres in the customer DO account and registers a
          Deploy peer (same CUSTOMER_ID, new INSTALL_ID).
        </p>
        <div className="row">
          <label>
            Install ID
            <input
              value={provForm.installId}
              onChange={(e) => setProvForm((f) => ({ ...f, installId: e.target.value }))}
              data-testid="do-prov-install-id"
            />
          </label>
          <label>
            Role
            <input
              value={provForm.installRole}
              onChange={(e) => setProvForm((f) => ({ ...f, installRole: e.target.value }))}
            />
          </label>
          <label>
            Region
            <input
              value={provForm.region}
              onChange={(e) => setProvForm((f) => ({ ...f, region: e.target.value }))}
            />
          </label>
        </div>
        <div className="row">
          <label>
            API keys (peer secret)
            <input
              type="password"
              value={provForm.apiKeys}
              onChange={(e) => setProvForm((f) => ({ ...f, apiKeys: e.target.value }))}
              data-testid="do-prov-api-keys"
            />
          </label>
          <label>
            JWT signing key
            <input
              type="password"
              value={provForm.authJwtSigningKey}
              onChange={(e) => setProvForm((f) => ({ ...f, authJwtSigningKey: e.target.value }))}
              data-testid="do-prov-jwt"
            />
          </label>
        </div>
        <Button
          variant="primary"
          busy={busy}
          disabled={
            !canMutate ||
            !provForm.installId.trim() ||
            !provForm.apiKeys.trim() ||
            !provForm.authJwtSigningKey.trim()
          }
          onClick={() =>
            void run(() =>
              provisionCloudEnvironment(bridge.fetch, {
                installId: provForm.installId.trim(),
                installRole: provForm.installRole.trim() || "dev",
                region: provForm.region.trim() || "nyc",
                apiKeys: provForm.apiKeys,
                authJwtSigningKey: provForm.authJwtSigningKey,
                apiInstanceSizeSlug: provForm.apiInstanceSizeSlug,
                workerInstanceSizeSlug: provForm.workerInstanceSizeSlug,
                databaseSize: provForm.databaseSize,
                databaseNodes: 1,
              }),
            )
          }
          data-testid="do-prov-submit"
        >
          Provision peer environment
        </Button>
        {envs?.provisionRuns && envs.provisionRuns.length > 0 ? (
          <details className="details-block" style={{ marginTop: "0.75rem" }}>
            <summary>Recent provision runs ({envs.provisionRuns.length})</summary>
            <pre className="log">{JSON.stringify(envs.provisionRuns.slice(0, 10), null, 2)}</pre>
          </details>
        ) : null}
      </div>
    </section>
  );
}
