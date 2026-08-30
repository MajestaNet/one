import { useCallback, useEffect, useState } from "react";
import type { AppBridge } from "../App";
import {
  createIntegration,
  deleteIntegration,
  listIntegrations,
  patchIntegration,
  revealIntegrationSecrets,
  rotateIntegrationSecrets,
  type Integration,
} from "../govern";
import { Button, EmptyState, PanelHeader, StatusBadge, ToolSurface } from "../ui";
import { IconGovern } from "../icons/Icons";
import { OutboundConnectorsPanel } from "./OutboundConnectorsPanel";

export function IntegrationsPanel({ bridge }: { bridge: AppBridge }) {
  const connected = Boolean(bridge.session?.baseUrl && bridge.session?.token);
  const [items, setItems] = useState<Integration[]>([]);
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [filter, setFilter] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [secretBanner, setSecretBanner] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [surface, setSurface] = useState<"apps" | "connectors">("apps");
  const [form, setForm] = useState({
    apiName: "",
    label: "",
    oauthFlows: "client_credentials",
    clientKind: "confidential",
  });
  const [edit, setEdit] = useState({ label: "", description: "", isActive: true });

  const selected = items.find((i) => i.apiName === selectedName) ?? null;

  const load = useCallback(async () => {
    if (!connected) return;
    setErr("");
    setBusy(true);
    try {
      setItems(await listIntegrations(bridge.fetch));
    } catch (e) {
      setErr(String(e));
      setItems([]);
    } finally {
      setBusy(false);
    }
  }, [bridge.fetch, connected]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (selected) {
      setEdit({
        label: selected.label ?? "",
        description: selected.description ?? "",
        isActive: selected.isActive !== false,
      });
    }
  }, [selected]);

  const filtered = items.filter((i) => {
    const q = filter.trim().toLowerCase();
    if (!q) return true;
    return (
      i.apiName.toLowerCase().includes(q) ||
      (i.label ?? "").toLowerCase().includes(q) ||
      (i.description ?? "").toLowerCase().includes(q)
    );
  });

  const showSecrets = (row: Integration) => {
    const parts: string[] = [];
    if (row.oneClientSecret) parts.push(`Majesta One secret: ${row.oneClientSecret}`);
    if (row.clientId) parts.push(`Client ID: ${row.clientId}`);
    if (parts.length) setSecretBanner(parts.join(" · "));
  };

  const onCreate = async () => {
    setErr("");
    setBusy(true);
    try {
      const created = await createIntegration(bridge.fetch, {
        apiName: form.apiName.trim(),
        label: form.label.trim() || undefined,
        clientKind: form.clientKind.trim() || undefined,
        oauthFlows: form.oauthFlows
          .split(/[,\s]+/)
          .map((s) => s.trim())
          .filter(Boolean),
      });
      showSecrets(created);
      setCreating(false);
      setForm({ apiName: "", label: "", oauthFlows: "client_credentials", clientKind: "confidential" });
      await load();
      setSelectedName(created.apiName);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onSave = async () => {
    if (!selectedName) return;
    setErr("");
    setBusy(true);
    try {
      await patchIntegration(bridge.fetch, selectedName, {
        label: edit.label.trim() || undefined,
        description: edit.description.trim() || undefined,
        isActive: edit.isActive,
      });
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onDelete = async () => {
    if (!selectedName) return;
    if (!confirm(`Delete integration ${selectedName}?`)) return;
    setBusy(true);
    try {
      await deleteIntegration(bridge.fetch, selectedName);
      setSelectedName(null);
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onRotate = async () => {
    if (!selectedName) return;
    if (!confirm("Rotate secrets? Previous secrets stop working.")) return;
    setBusy(true);
    try {
      const row = await rotateIntegrationSecrets(bridge.fetch, selectedName);
      showSecrets(row);
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onReveal = async () => {
    if (!selectedName) return;
    setBusy(true);
    try {
      const secrets = await revealIntegrationSecrets(bridge.fetch, selectedName);
      const parts: string[] = [];
      if (secrets.oneClientSecret) parts.push(`Majesta One secret: ${secrets.oneClientSecret}`);
      setSecretBanner(parts.length ? parts.join(" · ") : "No secrets returned");
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  if (!connected) {
    return (
      <ToolSurface testId="integrations-panel">
        <PanelHeader title="Integrations" subtitle="Connected apps on this install." />
        <EmptyState
          icon={<IconGovern size={28} />}
          title="Connect first"
          description="Open Settings → Environments to authenticate, then manage integrations here."
        />
      </ToolSurface>
    );
  }

  if (surface === "connectors") {
    return (
      <ToolSurface className="integrations-hub" testId="integrations-panel">
        <div className="integration-surface-switch" role="tablist" aria-label="Integration surfaces">
          <button type="button" role="tab" aria-selected={false} onClick={() => setSurface("apps")}>Connected apps</button>
          <button type="button" role="tab" aria-selected className="active" onClick={() => setSurface("connectors")}>Outbound connectors</button>
        </div>
        <OutboundConnectorsPanel bridge={bridge} />
      </ToolSurface>
    );
  }

  return (
    <ToolSurface testId="integrations-panel">
      <div className="integration-surface-switch" role="tablist" aria-label="Integration surfaces">
        <button type="button" role="tab" aria-selected className="active" onClick={() => setSurface("apps")}>Connected apps</button>
        <button type="button" role="tab" aria-selected={false} onClick={() => setSurface("connectors")}>Outbound connectors</button>
      </div>
      <PanelHeader
        title="Integrations"
        subtitle="Connected apps: OAuth clients, secrets rotate/reveal via Client API."
        actions={
          <>
            <Button variant="secondary" busy={busy} onClick={() => void load()}>
              Refresh
            </Button>
            <Button variant="primary" onClick={() => setCreating((v) => !v)}>
              {creating ? "Cancel" : "New integration"}
            </Button>
          </>
        }
      />
      {err && <p className="err">{err}</p>}
      {secretBanner ? (
        <div className="govern-secret-banner" data-testid="secret-banner">
          <p>{secretBanner}</p>
          <Button variant="ghost" onClick={() => setSecretBanner(null)}>
            Dismiss
          </Button>
        </div>
      ) : null}

      {creating ? (
        <div className="env-card" data-testid="integrations-create">
          <div className="row">
            <label>
              API name
              <input
                value={form.apiName}
                onChange={(e) => setForm((f) => ({ ...f, apiName: e.target.value }))}
                data-testid="integrations-api-name"
              />
            </label>
            <label>
              Label
              <input value={form.label} onChange={(e) => setForm((f) => ({ ...f, label: e.target.value }))} />
            </label>
          </div>
          <div className="row">
            <label>
              OAuth flows (comma-separated)
              <input
                value={form.oauthFlows}
                onChange={(e) => setForm((f) => ({ ...f, oauthFlows: e.target.value }))}
              />
            </label>
            <label>
              Client kind
              <input
                value={form.clientKind}
                onChange={(e) => setForm((f) => ({ ...f, clientKind: e.target.value }))}
              />
            </label>
          </div>
          <Button
            variant="primary"
            busy={busy}
            disabled={!form.apiName.trim()}
            onClick={() => void onCreate()}
            data-testid="integrations-create-btn"
          >
            Create
          </Button>
        </div>
      ) : null}

      <div className="row">
        <label>
          Filter
          <input
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="apiName, label…"
            data-testid="integrations-filter"
          />
        </label>
      </div>

      <div className="govern-master-detail">
        <ul className="govern-list" data-testid="integrations-list">
          {filtered.map((i) => (
            <li key={i.apiName}>
              <button
                type="button"
                className={`govern-list-item ${i.apiName === selectedName ? "active" : ""}`}
                onClick={() => setSelectedName(i.apiName)}
                data-testid={`integration-row-${i.apiName}`}
              >
                <strong>{i.label || i.apiName}</strong>
                <span className="muted">{i.apiName}</span>
                <StatusBadge tone={i.isActive === false ? "neutral" : "success"}>
                  {i.isActive === false ? "Inactive" : "Active"}
                </StatusBadge>
              </button>
            </li>
          ))}
          {filtered.length === 0 ? <li className="muted">No integrations</li> : null}
        </ul>

        <div className="env-card" data-testid="integrations-detail">
          {!selected ? (
            <EmptyState title="Select an integration" description="Choose a row to edit, rotate, or reveal secrets." />
          ) : (
            <>
              <PanelHeader title={selected.label || selected.apiName} subtitle={selected.apiName} />
              <div className="row">
                <label>
                  Label
                  <input value={edit.label} onChange={(e) => setEdit((x) => ({ ...x, label: e.target.value }))} />
                </label>
                <label>
                  Description
                  <input
                    value={edit.description}
                    onChange={(e) => setEdit((x) => ({ ...x, description: e.target.value }))}
                  />
                </label>
              </div>
              <label>
                <input
                  type="checkbox"
                  checked={edit.isActive}
                  onChange={(e) => setEdit((x) => ({ ...x, isActive: e.target.checked }))}
                />{" "}
                Active
              </label>
              <p className="muted">
                Secrets: Majesta One {selected.hasOneSecret ? "yes" : "no"}
              </p>
              <div className="row">
                <Button variant="primary" busy={busy} onClick={() => void onSave()}>
                  Save
                </Button>
                <Button variant="secondary" busy={busy} onClick={() => void onRotate()}>
                  Rotate secrets
                </Button>
                <Button variant="secondary" busy={busy} onClick={() => void onReveal()}>
                  Reveal secrets
                </Button>
                <Button variant="ghost" busy={busy} onClick={() => void onDelete()}>
                  Delete
                </Button>
              </div>
            </>
          )}
        </div>
      </div>
    </ToolSurface>
  );
}
