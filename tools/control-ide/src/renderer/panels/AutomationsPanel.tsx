import { useCallback, useEffect, useRef, useState } from "react";
import * as monaco from "monaco-editor";
import type { AppBridge } from "../App";
import { useTheme } from "../ThemeContext";
import { monacoThemeFor } from "../theme";
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
};

const STARTER_TS = `import type { AutomationContext, AutomationResult } from "one:automation";

export default async function run(ctx: AutomationContext): Promise<AutomationResult> {
  // Use ctx.createRecord / ctx.updateRecord / ctx.query — no third-party imports (ADR-014).
  ctx.log("automation ran for", ctx.trigger.recordId);
  return { ok: true };
}
`;

function yamlForAutomation(a: {
  apiName: string;
  label: string;
  objectApiName: string;
  triggerEvent: string;
  entryFile: string;
}): string {
  return [
    `apiName: ${a.apiName}`,
    `label: ${a.label}`,
    `objectApiName: ${a.objectApiName}`,
    `triggerEvent: ${a.triggerEvent}`,
    `active: true`,
    `runtime: code`,
    `execution: async`,
    `entryFile: ${a.entryFile}`,
    `ownership: custom`,
    `packageName: customer.default`,
    `actions: []`,
    "",
  ].join("\n");
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
  const [err, setErr] = useState("");
  const [warn, setWarn] = useState("");
  const [busy, setBusy] = useState(false);
  const [showNew, setShowNew] = useState(false);
  const [form, setForm] = useState({
    apiName: "",
    label: "",
    objectApiName: "Contact",
    triggerEvent: "create",
  });
  const [editorRel, setEditorRel] = useState("");
  const [dirty, setDirty] = useState(false);
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
    setWarn("");
  }, [loadList, refreshKey]);

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
      const model = editor.current?.getModel();
      if (model) monaco.editor.setModelLanguage(model, lang);
      editor.current?.setValue(text);
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

  const selectAutomation = async (apiName: string) => {
    setSelected(apiName);
    const row = list.find((a) => a.apiName === apiName);
    if (row?.entryFile && bridge.session?.repoPath) {
      await openLocalFile(row.entryFile);
    } else if (bridge.session?.repoPath) {
      await openLocalFile(`metadata/automations/${apiName}.yaml`);
    }
  };

  const mirrorLocal = async (apiName: string, label: string, objectApiName: string, entryFile: string) => {
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
          active: true,
          runtime: "code",
          execution: "async",
          entryFile,
          actions: [],
        }),
      });
      try {
        await mirrorLocal(apiName, label, objectApiName, entryFile);
      } catch (e) {
        setWarn(`Created on install; local YAML mirror skipped: ${e}`);
      }
      setShowNew(false);
      setForm({ apiName: "", label: "", objectApiName: "Contact", triggerEvent: "create" });
      await loadList();
      setSelected(apiName);
      if (bridge.session?.repoPath) await openLocalFile(entryFile);
    } catch (e) {
      setErr(String(e));
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
        subtitle="Create code automations on the active env. Edit entry TypeScript locally, commit in Repo, pack from Ship — no CLI required."
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
          <Button variant="primary" busy={busy} onClick={() => void createAutomation()} data-testid="automations-create-btn">
            Create code automation
          </Button>
        </div>
      ) : null}

      <div className="govern-master-detail">
        <div>
          <p className="muted">On install</p>
          {list.length === 0 ? (
            <EmptyState
              title="No automations yet"
              description="Create a code automation, or initialize the sample customer repo for CreateAccount_From_Contact."
            />
          ) : (
            <ul className="govern-list" data-testid="automations-list">
              {list.map((a) => (
                <li key={a.apiName}>
                  <button
                    type="button"
                    className={`govern-list-item ${selected === a.apiName ? "active" : ""}`}
                    onClick={() => void selectAutomation(a.apiName)}
                    data-testid={`automation-${a.apiName}`}
                  >
                    <strong>{a.label || a.apiName}</strong>
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
          <div className="editor-chrome">
            <span className="editor-breadcrumb">{editorRel || "src/automations/**"}</span>
            {dirty ? (
              <StatusBadge tone="warn">
                <span className="dirty-dot" /> Unsaved
              </StatusBadge>
            ) : editorRel ? (
              <StatusBadge tone="success">Saved</StatusBadge>
            ) : null}
            <Button variant="primary" busy={busy} disabled={!editorRel} onClick={() => void saveLocal()}>
              Save file
            </Button>
          </div>
          <div className="editor-shell" data-testid="automations-editor">
            <div ref={host} className="monaco-host" style={{ minHeight: 280, width: "100%" }} />
          </div>
          {selected ? (
            <div className="row" style={{ marginTop: "0.5rem" }}>
              <Button
                variant="ghost"
                onClick={() => void openLocalFile(`metadata/automations/${selected}.yaml`)}
              >
                Open YAML
              </Button>
              {list.find((a) => a.apiName === selected)?.entryFile ? (
                <Button
                  variant="ghost"
                  onClick={() => {
                    const ef = list.find((a) => a.apiName === selected)?.entryFile;
                    if (ef) void openLocalFile(ef);
                  }}
                >
                  Open entry TS
                </Button>
              ) : null}
            </div>
          ) : null}
        </div>
      </div>
    </ToolSurface>
  );
}
