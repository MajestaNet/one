import { useCallback, useEffect, useMemo, useState } from "react";
import type { AppBridge } from "../App";
import { Button, EmptyState, PanelHeader, StatusBadge, Spinner } from "../ui";
import { IconRecords } from "../icons/Icons";
import { listIdentityField, queryRecords, recordWritePayload, createRecord, updateRecord } from "./recordClient";
import { ActivityFeed } from "./ActivityFeed";
import { RecordForm, requiredMissing } from "../operate/RecordForm";
import { RelatedLists } from "../operate/RelatedLists";
import { WhatToDo } from "../operate/WhatToDo";
import {
  fetchEnabledPackages,
  relatedListsFor,
  tabsForPackages,
} from "../operate/packages";
import {
  deleteSavedView,
  loadSavedViews,
  SAVED_VIEWS_UPGRADE_NOTE,
  upsertSavedView,
  viewsForObject,
} from "../operate/views";
import type {
  BoardHandoff,
  DescribeField,
  DescribeObject,
  ObjectTab,
  QueryFilter,
  SavedView,
  SortSpec,
} from "../operate/types";
import { displayName, recordId } from "./types";
import { BOARD_HANDOFF_MIME } from "../operate/handoff";

type SeedRow = Record<string, unknown>;

const SEED: Record<string, SeedRow[]> = {
  Account: [
    { id: "a1", Name: "Acme Manufacturing", Type: "Customer", Industry: "Manufacturing" },
    { id: "a2", Name: "Northwind Logistics", Type: "Prospect", Industry: "Logistics" },
    { id: "a3", Name: "Brightline Health", Type: "Customer", Industry: "Healthcare" },
  ],
  Contact: [
    { id: "c1", FirstName: "Priya", LastName: "Shah", Email: "priya@acme.example" },
    { id: "c2", FirstName: "Marcus", LastName: "Webb", Email: "marcus@northwind.example" },
  ],
  Opportunity: [
    { id: "o1", Name: "Acme — Renewal", StageName: "Negotiation", Amount: 48000 },
    { id: "o2", Name: "Northwind — Pilot", StageName: "Discovery", Amount: 12000 },
  ],
};

const SEED_FIELDS: Record<string, DescribeField[]> = {
  Account: [
    { apiName: "Name", label: "Name", fieldType: "text", required: true, filterable: true, sortable: true },
    {
      apiName: "Type",
      label: "Type",
      fieldType: "picklist",
      picklistValues: ["Prospect", "Customer", "Partner"],
      filterable: true,
      sortable: true,
    },
    { apiName: "Industry", label: "Industry", fieldType: "text", filterable: true, sortable: true },
    { apiName: "Website", label: "Website", fieldType: "url", filterable: true },
    { apiName: "Phone", label: "Phone", fieldType: "phone", filterable: true },
  ],
  Contact: [
    { apiName: "FirstName", label: "First Name", fieldType: "text", filterable: true, sortable: true },
    { apiName: "LastName", label: "Last Name", fieldType: "text", required: true, filterable: true, sortable: true },
    { apiName: "Email", label: "Email", fieldType: "email", filterable: true, sortable: true },
    { apiName: "AccountId", label: "Account", fieldType: "lookup", filterable: true },
  ],
  Opportunity: [
    { apiName: "Name", label: "Name", fieldType: "text", required: true, filterable: true, sortable: true },
    {
      apiName: "StageName",
      label: "Stage",
      fieldType: "picklist",
      picklistValues: ["Discovery", "Negotiation", "Closed Won", "Closed Lost"],
      filterable: true,
      sortable: true,
    },
    { apiName: "Amount", label: "Amount", fieldType: "currency", filterable: true, sortable: true },
    { apiName: "AccountId", label: "Account", fieldType: "lookup", filterable: true },
  ],
};

