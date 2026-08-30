import { useCallback, useEffect, useMemo, useState } from "react";
import type { AppBridge } from "../App";
import {
  createPermissionSet,
  createRole,
  deleteRole,
  listPermissionSets,
  listRoles,
  patchPermissionSet,
  patchRole,
  type PermissionSet,
  type Role,
  type ToolAccessEntry,
} from "../govern";
import { IDE_CAPABILITY_GROUPS } from "../scopes";
import { Button, EmptyState, PanelHeader, StatusBadge, ToolSurface } from "../ui";
import { IconGovern } from "../icons/Icons";

type Section = "roles" | "sets";
type PsWizardStep = "identity" | "caps" | "tools" | "data" | "review";
type PsWizardMode = "create" | "edit";

const PS_WIZARD_STEPS: PsWizardStep[] = ["identity", "caps", "tools", "data", "review"];

const IDE_CAP_IDS = new Set(IDE_CAPABILITY_GROUPS.flatMap((g) => g.caps.map((c) => c.id)));

type ObjectPermRow = {
  objectApiName: string;
  canCreate: boolean;
  canRead: boolean;
  canUpdate: boolean;
  canDelete: boolean;
  viewAll: boolean;
  modifyAll: boolean;
};

type PsWizardForm = {
  apiName: string;
  label: string;
  description: string;
  systemPermissions: string;
  allTools: boolean;
  tools: ToolAccessEntry[];
  objectPermissions: ObjectPermRow[];
};

function emptyWizardForm(): PsWizardForm {
  return {
    apiName: "",
    label: "",
    description: "",
    systemPermissions: "",
    allTools: false,
    tools: [],
    objectPermissions: [],
  };
}

function parseCapList(raw: string): string[] {
  return raw
    .split(/[,\s]+/)
    .map((s) => s.trim())
    .filter(Boolean);
}

function formatCapList(caps: string[]): string {
  return caps.join(", ");
}

function formFromPermissionSet(pset: PermissionSet): PsWizardForm {
  const ta = pset.toolAccess;
  const objPerms = (pset.dataAccess?.objectPermissions ??
    (Array.isArray(pset.objectPermissions) ? pset.objectPermissions : [])) as ObjectPermRow[];
  return {
    apiName: pset.apiName,
    label: pset.label ?? "",
    description: pset.description ?? "",
    systemPermissions: (pset.systemPermissions ?? []).join(", "),
    allTools: Boolean(ta?.allTools),
    tools: Array.isArray(ta?.tools)
      ? ta.tools.map((t) => ({
          apiName: String(t.apiName ?? ""),
          canOpen: Boolean(t.canOpen),
          canInteract: Boolean(t.canInteract),
          canModify: Boolean(t.canModify),
          canPublish: Boolean(t.canPublish),
        }))
      : [],
    objectPermissions: Array.isArray(objPerms)
      ? objPerms.map((o) => ({
          objectApiName: String(o.objectApiName ?? ""),
          canCreate: Boolean(o.canCreate),
          canRead: Boolean(o.canRead),
          canUpdate: Boolean(o.canUpdate),
          canDelete: Boolean(o.canDelete),
          viewAll: Boolean(o.viewAll),
          modifyAll: Boolean(o.modifyAll),
        }))
      : [],
  };
}

function IdeCapCheckboxes({
  selected,
  disabled,
  onToggle,
}: {
  selected: Set<string>;
  disabled?: boolean;
  onToggle: (id: string) => void;
}) {
  return (
    <div className="ide-cap-groups" data-testid="ide-cap-checkboxes">
      {IDE_CAPABILITY_GROUPS.map((group) => (
        <fieldset key={group.label} className="ide-cap-group" disabled={disabled}>
          <legend>{group.label}</legend>
          <div className="ide-cap-list">
            {group.caps.map((cap) => (
              <label key={cap.id} className="ide-cap-item">
                <input
                  type="checkbox"
                  checked={selected.has(cap.id)}
                  onChange={() => onToggle(cap.id)}
                  data-testid={`ide-cap-${cap.id}`}
                />
                <span>{cap.label}</span>
                <span className="muted">{cap.id}</span>
              </label>
            ))}
          </div>
        </fieldset>
      ))}
    </div>
  );
}

