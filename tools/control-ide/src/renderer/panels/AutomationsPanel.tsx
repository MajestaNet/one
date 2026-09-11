import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import * as monaco from "monaco-editor";
import type { AppBridge } from "../App";
import { useTheme } from "../ThemeContext";
import { monacoThemeFor } from "../theme";
import { invokeAutomationRun } from "../run/automations";
import { Button, EmptyState, PanelHeader, StatusBadge, ToolSurface } from "../ui";
import { IconMetadata } from "../icons/Icons";

type AutomationRow = {
  apiName: string;
  label: string;
  objectApiName: string;
  triggerEvent?: string;
  active?: boolean;
  runtime?: string;
  execution?: string;
  entryFile?: string;
  ownership?: string;
  packageName?: string;
  description?: string;
  source?: string;
};

type OwnershipFilter = "all" | "package" | "custom";

const STARTER_TS = `import type { AutomationContext, AutomationResult } from "one:automation";

export default async function run(ctx: AutomationContext): Promise<AutomationResult> {
  // Use ctx.createRecord / ctx.updateRecord / ctx.query — no third-party imports (ADR-014).
  ctx.log("automation ran for", ctx.trigger.recordId);
  return { ok: true };
}
`;

function yamlScalar(value: unknown): string {
  if (value == null) return "null";
  if (typeof value === "boolean" || typeof value === "number") return String(value);
  const s = String(value);
  if (s === "" || /[:#\n'"{}[\],&*?|<>=!%@`]/.test(s) || /^\s|\s$/.test(s)) {
    return JSON.stringify(s);
  }
  return s;
}

function isManaged(row: AutomationRow | null | undefined): boolean {
  return row?.ownership === "managed";
}

function yamlForAutomation(a: {
  apiName: string;
  label: string;
  objectApiName: string;
  triggerEvent: string;
  entryFile: string;
  description: string;
}): string {
  return [
    `apiName: ${yamlScalar(a.apiName)}`,
    `label: ${yamlScalar(a.label)}`,
    `objectApiName: ${yamlScalar(a.objectApiName)}`,
    `triggerEvent: ${yamlScalar(a.triggerEvent)}`,
    `description: ${yamlScalar(a.description)}`,
    `active: true`,
    `runtime: code`,
    `execution: async`,
    `entryFile: ${yamlScalar(a.entryFile)}`,
    `ownership: custom`,
    `packageName: customer.default`,
    `actions: []`,
    "",
  ].join("\n");
}

function packDisabledMessage(err: unknown): string | null {
  const msg = err instanceof Error ? err.message : String(err);
  if (msg.includes("PACKAGE_NOT_ENABLED")) {
    return "Package is not enabled — this automation will not run (409 PACKAGE_NOT_ENABLED).";
  }
  return null;
}

export function AutomationsPanel({
  bridge,
  refreshKey = 0,
  focusPath,
  onFocusConsumed,
}: {
  bridge: AppBridge;
  refreshKey?: number;
  focusPath?: string | null;
  onFocusConsumed?: () => void;
}) {
  const theme = useTheme();
  const [list, setList] = useState<AutomationRow[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [managedDetail, setManagedDetail] = useState<AutomationRow | null>(null);
  const [ownershipFilter, setOwnershipFilter] = useState<OwnershipFilter>("all");
  const [err, setErr] = useState("");
  const [warn, setWarn] = useState("");
  const [busy, setBusy] = useState(false);
  const [showNew, setShowNew] = useState(false);
  const [form, setForm] = useState({
    apiName: "",
    label: "",
    objectApiName: "Contact",
    triggerEvent: "create",
    description: "",
  });
  const [editorRel, setEditorRel] = useState("");
  const [dirty, setDirty] = useState(false);
  const [runMsg, setRunMsg] = useState("");
  const host = useRef<HTMLDivElement>(null);
  const editor = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);

  useEffect(() => {
    if (!host.current || editor.current) return;
    editor.current = monaco.editor.create(host.current, {
      value: "// Select an automation entry file or YAML\n",
      language: "typescript",
      theme: monacoThemeFor(theme),
      minimap: { enabled: false },
      automaticLayout: true,
    });
    const sub = editor.current.onDidChangeModelContent(() => setDirty(true));
    return () => {
      sub.dispose();
      editor.current?.dispose();
      editor.current = null;
    };
  }, []);

  useEffect(() => {
    monaco.editor.setTheme(monacoThemeFor(theme));
  }, [theme]);

  const loadList = useCallback(async () => {
    if (!bridge.session?.token) {
      setList([]);
      return;
    }
    setBusy(true);
    setErr("");
    try {
      const row = (await bridge.fetch("/metadata/v1/automations")) as { automations?: AutomationRow[] };
      setList(row.automations ?? []);
    } catch (e) {
      setErr(String(e));
      setList([]);
    } finally {
      setBusy(false);
    }
  }, [bridge]);

  useEffect(() => {
    void loadList();
    setSelected(null);
    setManagedDetail(null);
    setWarn("");
  }, [loadList, refreshKey]);

  const filtered = useMemo(() => {
    return list.filter((a) => {
      if (ownershipFilter === "package") return isManaged(a);
      if (ownershipFilter === "custom") return !isManaged(a);
      return true;
    });
  }, [list, ownershipFilter]);

  const openLocalFile = async (rel: string) => {
    setErr("");
    const root = bridge.session?.repoPath;
    if (!root || !window.one) {
      setErr("Set a local repo path in Repo to edit automation source files.");
      return;
    }
    try {
      const text = await window.one.readText(root, rel);
      setEditorRel(rel);
      const lang = /\.ya?ml$/i.test(rel) ? "yaml" : "typescript";
      const model = editor.current?.getModel?.();
      if (model) monaco.editor.setModelLanguage(model, lang);
      editor.current?.updateOptions?.({ readOnly: false });
      editor.current?.setValue?.(text);
      setDirty(false);
    } catch (e) {
      setErr(String(e));
    }
  };

  useEffect(() => {
    if (!focusPath) return;
    void (async () => {
      await openLocalFile(focusPath);
      onFocusConsumed?.();
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- one-shot deep-link
  }, [focusPath]);

  const saveLocal = async () => {
    setErr("");
    const root = bridge.session?.repoPath;
    if (!root || !editorRel || !window.one || !editor.current) return;
    setBusy(true);
    try {
      await window.one.writeText(root, editorRel, editor.current.getValue());
      setDirty(false);
      setWarn("Saved local file — commit in Repo, then pack from Ship.");
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const showManagedSource = (detail: AutomationRow) => {
    const model = editor.current?.getModel?.();
    if (model) monaco.editor.setModelLanguage(model, "typescript");
    editor.current?.updateOptions?.({ readOnly: true });
    editor.current?.setValue?.(detail.source || "// No source on this managed automation.");
    setEditorRel("");
    setDirty(false);
  };

  const selectAutomation = async (apiName: string) => {
    setSelected(apiName);
    setRunMsg("");
    const row = list.find((a) => a.apiName === apiName);
    if (isManaged(row)) {
      setBusy(true);
      try {
        const detail = (await bridge.fetch(
          `/metadata/v1/automations/${encodeURIComponent(apiName)}`,
        )) as AutomationRow;
        setManagedDetail(detail);
        showManagedSource(detail);
      } catch (e) {
        setErr(String(e));
        setManagedDetail(row ?? null);
      } finally {
        setBusy(false);
      }
      return;
    }
    setManagedDetail(null);
    editor.current?.updateOptions?.({ readOnly: false });
    if (row?.entryFile && bridge.session?.repoPath) {
      await openLocalFile(row.entryFile);
    } else if (bridge.session?.repoPath) {
      await openLocalFile(`metadata/automations/${apiName}.yaml`);
    }
  };

  const mirrorLocal = async (
    apiName: string,
    label: string,
    objectApiName: string,
    entryFile: string,
    description: string,
  ) => {
    const root = bridge.session?.repoPath;
    if (!root || !window.one) return;
    const yamlRel = `metadata/automations/${apiName}.yaml`;
    await window.one.writeText(
      root,
      yamlRel,
      yamlForAutomation({
        apiName,
        label,
        objectApiName,
        triggerEvent: form.triggerEvent || "create",
        entryFile,
        description,
      }),
    );
    try {
      await window.one.readText(root, entryFile);
    } catch {
      await window.one.writeText(root, entryFile, STARTER_TS);
    }
  };

  const createAutomation = async () => {
    setErr("");
    setWarn("");
    if (!bridge.session?.token) {
      setErr("Connect first");
      return;
    }
    const apiName = form.apiName.trim();
    const label = form.label.trim();
    const objectApiName = form.objectApiName.trim();
    const description = form.description.trim();
    if (!apiName || !label || !objectApiName) {
      setErr("apiName, label, and objectApiName are required");
      return;
    }
    const entryFile = `src/automations/${apiName
      .replace(/([a-z])([A-Z])/g, "$1_$2")
      .replace(/__/g, "_")
      .toLowerCase()}.ts`;
    setBusy(true);
    try {
      await bridge.fetch("/metadata/v1/automations", {
        method: "POST",
        body: JSON.stringify({
          apiName,
          label,
          objectApiName,
          triggerEvent: form.triggerEvent || "create",
          description,
          active: true,
          runtime: "code",
          execution: "async",
          entryFile,
          actions: [],
        }),
      });
      try {
        await mirrorLocal(apiName, label, objectApiName, entryFile, description);
      } catch (e) {
        setWarn(`Created on install; local YAML mirror skipped: ${e}`);
      }
      setShowNew(false);
      setForm({ apiName: "", label: "", objectApiName: "Contact", triggerEvent: "create", description: "" });
      await loadList();
      setSelected(apiName);
      if (bridge.session?.repoPath) await openLocalFile(entryFile);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const selectedRow = (selected && (managedDetail?.apiName === selected ? managedDetail : null))
    ?? list.find((a) => a.apiName === selected)
    ?? null;
  const selectedManaged = isManaged(selectedRow);

  const toggleActive = async (apiName: string, active: boolean) => {
    setErr("");
    setBusy(true);
    try {
      await bridge.fetch(`/metadata/v1/automations/${encodeURIComponent(apiName)}`, {
        method: "PATCH",
        body: JSON.stringify({ active }),
      });
      await loadList();
      if (selectedManaged) {
        setManagedDetail((cur) => (cur && cur.apiName === apiName ? { ...cur, active } : cur));
      }
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const runNow = async (apiName: string) => {
    setErr("");
    setRunMsg("");
    setBusy(true);
    try {
      const run = await invokeAutomationRun(bridge.fetch, apiName);
      setRunMsg(run.lastError ? `${run.status}: ${run.lastError}` : `${run.status}${run.id ? ` · ${run.id}` : ""}`);
      if (run.status === "failed") setErr(run.lastError || "Automation run failed");
    } catch (e) {
      const packMsg = packDisabledMessage(e);
      setErr(packMsg || String(e));
    } finally {
      setBusy(false);
    }
  };

  if (!bridge.session?.token) {
    return (
      <ToolSurface testId="automations-panel">
        <PanelHeader title="Automations" subtitle="Code automations on the active install (ADR-014)." />
        <EmptyState
          icon={<IconMetadata size={28} />}
          title="Connect to build automations"
          description="Open Settings → Environments to authenticate, then create code automations and edit TypeScript in the local customer repo."
        />
      </ToolSurface>
    );
  }

  return (
    <ToolSurface testId="automations-panel">
      <PanelHeader
        title="Automations"
        subtitle="Package process is managed on the install. Custom automations edit TypeScript locally, then pack from Ship."
        actions={
          <>
            <Button variant="secondary" busy={busy} onClick={() => void loadList()}>
              Refresh
            </Button>
            <Button variant="primary" onClick={() => setShowNew((v) => !v)} data-testid="automations-new">
              {showNew ? "Cancel" : "New automation"}
            </Button>
          </>
        }
      />
      {err && <p className="err">{err}</p>}
      {warn && <p className="muted" data-testid="automations-warn">{warn}</p>}

      <div className="agents-section-filters" data-testid="automations-filters">
        {([
          ["all", "All"],
          ["package", "Package"],
          ["custom", "Custom"],
        ] as const).map(([id, label]) => (
          <button
            key={id}
            type="button"
            className={`agents-section-chip ${ownershipFilter === id ? "active" : ""}`}
            onClick={() => setOwnershipFilter(id)}
            data-testid={`automations-filter-${id}`}
          >
            {label}
          </button>
        ))}
      </div>

      {showNew ? (
        <div className="env-card" data-testid="automations-create">
          <div className="row">
            <label>
              API name
              <input
                value={form.apiName}
                onChange={(e) => setForm((f) => ({ ...f, apiName: e.target.value }))}
                placeholder="CreateAccount_From_Contact"
                data-testid="automations-api-name"
              />
            </label>
            <label>
              Label
              <input
                value={form.label}
                onChange={(e) => setForm((f) => ({ ...f, label: e.target.value }))}
                placeholder="Create Account from Contact"
              />
            </label>
          </div>
          <div className="row">
            <label>
              Object
              <input
                value={form.objectApiName}
                onChange={(e) => setForm((f) => ({ ...f, objectApiName: e.target.value }))}
                placeholder="Contact"
              />
            </label>
            <label>
              Trigger
              <select
                value={form.triggerEvent}
                onChange={(e) => setForm((f) => ({ ...f, triggerEvent: e.target.value }))}
              >
                <option value="create">create</option>
                <option value="update">update</option>
                <option value="delete">delete</option>
              </select>
            </label>
          </div>
          <label>
            Description
            <input
              value={form.description}
              onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
              placeholder="What this automation does (optional, ≤500 characters)"
              data-testid="automations-description"
              maxLength={500}
            />
          </label>
          <Button variant="primary" busy={busy} onClick={() => void createAutomation()} data-testid="automations-create-btn">
            Create code automation
          </Button>
        </div>
      ) : null}

      <div className="govern-master-detail">
        <div>
          <p className="muted">On install</p>
          {filtered.length === 0 ? (
            <EmptyState
              title="No automations yet"
              description="Create a code automation, or initialize the sample customer repo for CreateAccount_From_Contact."
            />
          ) : (
            <ul className="govern-list" data-testid="automations-list">
              {filtered.map((a) => (
                <li key={a.apiName}>
                  <button
                    type="button"
                    className={`govern-list-item ${selected === a.apiName ? "active" : ""}`}
                    onClick={() => void selectAutomation(a.apiName)}
                    data-testid={`automation-${a.apiName}`}
                  >
                    <strong>{a.label || a.apiName}</strong>
                    <span className="row" style={{ gap: "0.35rem", flexWrap: "wrap" }}>
                      <StatusBadge tone={isManaged(a) ? "neutral" : "accent"}>
                        {isManaged(a) ? "managed" : "custom"}
                      </StatusBadge>
                      <StatusBadge tone={a.active === false ? "neutral" : "success"}>
                        {a.active === false ? "Off" : "On"}
                      </StatusBadge>
                      {a.packageName ? <span className="muted mono">{a.packageName}</span> : null}
                    </span>
                    {a.description ? (
                      <span className="muted" data-testid={`automation-desc-${a.apiName}`}>
                        {a.description}
                      </span>
                    ) : null}
                    <span className="muted">
                      {a.objectApiName} · {a.triggerEvent ?? "create"} · {a.runtime ?? "actions"}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
        <div>
          {selectedManaged ? (
            <div className="editor-chrome">
              <span className="editor-breadcrumb" data-testid="automations-managed-source">
                Managed source (read-only)
              </span>
              <StatusBadge tone="neutral">Package</StatusBadge>
            </div>
          ) : (
            <div className="editor-chrome">
              <span className="editor-breadcrumb">{editorRel || "src/automations/**"}</span>
              {dirty ? (
                <StatusBadge tone="warn">
                  <span className="dirty-dot" /> Unsaved
                </StatusBadge>
              ) : editorRel ? (
                <StatusBadge tone="success">Saved</StatusBadge>
              ) : null}
              <Button
                variant="primary"
                busy={busy}
                disabled={!editorRel}
                onClick={() => void saveLocal()}
                data-testid="automations-save"
              >
                Save file
              </Button>
            </div>
          )}
          <div className="editor-shell" data-testid="automations-editor">
            <div ref={host} className="monaco-host" style={{ minHeight: 280, width: "100%" }} />
          </div>
          {selected ? (
            <div className="row" style={{ marginTop: "0.5rem" }}>
              {selectedManaged ? (
                <p className="muted">
                  {selectedRow?.description || "Managed package automation."} Trigger{" "}
                  {selectedRow?.triggerEvent ?? "update"} · {selectedRow?.execution ?? "sync"}. Source is
                  product-owned — toggle Active to turn process off.
                </p>
              ) : (
                <>
                  <Button
                    variant="ghost"
                    onClick={() => void openLocalFile(`metadata/automations/${selected}.yaml`)}
                  >
                    Open YAML
                  </Button>
                  {selectedRow?.entryFile ? (
                    <Button
                      variant="ghost"
                      onClick={() => {
                        if (selectedRow.entryFile) void openLocalFile(selectedRow.entryFile);
                      }}
                    >
                      Open entry TS
                    </Button>
                  ) : null}
                </>
              )}
              <label className="row">
                <input
                  type="checkbox"
                  checked={selectedRow?.active !== false}
                  onChange={(e) => void toggleActive(selected, e.target.checked)}
                  data-testid="automations-active"
                />
                Active
              </label>
              <Button
                variant="primary"
                busy={busy}
                onClick={() => void runNow(selected)}
                data-testid="automations-run"
              >
                Run now
              </Button>
            </div>
          ) : null}
          {runMsg ? <p className="muted" data-testid="automations-run-status">{runMsg}</p> : null}
        </div>
      </div>
    </ToolSurface>
  );
}