function filterSeed(rows: SeedRow[], filters: QueryFilter[], sort: SortSpec[]): SeedRow[] {
  let out = [...rows];
  for (const f of filters) {
    out = out.filter((r) => {
      const v = r[f.field];
      if (f.op === "like") return String(v ?? "").toLowerCase().includes(String(f.value ?? "").toLowerCase());
      if (f.op === "eq") return String(v ?? "") === String(f.value ?? "");
      if (f.op === "ne") return String(v ?? "") !== String(f.value ?? "");
      return true;
    });
  }
  if (sort[0]) {
    const { field, direction } = sort[0];
    out.sort((a, b) => {
      const av = String(a[field] ?? "");
      const bv = String(b[field] ?? "");
      return direction === "desc" ? bv.localeCompare(av) : av.localeCompare(bv);
    });
  }
  return out;
}

/**
 * Operate CRM board — metadata-driven list/form/related/activity feed + BoardHandoff (BP-018 / BP-024A).
 */
export function CrmPanel({
  onClose,
  bridge,
  handoff,
  onHandoffConsumed,
  onStagedMutations,
  onAskAgent: _onAskAgent,
}: {
  onClose?: () => void;
  bridge?: AppBridge;
  handoff?: BoardHandoff | null;
  onHandoffConsumed?: () => void;
  onStagedMutations?: (count: number) => void;
  /** Route a prompt into the primary Operate chat. */
  onAskAgent?: (prompt: string) => void;
}) {
  const connected = Boolean(bridge?.session?.baseUrl && bridge?.session?.token);
  const [enabledPkgs, setEnabledPkgs] = useState<Set<string> | null>(null);
  const tabs = useMemo(() => tabsForPackages(enabledPkgs, connected), [enabledPkgs, connected]);
  const [tabId, setTabId] = useState(tabs[0]?.id ?? "Account");
  const activeTab: ObjectTab = tabs.find((t) => t.id === tabId) ?? tabs[0] ?? OFFLINE_FALLBACK;
  const objectName = activeTab.objectApiName;

  const [fields, setFields] = useState<DescribeField[]>([]);
  const [records, setRecords] = useState<SeedRow[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [formValues, setFormValues] = useState<Record<string, unknown>>({});
  const [creating, setCreating] = useState(false);
  const [live, setLive] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [filterField, setFilterField] = useState("Name");
  const [filterOp, setFilterOp] = useState<QueryFilter["op"]>("like");
  const [filterValue, setFilterValue] = useState("");
  const [sortField, setSortField] = useState("Name");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");
  const [activeFilters, setActiveFilters] = useState<QueryFilter[]>([]);
  const [activeSort, setActiveSort] = useState<SortSpec[]>([{ field: "Name", direction: "asc" }]);
  const [highlightIds, setHighlightIds] = useState<string[]>([]);
  const [intentRunId, setIntentRunId] = useState<string | null>(null);
  const [whatToDoOpen, setWhatToDoOpen] = useState(false);
  const [whatToDo, setWhatToDo] = useState<BoardHandoff | null>(null);
  const [savedViews, setSavedViews] = useState<SavedView[]>(() => loadSavedViews());
  const [listMode, setListMode] = useState<"list" | "board">("list");
  const [dropActive, setDropActive] = useState(false);

  const selected = records.find((r) => recordId(r) === selectedId) ?? null;
  const activitiesEnabled = Boolean(enabledPkgs?.has("activities"));
  const relatedDefs = relatedListsFor(objectName, enabledPkgs);
  const objectViews = viewsForObject(savedViews, objectName);

  const filterableFields = fields.filter((f) => f.filterable !== false);
  const sortableFields = fields.filter((f) => f.sortable !== false);

  useEffect(() => {
    if (!connected || !bridge) {
      setEnabledPkgs(null);
      return;
    }
    let cancelled = false;
    void fetchEnabledPackages(bridge.fetch).then((set) => {
      if (!cancelled) setEnabledPkgs(set);
    });
    return () => {
      cancelled = true;
    };
  }, [bridge, connected]);

  useEffect(() => {
    if (!tabs.some((t) => t.id === tabId) && tabs[0]) {
      setTabId(tabs[0].id);
    }
  }, [tabs, tabId]);

  const loadObject = useCallback(
    async (
      objectApiName: string,
      filters: QueryFilter[],
      sort: SortSpec[],
      preferIds?: string[],
    ) => {
      if (!connected || !bridge) {
        setLive(false);
        const seed = SEED[objectApiName] ?? [];
        const mapped = filterSeed(seed, filters, sort);
        setFields(SEED_FIELDS[objectApiName] ?? SEED_FIELDS.Account);
        setRecords(mapped);
        const first = preferIds?.length
          ? mapped.find((r) => preferIds.includes(recordId(r))) ?? mapped[0]
          : mapped[0];
        setSelectedId(first ? recordId(first) : null);
        setFormValues(first ? { ...first } : {});
        setCreating(false);
        return;
      }
      setBusy(true);
      setErr("");
      try {
        const describe = (await bridge.fetch(
          `/client/v1/describe/${encodeURIComponent(objectApiName)}`,
        )) as DescribeObject;
        const descFields = (describe.fields ?? []).map((f) => ({
          ...f,
          apiName: f.apiName,
        }));
        setFields(descFields);

        const validFields = new Set(descFields.map((f) => f.apiName).filter(Boolean));
        const usableFilters = filters.filter((f) => validFields.has(f.field));
        const identity = listIdentityField(objectApiName);
        const fallbackSort = validFields.has(identity)
          ? identity
          : (descFields.find((f) => f.sortable !== false)?.apiName ?? identity);
        const usableSort =
          sort.filter((s) => validFields.has(s.field)).length > 0
            ? sort.filter((s) => validFields.has(s.field))
            : [{ field: fallbackSort, direction: "asc" as const }];
        const q = await queryRecords(bridge.fetch, {
          object: objectApiName,
          limit: 50,
          filters: usableFilters.length ? usableFilters : undefined,
          sort: usableSort.length ? usableSort : undefined,
        });
        let mapped = q.records ?? [];
        if (preferIds?.length) {
          const rank = new Map(preferIds.map((id, i) => [id, i]));
          mapped = [...mapped].sort((a, b) => {
            const ai = rank.get(recordId(a));
            const bi = rank.get(recordId(b));
            if (ai == null && bi == null) return 0;
            if (ai == null) return 1;
            if (bi == null) return -1;
            return ai - bi;
          });
        }
        setRecords(mapped);
        setLive(true);
        const first = preferIds?.length
          ? mapped.find((r) => preferIds.includes(recordId(r))) ?? mapped[0]
          : mapped[0];
        setSelectedId(first ? recordId(first) : null);
        setFormValues(first ? { ...first } : {});
        setCreating(false);
      } catch (e) {
        setErr(String(e));
        setLive(false);
        setRecords([]);
        setSelectedId(null);
        setFormValues({});
        setFields([]);
      } finally {
        setBusy(false);
      }
    },
    [bridge, connected],
  );

  useEffect(() => {
    void loadObject(objectName, activeFilters, activeSort, highlightIds);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- reload on object/filter/sort; highlight applied separately
  }, [objectName, activeFilters, activeSort, loadObject]);

  const applyHandoff = useCallback(
    (h: BoardHandoff) => {
      if (h.runId) setIntentRunId(h.runId);
      if (h.objectApiName) {
        const match = tabs.find((t) => t.objectApiName === h.objectApiName);
        if (match) setTabId(match.id);
        else setTabId(h.objectApiName);
      }
      const filters = h.view?.filters ?? [];
      const sort = h.view?.sort ?? activeSort;
      setActiveFilters(filters);
      if (h.view?.sort) setActiveSort(h.view.sort);
      setHighlightIds(h.recordIds ?? []);
      if (h.recordIds?.length) {
        setSelectedId(h.recordIds[0]);
      }
      setWhatToDo(h);
      setWhatToDoOpen(Boolean(h.suggestions?.length));
      onStagedMutations?.(h.proposedMutations?.length ?? 0);
      void loadObject(h.objectApiName ?? objectName, filters, sort, h.recordIds);
      onHandoffConsumed?.();
    },
    [activeSort, loadObject, objectName, onHandoffConsumed, onStagedMutations, tabs],
  );

  useEffect(() => {
    if (handoff) applyHandoff(handoff);
  }, [handoff, applyHandoff]);

  const selectTab = (t: ObjectTab) => {
    setTabId(t.id);
    setActiveFilters([]);
    setHighlightIds([]);
    setFilterValue("");
    setListMode(t.boardField ? listMode : "list");
    const nameField = listIdentityField(t.objectApiName);
    setFilterField(nameField);
    setSortField(nameField);
    setActiveSort([{ field: nameField, direction: "asc" }]);
  };

  const applyFilter = () => {
    if (!filterValue.trim() && filterOp !== "is_null" && filterOp !== "is_not_null") {
      setActiveFilters([]);
      return;
    }
    setActiveFilters([
      {
        field: filterField,
        op: filterOp,
        value: filterOp === "is_null" || filterOp === "is_not_null" ? undefined : filterValue.trim(),
      },
    ]);
  };

  const clearFilters = () => {
    setFilterValue("");
    setActiveFilters([]);
    setHighlightIds([]);
  };

  const applySort = () => {
    setActiveSort([{ field: sortField, direction: sortDir }]);
  };

  const saveCurrentView = () => {
    const name = window.prompt("Saved view name?");
    if (!name?.trim()) return;
    const view: SavedView = {
      id: `view-${Date.now()}`,
      name: name.trim(),
      objectApiName: objectName,
      filters: activeFilters,
      sort: activeSort,
      limit: 50,
    };
    setSavedViews(upsertSavedView(savedViews, view));
  };

  const startCreate = () => {
    if (!connected) {
      setErr("Offline stub — connect a JWT session to mutate records.");
      return;
    }
    setCreating(true);
    setSelectedId(null);
    const blank: Record<string, unknown> = {};
    for (const f of fields) {
      if (f.apiName === "Name" || f.apiName === "LastName" || f.apiName === "Subject") blank[f.apiName] = "";
    }
    setFormValues(blank);
  };

  const saveRecord = async () => {
    if (!connected) {
      setErr("Offline stub — connect a JWT session to mutate records.");
      return;
    }
    const missing = requiredMissing(fields, formValues);
    if (missing.length) {
      setErr(`Required: ${missing.join(", ")}`);
      return;
    }
    if (live && bridge) {
      setBusy(true);
      setErr("");
      try {
        const payload = recordWritePayload(fields, formValues);
        if (creating) {
          const created = await createRecord(bridge.fetch, objectName, payload);
          await loadObject(objectName, activeFilters, activeSort);
          const id = recordId(created);
          if (id) {
            setSelectedId(id);
            setCreating(false);
          }
        } else if (selectedId) {
          await updateRecord(bridge.fetch, objectName, selectedId, payload);
          await loadObject(objectName, activeFilters, activeSort, [selectedId]);
        }
      } catch (e) {
        setErr(String(e));
      } finally {
        setBusy(false);
      }
      return;
    }
    setErr("Offline stub — connect a JWT session to mutate records.");
  };

  const removeSelected = async () => {
    if (!selectedId || creating) return;
    if (!connected) {
      setErr("Offline stub — connect a JWT session to mutate records.");
      return;
    }
    if (live && bridge) {
      setBusy(true);
      setErr("");
      try {
        await bridge.fetch(
          `/client/v1/sobjects/${encodeURIComponent(objectName)}/${encodeURIComponent(selectedId)}`,
          { method: "DELETE" },
        );
        await loadObject(objectName, activeFilters, activeSort);
      } catch (e) {
        setErr(String(e));
      } finally {
        setBusy(false);
      }
      return;
    }
    setErr("Offline stub — connect a JWT session to mutate records.");
  };

  const onSuggestionAction = (action: string) => {
    if (action === "focus_ids" && highlightIds.length) {
      setSelectedId(highlightIds[0]);
      setWhatToDoOpen(false);
      return;
    }
    if (action === "open_object") {
      setWhatToDoOpen(false);
      return;
    }
    if (action === "filter_type_customer") {
      setFilterField("Type");
      setFilterOp("eq");
      setFilterValue("Customer");
      setActiveFilters([{ field: "Type", op: "eq", value: "Customer" }]);
      setWhatToDoOpen(false);
      return;
    }
    setWhatToDoOpen(false);
  };

  const boardField = activeTab.boardField;
  const boardColumns = useMemo(() => {
    if (!boardField || listMode !== "board") return [];
    const field = fields.find((f) => f.apiName === boardField);
    const stages = field?.picklistValues?.length
      ? field.picklistValues
      : Array.from(new Set(records.map((r) => String(r[boardField] ?? "—"))));
    return stages;
  }, [boardField, fields, listMode, records]);

  const columnsPreview = fields.slice(0, 8).map((f) => f.apiName);

  return (
    <div
      className={`panel crm-panel tool-surface ${dropActive ? "crm-drop-active" : ""}`}
      data-testid="crm-panel"
      data-tool-surface="true"
      onDragOver={(e) => {
        if (![...e.dataTransfer.types].includes(BOARD_HANDOFF_MIME) && !e.dataTransfer.types.includes("text/plain")) {
          return;
        }
        e.preventDefault();
        setDropActive(true);
      }}
      onDragLeave={() => setDropActive(false)}
      onDrop={(e) => {
        e.preventDefault();
        setDropActive(false);
        const raw =
          e.dataTransfer.getData(BOARD_HANDOFF_MIME) || e.dataTransfer.getData("text/plain");
        if (!raw) return;
        try {
          const parsed = JSON.parse(raw) as BoardHandoff;
          if (parsed && (parsed.objectApiName || parsed.recordIds || parsed.view)) {
            applyHandoff({ ...parsed, source: parsed.source ?? "tool_result" });
          }
        } catch {
          /* ignore */
        }
      }}
    >
      <PanelHeader
        title="CRM"
        subtitle={
          live
            ? "Live Client query · describe-driven forms · handoff-ready."
            : connected
              ? "Loading live records…"
              : "Offline stub — connect for metadata-driven Client data."
        }
        actions={
          <div className="row" style={{ gap: "0.5rem", alignItems: "center" }}>
            {busy ? <Spinner /> : null}
            {intentRunId ? (
              <span className="crm-intent-chip" data-testid="crm-intent-chip">
                <StatusBadge tone="accent">Run {intentRunId.slice(0, 8)}</StatusBadge>
                <button
                  type="button"
                  className="crm-intent-clear"
                  aria-label="Clear intent"
                  onClick={() => setIntentRunId(null)}
                >
                  ×
                </button>
              </span>
            ) : null}
            {live ? (
              <StatusBadge tone="success">Live</StatusBadge>
            ) : connected ? (
              <StatusBadge tone="neutral">Draft</StatusBadge>
            ) : (
              <span data-testid="crm-offline-stub">
                <StatusBadge tone="warn">Offline stub</StatusBadge>
              </span>
            )}
            {onClose ? (
              <Button variant="ghost" onClick={onClose}>
                Close
              </Button>
            ) : null}
          </div>
        }
      />
      {err ? <p className="err">{err}</p> : null}
      <WhatToDo
        open={whatToDoOpen}
        rationale={whatToDo?.rationale}
        suggestions={whatToDo?.suggestions ?? []}
        onDismiss={() => setWhatToDoOpen(false)}
        onAction={onSuggestionAction}
      />
      <div className="crm-tabs" role="tablist" aria-label="CRM objects">
        {tabs.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            aria-selected={tabId === t.id}
            className={`crm-tab ${tabId === t.id ? "active" : ""}`}
            data-testid={`crm-tab-${tabTestId(t.objectApiName)}`}
            onClick={() => selectTab(t)}
          >
            {t.label}
          </button>
        ))}
      </div>
      <p className="muted crm-columns" data-testid="crm-columns">
        Columns: {columnsPreview.join(", ") || "—"}
      </p>
      <div className="crm-query-bar" data-testid="crm-query-bar">
        <label className="composer-label">
          Filter field
          <select
            value={filterField}
            onChange={(e) => setFilterField(e.target.value)}
            aria-label="Filter field"
          >
            {(filterableFields.length ? filterableFields : fields).map((f) => (
              <option key={f.apiName} value={f.apiName}>
                {f.label ?? f.apiName}
              </option>
            ))}
          </select>
        </label>
        <label className="composer-label">
          Op
          <select value={filterOp} onChange={(e) => setFilterOp(e.target.value as QueryFilter["op"])} aria-label="Filter operator">
            <option value="like">contains</option>
            <option value="eq">equals</option>
            <option value="ne">not equals</option>
          </select>
        </label>
        <label className="composer-label">
          Value
          <input
            value={filterValue}
            onChange={(e) => setFilterValue(e.target.value)}
            placeholder="Filter value…"
            aria-label="Filter value"
            onKeyDown={(e) => {
              if (e.key === "Enter") applyFilter();
            }}
          />
        </label>
        <Button variant="secondary" onClick={applyFilter}>
          Apply filter
        </Button>
        <Button variant="ghost" onClick={clearFilters}>
          Clear
        </Button>
        <label className="composer-label">
          Sort
          <select value={sortField} onChange={(e) => setSortField(e.target.value)} aria-label="Sort field">
            {(sortableFields.length ? sortableFields : fields).map((f) => (
              <option key={f.apiName} value={f.apiName}>
                {f.label ?? f.apiName}
              </option>
            ))}
          </select>
        </label>
        <select value={sortDir} onChange={(e) => setSortDir(e.target.value as "asc" | "desc")} aria-label="Sort direction">
          <option value="asc">asc</option>
          <option value="desc">desc</option>
        </select>
        <Button variant="ghost" onClick={applySort}>
          Sort
        </Button>
        <Button variant="ghost" onClick={saveCurrentView} title={SAVED_VIEWS_UPGRADE_NOTE}>
          Save view
        </Button>
        {boardField ? (
          <Button
            variant={listMode === "board" ? "secondary" : "ghost"}
            onClick={() => setListMode((m) => (m === "board" ? "list" : "board"))}
            data-testid="crm-board-toggle"
          >
            {listMode === "board" ? "List" : "Pipeline"}
          </Button>
        ) : null}
      </div>
      {objectViews.length > 0 ? (
        <div className="crm-saved-views" data-testid="crm-saved-views">
          {objectViews.map((v) => (
            <span key={v.id} className="crm-saved-view-chip">
              <button
                type="button"
                className="secondary"
                onClick={() => {
                  setActiveFilters(v.filters);
                  setActiveSort(v.sort);
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
      <div className="crm-body">
        <div className="crm-list" data-testid="crm-list">
          <div className="crm-toolbar">
            <Button variant="primary" onClick={startCreate} busy={busy} disabled={!connected}>
              New
            </Button>
            {activeFilters.length ? (
              <StatusBadge tone="accent">{activeFilters.length} filter(s)</StatusBadge>
            ) : null}
          </div>
          {records.length === 0 ? (
            <EmptyState
              icon={<IconRecords size={28} />}
              title={`No ${activeTab.label.toLowerCase()}`}
              description={
                live
                  ? "Create a record or clear filters — query returned no rows."
                  : connected
                    ? "Live query returned no rows."
                    : "Offline stub — connect a JWT session for Client records."
              }
            />
          ) : listMode === "board" && boardField ? (
            <div className="crm-pipeline" data-testid="crm-pipeline">
              {boardColumns.map((stage) => (
                <div key={stage} className="crm-pipeline-col">
                  <p className="crm-pipeline-title">{stage}</p>
                  <ul>
                    {records
                      .filter((r) => String(r[boardField] ?? "—") === stage)
                      .map((r) => {
                        const id = recordId(r);
                        return (
                          <li key={id}>
                            <button
                              type="button"
                              className={`crm-row-btn ${selectedId === id ? "active" : ""} ${highlightIds.includes(id) ? "highlight" : ""}`}
                              onClick={() => {
                                setCreating(false);
                                setSelectedId(id);
                                setFormValues({ ...r });
                              }}
                            >
                              <span className="crm-row-name">{displayName(r)}</span>
                            </button>
                          </li>
                        );
                      })}
                  </ul>
                </div>
              ))}
            </div>
          ) : (
            <ul className="crm-row-picker" aria-label={`${activeTab.label} picker`}>
              {records.map((r) => {
                const id = recordId(r);
                const status = String(r.StageName ?? r.Status ?? r.Type ?? "—");
                return (
                  <li key={id}>
                    <button
                      type="button"
                      className={`crm-row-btn ${selectedId === id ? "active" : ""} ${highlightIds.includes(id) ? "highlight" : ""}`}
                      onClick={() => {
                        setCreating(false);
                        setSelectedId(id);
                        setFormValues({ ...r });
                      }}
                    >
                      <span className="crm-row-main">
                        <span className="crm-row-name">{displayName(r)}</span>
                        <span className="muted crm-row-owner">{String(r.Email ?? r.Industry ?? r.Amount ?? "")}</span>
                      </span>
                      <StatusBadge tone={status === "Customer" || status === "Negotiation" ? "success" : "neutral"}>
                        {status}
                      </StatusBadge>
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
        <aside className="crm-detail" data-testid="crm-detail" aria-label="Record detail">
          {creating || selected ? (
            <>
              <p className="crm-detail-kicker">{creating ? `New ${objectName}` : objectName}</p>
              <h3>{creating ? "Create record" : displayName(selected!)}</h3>
              <RecordForm
                fields={fields}
                values={formValues}
                onChange={setFormValues}
                mode={creating ? "create" : "edit"}
              />
              <div className="row">
                <Button variant="primary" onClick={() => void saveRecord()} busy={busy} disabled={!connected}>
                  Save
                </Button>
                {!creating ? (
                  <Button variant="danger" onClick={() => void removeSelected()} busy={busy} disabled={!connected}>
                    Delete
                  </Button>
                ) : (
                  <Button
                    variant="ghost"
                    onClick={() => {
                      setCreating(false);
                      if (selected) setFormValues({ ...selected });
                    }}
                  >
                    Cancel
                  </Button>
                )}
              </div>
              {!creating && selectedId ? (
                <>
                  <RelatedLists
                    bridge={bridge}
                    parentId={selectedId}
                    defs={relatedDefs}
                    onOpenRelated={(obj, id) => {
                      const match = tabs.find((t) => t.objectApiName === obj);
                      if (match) {
                        setTabId(match.id);
                        setHighlightIds([id]);
                        setSelectedId(id);
                      }
                    }}
                  />
                  <ActivityFeed
                    bridge={bridge}
                    parentType={objectName}
                    parentId={selectedId}
                    activitiesEnabled={activitiesEnabled || !connected}
                  />
                  {!connected ? (
                    <p className="muted crm-footnote">
                      Activity feed preview (enable package `activities` when connected).
                    </p>
                  ) : null}
                </>
              ) : null}
              <p className="muted crm-footnote">
                {live
                  ? "Writes go through Client API; AuthZ stays on the install."
                  : "Offline stub — connect a JWT session for live Client CRUD."}
              </p>
            </>
          ) : (
            <EmptyState title="Select a record" description="Pick a row to edit describe-driven fields." />
          )}
        </aside>
      </div>
    </div>
  );
}

const OFFLINE_FALLBACK: ObjectTab = {
  id: "Account",
  objectApiName: "Account",
  label: "Accounts",
};

function tabTestId(objectApiName: string): string {
  switch (objectApiName) {
    case "Account":
      return "accounts";
    case "Contact":
      return "contacts";
    case "Opportunity":
      return "opportunities";
    case "Quote":
      return "quotes";
    case "Case":
      return "cases";
    default:
      return objectApiName.toLowerCase();
  }
}
