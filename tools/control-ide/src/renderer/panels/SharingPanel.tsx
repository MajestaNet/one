import { useCallback, useEffect, useState } from "react";
import type { AppBridge } from "../App";
import {
  createSharingRule,
  deleteSharingRule,
  enableSharing,
  getSharingSettings,
  listSharingObjects,
  listSharingRules,
  patchSharingObject,
  type ObjectSharing,
  type SharingRule,
  type SharingSettings,
} from "../govern/sharing";
import { Button, EmptyState, PanelHeader, StatusBadge, ToolSurface } from "../ui";
import { IconGovern } from "../icons/Icons";

const OWD_OPTIONS = ["private", "public_read", "public_read_write", "controlled_by_parent"];

export function SharingPanel({ bridge }: { bridge: AppBridge }) {
  const connected = Boolean(bridge.session?.baseUrl && bridge.session?.token);
  const [settings, setSettings] = useState<SharingSettings | null>(null);
  const [objects, setObjects] = useState<ObjectSharing[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [rules, setRules] = useState<SharingRule[]>([]);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [ruleForm, setRuleForm] = useState({
    apiName: "",
    label: "",
    sharedToDataRoleApiName: "",
    accessLevel: "read",
    criteriaJson: '{"filters":[{"field":"Id","op":"neq","value":""}]}',
  });

  const load = useCallback(async () => {
    if (!connected) return;
    setErr("");
    setBusy(true);
    try {
      const next = await getSharingSettings(bridge.fetch);
      setSettings(next);
      if (next.recordSharingEnabled) {
        setObjects(await listSharingObjects(bridge.fetch));
      } else {
        setObjects([]);
        setRules([]);
      }
    } catch (e) {
      setErr(String(e));
      setSettings(null);
      setObjects([]);
    } finally {
      setBusy(false);
    }
  }, [bridge.fetch, connected]);

  useEffect(() => {
    void load();
  }, [load]);

  const loadRules = async (objectApiName: string) => {
    setSelected(objectApiName);
    setErr("");
    try {
      setRules(await listSharingRules(bridge.fetch, objectApiName));
    } catch (e) {
      setErr(String(e));
      setRules([]);
    }
  };

  const onEnable = async () => {
    setBusy(true);
    setErr("");
    try {
      setSettings(await enableSharing(bridge.fetch));
      setObjects(await listSharingObjects(bridge.fetch));
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onPatchOwd = async (objectApiName: string, defaultAccess: string) => {
    setBusy(true);
    setErr("");
    try {
      const patched = await patchSharingObject(bridge.fetch, objectApiName, {
        defaultAccess,
        sharingRulesEnabled: true,
      });
      setObjects((prev) => prev.map((o) => (o.objectApiName === objectApiName ? patched : o)));
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onCreateRule = async () => {
    if (!selected) return;
    setBusy(true);
    setErr("");
    try {
      let criteria: unknown = {};
      try {
        criteria = JSON.parse(ruleForm.criteriaJson);
      } catch {
        throw new Error("Criteria must be JSON with a filters array");
      }
      await createSharingRule(bridge.fetch, selected, {
        apiName: ruleForm.apiName.trim(),
        label: ruleForm.label.trim() || ruleForm.apiName.trim(),
        sharedToDataRoleApiName: ruleForm.sharedToDataRoleApiName.trim(),
        accessLevel: ruleForm.accessLevel,
        active: true,
        criteria,
      });
      setRuleForm({
        apiName: "",
        label: "",
        sharedToDataRoleApiName: "",
        accessLevel: "read",
        criteriaJson: '{"filters":[{"field":"Id","op":"neq","value":""}]}',
      });
      await loadRules(selected);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onDeleteRule = async (ruleApiName: string) => {
    if (!selected) return;
    setBusy(true);
    setErr("");
    try {
      await deleteSharingRule(bridge.fetch, selected, ruleApiName);
      await loadRules(selected);
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
        title="Sharing"
        description="Connect to an install to enable record sharing, OWD, and rules. Enforcement stays on the install."
      />
    );
  }

  return (
    <ToolSurface testId="sharing-panel">
      <PanelHeader
        title="Sharing"
        subtitle="Organization-wide defaults and sharing rules. AuthZ is enforced on the install (ADR-006)."
        actions={
          <Button variant="secondary" busy={busy} onClick={() => void load()}>
            Refresh
          </Button>
        }
      />
      {err ? <p className="panel-error">{err}</p> : null}
      <div className="row" style={{ marginBottom: "0.75rem" }}>
        <StatusBadge tone={settings?.recordSharingEnabled ? "success" : "warn"}>
          {settings?.recordSharingEnabled ? "Record sharing enabled" : "Record sharing off"}
        </StatusBadge>
        {!settings?.recordSharingEnabled ? (
          <Button variant="primary" busy={busy} onClick={() => void onEnable()} data-testid="sharing-enable">
            Enable sharing
          </Button>
        ) : null}
      </div>
      {!settings?.recordSharingEnabled ? (
        <p className="muted">Enable sharing before setting object OWD or creating rules.</p>
      ) : (
        <div className="govern-master-detail">
          <div>
            <p className="muted">Objects</p>
            {objects.length === 0 ? (
              <EmptyState title="No sharing objects" description="Objects appear here after sharing is enabled." />
            ) : (
              <ul className="govern-list" data-testid="sharing-object-list">
                {objects.map((o) => (
                  <li key={o.objectApiName}>
                    <button
                      type="button"
                      className={`govern-list-item ${selected === o.objectApiName ? "active" : ""}`}
                      onClick={() => void loadRules(o.objectApiName)}
                    >
                      <strong>{o.objectApiName}</strong>
                      <span className="muted">{o.defaultAccess ?? "—"}</span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div>
            {selected ? (
              <>
                <label>
                  Default access (OWD)
                  <select
                    aria-label="Default access"
                    value={objects.find((o) => o.objectApiName === selected)?.defaultAccess ?? "private"}
                    onChange={(e) => void onPatchOwd(selected, e.target.value)}
                    data-testid="sharing-owd"
                  >
                    {OWD_OPTIONS.map((opt) => (
                      <option key={opt} value={opt}>
                        {opt}
                      </option>
                    ))}
                  </select>
                </label>
                <h4>Rules</h4>
                {rules.length === 0 ? (
                  <p className="muted">No rules on {selected}.</p>
                ) : (
                  <ul data-testid="sharing-rule-list">
                    {rules.map((r) => (
                      <li key={r.apiName} className="row">
                        <span>
                          {r.label || r.apiName} · {r.accessLevel} → {r.sharedToDataRoleApiName || "—"}
                        </span>
                        <Button variant="ghost" onClick={() => void onDeleteRule(r.apiName)}>
                          Delete
                        </Button>
                      </li>
                    ))}
                  </ul>
                )}
                <div className="env-card" data-testid="sharing-rule-create">
                  <p className="eyebrow">New rule</p>
                  <label>
                    API name
                    <input
                      value={ruleForm.apiName}
                      onChange={(e) => setRuleForm((f) => ({ ...f, apiName: e.target.value }))}
                    />
                  </label>
                  <label>
                    Label
                    <input
                      value={ruleForm.label}
                      onChange={(e) => setRuleForm((f) => ({ ...f, label: e.target.value }))}
                    />
                  </label>
                  <label>
                    Data role API name
                    <input
                      value={ruleForm.sharedToDataRoleApiName}
                      onChange={(e) => setRuleForm((f) => ({ ...f, sharedToDataRoleApiName: e.target.value }))}
                      placeholder="Sales"
                    />
                  </label>
                  <label>
                    Access
                    <select
                      value={ruleForm.accessLevel}
                      onChange={(e) => setRuleForm((f) => ({ ...f, accessLevel: e.target.value }))}
                    >
                      <option value="read">read</option>
                      <option value="read_write">read_write</option>
                    </select>
                  </label>
                  <label>
                    Criteria JSON
                    <textarea
                      rows={3}
                      value={ruleForm.criteriaJson}
                      onChange={(e) => setRuleForm((f) => ({ ...f, criteriaJson: e.target.value }))}
                    />
                  </label>
                  <Button
                    variant="primary"
                    busy={busy}
                    disabled={!ruleForm.apiName.trim() || !ruleForm.sharedToDataRoleApiName.trim()}
                    onClick={() => void onCreateRule()}
                    data-testid="sharing-rule-create-btn"
                  >
                    Create rule
                  </Button>
                </div>
              </>
            ) : (
              <p className="muted">Select an object to set OWD and rules.</p>
            )}
          </div>
        </div>
      )}
    </ToolSurface>
  );
}
