import { useCallback, useEffect, useState } from "react";
import type { AppBridge } from "../App";
import {
  createExperience,
  deleteExperience,
  listExperiences,
  patchExperience,
  type Experience,
} from "../govern/experiences";
import { Button, EmptyState, PanelHeader, StatusBadge, ToolSurface } from "../ui";
import { IconGovern } from "../icons/Icons";

export function ExperiencesPanel({ bridge }: { bridge: AppBridge }) {
  const connected = Boolean(bridge.session?.baseUrl && bridge.session?.token);
  const [items, setItems] = useState<Experience[]>([]);
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState({ apiName: "", label: "", homeUrl: "", connectedAppApiName: "" });
  const [edit, setEdit] = useState({ label: "", homeUrl: "", connectedAppApiName: "", active: true });

  const selected = items.find((i) => i.apiName === selectedName) ?? null;

  const load = useCallback(async () => {
    if (!connected) return;
    setErr("");
    setBusy(true);
    try {
      setItems(await listExperiences(bridge.fetch));
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
        homeUrl: selected.homeUrl ?? "",
        connectedAppApiName: selected.connectedAppApiName ?? "",
        active: selected.active !== false,
      });
    }
  }, [selected]);

  const onCreate = async () => {
    setBusy(true);
    setErr("");
    try {
      const created = await createExperience(bridge.fetch, {
        apiName: form.apiName.trim(),
        label: form.label.trim() || form.apiName.trim(),
        homeUrl: form.homeUrl.trim() || undefined,
        connectedAppApiName: form.connectedAppApiName.trim() || undefined,
        active: true,
      });
      setCreating(false);
      setForm({ apiName: "", label: "", homeUrl: "", connectedAppApiName: "" });
      await load();
      setSelectedName(created.apiName);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onSave = async () => {
    if (!selected) return;
    setBusy(true);
    setErr("");
    try {
      await patchExperience(bridge.fetch, selected.apiName, {
        label: edit.label.trim(),
        homeUrl: edit.homeUrl.trim(),
        connectedAppApiName: edit.connectedAppApiName.trim(),
        active: edit.active,
      });
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onDelete = async () => {
    if (!selected) return;
    setBusy(true);
    setErr("");
    try {
      await deleteExperience(bridge.fetch, selected.apiName);
      setSelectedName(null);
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  if (!connected) {
    return (
      <EmptyState
        icon={<IconGovern size={28} />}
        title="Not connected"
        description="Connect to an install in Settings → Environments."
      />
    );
  }

  return (
    <ToolSurface testId="experiences-panel" className="experiences-panel">
      <PanelHeader
        title="Experiences"
        subtitle="Client Experience apps (config only — code hosted on customer infra)."
        actions={
          <Button variant="primary" onClick={() => setCreating((v) => !v)} data-testid="experiences-new">
            {creating ? "Cancel" : "New experience"}
          </Button>
        }
      />
      {err ? <p className="panel-error">{err}</p> : null}
      {creating ? (
        <div className="env-card" data-testid="experiences-create">
          <label>
            API name
            <input
              value={form.apiName}
              onChange={(e) => setForm((f) => ({ ...f, apiName: e.target.value }))}
              data-testid="experiences-api-name"
            />
          </label>
          <label>
            Label
            <input
              value={form.label}
              onChange={(e) => setForm((f) => ({ ...f, label: e.target.value }))}
            />
          </label>
          <label>
            Home URL
            <input
              value={form.homeUrl}
              onChange={(e) => setForm((f) => ({ ...f, homeUrl: e.target.value }))}
              placeholder="https://portal.example"
            />
          </label>
          <label>
            Connected app
            <input
              value={form.connectedAppApiName}
              onChange={(e) => setForm((f) => ({ ...f, connectedAppApiName: e.target.value }))}
            />
          </label>
          <Button
            variant="primary"
            busy={busy}
            disabled={!form.apiName.trim()}
            onClick={() => void onCreate()}
            data-testid="experiences-create-btn"
          >
            Create
          </Button>
        </div>
      ) : null}
      {busy && items.length === 0 && !err ? <p className="panel-muted">Loading…</p> : null}
      {items.length === 0 && !busy && !creating ? (
        <EmptyState
          title="No experiences"
          description="Register a Client Experience against this install — no curl required."
        />
      ) : (
        <div className="govern-master-detail">
          <ul className="govern-list" data-testid="experiences-list">
            {items.map((ex) => (
              <li key={ex.apiName}>
                <button
                  type="button"
                  className={`govern-list-item ${selectedName === ex.apiName ? "active" : ""}`}
                  onClick={() => setSelectedName(ex.apiName)}
                >
                  <strong>{ex.label ?? ex.apiName}</strong>
                  <span className="muted">{ex.apiName}</span>
                </button>
              </li>
            ))}
          </ul>
          {selected ? (
            <div className="env-card" data-testid="experiences-detail">
              <p className="mono">{selected.apiName}</p>
              <StatusBadge tone={edit.active ? "success" : "neutral"}>{edit.active ? "Active" : "Inactive"}</StatusBadge>
              <label>
                Label
                <input value={edit.label} onChange={(e) => setEdit((f) => ({ ...f, label: e.target.value }))} />
              </label>
              <label>
                Home URL
                <input value={edit.homeUrl} onChange={(e) => setEdit((f) => ({ ...f, homeUrl: e.target.value }))} />
              </label>
              <label>
                Connected app
                <input
                  value={edit.connectedAppApiName}
                  onChange={(e) => setEdit((f) => ({ ...f, connectedAppApiName: e.target.value }))}
                />
              </label>
              <label className="row">
                <input
                  type="checkbox"
                  checked={edit.active}
                  onChange={(e) => setEdit((f) => ({ ...f, active: e.target.checked }))}
                />
                Active
              </label>
              <div className="row">
                <Button variant="primary" busy={busy} onClick={() => void onSave()} data-testid="experiences-save">
                  Save
                </Button>
                <Button variant="danger" busy={busy} onClick={() => void onDelete()} data-testid="experiences-delete">
                  Delete
                </Button>
              </div>
            </div>
          ) : null}
        </div>
      )}
    </ToolSurface>
  );
}
