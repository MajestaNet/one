import { useCallback, useEffect, useMemo, useState } from "react";
import type { AppBridge } from "../App";
import { Button, EmptyState, PanelHeader, Spinner, StatusBadge, ToolSurface } from "../ui";
import { IconRecords } from "../icons/Icons";
import { useAgentExcerptBridge } from "../workspace/AgentExcerptContext";
import { rowsToContextExcerpt } from "../workspace/contextExcerpt";
import {
  describeCache,
  normalizeDescribeObject,
  normalizeGlobalObjects,
  type GlobalDescribeObject,
} from "../operate/describeCache";
import type { DescribeField, DescribeObject, QueryFilter, SavedView } from "../operate/types";
import { recordId } from "../operate/types";
import {
  deleteSavedView,
  loadSavedViews,
  upsertSavedView,
  viewsForObject,
} from "../operate/views";
import { KeyValueList } from "../ui/KeyValueList";
import { RecordForm, requiredMissing } from "../operate/RecordForm";
import {
  createRecord,
  getRecord,
  listIdentityField,
  queryRecords,
  recordWritePayload,
  updateRecord,
  type QueryRecordsInput,
} from "../operate/recordClient";
import { pinRecordToHomeGraph } from "./graph/pinRecord";
import { buildBulkPatchRequests, summarizeCompositeResponse } from "../operate/bulkComposite";

type Row = Record<string, unknown>;

const DEFAULT_COLUMN_COUNT = 5;

export function objectHomeRailId(apiName: string): string {
  return `object:${apiName}`;
}

export function parseObjectHomeRailId(id: string): string | null {
  if (!id.startsWith("object:")) return null;
  return id.slice("object:".length).trim() || null;
}

/** Prefer Name-like fields first; default to 5 data columns (no forced Id column). */
export function defaultFieldColumns(fields: DescribeField[], max = DEFAULT_COLUMN_COUNT): { key: string; label: string }[] {
  const preferred = [
    "Name",
    "LastName",
    "FirstName",
    "DisplayName",
    "Subject",
    "Title",
    "Email",
    "StageName",
    "Status",
    "Industry",
    "Phone",
    "MobilePhone",
  ];
  const byName = new Map(fields.map((f) => [f.apiName, f]));
  const cols: { key: string; label: string }[] = [];
  for (const key of preferred) {
    const f = byName.get(key);
    if (!f) continue;
    cols.push({ key: f.apiName, label: f.label || f.apiName });
    if (cols.length >= max) return cols;
  }
  for (const f of fields) {
    if (cols.some((c) => c.key === f.apiName)) continue;
    if (f.apiName === "id" || f.apiName === "Id") continue;
    if (f.fieldType === "textarea" || f.fieldType === "longtextarea") continue;
    cols.push({ key: f.apiName, label: f.label || f.apiName });
    if (cols.length >= max) break;
  }
  return cols;
}

function flattenRecord(rec: Row): Row {
  const out: Row = {};
  for (const [k, v] of Object.entries(rec)) {
    if (v != null && typeof v === "object" && !Array.isArray(v)) {
      if (k === "data") {
        for (const [dk, dv] of Object.entries(v as Row)) {
          out[dk] = dv;
        }
      } else {
        out[k] = JSON.stringify(v);
      }
    } else {
      out[k] = v;
    }
  }
  if (out.id == null && out.Id != null) out.id = out.Id;
  return out;
}

function formatFilterChip(f: QueryFilter): string {
  const op =
    f.op === "like" ? "contains" : f.op === "eq" ? "=" : f.op === "ne" ? "≠" : f.op;
  return `${f.field} ${op} ${f.value == null ? "" : String(f.value)}`.trim();
}

/**
 * Run List View: object toggle → Client query list → get-by-id record.
 * Always AuthZ-honest (FLS + sharing via Client API). ADR-021 / BP-050 P1.
 */
