import { useCallback, useEffect, useState } from "react";
import type { AppBridge } from "../App";
import {
  assignPermissionSet,
  assignRole,
  createCredential,
  createPrincipal,
  freezePrincipal,
  listCredentials,
  listPrincipals,
  listPermissionSets,
  listRoles,
  patchPrincipal,
  revokeCredential,
  setPrincipalPassword,
  unassignPermissionSet,
  unassignRole,
  unfreezePrincipal,
  type Credential,
  type Principal,
  type PermissionSet,
  type Role,
} from "../govern";
import { Button, EmptyState, PanelHeader, StatusBadge, ToolSurface } from "../ui";
import { IconGovern } from "../icons/Icons";

export function UsersPanel({ bridge }: { bridge: AppBridge }) {
  const connected = Boolean(bridge.session?.baseUrl && bridge.session?.token);
  const [items, setItems] = useState<Principal[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [filter, setFilter] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [creds, setCreds] = useState<Credential[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [sets, setSets] = useState<PermissionSet[]>([]);
  const [secretBanner, setSecretBanner] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState({
    email: "",
    displayName: "",
    principalType: "user",
    roleApiName: "",
  });
  const [edit, setEdit] = useState({ displayName: "", title: "", department: "" });
  const [assignRoleName, setAssignRoleName] = useState("");
  const [assignPsName, setAssignPsName] = useState("");
  const [freezeReason, setFreezeReason] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [passwordSaved, setPasswordSaved] = useState(false);

  const selected = items.find((p) => p.id === selectedId) ?? null;

  const load = useCallback(async () => {
    if (!connected) return;
    setErr("");
    setBusy(true);
    try {
      const list = await listPrincipals(bridge.fetch);
      setItems(list);
      try {
        setRoles(await listRoles(bridge.fetch));
      } catch {
        setRoles([]);
      }
      try {
        setSets(await listPermissionSets(bridge.fetch));
      } catch {
        setSets([]);
      }
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
    if (!selectedId || !connected) {
      setCreds([]);
      return;
    }
    void (async () => {
      try {
        setCreds(await listCredentials(bridge.fetch, selectedId));
      } catch {
        setCreds([]);
      }
    })();
  }, [selectedId, connected, bridge.fetch]);

  useEffect(() => {
    if (selected) {
      setEdit({
        displayName: selected.displayName ?? "",
        title: (selected.title as string) ?? "",
        department: (selected.department as string) ?? "",
      });
    }
  }, [selected]);

  const filtered = items.filter((p) => {
    const q = filter.trim().toLowerCase();
    if (!q) return true;
    return (
      (p.email ?? "").toLowerCase().includes(q) ||
      (p.displayName ?? "").toLowerCase().includes(q) ||
      (p.principalType ?? "").toLowerCase().includes(q) ||
      p.id.toLowerCase().includes(q)
    );
  });

  const onCreate = async () => {
    setErr("");
    setBusy(true);
    try {
      const created = await createPrincipal(bridge.fetch, {
        email: form.email.trim() || undefined,
        displayName: form.displayName.trim() || undefined,
        principalType: form.principalType as "user" | "service" | "agent",
        roleApiName: form.roleApiName.trim() || undefined,
      });
      setCreating(false);
      setForm({ email: "", displayName: "", principalType: "user", roleApiName: "" });
      await load();
      setSelectedId(created.id);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onSave = async () => {
    if (!selectedId) return;
    setErr("");
    setBusy(true);
    try {
      await patchPrincipal(bridge.fetch, selectedId, {
        displayName: edit.displayName.trim() || undefined,
        title: edit.title.trim() || undefined,
        department: edit.department.trim() || undefined,
      });
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onFreeze = async () => {
    if (!selectedId) return;
    setErr("");
    setBusy(true);
    try {
      await freezePrincipal(bridge.fetch, selectedId, freezeReason.trim() || "Frozen from Control IDE");
      setFreezeReason("");
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onUnfreeze = async () => {
    if (!selectedId) return;
    setErr("");
    setBusy(true);
    try {
      await unfreezePrincipal(bridge.fetch, selectedId, true);
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onDeactivate = async () => {
    if (!selectedId) return;
    if (!confirm("Deactivate this principal? Credentials will be revoked.")) return;
    setErr("");
    setBusy(true);
    try {
      await patchPrincipal(bridge.fetch, selectedId, { isActive: false });
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onCreateCred = async () => {
    if (!selectedId) return;
    setErr("");
    setBusy(true);
    try {
      const cred = await createCredential(bridge.fetch, selectedId, "Control IDE");
      if (cred.clientSecret) {
        setSecretBanner(`One-time client secret: ${cred.clientSecret}`);
      }
      setCreds(await listCredentials(bridge.fetch, selectedId));
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onRevokeCred = async (credId: string) => {
    if (!selectedId) return;
    if (!confirm("Revoke this credential?")) return;
    setBusy(true);
    try {
      await revokeCredential(bridge.fetch, selectedId, credId);
      setCreds(await listCredentials(bridge.fetch, selectedId));
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onSetPassword = async () => {
    if (!selectedId || !newPassword.trim()) return;
    setErr("");
    setPasswordSaved(false);
    setBusy(true);
    try {
      await setPrincipalPassword(bridge.fetch, selectedId, newPassword.trim());
      setNewPassword("");
      setPasswordSaved(true);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onAssignRole = async () => {
    if (!selectedId || !assignRoleName.trim()) return;
    setBusy(true);
    try {
      await assignRole(bridge.fetch, selectedId, assignRoleName.trim());
      setAssignRoleName("");
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onUnassignRole = async (roleApiName: string) => {
    if (!selectedId) return;
    setBusy(true);
    try {
      await unassignRole(bridge.fetch, selectedId, roleApiName);
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onAssignPs = async () => {
    if (!selectedId || !assignPsName.trim()) return;
    setBusy(true);
    try {
      await assignPermissionSet(bridge.fetch, selectedId, assignPsName.trim());
      setAssignPsName("");
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onUnassignPs = async (apiName: string) => {
    if (!selectedId) return;
    setBusy(true);
    try {
      await unassignPermissionSet(bridge.fetch, selectedId, apiName);
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  if (!connected) {
    return (
      <ToolSurface testId="users-panel">
        <PanelHeader title="Users" subtitle="Principals on this install." />
        <EmptyState
          icon={<IconGovern size={28} />}
          title="Connect first"
          description="Open Settings → Environments to authenticate, then manage principals here."
        />
      </ToolSurface>
    );
  }

  return (
    <ToolSurface testId="users-panel">
      <PanelHeader
        title="Users"
        subtitle="Principals, credentials, freeze, and role / permission-set assignment via Client identity admin."
        actions={
          <>
            <Button variant="secondary" busy={busy} onClick={() => void load()}>
              Refresh
            </Button>
            <Button variant="primary" onClick={() => setCreating((v) => !v)}>
              {creating ? "Cancel" : "New principal"}
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
        <div className="env-card" data-testid="users-create">
          <div className="row">
            <label>
              Email
              <input
                value={form.email}
                onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))}
                data-testid="users-email"
              />
            </label>
            <label>
              Display name
              <input
                value={form.displayName}
                onChange={(e) => setForm((f) => ({ ...f, displayName: e.target.value }))}
              />
            </label>
          </div>
          <div className="row">
            <label>
              Type
              <select
                value={form.principalType}
                onChange={(e) => setForm((f) => ({ ...f, principalType: e.target.value }))}
              >
                <option value="user">user</option>
                <option value="service">service</option>
                <option value="agent">agent</option>
              </select>
            </label>
            <label>
              Role API name (required)
              <input
                value={form.roleApiName}
                onChange={(e) => setForm((f) => ({ ...f, roleApiName: e.target.value }))}
                list="govern-role-options"
                data-testid="users-role"
              />
            </label>
          </div>
          <Button variant="primary" busy={busy} onClick={() => void onCreate()} data-testid="users-create-btn">
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
            placeholder="email, name, type…"
            data-testid="users-filter"
          />
        </label>
      </div>

      <div className="govern-master-detail">
        <ul className="govern-list" data-testid="users-list">
          {filtered.map((p) => (
            <li key={p.id}>
              <button
                type="button"
                className={`govern-list-item ${p.id === selectedId ? "active" : ""}`}
                onClick={() => setSelectedId(p.id)}
                data-testid={`user-row-${p.id}`}
              >
                <strong>{p.displayName || p.email || p.id}</strong>
                <span className="muted">
                  {p.principalType ?? "user"} · {p.email || p.id}
                </span>
                <StatusBadge tone={p.frozenAt ? "neutral" : p.isActive === false ? "neutral" : "success"}>
                  {p.frozenAt ? "Frozen" : p.isActive === false ? "Inactive" : "Active"}
                </StatusBadge>
              </button>
            </li>
          ))}
          {filtered.length === 0 ? <li className="muted">No principals</li> : null}
        </ul>

        <div className="env-card" data-testid="users-detail">
          {!selected ? (
            <EmptyState title="Select a principal" description="Choose a row to edit profile, credentials, and assignments." />
          ) : (
            <>
              <PanelHeader
                title={selected.displayName || selected.email || selected.id}
                subtitle={`${selected.principalType ?? "user"} · ${selected.id}`}
              />
              <div className="row">
                <label>
                  Display name
                  <input
                    value={edit.displayName}
                    onChange={(e) => setEdit((x) => ({ ...x, displayName: e.target.value }))}
                  />
                </label>
                <label>
                  Title
                  <input value={edit.title} onChange={(e) => setEdit((x) => ({ ...x, title: e.target.value }))} />
                </label>
              </div>
              <div className="row">
                <label>
                  Department
                  <input
                    value={edit.department}
                    onChange={(e) => setEdit((x) => ({ ...x, department: e.target.value }))}
                  />
                </label>
              </div>
              <div className="row">
                <Button variant="primary" busy={busy} onClick={() => void onSave()}>
                  Save profile
                </Button>
                {selected.frozenAt ? (
                  <Button variant="secondary" busy={busy} onClick={() => void onUnfreeze()}>
                    Unfreeze
                  </Button>
                ) : (
                  <>
                    <input
                      placeholder="Freeze reason"
                      value={freezeReason}
                      onChange={(e) => setFreezeReason(e.target.value)}
                    />
                    <Button variant="secondary" busy={busy} onClick={() => void onFreeze()}>
                      Freeze
                    </Button>
                  </>
                )}
                <Button variant="ghost" busy={busy} onClick={() => void onDeactivate()}>
                  Deactivate
                </Button>
              </div>

              <h4>Roles</h4>
              <ul className="repo-file-list">
                {(selected.roleApiNames ?? []).map((r) => (
                  <li key={r}>
                    {r}{" "}
                    <Button variant="ghost" onClick={() => void onUnassignRole(r)}>
                      Unassign
                    </Button>
                  </li>
                ))}
              </ul>
              <div className="row">
                <input
                  list="govern-role-options"
                  value={assignRoleName}
                  onChange={(e) => setAssignRoleName(e.target.value)}
                  placeholder="Role API name"
                />
                <Button variant="secondary" onClick={() => void onAssignRole()}>
                  Assign role
                </Button>
              </div>

              <h4>Permission sets</h4>
              <ul className="repo-file-list">
                {(selected.permissionSetApiNames ?? []).map((r) => (
                  <li key={r}>
                    {r}{" "}
                    <Button variant="ghost" onClick={() => void onUnassignPs(r)}>
                      Unassign
                    </Button>
                  </li>
                ))}
              </ul>
              <div className="row">
                <input
                  list="govern-ps-options"
                  value={assignPsName}
                  onChange={(e) => setAssignPsName(e.target.value)}
                  placeholder="Permission set API name"
                />
                <Button variant="secondary" onClick={() => void onAssignPs()}>
                  Assign permission set
                </Button>
              </div>

              <h4>Credentials</h4>
              <div className="row">
                <Button variant="secondary" busy={busy} onClick={() => void onCreateCred()}>
                  Create credential
                </Button>
              </div>
              <ul className="repo-file-list">
                {creds.map((c) => (
                  <li key={c.id}>
                    {c.label || c.id}
                    {c.credentialKind ? ` (${c.credentialKind})` : ""} {c.revokedAt ? "(revoked)" : ""}
                    {!c.revokedAt ? (
                      <Button variant="ghost" onClick={() => void onRevokeCred(c.id)}>
                        Revoke
                      </Button>
                    ) : null}
                  </li>
                ))}
              </ul>

              {selected.principalType === "user" || !selected.principalType ? (
                <>
                  <h4>Password login</h4>
                  <p className="muted">
                    Set or rotate a local password (no email reset). Prefer customer SSO when configured.
                  </p>
                  <div className="row">
                    <label>
                      New password
                      <input
                        type="password"
                        value={newPassword}
                        onChange={(e) => {
                          setNewPassword(e.target.value);
                          setPasswordSaved(false);
                        }}
                        placeholder="At least 10 characters"
                        data-testid="users-set-password"
                        autoComplete="new-password"
                      />
                    </label>
                    <Button
                      variant="secondary"
                      busy={busy}
                      onClick={() => void onSetPassword()}
                      data-testid="users-set-password-save"
                    >
                      Set password
                    </Button>
                  </div>
                  {passwordSaved ? <p className="muted">Password updated.</p> : null}
                </>
              ) : null}
            </>
          )}
        </div>
      </div>

      <datalist id="govern-role-options">
        {roles.map((r) => (
          <option key={r.apiName} value={r.apiName} />
        ))}
      </datalist>
      <datalist id="govern-ps-options">
        {sets.map((s) => (
          <option key={s.apiName} value={s.apiName} />
        ))}
      </datalist>
    </ToolSurface>
  );
}
