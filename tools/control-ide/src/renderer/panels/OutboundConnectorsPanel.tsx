import { useCallback, useEffect, useState } from "react";
import type { AppBridge } from "../App";
import { openExternalUrl } from "../external";
import {
  CONNECTOR_CATALOG,
  deleteOutboundConnector,
  getConnectorStatus,
  installCatalogConnector,
  listOutboundConnectors,
  startConnectorAuthorization,
  type ConnectorCatalogEntry,
  type ConnectorStatus,
  type OutboundConnector,
} from "../govern/connectors";
import { Button, EmptyState, PanelHeader, StatusBadge } from "../ui";

export function OutboundConnectorsPanel({ bridge }: { bridge: AppBridge }) {
  const [items, setItems] = useState<OutboundConnector[]>([]);
  const [statuses, setStatuses] = useState<Record<string, ConnectorStatus>>({});
  const [selected, setSelected] = useState<ConnectorCatalogEntry | null>(null);
  const [form, setForm] = useState({ apiName: "", label: "", baseUrl: "", clientId: "", secret: "", scopes: "" });
  const [busy, setBusy] = useState("");
  const [err, setErr] = useState("");
  const [notice, setNotice] = useState("");

  const load = useCallback(async () => {
    setBusy("load");
    setErr("");
    try {
      const connectors = await listOutboundConnectors(bridge.fetch);
      setItems(connectors);
      const statusRows = await Promise.allSettled(connectors.map((item) => getConnectorStatus(bridge.fetch, item.apiName)));
      const next: Record<string, ConnectorStatus> = {};
      statusRows.forEach((result) => {
        if (result.status === "fulfilled") next[result.value.apiName] = result.value;
      });
      setStatuses(next);
    } catch (error) {
      setErr(String(error));
    } finally {
      setBusy("");
    }
  }, [bridge]);

  useEffect(() => { void load(); }, [load]);

  const choose = (entry: ConnectorCatalogEntry) => {
    setSelected(entry);
    setForm({
      apiName: entry.id.replace(/-/g, "_"),
      label: entry.label,
      baseUrl: entry.baseUrl,
      clientId: "",
      secret: "",
      scopes: entry.oauthFlow?.scopes?.join(" ") ?? "",
    });
    setErr("");
    setNotice("");
  };

  const install = async () => {
    if (!selected) return;
    setBusy(selected.id);
    setErr("");
    try {
      await installCatalogConnector(bridge.fetch, {
        catalog: selected,
        apiName: form.apiName.trim(),
        label: form.label.trim(),
        baseUrl: form.baseUrl.trim(),
        clientId: form.clientId.trim() || undefined,
        secret: form.secret || undefined,
        scopes: form.scopes.split(/[,\s]+/).filter(Boolean),
      });
      setNotice(`${form.label} installed with its egress policy and secret reference.`);
      setSelected(null);
      await load();
    } catch (error) {
      setErr(String(error));
    } finally {
      setBusy("");
    }
  };

  const authorize = async (apiName: string) => {
    setBusy(`auth:${apiName}`);
    setErr("");
    try {
      const url = await startConnectorAuthorization(bridge.fetch, apiName);
      await openExternalUrl(url);
      setNotice("Authorization opened in your browser. Refresh after consent completes.");
    } catch (error) {
      setErr(String(error));
    } finally {
      setBusy("");
    }
  };

  const remove = async (apiName: string) => {
    if (!confirm(`Delete connector ${apiName}?`)) return;
    setBusy(`delete:${apiName}`);
    setErr("");
    try {
      await deleteOutboundConnector(bridge.fetch, apiName);
      setNotice(`${apiName} deleted. Its install-local secret and shared egress entries were retained.`);
      await load();
    } catch (error) {
      setErr(String(error));
    } finally {
      setBusy("");
    }
  };

  return (
    <section className="outbound-connectors tool-surface" data-testid="outbound-connectors-panel" data-tool-surface="true">
      <PanelHeader
        title="Outbound connectors"
        subtitle="Catalog-driven connector metadata, install-local secrets, OAuth status, and egress policy."
        actions={<Button variant="secondary" busy={busy === "load"} onClick={() => void load()}>Refresh</Button>}
      />
      <div className="starter-pack-grid connector-catalog-grid">
        {CONNECTOR_CATALOG.map((entry) => (
          <article className="starter-pack-card" key={entry.id}>
            <p className="eyebrow">{entry.authType.replace(/_/g, " ")}</p>
            <h4>{entry.label}</h4>
            <p className="muted">{entry.description}</p>
            <Button variant="secondary" onClick={() => choose(entry)}>Configure</Button>
          </article>
        ))}
      </div>

      {selected ? (
        <div className="env-card connector-wizard" data-testid="connector-wizard">
          <div className="starter-pack-heading"><div><p className="eyebrow">Connector wizard</p><h3>{selected.label}</h3></div><Button variant="ghost" onClick={() => setSelected(null)}>Cancel</Button></div>
          <div className="panel-form-grid">
            <label>API name<input value={form.apiName} onChange={(e) => setForm((v) => ({ ...v, apiName: e.target.value }))} /></label>
            <label>Label<input value={form.label} onChange={(e) => setForm((v) => ({ ...v, label: e.target.value }))} /></label>
            <label>Base URL<input value={form.baseUrl} onChange={(e) => setForm((v) => ({ ...v, baseUrl: e.target.value }))} /></label>
            {selected.oauthFlow ? <label>OAuth client ID<input value={form.clientId} onChange={(e) => setForm((v) => ({ ...v, clientId: e.target.value }))} /></label> : null}
            <label>{selected.oauthFlow ? "OAuth client secret" : "Bearer token"}<input type="password" value={form.secret} onChange={(e) => setForm((v) => ({ ...v, secret: e.target.value }))} autoComplete="new-password" /></label>
            {selected.oauthFlow ? <label>Scopes<input value={form.scopes} onChange={(e) => setForm((v) => ({ ...v, scopes: e.target.value }))} /></label> : null}
          </div>
          <p className="muted">Credentials remain install-local. Only secret references and connector definitions promote.</p>
          <Button variant="primary" busy={busy === selected.id} disabled={!form.apiName.trim() || !form.label.trim() || !form.baseUrl.trim() || (Boolean(selected.oauthFlow) && !form.clientId.trim())} onClick={() => void install()}>Install connector</Button>
        </div>
      ) : null}

      {err ? <p className="panel-error" role="alert">{err}</p> : null}
      {notice ? <p className="panel-success">{notice}</p> : null}

      <div className="connector-installed-heading"><h3>Installed connectors</h3><StatusBadge tone="neutral">{items.length}</StatusBadge></div>
      {items.length === 0 ? (
        <EmptyState title="No outbound connectors" description="Choose a catalog recipe to create the connector, secret reference, and required egress entries together." />
      ) : (
        <div className="connector-installed-list">
          {items.map((item) => {
            const status = statuses[item.apiName];
            return (
              <article className="env-card connector-installed-card" key={item.apiName}>
                <div><p className="eyebrow">{item.apiName}</p><h4>{item.label || item.apiName}</h4><p className="muted mono">{item.baseUrl}</p></div>
                <div className="connector-installed-actions">
                  <StatusBadge tone={status?.connected ? "success" : "neutral"}>{status?.connected ? "Connected" : item.authType === "static_bearer" && item.secretRef ? "Secret bound" : "Setup needed"}</StatusBadge>
                  {item.authType === "oauth2_authorization_code" ? <Button variant="secondary" busy={busy === `auth:${item.apiName}`} onClick={() => void authorize(item.apiName)}>{status?.connected ? "Reconnect" : "Connect OAuth"}</Button> : null}
                  <Button variant="ghost" busy={busy === `delete:${item.apiName}`} onClick={() => void remove(item.apiName)}>Delete</Button>
                </div>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}