export function RunObjectHomePanel({
  bridge,
  refreshKey = 0,
  initialObjectApiName,
  initialSelectedId,
  selectedIdEpoch = 0,
  /** Agent- or handoff-supplied filters; applied when the reference changes. */
  initialFilters,
  filtersEpoch = 0,
  /** After a successful pin, e.g. open/refresh My graph. */
  onPinnedToGraph,
  variant = "panel",
  lockObject = false,
  collectionNodeId,
}: {
  bridge: AppBridge;
  refreshKey?: number;
  /** Dynamic Run-rail object home to open immediately. */
  initialObjectApiName?: string | null;
  /** Record id to select after the list loads (Operate search / handoff). */
  initialSelectedId?: string | null;
  /** Bump when a new search hit should re-select even if the id is unchanged. */
  selectedIdEpoch?: number;
  initialFilters?: QueryFilter[] | null;
  /** Bump when an agent updates filters so the panel re-applies them. */
  filtersEpoch?: number;
  onPinnedToGraph?: (nodeId?: string) => void;
  /** Graph focus embeds the list without List View chrome. */
  variant?: "panel" | "embedded";
  /** Keep the object picker locked to initialObjectApiName. */
  lockObject?: boolean;
  /** When set, Pin writes derivedFrom to this collection node. */
  collectionNodeId?: string;
}) {
  const connected = Boolean(bridge.session?.baseUrl && bridge.session?.token);
  const installId = bridge.session?.activeInstallId ?? "default";
  const excerptBridge = useAgentExcerptBridge();
  const [selectedRowIds, setSelectedRowIds] = useState<Set<string>>(new Set());

  const [objects, setObjects] = useState<GlobalDescribeObject[]>([]);
  const [objectName, setObjectName] = useState(initialObjectApiName ?? "");
  const [desc, setDesc] = useState<DescribeObject | null>(null);
  const [rows, setRows] = useState<Row[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [record, setRecord] = useState<Row | null>(null);
  const [formMode, setFormMode] = useState<"create" | "edit" | null>(null);
  const [formValues, setFormValues] = useState<Row>({});
  const [busy, setBusy] = useState<string | null>(null);
  const [err, setErr] = useState("");
  const [pinMsg, setPinMsg] = useState("");
  const [ownerIdDraft, setOwnerIdDraft] = useState("");
  const [statusDraft, setStatusDraft] = useState("");
  const [bulkMsg, setBulkMsg] = useState("");

  const [columnKeys, setColumnKeys] = useState<string[]>([]);
  const [columnPickerOpen, setColumnPickerOpen] = useState(false);
  const [filterMenuOpen, setFilterMenuOpen] = useState(false);
  const [activeFilters, setActiveFilters] = useState<QueryFilter[]>([]);
  const [filterField, setFilterField] = useState("Name");
  const [filterOp, setFilterOp] = useState<QueryFilter["op"]>("like");
  const [filterValue, setFilterValue] = useState("");
  const [savedViews, setSavedViews] = useState<SavedView[]>(() => loadSavedViews());

  const availableFields = useMemo(() => desc?.fields ?? [], [desc]);
  const filterableFields = useMemo(
    () => availableFields.filter((f) => f.filterable !== false),
    [availableFields],
  );
  const objectViews = useMemo(
    () => viewsForObject(savedViews, objectName),
    [savedViews, objectName],
  );

  const statusField = useMemo(
    () =>
      availableFields.find(
        (f) =>
          (f.apiName === "Status" || f.apiName === "StageName") &&
          Array.isArray(f.picklistValues) &&
          f.picklistValues.length > 0,
      ) ?? null,
    [availableFields],
  );

  const columns = useMemo(() => {
    const byName = new Map(availableFields.map((f) => [f.apiName, f]));
    return columnKeys.map((key) => ({
      key,
      label: key === "id" || key === "Id" ? "Id" : byName.get(key)?.label || key,
    }));
  }, [availableFields, columnKeys]);

  const rowIds = useMemo(() => rows.map((r) => recordId(r)).filter(Boolean), [rows]);
  const allSelected = rowIds.length > 0 && rowIds.every((id) => selectedRowIds.has(id));

  const toggleRowSelection = useCallback((id: string) => {
    setSelectedRowIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const toggleSelectAll = useCallback(() => {
    setSelectedRowIds((prev) => {
      if (rowIds.length === 0) return prev;
      if (rowIds.every((id) => prev.has(id))) return new Set();
      return new Set(rowIds);
    });
  }, [rowIds]);

  const toggleColumn = useCallback((key: string) => {
    setColumnKeys((prev) => {
      if (prev.includes(key)) {
        if (prev.length <= 1) return prev;
        return prev.filter((k) => k !== key);
      }
      return [...prev, key];
    });
  }, []);

  const addSelectedToChat = useCallback(() => {
    if (!excerptBridge || selectedRowIds.size === 0) return;
    const selected = rows.filter((row) => selectedRowIds.has(recordId(row)));
    excerptBridge.addExcerptToOpenChat(
      rowsToContextExcerpt({
        rows: selected.map(flattenRecord),
        columns,
        objectApiName: objectName,
        source: "tool_rows",
      }),
    );
    setSelectedRowIds(new Set());
  }, [columns, excerptBridge, objectName, rows, selectedRowIds]);

  const loadCatalog = useCallback(async () => {
    if (!connected) return;
    setBusy("catalog");
    setErr("");
    try {
      let list = describeCache.getGlobal(installId);
      if (!list) {
        const raw = await bridge.fetch("/client/v1/describe");
        list = normalizeGlobalObjects(raw);
        describeCache.setGlobal(installId, list);
      }
      setObjects(list);
      if (list.length && !list.some((o) => o.apiName === objectName)) {
        setObjectName(list[0].apiName);
      }
    } catch (e) {
      setErr(String(e));
      setObjects([]);
    } finally {
      setBusy(null);
    }
  }, [bridge, connected, installId, objectName]);

  const loadList = useCallback(
    async (apiName: string, filters: QueryFilter[] = activeFilters) => {
      if (!connected || !apiName) return;
      setBusy("list");
      setErr("");
      setSelectedId(null);
      setRecord(null);
      setFormMode(null);
      setFormValues({});
      setSelectedRowIds(new Set());
      try {
        let objectDesc = describeCache.getObject(installId, apiName);
        if (!objectDesc) {
          const raw = await bridge.fetch(`/client/v1/describe/${encodeURIComponent(apiName)}`);
          objectDesc = normalizeDescribeObject(raw, apiName);
          describeCache.setObject(installId, apiName, objectDesc);
        }
        setDesc(objectDesc);
        const defaults = defaultFieldColumns(objectDesc.fields ?? []);
        setColumnKeys((prev) => {
          // Reset columns when switching objects or first load.
          if (prev.length === 0) return defaults.map((c) => c.key);
          const valid = new Set((objectDesc.fields ?? []).map((f) => f.apiName));
          const kept = prev.filter((k) => valid.has(k) || k === "id" || k === "Id");
          return kept.length ? kept : defaults.map((c) => c.key);
        });
        const validFields = new Set((objectDesc.fields ?? []).map((f) => f.apiName));
        const usableFilters = filters.filter((f) => validFields.has(f.field));
        if (usableFilters.length !== filters.length) {
          setActiveFilters(usableFilters);
        }
        const body: QueryRecordsInput = { object: apiName, limit: 50 };
        if (usableFilters.length) body.filters = usableFilters;
        const q = await queryRecords(bridge.fetch, body);
        setRows((q.records ?? []).map(flattenRecord));
        const identity = listIdentityField(apiName);
        const firstFilterable =
          (objectDesc.fields ?? []).find((f) => f.apiName === identity)?.apiName ||
          (objectDesc.fields ?? []).find((f) => f.filterable !== false)?.apiName ||
          identity;
        setFilterField((cur) =>
          (objectDesc.fields ?? []).some((f) => f.apiName === cur) ? cur : firstFilterable,
        );
      } catch (e) {
        setErr(String(e));
        setRows([]);
        setDesc(null);
      } finally {
        setBusy(null);
      }
    },
    [activeFilters, bridge, connected, installId],
  );

  const loadRecord = useCallback(
    async (apiName: string, id: string) => {
      if (!connected || !apiName || !id) return;
      setBusy("record");
      setErr("");
      try {
        const raw = await getRecord(bridge.fetch, apiName, id);
        setRecord(flattenRecord(raw));
        setFormValues(flattenRecord(raw));
        setFormMode(null);
        setSelectedId(id);
      } catch (e) {
        setErr(String(e));
        setRecord(null);
      } finally {
        setBusy(null);
      }
    },
    [bridge, connected],
  );

  useEffect(() => {
    void loadCatalog();
  }, [loadCatalog, refreshKey]);

  useEffect(() => {
    if (
      initialObjectApiName &&
      initialObjectApiName !== objectName &&
      (objects.length === 0 || objects.some((object) => object.apiName === initialObjectApiName))
    ) {
      setObjectName(initialObjectApiName);
      setColumnKeys([]);
    }
  }, [initialObjectApiName, objectName, objects]);

  useEffect(() => {
    if (initialFilters != null) {
      setActiveFilters(initialFilters);
    }
  }, [filtersEpoch, initialFilters]);

  useEffect(() => {
    if (!objectName) return;
    let cancelled = false;
    void (async () => {
      await loadList(objectName, activeFilters);
      if (!cancelled && initialSelectedId) {
        await loadRecord(objectName, initialSelectedId);
      }
    })();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- reload on object / filter / refresh / search hit
  }, [objectName, activeFilters, refreshKey, initialSelectedId, selectedIdEpoch]);

  const applyFilter = () => {
    if (!filterField.trim()) return;
    const next: QueryFilter = {
      field: filterField,
      op: filterOp,
      value: filterOp === "is_null" || filterOp === "is_not_null" ? undefined : filterValue,
    };
    setActiveFilters((prev) => {
      const without = prev.filter((f) => !(f.field === next.field && f.op === next.op));
      return [...without, next];
    });
    setFilterMenuOpen(false);
  };

  const clearFilters = () => {
    setActiveFilters([]);
    setFilterValue("");
    setFilterMenuOpen(false);
  };

  const removeFilter = (idx: number) => {
    setActiveFilters((prev) => prev.filter((_, i) => i !== idx));
  };

  const saveCurrentView = () => {
    const name = window.prompt("Saved list view name?");
    if (!name?.trim() || !objectName) return;
    const view: SavedView = {
      id: `view-${Date.now()}`,
      name: name.trim(),
      objectApiName: objectName,
      filters: activeFilters,
      sort: [],
      limit: 50,
    };
    setSavedViews(upsertSavedView(savedViews, view));
  };

  const startCreate = () => {
    const blank: Row = {};
    for (const field of desc?.fields ?? []) {
      if (["Name", "LastName", "Subject"].includes(field.apiName)) blank[field.apiName] = "";
    }
    setSelectedId(null);
    setRecord(null);
    setFormValues(blank);
    setFormMode("create");
  };

  const saveRecord = async () => {
    if (!objectName || !desc) return;
    const missing = requiredMissing(desc.fields ?? [], formValues);
    if (missing.length) {
      setErr(`Required: ${missing.join(", ")}`);
      return;
    }
    setBusy("save");
    setErr("");
    try {
      const payload = recordWritePayload(desc.fields ?? [], formValues);
      let id = selectedId;
      if (formMode === "create") {
        const created = await createRecord(bridge.fetch, objectName, payload);
        id = recordId(created);
      } else if (formMode === "edit" && selectedId) {
        await updateRecord(bridge.fetch, objectName, selectedId, payload);
      }
      await loadList(objectName, activeFilters);
      if (id) await loadRecord(objectName, id);
      setFormMode(null);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(null);
    }
  };

  const runBulk = async (body: Record<string, unknown>) => {
    if (!objectName || selectedRowIds.size === 0) return;
    const ordered = rows.map((r) => recordId(r)).filter((id): id is string => Boolean(id) && selectedRowIds.has(id));
    const { requests, deferred } = buildBulkPatchRequests(objectName, ordered, body);
    if (!requests.length) return;
    setBusy("bulk");
    setErr("");
    setBulkMsg("");
    try {
      const raw = await bridge.fetch("/client/v1/composite", {
        method: "POST",
        body: JSON.stringify({ compositeRequest: requests }),
      });
      const summary = summarizeCompositeResponse(raw);
      setBulkMsg(deferred > 0 ? `${summary.message}. ${deferred} remaining — update next 25.` : summary.message);
      await loadList(objectName, activeFilters);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(null);
    }
  };

  const pinToGraph = async () => {
    if (!objectName || !selectedId) return;
    setBusy("pin");
    setErr("");
    setPinMsg("");
    try {
      const result = await pinRecordToHomeGraph(bridge.fetch, {
        objectApiName: objectName,
        recordId: selectedId,
        collectionId: collectionNodeId,
      });
      if (!result.ok) {
        setErr("error" in result ? String(result.error) : "Failed to pin record");
        return;
      }
      setPinMsg("Pinned to My graph");
      onPinnedToGraph?.(typeof result.nodeId === "string" ? result.nodeId : undefined);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(null);
    }
  };

  if (!connected) {
    return (
      <ToolSurface testId="run-object-home-panel">
        <PanelHeader title="List View" subtitle="Browse records for any object." />
        <EmptyState
          icon={<IconRecords size={28} />}
          title="Connect first"
          description="Open Settings → Environments and authenticate with Client scope to browse enabled objects."
        />
      </ToolSurface>
    );
  }

  const selectedLabel = objects.find((o) => o.apiName === objectName)?.label || objectName;
  const embedded = variant === "embedded";

  return (
    <ToolSurface className={`run-object-home-panel${embedded ? " is-embedded" : ""}`} testId="run-object-home-panel">
      {embedded ? null : (
      <PanelHeader
        title="List View"
        subtitle="Client-backed list → record (FLS + sharing enforced on the install)"
        actions={
          <>
            <Button variant="primary" disabled={!objectName || !desc} onClick={startCreate}>
              New {selectedLabel || "record"}
            </Button>
            <Button
              variant="secondary"
              disabled={selectedRowIds.size === 0 || !excerptBridge}
              onClick={addSelectedToChat}
              data-testid="run-object-home-add-to-chat"
            >
              Add {selectedRowIds.size > 0 ? selectedRowIds.size : ""} to chat
            </Button>
            <Button
              variant="secondary"
              busy={busy === "catalog" || busy === "list"}
              onClick={() => {
                describeCache.invalidateInstall(installId);
                void loadCatalog();
                if (objectName) void loadList(objectName, activeFilters);
              }}
            >
              Refresh
            </Button>
          </>
        }
      />
      )}

      <div className="run-object-home-toolbar">
        {lockObject ? (
          <p className="muted" data-testid="run-object-home-locked-object">{selectedLabel || objectName}</p>
        ) : (
        <label>
          Object
          <select
            data-testid="run-object-home-picker"
            value={objectName}
            onChange={(e) => {
              setColumnKeys([]);
              setActiveFilters([]);
              setObjectName(e.target.value);
            }}
          >
            {objects.length === 0 ? <option value="">No objects</option> : null}
            {objects.map((o) => (
              <option key={o.apiName} value={o.apiName}>
                {o.label || o.apiName}
              </option>
            ))}
          </select>
        </label>
        )}
        {embedded ? (
          <div className="run-object-home-toolbar-actions">
            <Button variant="primary" disabled={!objectName || !desc} onClick={startCreate}>
              New {selectedLabel || "record"}
            </Button>
            <Button
              variant="secondary"
              busy={busy === "catalog" || busy === "list"}
              onClick={() => {
                describeCache.invalidateInstall(installId);
                void loadCatalog();
                if (objectName) void loadList(objectName, activeFilters);
              }}
            >
              Refresh
            </Button>
          </div>
        ) : null}

        <div className="run-object-home-toolbar-actions">
          <div className="run-list-dropdown" data-testid="run-list-columns">
            <Button
              variant="secondary"
              aria-expanded={columnPickerOpen}
              onClick={() => {
                setColumnPickerOpen((v) => !v);
                setFilterMenuOpen(false);
              }}
              data-testid="run-list-columns-toggle"
            >
              Columns ({columns.length})
            </Button>
            {columnPickerOpen ? (
              <div className="run-list-dropdown-menu" role="menu" data-testid="run-list-columns-menu">
                <p className="muted">Select up to any fields to show (default {DEFAULT_COLUMN_COUNT}).</p>
                <div className="ide-cap-list">
                  {availableFields
                    .filter((f) => f.fieldType !== "textarea" && f.fieldType !== "longtextarea")
                    .map((f) => (
                      <label key={f.apiName} className="ide-cap-item">
                        <input
                          type="checkbox"
                          checked={columnKeys.includes(f.apiName)}
                          onChange={() => toggleColumn(f.apiName)}
                          data-testid={`run-list-col-${f.apiName}`}
                        />
                        <span>{f.label || f.apiName}</span>
                      </label>
                    ))}
                </div>
              </div>
            ) : null}
          </div>

          <div className="run-list-dropdown" data-testid="run-list-filters">
            <Button
              variant="secondary"
              aria-expanded={filterMenuOpen}
              onClick={() => {
                setFilterMenuOpen((v) => !v);
                setColumnPickerOpen(false);
              }}
              data-testid="run-list-filters-toggle"
            >
              Filters{activeFilters.length ? ` (${activeFilters.length})` : ""}
            </Button>
            {filterMenuOpen ? (
              <div className="run-list-dropdown-menu" role="dialog" data-testid="run-list-filters-menu">
                <div className="crm-query-bar run-list-filter-bar">
                  <label className="composer-label">
                    Field
                    <select
                      value={filterField}
                      onChange={(e) => setFilterField(e.target.value)}
                      aria-label="Filter field"
                      data-testid="run-list-filter-field"
                    >
                      {(filterableFields.length ? filterableFields : availableFields).map((f) => (
                        <option key={f.apiName} value={f.apiName}>
                          {f.label ?? f.apiName}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="composer-label">
                    Op
                    <select
                      value={filterOp}
                      onChange={(e) => setFilterOp(e.target.value as QueryFilter["op"])}
                      aria-label="Filter operator"
                      data-testid="run-list-filter-op"
                    >
                      <option value="like">contains</option>
                      <option value="eq">equals</option>
                      <option value="ne">not equals</option>
                      <option value="gt">gt</option>
                      <option value="lt">lt</option>
                    </select>
                  </label>
                  <label className="composer-label">
                    Value
                    <input
                      value={filterValue}
                      onChange={(e) => setFilterValue(e.target.value)}
                      placeholder="Filter value…"
                      aria-label="Filter value"
                      data-testid="run-list-filter-value"
                      onKeyDown={(e) => {
                        if (e.key === "Enter") applyFilter();
                      }}
                    />
                  </label>
                  <Button variant="primary" onClick={applyFilter} data-testid="run-list-filter-apply">
                    Apply
                  </Button>
                  <Button variant="ghost" onClick={clearFilters} data-testid="run-list-filter-clear">
                    Clear
                  </Button>
                  <Button variant="ghost" onClick={saveCurrentView} data-testid="run-list-filter-save">
                    Save view
                  </Button>
                </div>
                {objectViews.length > 0 ? (
                  <div className="crm-saved-views" data-testid="run-list-saved-views">
                    {objectViews.map((v) => (
                      <span key={v.id} className="crm-saved-view-chip">
                        <button
                          type="button"
                          className="secondary"
                          onClick={() => {
                            setActiveFilters(v.filters);
                            setFilterMenuOpen(false);
                          }}
                        >
                          {v.name}
                        </button>
                        <button
                          type="button"
                          className="ghost"
                          aria-label={`Delete view ${v.name}`}
                          onClick={() => setSavedViews(deleteSavedView(savedViews, v.id))}
                        >
                          ×
                        </button>
                      </span>
                    ))}
                  </div>
                ) : null}
                <p className="muted">Agents can also set these filters via list-view handoff.</p>
              </div>
            ) : null}
          </div>
        </div>

        {busy ? (
          <span className="muted">
            <Spinner /> {busy}
          </span>
        ) : (
          <StatusBadge tone="neutral">{rows.length} rows</StatusBadge>
        )}
      </div>

      {activeFilters.length > 0 ? (
        <div className="run-list-active-filters" data-testid="run-list-active-filters">
          {activeFilters.map((f, i) => (
            <span key={`${f.field}-${f.op}-${i}`} className="crm-saved-view-chip">
              <StatusBadge tone="accent">{formatFilterChip(f)}</StatusBadge>
              <button
                type="button"
                className="ghost"
                aria-label={`Remove filter ${f.field}`}
                onClick={() => removeFilter(i)}
              >
                ×
              </button>
            </span>
          ))}
        </div>
      ) : null}

      {err ? (
        <p className="error" data-testid="run-object-home-error">
          {err}
        </p>
      ) : null}

      {selectedRowIds.size > 0 || bulkMsg ? (
        <div className="run-object-home-bulk" data-testid="run-object-home-bulk">
          {selectedRowIds.size > 0 ? (
            <>
              <span data-testid="run-object-home-bulk-count">{selectedRowIds.size} selected</span>
              <label>
                Owner id
                <input
                  data-testid="run-object-home-bulk-owner"
                  value={ownerIdDraft}
                  onChange={(e) => setOwnerIdDraft(e.target.value)}
                  placeholder="User id"
                />
              </label>
              <Button
                variant="secondary"
                busy={busy === "bulk"}
                disabled={!ownerIdDraft.trim()}
                data-testid="run-object-home-bulk-assign"
                onClick={() => void runBulk({ OwnerId: ownerIdDraft.trim() })}
              >
                Assign owner
              </Button>
              {statusField ? (
                <>
                  <label>
                    {statusField.label || statusField.apiName}
                    <select
                      data-testid="run-object-home-bulk-status"
                      value={statusDraft}
                      onChange={(e) => setStatusDraft(e.target.value)}
                    >
                      <option value="">Select…</option>
                      {(statusField.picklistValues ?? []).map((v) => (
                        <option key={v} value={v}>
                          {v}
                        </option>
                      ))}
                    </select>
                  </label>
                  <Button
                    variant="secondary"
                    busy={busy === "bulk"}
                    disabled={!statusDraft}
                    data-testid="run-object-home-bulk-status-apply"
                    onClick={() => void runBulk({ [statusField.apiName]: statusDraft })}
                  >
                    Change {statusField.apiName === "StageName" ? "stage" : "status"}
                  </Button>
                </>
              ) : null}
              <Button
                variant="ghost"
                data-testid="run-object-home-bulk-clear"
                onClick={() => {
                  setSelectedRowIds(new Set());
                  setBulkMsg("");
                }}
              >
                Clear selection
              </Button>
              {selectedRowIds.size > 25 ? <span className="muted">Update next 25</span> : null}
            </>
          ) : null}
          {bulkMsg ? (
            <span data-testid="run-object-home-bulk-result">
              <StatusBadge tone="accent">{bulkMsg}</StatusBadge>
            </span>
          ) : null}
        </div>
      ) : null}

      <div className="run-object-home-split">
        <section className="run-object-home-list" aria-label={`${selectedLabel} list`}>
          <h3 className="run-object-home-heading">{selectedLabel || "List"}</h3>
          {rows.length === 0 ? (
            <div data-testid="run-object-home-empty">
              <EmptyState
                icon={<IconRecords size={24} />}
                title={
                  activeFilters.length > 0
                    ? `No matching ${selectedLabel || "object"} records`
                    : `No ${selectedLabel || "object"} records yet`
                }
                description={
                  activeFilters.length > 0
                    ? "No available records match the active filters. Clear them or try different values."
                    : bridge.session?.isAdmin
                      ? "This install has no records for this object. Product setup installs object definitions without sample customer data."
                      : "No records are available to your account. Create one, or ask an administrator to share existing records."
                }
                action={
                  activeFilters.length > 0 ? (
                    <Button variant="secondary" onClick={clearFilters}>
                      Clear filters
                    </Button>
                  ) : (
                    <Button variant="primary" disabled={!objectName || !desc} onClick={startCreate}>
                      Create {selectedLabel || "record"}
                    </Button>
                  )
                }
              />
            </div>
          ) : (
            <div className="run-object-home-table-wrap">
              <table className="data-table" data-testid="run-object-home-table">
                <thead>
                  <tr>
                    <th className="run-tool-table-select-col">
                      <input
                        type="checkbox"
                        checked={allSelected}
                        onChange={toggleSelectAll}
                        aria-label="Select all rows"
                        data-testid="run-list-select-all"
                      />
                    </th>
                    {columns.map((c) => (
                      <th key={c.key}>{c.label}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {rows.map((row) => {
                    const id = recordId(row);
                    const flat = flattenRecord(row);
                    const isRowSelected = id ? selectedRowIds.has(id) : false;
                    return (
                      <tr
                        key={id || JSON.stringify(row)}
                        className={selectedId === id ? "is-selected" : undefined}
                        data-testid={`run-object-home-row-${id}`}
                        onClick={(e) => {
                          if (e.shiftKey && id) {
                            toggleRowSelection(id);
                            return;
                          }
                          if (id) void loadRecord(objectName, id);
                        }}
                      >
                        <td className="run-tool-table-select-col">
                          <input
                            type="checkbox"
                            checked={isRowSelected}
                            onChange={() => id && toggleRowSelection(id)}
                            onClick={(e) => e.stopPropagation()}
                            aria-label="Select row for chat"
                          />
                        </td>
                        {columns.map((c) => (
                          <td key={c.key}>
                            {c.key === "id" || c.key === "Id"
                              ? id
                              : flat[c.key] == null
                                ? "—"
                                : String(flat[c.key])}
                          </td>
                        ))}
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section className="run-object-home-detail" aria-label="Record detail">
          <h3 className="run-object-home-heading">Record</h3>
          {formMode ? (
            <div data-testid="run-object-home-form">
              <p className="muted mono">{formMode === "create" ? `New ${selectedLabel}` : `${objectName} · ${selectedId}`}</p>
              <RecordForm
                fields={desc?.fields ?? []}
                values={formValues}
                onChange={setFormValues}
                mode={formMode}
              />
              <div className="row">
                <Button variant="primary" busy={busy === "save"} onClick={() => void saveRecord()}>
                  Save
                </Button>
                <Button
                  variant="ghost"
                  onClick={() => {
                    setFormMode(null);
                    setFormValues(record ?? {});
                  }}
                >
                  Cancel
                </Button>
              </div>
            </div>
          ) : !selectedId ? (
            <p className="muted">Select a row to open the record page.</p>
          ) : !record ? (
            <p className="muted">
              <Spinner /> Loading record…
            </p>
          ) : (
            <div data-testid="run-object-home-record">
              <div className="run-object-home-record-heading">
                <p className="muted mono">{objectName} · {selectedId}</p>
                <div className="row">
                  <Button
                    variant="secondary"
                    busy={busy === "pin"}
                    data-testid="run-object-home-pin"
                    onClick={() => void pinToGraph()}
                  >
                    Pin to graph
                  </Button>
                  <Button variant="secondary" onClick={() => setFormMode("edit")}>Edit</Button>
                </div>
              </div>
              {pinMsg ? (
                <p className="muted" data-testid="run-object-home-pin-status" role="status">
                  {pinMsg}
                </p>
              ) : null}
              <KeyValueList
                items={Object.entries(record)
                  .filter(([k]) => k !== "data")
                  .map(([k, v]) => ({
                    label: desc?.fields?.find((f) => f.apiName === k)?.label || k,
                    value: v == null ? "—" : typeof v === "object" ? JSON.stringify(v) : String(v),
                  }))}
              />
            </div>
          )}
        </section>
      </div>
    </ToolSurface>
  );
}
