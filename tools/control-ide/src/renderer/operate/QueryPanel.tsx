import { useEffect, useMemo, useRef, useState } from "react";
import * as monaco from "monaco-editor";
import type { AppBridge } from "../App";
import { useTheme } from "../ThemeContext";
import { monacoThemeFor } from "../theme";
import { Button, DataTable, EmptyState, PanelHeader, Spinner, ToolSurface } from "../ui";
import { IconQuery } from "../icons/Icons";
import {
  describeCache,
  normalizeDescribeObject,
  normalizeGlobalObjects,
  type GlobalDescribeObject,
} from "./describeCache";
import {
  defaultQueryJson,
  fieldSuggestions,
  flattenRecordRow,
  objectSuggestions,
  opSuggestions,
  rankSuggestions,
  resultColumns,
} from "./queryAutocomplete";
import { isKernelIdentityObject, queryRecords } from "./recordClient";
import type { QueryFilter, SortSpec } from "./types";

const RESULT_CAP = 50;

export function QueryPanel({
  bridge,
  refreshKey = 0,
  onAskAgent,
}: {
  bridge: AppBridge;
  refreshKey?: number;
  /** Route a prompt about the selected row into the primary Operate chat. */
  onAskAgent?: (prompt: string) => void;
}) {
  const theme = useTheme();
  const installId = bridge.session?.activeInstallId ?? bridge.session?.baseUrl ?? "default";
  const connected = Boolean(bridge.session?.token && bridge.session?.baseUrl);

  const [objects, setObjects] = useState<GlobalDescribeObject[]>([]);
  const [objectName, setObjectName] = useState(() => {
    try {
      return sessionStorage.getItem("one.operate.queryObject") || "Account";
    } catch {
      return "Account";
    }
  });
  const [busy, setBusy] = useState<"meta" | "run" | null>(null);
  const [err, setErr] = useState("");
  const [rows, setRows] = useState<Record<string, unknown>[] | null>(null);
  const host = useRef<HTMLDivElement>(null);
  const editor = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);
  const completion = useRef<monaco.IDisposable | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    if (!host.current || editor.current) return;
    editor.current = monaco.editor.create(host.current, {
      value: defaultQueryJson("Account"),
      language: "json",
      theme: monacoThemeFor(theme),
      minimap: { enabled: false },
      automaticLayout: true,
      fontSize: 13,
      scrollBeyondLastLine: false,
    });
    return () => {
      completion.current?.dispose();
      completion.current = null;
      editor.current?.dispose();
      editor.current = null;
    };
    // mount once
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    monaco.editor.setTheme(monacoThemeFor(theme));
  }, [theme]);

  useEffect(() => {
    describeCache.invalidateInstall(installId);
    setRows(null);
    setErr("");
    try {
      const handoff = sessionStorage.getItem("one.operate.queryObject");
      if (handoff) {
        setObjectName(handoff);
        sessionStorage.removeItem("one.operate.queryObject");
      }
    } catch {
      /* ignore */
    }
  }, [installId, refreshKey]);

  useEffect(() => {
    if (!connected) {
      setObjects([]);
      return;
    }
    let cancelled = false;
    void (async () => {
      setBusy("meta");
      setErr("");
      try {
        let list = describeCache.getGlobal(installId);
        if (!list) {
          const raw = await bridge.fetch("/client/v1/describe");
          list = normalizeGlobalObjects(raw);
          describeCache.setGlobal(installId, list);
        }
        if (cancelled) return;
        setObjects(list);
        if (list.length && !list.some((o) => o.apiName === objectName)) {
          setObjectName(list[0].apiName);
        }
      } catch (e) {
        if (!cancelled) setErr(String(e));
      } finally {
        if (!cancelled) setBusy(null);
      }
    })();
    return () => {
      cancelled = true;
    };
    // objectName only used to decide a one-shot default when catalog loads
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bridge, connected, installId, refreshKey]);

  // Keep editor object field in sync when picker changes.
  useEffect(() => {
    const ed = editor.current;
    if (!ed) return;
    try {
      const parsed = JSON.parse(ed.getValue()) as Record<string, unknown>;
      if (parsed.object === objectName) return;
      parsed.object = objectName;
      if (!Array.isArray(parsed.filters)) parsed.filters = [];
      if (!Array.isArray(parsed.sort)) parsed.sort = [];
      if (typeof parsed.limit !== "number") parsed.limit = 25;
      ed.setValue(JSON.stringify(parsed, null, 2));
    } catch {
      ed.setValue(defaultQueryJson(objectName));
    }
  }, [objectName]);

  // Register completion provider against current object describe.
  useEffect(() => {
    completion.current?.dispose();
    completion.current = monaco.languages.registerCompletionItemProvider("json", {
      triggerCharacters: ['"', ":"],
      provideCompletionItems: async (model, position) => {
        if (model !== editor.current?.getModel()) {
          return { suggestions: [] };
        }
        const word = model.getWordUntilPosition(position);
        const range = {
          startLineNumber: position.lineNumber,
          endLineNumber: position.lineNumber,
          startColumn: word.startColumn,
          endColumn: word.endColumn,
        };
        const line = model.getLineContent(position.lineNumber);
        let suggestions = objectSuggestions(objects.map((o) => o.apiName));
        if (/"op"\s*:/.test(line) || line.includes('"op"')) {
          suggestions = opSuggestions();
        } else if (/field|sort/.test(line) || line.includes('"field"')) {
          let desc = describeCache.getObject(installId, objectName);
          if (!desc && connected) {
            try {
              const raw = await bridge.fetch(`/client/v1/describe/${encodeURIComponent(objectName)}`);
              desc = normalizeDescribeObject(raw, objectName);
              describeCache.setObject(installId, objectName, desc);
            } catch {
              desc = { apiName: objectName, fields: [] };
            }
          }
          suggestions = fieldSuggestions(desc?.fields ?? []);
        }
        const ranked = rankSuggestions(suggestions, word.word);
        return {
          suggestions: ranked.map((s) => ({
            label: s.label,
            kind:
              s.kind === "field"
                ? monaco.languages.CompletionItemKind.Field
                : s.kind === "op"
                  ? monaco.languages.CompletionItemKind.Enum
                  : monaco.languages.CompletionItemKind.Value,
            insertText: s.insertText,
            detail: s.detail,
            range,
          })),
        };
      },
    });
    return () => {
      completion.current?.dispose();
      completion.current = null;
    };
  }, [bridge, connected, installId, objectName, objects]);

  const runQuery = async () => {
    if (!connected || !editor.current) return;
    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;
    setBusy("run");
    setErr("");
    try {
      let body: Record<string, unknown>;
      try {
        body = JSON.parse(editor.current.getValue()) as Record<string, unknown>;
      } catch {
        setErr("Query JSON is invalid.");
        setRows(null);
        return;
      }
      if (!body.object) body.object = objectName;
      const limit = typeof body.limit === "number" ? body.limit : 25;
      const cappedLimit = Math.min(Math.max(1, Math.trunc(limit)), RESULT_CAP);
      body.limit = cappedLimit;
      const objectApiName = String(body.object);
      const q = isKernelIdentityObject(objectApiName)
        ? await queryRecords(bridge.fetch, {
            object: objectApiName,
            select: Array.isArray(body.select) ? (body.select as string[]) : undefined,
            filters: Array.isArray(body.filters) ? (body.filters as QueryFilter[]) : undefined,
            sort: Array.isArray(body.sort) ? (body.sort as SortSpec[]) : undefined,
            limit: cappedLimit,
          })
        : ((await bridge.fetch("/client/v1/query", {
            method: "POST",
            body: JSON.stringify(body),
            signal: ac.signal,
          })) as { records?: Record<string, unknown>[] });
      if (ac.signal.aborted) return;
      const records = (q.records ?? []).slice(0, RESULT_CAP).map(flattenRecordRow);
      setRows(records);
    } catch (e) {
      if ((e as Error)?.name === "AbortError") return;
      setErr(String(e));
      setRows(null);
    } finally {
      if (!ac.signal.aborted) setBusy(null);
    }
  };

  useEffect(() => () => abortRef.current?.abort(), []);

  const columns = useMemo(() => {
    const keys = resultColumns(rows ?? []);
    return keys.map((key) => ({
      key,
      label: key,
      mono: key === "id" || key === "Id",
    }));
  }, [rows]);

  return (
    <ToolSurface className="operate-query-panel" testId="operate-query-panel">
      <PanelHeader
        title="Query"
        subtitle="Compose a Client JSON query with metadata autocomplete for the active env."
        actions={
          <Button
            variant="secondary"
            disabled={!connected || busy === "meta"}
            onClick={() => {
              describeCache.invalidateInstall(installId);
              setBusy("meta");
              void bridge
                .fetch("/client/v1/describe")
                .then((raw) => {
                  const list = normalizeGlobalObjects(raw);
                  describeCache.setGlobal(installId, list);
                  setObjects(list);
                })
                .catch((e) => setErr(String(e)))
                .finally(() => setBusy(null));
            }}
          >
            Refresh metadata
          </Button>
        }
      />

      {!connected ? (
        <EmptyState
          icon={<IconQuery size={28} />}
          title="Connect to query"
          description="Connect an environment to load describe metadata and run Client queries."
        />
      ) : (
        <>
          <div className="row operate-query-toolbar">
            <label>
              Object
              <select
                data-testid="query-object-select"
                value={objectName}
                onChange={(e) => setObjectName(e.target.value)}
                disabled={busy === "meta"}
              >
                {(objects.length ? objects : [{ apiName: objectName }]).map((o) => (
                  <option key={o.apiName} value={o.apiName}>
                    {o.label ? `${o.label} (${o.apiName})` : o.apiName}
                  </option>
                ))}
              </select>
            </label>
            <Button variant="primary" busy={busy === "run"} onClick={() => void runQuery()}>
              Run
            </Button>
            <Button
              variant="ghost"
              onClick={() => {
                editor.current?.setValue(defaultQueryJson(objectName));
                setRows(null);
                setErr("");
              }}
            >
              Clear
            </Button>
            <Button
              variant="ghost"
              onClick={() => {
                const text = editor.current?.getValue() ?? "";
                void navigator.clipboard?.writeText(text);
              }}
            >
              Copy JSON
            </Button>
            {busy === "meta" ? <Spinner /> : null}
          </div>
          <div className="monaco-host operate-query-editor" ref={host} data-testid="query-editor" />
          {err ? <p className="err">{err}</p> : null}
          <section className="operate-query-results" aria-label="Query results">
            <h3>Results</h3>
            {rows == null ? (
              <p className="muted">Run a query to list matching records below.</p>
            ) : (
              <>
                <DataTable columns={columns} rows={rows} emptyLabel="No matching records." />
                {rows.length > 0 && onAskAgent ? (
                  <div className="row operate-query-ask-agent">
                    <Button
                      variant="ghost"
                      data-testid="query-ask-agent"
                      onClick={() => {
                        const sample = rows[0];
                        const id = String(sample?.id ?? sample?.Id ?? "");
                        const prompt = `Tell me about this ${objectName}${id ? ` (id: ${id})` : ""} and the ${rows.length} result(s) from my query.`;
                        onAskAgent(prompt);
                      }}
                    >
                      Ask agent about results
                    </Button>
                  </div>
                ) : null}
              </>
            )}
          </section>
        </>
      )}
    </ToolSurface>
  );
}
