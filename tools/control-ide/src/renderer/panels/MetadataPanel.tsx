import { useEffect, useRef, useState } from "react";
import * as monaco from "monaco-editor";
import type { AppBridge } from "../App";
import { useTheme } from "../ThemeContext";
import { monacoThemeFor } from "../theme";
import { Button, EmptyState, PanelHeader, StatusBadge } from "../ui";
import { IconMetadata } from "../icons/Icons";

export function MetadataPanel({
  bridge,
  focusPath,
  onFocusConsumed,
}: {
  bridge: AppBridge;
  /** Deep-link from Repo dirty list — open this metadata YAML once. */
  focusPath?: string | null;
  onFocusConsumed?: () => void;
}) {
  const theme = useTheme();
  const [files, setFiles] = useState<string[]>([]);
  const [active, setActive] = useState("");
  const [dirty, setDirty] = useState(false);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const host = useRef<HTMLDivElement>(null);
  const editor = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);

  useEffect(() => {
    if (!host.current || editor.current) return;
    editor.current = monaco.editor.create(host.current, {
      value: "# select a YAML file\n",
      language: "yaml",
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
  }, []); // mount once; theme synced below

  useEffect(() => {
    monaco.editor.setTheme(monacoThemeFor(theme));
  }, [theme]);

  const refresh = async () => {
    setErr("");
    const root = bridge.session?.repoPath;
    if (!root || !window.one) {
      setErr("Set repo path in Repo panel (Electron required for filesystem).");
      return;
    }
    setBusy(true);
    try {
      const list = await window.one.listTree(root, "metadata");
      setFiles(list);
      return list;
    } finally {
      setBusy(false);
    }
  };

  const openFile = async (rel: string) => {
    setErr("");
    const root = bridge.session?.repoPath;
    if (!root || !window.one) return;
    try {
      const text = await window.one.readText(root, rel);
      setActive(rel);
      editor.current?.setValue(text);
      setDirty(false);
    } catch (e) {
      setErr(String(e));
    }
  };

  useEffect(() => {
    if (!focusPath) return;
    void (async () => {
      const list = files.length ? files : (await refresh()) ?? [];
      if (list && !list.includes(focusPath)) {
        setFiles((prev) => (prev.includes(focusPath) ? prev : [...prev, focusPath]));
      }
      await openFile(focusPath);
      onFocusConsumed?.();
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- intentional one-shot deep-link
  }, [focusPath]);

  const save = async () => {
    setErr("");
    const root = bridge.session?.repoPath;
    if (!root || !active || !window.one || !editor.current) return;
    setBusy(true);
    try {
      await window.one.writeText(root, active, editor.current.getValue());
      setDirty(false);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="panel tool-surface" data-tool-surface="true">
      <PanelHeader
        title="Metadata explorer"
        subtitle="Edit metadata/** YAML from the local customer repo. Pack/promote lives in Change / Deploy."
        actions={
          <>
            <Button variant="secondary" busy={busy} onClick={() => void refresh()}>
              Refresh tree
            </Button>
            <Button variant="primary" busy={busy} onClick={() => void save()} disabled={!active}>
              Save file
            </Button>
          </>
        }
      />
      {err && <p className="err">{err}</p>}
      <div className="editor-chrome">
        <span className="editor-breadcrumb">{active || "metadata/**"}</span>
        {dirty ? (
          <StatusBadge tone="warn">
            <span className="dirty-dot" /> Unsaved
          </StatusBadge>
        ) : active ? (
          <StatusBadge tone="success">Saved</StatusBadge>
        ) : null}
      </div>
      <div className="editor-shell">
        <div className="file-list">
          {files.length === 0 ? (
            <div className="file-list-empty">
              <EmptyState
                icon={<IconMetadata size={24} />}
                title="No metadata files"
                description="Set a repo path, then use Refresh tree to load YAML under metadata/."
              />
            </div>
          ) : (
            files.map((f) => (
              <button key={f} type="button" className={active === f ? "active" : ""} onClick={() => void openFile(f)}>
                {f}
              </button>
            ))
          )}
        </div>
        <div className="monaco-host" ref={host} />
      </div>
    </div>
  );
}