export function PermissionsPanel({ bridge }: { bridge: AppBridge }) {
  const connected = Boolean(bridge.session?.baseUrl && bridge.session?.token);
  const [section, setSection] = useState<Section>("roles");
  const [roles, setRoles] = useState<Role[]>([]);
  const [sets, setSets] = useState<PermissionSet[]>([]);
  const [selectedRole, setSelectedRole] = useState<string | null>(null);
  const [selectedSet, setSelectedSet] = useState<string | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [creating, setCreating] = useState(false);
  const [roleForm, setRoleForm] = useState({ apiName: "", label: "", scopes: "client" });
  const [roleEdit, setRoleEdit] = useState({ label: "", scopes: "" });

  const [psWizardOpen, setPsWizardOpen] = useState(false);
  const [psWizardMode, setPsWizardMode] = useState<PsWizardMode>("create");
  const [psWizardStep, setPsWizardStep] = useState<PsWizardStep>("identity");
  const [psForm, setPsForm] = useState<PsWizardForm>(emptyWizardForm);
  const [newObjectApiName, setNewObjectApiName] = useState("");

  const role = roles.find((r) => r.apiName === selectedRole) ?? null;
  const pset = sets.find((s) => s.apiName === selectedSet) ?? null;
  const stepIndex = PS_WIZARD_STEPS.indexOf(psWizardStep);

  const load = useCallback(async () => {
    if (!connected) return;
    setErr("");
    setBusy(true);
    try {
      const [r, s] = await Promise.all([
        listRoles(bridge.fetch).catch(() => [] as Role[]),
        listPermissionSets(bridge.fetch, {
          includeDataAccess: true,
          includeToolAccess: true,
        }).catch(() => [] as PermissionSet[]),
      ]);
      setRoles(r);
      setSets(s);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  }, [bridge.fetch, connected]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (role) {
      setRoleEdit({
        label: role.label ?? "",
        scopes: (role.scopes ?? []).join(", "),
      });
    }
  }, [role]);

  const resetPsWizard = () => {
    setPsWizardOpen(false);
    setPsWizardMode("create");
    setPsWizardStep("identity");
    setPsForm(emptyWizardForm());
    setNewObjectApiName("");
  };

  const openCreateWizard = () => {
    setPsWizardMode("create");
    setPsWizardStep("identity");
    setPsForm(emptyWizardForm());
    setNewObjectApiName("");
    setPsWizardOpen(true);
    setCreating(false);
  };

  const openEditWizard = (target: PermissionSet) => {
    setSelectedSet(target.apiName);
    setPsWizardMode("edit");
    setPsWizardStep("identity");
    setPsForm(formFromPermissionSet(target));
    setNewObjectApiName("");
    setPsWizardOpen(true);
    setCreating(false);
  };

  const onCreateRole = async () => {
    setBusy(true);
    setErr("");
    try {
      const created = await createRole(bridge.fetch, {
        apiName: roleForm.apiName.trim(),
        label: roleForm.label.trim() || undefined,
        scopes: roleForm.scopes
          .split(/[,\s]+/)
          .map((s) => s.trim())
          .filter(Boolean),
      });
      setCreating(false);
      setRoleForm({ apiName: "", label: "", scopes: "client" });
      await load();
      setSelectedRole(created.apiName);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onSaveRole = async () => {
    if (!selectedRole) return;
    setBusy(true);
    try {
      await patchRole(bridge.fetch, selectedRole, {
        label: roleEdit.label.trim() || undefined,
        scopes: roleEdit.scopes
          .split(/[,\s]+/)
          .map((s) => s.trim())
          .filter(Boolean),
      });
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const onDeleteRole = async () => {
    if (!selectedRole) return;
    if (!confirm(`Delete role ${selectedRole}?`)) return;
    setBusy(true);
    try {
      await deleteRole(bridge.fetch, selectedRole, true);
      setSelectedRole(null);
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const formCaps = useMemo(() => new Set(parseCapList(psForm.systemPermissions)), [psForm.systemPermissions]);
  const otherFormCaps = parseCapList(psForm.systemPermissions).filter((c) => !IDE_CAP_IDS.has(c));

  const toggleFormCap = (id: string) => {
    const next = new Set(formCaps);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    const ideSelected = [...next].filter((c) => IDE_CAP_IDS.has(c));
    setPsForm((f) => ({
      ...f,
      systemPermissions: formatCapList([...otherFormCaps, ...ideSelected]),
    }));
  };

  const canAdvancePs = (): boolean => {
    switch (psWizardStep) {
      case "identity":
        return Boolean(psForm.apiName.trim()) && Boolean(psForm.label.trim() || psForm.apiName.trim());
      case "caps":
      case "tools":
      case "data":
      case "review":
        return true;
      default:
        return false;
    }
  };

  const submitPsWizard = async () => {
    setBusy(true);
    setErr("");
    try {
      const systemPermissions = parseCapList(psForm.systemPermissions);
      const toolAccess = {
        allTools: psForm.allTools,
        tools: psForm.tools.filter((t) => t.apiName),
      };
      const objectPermissions = psForm.objectPermissions.filter((o) => o.objectApiName.trim());
      const label = psForm.label.trim() || psForm.apiName.trim();
      const description = psForm.description.trim() || undefined;
      if (psWizardMode === "create") {
        const created = await createPermissionSet(bridge.fetch, {
          apiName: psForm.apiName.trim(),
          label,
          description,
          systemPermissions,
          toolAccess,
          objectPermissions,
          dataAccess: { objectPermissions },
        });
        resetPsWizard();
        await load();
        setSelectedSet(created.apiName);
      } else {
        await patchPermissionSet(bridge.fetch, psForm.apiName, {
          label,
          description,
          systemPermissions,
          toolAccess,
          dataAccess: { objectPermissions },
        });
        resetPsWizard();
        await load();
        setSelectedSet(psForm.apiName);
      }
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const addObjectPerm = () => {
    const apiName = newObjectApiName.trim();
    if (!apiName) return;
    if (psForm.objectPermissions.some((o) => o.objectApiName === apiName)) {
      setNewObjectApiName("");
      return;
    }
    setPsForm((f) => ({
      ...f,
      objectPermissions: [
        ...f.objectPermissions,
        {
          objectApiName: apiName,
          canCreate: false,
          canRead: true,
          canUpdate: false,
          canDelete: false,
          viewAll: false,
          modifyAll: false,
        },
      ],
    }));
    setNewObjectApiName("");
  };

  if (!connected) {
    return (
      <ToolSurface testId="permissions-panel">
        <PanelHeader title="Permissions" subtitle="Roles and permission sets." />
        <EmptyState
          icon={<IconGovern size={28} />}
          title="Connect first"
          description="Open Settings → Environments to authenticate, then manage roles and permission sets here."
        />
      </ToolSurface>
    );
  }

  return (
    <ToolSurface testId="permissions-panel">
      <PanelHeader
        title="Permissions"
        subtitle="Roles grant API family scopes. Permission sets grant system and data-access capabilities. Assign to principals from Users."
        actions={
          <>
            <Button variant="secondary" busy={busy} onClick={() => void load()}>
              Refresh
            </Button>
            <Button
              variant="primary"
              onClick={() => {
                if (section === "roles") {
                  setCreating((v) => !v);
                  resetPsWizard();
                } else if (psWizardOpen && psWizardMode === "create") {
                  resetPsWizard();
                } else {
                  openCreateWizard();
                }
              }}
            >
              {section === "roles"
                ? creating
                  ? "Cancel"
                  : "New role"
                : psWizardOpen && psWizardMode === "create"
                  ? "Cancel"
                  : "New permission set"}
            </Button>
          </>
        }
      />

      <div className="panel-tabs" role="tablist" data-testid="permissions-tabs">
        <Button
          variant={section === "roles" ? "secondary" : "ghost"}
          role="tab"
          aria-selected={section === "roles"}
          onClick={() => {
            setSection("roles");
            setCreating(false);
            resetPsWizard();
          }}
        >
          Roles
        </Button>
        <Button
          variant={section === "sets" ? "secondary" : "ghost"}
          role="tab"
          aria-selected={section === "sets"}
          onClick={() => {
            setSection("sets");
            setCreating(false);
          }}
        >
          Permission sets
        </Button>
      </div>

      {err && <p className="err">{err}</p>}

      {section === "roles" ? (
        <>
          {creating ? (
            <div className="env-card" data-testid="roles-create">
              <div className="row">
                <label>
                  API name
                  <input
                    value={roleForm.apiName}
                    onChange={(e) => setRoleForm((f) => ({ ...f, apiName: e.target.value }))}
                    data-testid="role-api-name"
                  />
                </label>
                <label>
                  Label
                  <input
                    value={roleForm.label}
                    onChange={(e) => setRoleForm((f) => ({ ...f, label: e.target.value }))}
                  />
                </label>
              </div>
              <label>
                Scopes (comma-separated)
                <input
                  value={roleForm.scopes}
                  onChange={(e) => setRoleForm((f) => ({ ...f, scopes: e.target.value }))}
                />
              </label>
              <Button
                variant="primary"
                busy={busy}
                disabled={!roleForm.apiName.trim()}
                onClick={() => void onCreateRole()}
                data-testid="role-create-btn"
              >
                Create role
              </Button>
            </div>
          ) : null}

          <div className="govern-master-detail">
            <ul className="govern-list" data-testid="roles-list">
              {roles.map((r) => (
                <li key={r.apiName}>
                  <button
                    type="button"
                    className={`govern-list-item ${r.apiName === selectedRole ? "active" : ""}`}
                    onClick={() => setSelectedRole(r.apiName)}
                    data-testid={`role-row-${r.apiName}`}
                  >
                    <strong>{r.label || r.apiName}</strong>
                    <span className="muted">{(r.scopes ?? []).join(", ") || "no scopes"}</span>
                    {r.isSystem ? <StatusBadge tone="neutral">System</StatusBadge> : null}
                  </button>
                </li>
              ))}
              {roles.length === 0 ? <li className="muted">No roles</li> : null}
            </ul>
            <div className="env-card" data-testid="roles-detail">
              {!role ? (
                <EmptyState title="Select a role" description="Choose a role to edit scopes or delete." />
              ) : (
                <>
                  <PanelHeader title={role.label || role.apiName} subtitle={role.apiName} />
                  <div className="row">
                    <label>
                      Label
                      <input
                        value={roleEdit.label}
                        onChange={(e) => setRoleEdit((x) => ({ ...x, label: e.target.value }))}
                        disabled={role.isSystem}
                      />
                    </label>
                  </div>
                  <label>
                    Scopes
                    <input
                      value={roleEdit.scopes}
                      onChange={(e) => setRoleEdit((x) => ({ ...x, scopes: e.target.value }))}
                      disabled={role.isSystem}
                    />
                  </label>
                  <div className="row">
                    <Button
                      variant="primary"
                      busy={busy}
                      disabled={role.isSystem}
                      onClick={() => void onSaveRole()}
                    >
                      Save
                    </Button>
                    <Button
                      variant="ghost"
                      busy={busy}
                      disabled={role.isSystem}
                      onClick={() => void onDeleteRole()}
                    >
                      Delete
                    </Button>
                  </div>
                </>
              )}
            </div>
          </div>
        </>
      ) : (
        <>
          {psWizardOpen ? (
            <div
              className="om-form om-new-object-form agents-wizard"
              data-testid={psWizardMode === "create" ? "sets-create" : "sets-edit"}
            >
              <div className="agents-wizard-steps" data-testid="ps-wizard-steps">
                {PS_WIZARD_STEPS.map((s, i) => (
                  <span
                    key={s}
                    className={`agents-wizard-step ${s === psWizardStep ? "active" : ""} ${
                      i < stepIndex ? "done" : ""
                    }`}
                  >
                    {i + 1}. {s}
                  </span>
                ))}
              </div>

              {psWizardStep === "identity" ? (
                <div data-testid="ps-wizard-identity">
                  <div className="row">
                    <label>
                      API name
                      <input
                        value={psForm.apiName}
                        onChange={(e) => setPsForm((f) => ({ ...f, apiName: e.target.value }))}
                        data-testid="ps-api-name"
                        disabled={psWizardMode === "edit"}
                      />
                    </label>
                    <label>
                      Label
                      <input
                        value={psForm.label}
                        onChange={(e) => setPsForm((f) => ({ ...f, label: e.target.value }))}
                        data-testid="ps-label"
                      />
                    </label>
                  </div>
                  <label>
                    Description
                    <input
                      value={psForm.description}
                      onChange={(e) => setPsForm((f) => ({ ...f, description: e.target.value }))}
                      data-testid="ps-description"
                    />
                  </label>
                </div>
              ) : null}

              {psWizardStep === "caps" ? (
                <div data-testid="ps-wizard-caps">
                  <label>
                    API / system capabilities (comma-separated)
                    <input
                      value={otherFormCaps.join(", ")}
                      onChange={(e) => {
                        const apiCaps = parseCapList(e.target.value);
                        const ideSelected = [...formCaps].filter((c) => IDE_CAP_IDS.has(c));
                        setPsForm((f) => ({
                          ...f,
                          systemPermissions: formatCapList([...apiCaps, ...ideSelected]),
                        }));
                      }}
                      placeholder="identity.users, authz.manage, metadata.build…"
                    />
                  </label>
                  <IdeCapCheckboxes selected={formCaps} onToggle={toggleFormCap} />
                </div>
              ) : null}

              {psWizardStep === "tools" ? (
                <div data-testid="ps-wizard-tools">
                  <fieldset className="tool-access-fieldset" data-testid="tool-access-fieldset">
                    <legend>Run Tools (ToolSpec permission matrix)</legend>
                    <label className="ide-cap-item">
                      <input
                        type="checkbox"
                        checked={psForm.allTools}
                        onChange={(e) => setPsForm((f) => ({ ...f, allTools: e.target.checked }))}
                        data-testid="tool-access-all"
                      />
                      <span>Open and interact with all Tools</span>
                      <span className="muted">all_tools</span>
                    </label>
                    {psForm.tools.length === 0 ? (
                      <p className="muted">No ToolSpecs in catalog yet — you can still grant All Tools.</p>
                    ) : (
                      <div className="ide-cap-list">
                        {psForm.tools.map((t) => (
                          <fieldset key={t.apiName} className="ide-cap-group">
                            <legend>{t.apiName}</legend>
                            {(["canOpen", "canInteract", "canModify", "canPublish"] as const).map((permission) => (
                              <label key={permission} className="ide-cap-item">
                                <input
                                  type="checkbox"
                                  checked={
                                    psForm.allTools && (permission === "canOpen" || permission === "canInteract")
                                      ? true
                                      : t[permission]
                                  }
                                  disabled={
                                    psForm.allTools && (permission === "canOpen" || permission === "canInteract")
                                  }
                                  onChange={() =>
                                    setPsForm((f) => ({
                                      ...f,
                                      tools: f.tools.map((row) =>
                                        row.apiName === t.apiName
                                          ? permission === "canOpen" && row.canOpen
                                            ? { ...row, canOpen: false, canInteract: false, canModify: false, canPublish: false }
                                            : { ...row, [permission]: !row[permission], canOpen: true }
                                          : row,
                                      ),
                                    }))
                                  }
                                  data-testid={`tool-access-${t.apiName}-${permission}`}
                                />
                                <span>{permission.replace(/^can/, "Can ")}</span>
                              </label>
                            ))}
                          </fieldset>
                        ))}
                      </div>
                    )}
                  </fieldset>
                </div>
              ) : null}

              {psWizardStep === "data" ? (
                <div data-testid="ps-wizard-data">
                  <p className="muted">
                    Object CRUD grants for this permission set. Field-level grants stay on the install catalog.
                  </p>
                  <div className="row">
                    <label>
                      Object API name
                      <input
                        value={newObjectApiName}
                        onChange={(e) => setNewObjectApiName(e.target.value)}
                        placeholder="Account"
                        data-testid="ps-object-api-name"
                      />
                    </label>
                    <Button
                      variant="secondary"
                      onClick={addObjectPerm}
                      disabled={!newObjectApiName.trim()}
                      data-testid="ps-object-add"
                    >
                      Add object
                    </Button>
                  </div>
                  {psForm.objectPermissions.length === 0 ? (
                    <p className="muted">No object grants yet — optional for create.</p>
                  ) : (
                    <table className="data-table" data-testid="ps-object-perms-table">
                      <thead>
                        <tr>
                          <th>Object</th>
                          <th>C</th>
                          <th>R</th>
                          <th>U</th>
                          <th>D</th>
                          <th>View all</th>
                          <th>Modify all</th>
                          <th />
                        </tr>
                      </thead>
                      <tbody>
                        {psForm.objectPermissions.map((row) => (
                          <tr key={row.objectApiName}>
                            <td className="mono">{row.objectApiName}</td>
                            {(
                              [
                                ["canCreate", "C"],
                                ["canRead", "R"],
                                ["canUpdate", "U"],
                                ["canDelete", "D"],
                                ["viewAll", "View all"],
                                ["modifyAll", "Modify all"],
                              ] as const
                            ).map(([key, label]) => (
                              <td key={key}>
                                <input
                                  type="checkbox"
                                  checked={row[key]}
                                  aria-label={`${row.objectApiName} ${label}`}
                                  onChange={() =>
                                    setPsForm((f) => ({
                                      ...f,
                                      objectPermissions: f.objectPermissions.map((o) =>
                                        o.objectApiName === row.objectApiName
                                          ? { ...o, [key]: !o[key] }
                                          : o,
                                      ),
                                    }))
                                  }
                                />
                              </td>
                            ))}
                            <td>
                              <Button
                                variant="ghost"
                                onClick={() =>
                                  setPsForm((f) => ({
                                    ...f,
                                    objectPermissions: f.objectPermissions.filter(
                                      (o) => o.objectApiName !== row.objectApiName,
                                    ),
                                  }))
                                }
                              >
                                Remove
                              </Button>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                </div>
              ) : null}

              {psWizardStep === "review" ? (
                <div data-testid="ps-wizard-review">
                  <ul className="agents-harness-preview">
                    <li>
                      <strong>API name:</strong> {psForm.apiName}
                    </li>
                    <li>
                      <strong>Label:</strong> {psForm.label || psForm.apiName}
                    </li>
                    <li>
                      <strong>System caps:</strong>{" "}
                      {parseCapList(psForm.systemPermissions).join(", ") || "none"}
                    </li>
                    <li>
                      <strong>Tools:</strong>{" "}
                      {psForm.allTools
                        ? "All Tools"
                        : psForm.tools.filter((t) => t.canOpen).map((t) => t.apiName).join(", ") ||
                          "none"}
                    </li>
                    <li>
                      <strong>Object grants:</strong>{" "}
                      {psForm.objectPermissions.map((o) => o.objectApiName).join(", ") || "none"}
                    </li>
                  </ul>
                </div>
              ) : null}

              <div className="row agents-wizard-nav">
                <Button
                  variant="secondary"
                  disabled={stepIndex <= 0}
                  onClick={() => setPsWizardStep(PS_WIZARD_STEPS[Math.max(0, stepIndex - 1)])}
                  data-testid="ps-wizard-back"
                >
                  Back
                </Button>
                {psWizardStep !== "review" ? (
                  <Button
                    variant="primary"
                    disabled={!canAdvancePs()}
                    onClick={() =>
                      setPsWizardStep(PS_WIZARD_STEPS[Math.min(PS_WIZARD_STEPS.length - 1, stepIndex + 1)])
                    }
                    data-testid="ps-wizard-next"
                  >
                    Next
                  </Button>
                ) : (
                  <Button
                    variant="primary"
                    busy={busy}
                    disabled={!canAdvancePs()}
                    onClick={() => void submitPsWizard()}
                    data-testid="ps-create-btn"
                  >
                    {psWizardMode === "create" ? "Create permission set" : "Save permission set"}
                  </Button>
                )}
                <Button variant="ghost" onClick={resetPsWizard}>
                  Cancel
                </Button>
              </div>
            </div>
          ) : null}

          <div className="govern-master-detail">
            <ul className="govern-list" data-testid="sets-list">
              {sets.map((s) => (
                <li key={s.apiName}>
                  <button
                    type="button"
                    className={`govern-list-item ${s.apiName === selectedSet ? "active" : ""}`}
                    onClick={() => {
                      setSelectedSet(s.apiName);
                      if (psWizardOpen) resetPsWizard();
                    }}
                    data-testid={`ps-row-${s.apiName}`}
                  >
                    <strong>{s.label || s.apiName}</strong>
                    <span className="muted">
                      {(s.systemPermissions ?? []).slice(0, 3).join(", ") || "no system perms"}
                    </span>
                    {s.isSystem ? <StatusBadge tone="neutral">System</StatusBadge> : null}
                  </button>
                </li>
              ))}
              {sets.length === 0 ? <li className="muted">No permission sets</li> : null}
            </ul>
            <div className="env-card" data-testid="sets-detail">
              {!pset ? (
                <EmptyState
                  title="Select a permission set"
                  description="Choose a set to review, or create one with the wizard."
                />
              ) : (
                <>
                  <PanelHeader
                    title={pset.label || pset.apiName}
                    subtitle={pset.apiName}
                    actions={
                      <Button
                        variant="primary"
                        disabled={pset.isSystem}
                        onClick={() => openEditWizard(pset)}
                        data-testid="ps-edit-btn"
                      >
                        Edit
                      </Button>
                    }
                  />
                  <p className="muted">{pset.description || "No description"}</p>
                  <p>
                    <strong>System caps:</strong>{" "}
                    {(pset.systemPermissions ?? []).join(", ") || "none"}
                  </p>
                  <p>
                    <strong>Tools:</strong>{" "}
                    {pset.toolAccess?.allTools
                      ? "All Tools"
                      : (pset.toolAccess?.tools ?? [])
                          .filter((t) => t.canOpen || t.canInteract || t.canModify || t.canPublish)
                          .map((t) => `${t.apiName} (${[
                            t.canOpen && "open",
                            t.canInteract && "interact",
                            t.canModify && "modify",
                            t.canPublish && "publish",
                          ].filter(Boolean).join(", ")})`)
                          .join("; ") || "none"}
                  </p>
                  <p>
                    <strong>Object grants:</strong>{" "}
                    {(
                      (pset.dataAccess?.objectPermissions as ObjectPermRow[] | undefined) ??
                      []
                    )
                      .map((o) => o.objectApiName)
                      .join(", ") || "none"}
                  </p>
                </>
              )}
            </div>
          </div>
        </>
      )}
    </ToolSurface>
  );
}
