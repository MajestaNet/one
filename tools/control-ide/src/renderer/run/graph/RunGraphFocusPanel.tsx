import { useCallback, useEffect, useMemo, useState } from "react";
import type { AppBridge } from "../../App";
import { ActivityFeed } from "../../operate/ActivityFeed";
import { describeCache, normalizeDescribeObject } from "../../operate/describeCache";
import { fetchEnabledPackages, relatedListsFor } from "../../operate/packages";
import { editableFields, RecordForm, requiredMissing } from "../../operate/RecordForm";
import { RelatedLists } from "../../operate/RelatedLists";
import { displayName } from "../../operate/types";
import { resultColumns } from "../../operate/queryAutocomplete";
import type { DescribeObject } from "../../operate/types";
import { Button, DataTable, Spinner } from "../../ui";
import type { RunGraphFetch } from "./api";
import { runGraphNodeLabel } from "./nodes/RunGraphNodeCard";
import {
  applyProposalMutations,
  ProposalApplyPartialError,
  type ProposalStagingStore,
} from "./proposalStaging";
import type { RunGraphSignalResult } from "./signalBindings";
import { RunObjectHomePanel } from "../RunObjectHomePanel";
import { RunToolPanel } from "../RunToolPanel";
import { collectionListFilters } from "./collection";
import type { RunGraphBinding, RunGraphNode, RunGraphResolveResult } from "./types";
import { runGraphKindLabel } from "./labels";

type RecordMap = Record<string, unknown>;

function flattenRecord(record: RecordMap): RecordMap {
  const flattened: RecordMap = {};
  for (const [key, value] of Object.entries(record)) {
    if (key === "data" && value && typeof value === "object" && !Array.isArray(value)) {
      Object.assign(flattened, value);
    } else {
      flattened[key] = value;
    }
  }
  if (flattened.id == null && flattened.Id != null) flattened.id = flattened.Id;
  return flattened;
}

function RunGraphRecordFocus({
  node,
  resolve,
  fetchFn,
  bridge,
  onOpenRecord,
  onRecordSaved,
}: {
  node: RunGraphNode;
  resolve?: RunGraphResolveResult;
  fetchFn: RunGraphFetch;
  bridge?: AppBridge;
  onOpenRecord?: (objectApiName: string, recordId: string) => void;
  onRecordSaved?: (node: RunGraphNode) => void;
}) {
  const objectApiName = node.ref?.objectApiName ?? "";
  const recordId = node.ref?.recordId ?? "";
  const installId = bridge?.session?.activeInstallId ?? bridge?.session?.baseUrl ?? "run-graph";
  const [describe, setDescribe] = useState<DescribeObject | null>(null);
  const [record, setRecord] = useState<RecordMap | null>(null);
  const [formValues, setFormValues] = useState<RecordMap>({});
  const [enabledPackages, setEnabledPackages] = useState<Set<string> | null>(null);
  const [busy, setBusy] = useState<"load" | "save" | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [expanded, setExpanded] = useState(false);

  const load = useCallback(async () => {
    if (!objectApiName || !recordId) return;
    setBusy("load");
    setError("");
    try {
      const cachedDescribe = describeCache.getObject(installId, objectApiName);
      const [nextDescribe, rawRecord, packages] = await Promise.all([
        cachedDescribe
          ? Promise.resolve(cachedDescribe)
          : fetchFn(`/client/v1/describe/${encodeURIComponent(objectApiName)}`).then((raw) => {
              const normalized = normalizeDescribeObject(raw, objectApiName);
              describeCache.setObject(installId, objectApiName, normalized);
              return normalized;
            }),
        fetchFn(
          `/client/v1/sobjects/${encodeURIComponent(objectApiName)}/${encodeURIComponent(recordId)}`,
        ),
        fetchEnabledPackages(fetchFn),
      ]);
      const nextRecord = flattenRecord(rawRecord as RecordMap);
      setDescribe(nextDescribe);
      setRecord(nextRecord);
      setFormValues(nextRecord);
      setEnabledPackages(packages);
    } catch (reason) {
      setDescribe(null);
      setRecord(null);
      setFormValues({});
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setBusy(null);
    }
  }, [fetchFn, installId, objectApiName, recordId]);

  useEffect(() => {
    setNotice("");
    void load();
  }, [load]);

  useEffect(() => {
    setExpanded(false);
  }, [node.id]);

  const related = useMemo(
    () => relatedListsFor(objectApiName, enabledPackages),
    [enabledPackages, objectApiName],
  );
  const activitiesEnabled = Boolean(enabledPackages?.has("activities"));
  const compactFields = useMemo(() => {
    const fields = describe?.fields ?? [];
    const preferred = new Set([
      "Name", "FirstName", "LastName", "Subject", "Status", "StageName",
      "Email", "Phone", "AccountNumber", "OwnerId",
    ]);
    const chosen = fields.filter((field) => field.required || preferred.has(field.apiName));
    return chosen.length ? chosen.slice(0, 9) : fields.slice(0, 7);
  }, [describe?.fields]);

  const save = async () => {
    const fields = describe?.fields ?? [];
    const missing = requiredMissing(fields, formValues);
    if (missing.length) {
      setError(`Required: ${missing.join(", ")}`);
      return;
    }
    setBusy("save");
    setError("");
    setNotice("");
    try {
      const payload = Object.fromEntries(
        editableFields(fields).flatMap((field) =>
          Object.prototype.hasOwnProperty.call(formValues, field.apiName)
            ? [[field.apiName, formValues[field.apiName]]]
            : [],
        ),
      );
      await fetchFn(
        `/client/v1/sobjects/${encodeURIComponent(objectApiName)}/${encodeURIComponent(recordId)}`,
        { method: "PATCH", body: JSON.stringify(payload) },
      );
      await load();
      onRecordSaved?.(node);
      setNotice("Record saved.");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setBusy(null);
    }
  };

  if (!objectApiName || !recordId) {
    return <p className="error">This record pin is missing its reference.</p>;
  }
  if (busy === "load" && !record) {
    return <p className="muted"><Spinner /> Loading record…</p>;
  }
  if (error && !record) {
    return (
      <div>
        <p className="error" role="alert">{error}</p>
        <p className="muted">
          {resolve?.code === "FORBIDDEN" ? "You no longer have access to this record." : "This record could not be opened."}
        </p>
      </div>
    );
  }

  return (
    <>
      <p className="run-graph-focus-kicker">{objectApiName}</p>
      <h3>{record ? displayName(record) : runGraphNodeLabel(node, resolve).title}</h3>
      {describe ? (
        <RecordForm
          fields={expanded ? describe.fields ?? [] : compactFields}
          values={formValues}
          onChange={setFormValues}
          mode="edit"
        />
      ) : null}
      <div className="row">
        <Button variant="primary" busy={busy === "save"} disabled={!record || busy !== null} onClick={() => void save()}>
          Save record
        </Button>
        <Button variant="ghost" busy={busy === "load"} disabled={busy !== null} onClick={() => void load()}>
          Refresh
        </Button>
        <Button variant="ghost" onClick={() => setExpanded((current) => !current)}>
          {expanded ? "Compact view" : "Open full record"}
        </Button>
      </div>
      {notice ? <p className="run-graph-notice" role="status">{notice}</p> : null}
      {error ? <p className="error" role="alert">{error}</p> : null}
      {record && bridge && expanded ? (
        <RelatedLists
          bridge={bridge}
          parentId={recordId}
          defs={related}
          onOpenRelated={onOpenRecord}
        />
      ) : null}
      {record && bridge && expanded ? (
        <ActivityFeed
          bridge={bridge}
          parentType={objectApiName}
          parentId={recordId}
          activitiesEnabled={activitiesEnabled}
        />
      ) : null}
      <p className="muted crm-footnote">Changes save to the active record.</p>
    </>
  );
}

function RunGraphProposalFocus({
  node,
  fetchFn,
  proposalStore,
  onResolveProposal,
}: {
  node: RunGraphNode;
  fetchFn: RunGraphFetch;
  proposalStore?: ProposalStagingStore;
  onResolveProposal?: (status: "applied" | "rejected") => Promise<void>;
}) {
  const proposalId = node.proposalId ?? "";
  const staging = proposalId ? proposalStore?.get(proposalId) : undefined;
  const [busy, setBusy] = useState<"apply" | "reject" | null>(null);
  const [error, setError] = useState("");

  const resolve = async (status: "applied" | "rejected") => {
    if (!onResolveProposal) return;
    setBusy(status === "applied" ? "apply" : "reject");
    setError("");
    try {
      if (status === "applied") {
        if (!staging) throw new Error("Proposal unavailable — re-ask the agent.");
        await applyProposalMutations(fetchFn, staging, proposalStore);
      }
      await onResolveProposal(status);
    } catch (reason) {
      if (reason instanceof ProposalApplyPartialError) {
        setError(reason.message);
      } else {
        setError(reason instanceof Error ? reason.message : String(reason));
      }
    } finally {
      setBusy(null);
    }
  };

  if (!staging) {
    return (
      <div className="run-graph-proposal" data-testid="run-graph-proposal-unavailable">
        <p className="error">Proposal unavailable — re-ask the agent.</p>
        <p className="muted">This review is no longer available. Ask the agent to prepare it again.</p>
        {onResolveProposal ? (
          <Button variant="danger" busy={busy === "reject"} onClick={() => void resolve("rejected")}>
            Remove unavailable proposal
          </Button>
        ) : null}
        {error ? <p className="error" role="alert">{error}</p> : null}
      </div>
    );
  }

  return (
    <div className="run-graph-proposal" data-testid="run-graph-proposal-review">
      {staging.rationale ? <p>{staging.rationale}</p> : null}
      <ol className="run-graph-proposal-list">
        {staging.mutations.map((mutation, index) => (
          <li key={`${mutation.op}-${mutation.object}-${mutation.id ?? index}`}>
            <div>
              <strong>{mutation.op}</strong> {mutation.object}
              {mutation.id ? <span className="muted"> · {mutation.id}</span> : null}
            </div>
            {mutation.data ? <pre>{JSON.stringify(mutation.data, null, 2)}</pre> : null}
          </li>
        ))}
      </ol>
      <p className="muted crm-footnote">
        Applying makes these changes to the active records. This review is available for this session.
        {typeof staging.appliedThrough === "number" && staging.appliedThrough > 0
          ? ` ${staging.appliedThrough} of ${staging.mutations.length} mutations already applied — retry continues from the remainder.`
          : " Changes are checked first; if one fails, the review stays here so you can retry the remainder."}
      </p>
      <div className="row">
        <Button
          variant="primary"
          busy={busy === "apply"}
          disabled={busy !== null || !onResolveProposal}
          onClick={() => void resolve("applied")}
        >
          Approve and apply
        </Button>
        <Button
          variant="danger"
          busy={busy === "reject"}
          disabled={busy !== null || !onResolveProposal}
          onClick={() => void resolve("rejected")}
        >
          Reject
        </Button>
      </div>
      {error ? <p className="error" role="alert">{error}</p> : null}
    </div>
  );
}

function RunGraphSignalFocus({
  result,
  error,
  onPinRows,
}: {
  result?: RunGraphSignalResult;
  error?: string;
  onPinRows?: (result: RunGraphSignalResult) => Promise<number>;
}) {
  const [pinning, setPinning] = useState(false);
  const [pinError, setPinError] = useState("");
  const [notice, setNotice] = useState("");
  if (error) return <p className="error" role="alert">{error}</p>;
  if (!result) return <p className="muted"><Spinner /> Refreshing live list…</p>;
  const columns = resultColumns(result.rows).map((key) => ({ key, label: key }));
  const pinnable = result.rows.some((row) => Boolean(row.id ?? row.Id));

  const pin = async () => {
    if (!onPinRows) return;
    setPinning(true);
    setPinError("");
    setNotice("");
    try {
      const count = await onPinRows(result);
      setNotice(`Ensured ${count} survivor pin${count === 1 ? "" : "s"} in your personal graph.`);
    } catch (reason) {
      setPinError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setPinning(false);
    }
  };

  return (
    <div className="run-graph-signal" data-testid="run-graph-signal-live">
      <p className="muted">
        Live list · {result.objectApiName} · {result.rows.length} row{result.rows.length === 1 ? "" : "s"}
      </p>
      <DataTable columns={columns} rows={result.rows} emptyLabel="No live survivors." />
      <Button
        variant="primary"
        busy={pinning}
        disabled={!pinnable || !onPinRows}
        onClick={() => void pin()}
      >
        Pin survivors
      </Button>
      {notice ? <p className="run-graph-notice" role="status">{notice}</p> : null}
      {pinError ? <p className="error" role="alert">{pinError}</p> : null}
      <p className="muted crm-footnote">Pin the records you want to keep close.</p>
    </div>
  );
}

function RunGraphCollectionFocus({
  node,
  binding,
  bridge,
  onPinnedToGraph,
  selectedRecordId,
  selectedRecordEpoch = 0,
}: {
  node: RunGraphNode;
  binding?: RunGraphBinding;
  bridge?: AppBridge;
  onPinnedToGraph?: (nodeId?: string) => void;
  selectedRecordId?: string | null;
  selectedRecordEpoch?: number;
}) {
  const objectApiName = node.ref?.objectApiName ?? "";
  const filters = collectionListFilters(node, binding);
  if (!objectApiName) {
    return <p className="error">This collection is missing its object.</p>;
  }
  if (!bridge) {
    return <p className="muted">Connect to browse this list.</p>;
  }
  return (
    <div className="run-graph-collection-focus" data-testid="run-graph-collection-focus">
      {node.searchQ ? <p className="muted">Saved find · {node.searchQ}</p> : null}
      {binding ? <p className="muted">Saved list · {binding.id}</p> : null}
      <RunObjectHomePanel
        bridge={bridge}
        variant="embedded"
        lockObject
        collectionNodeId={node.id}
        initialObjectApiName={objectApiName}
        initialSelectedId={selectedRecordId}
        selectedIdEpoch={selectedRecordEpoch}
        initialFilters={filters}
        filtersEpoch={filters.length}
        onPinnedToGraph={onPinnedToGraph}
      />
      <p className="muted crm-footnote">
        Browse and pin records without leaving your graph.
      </p>
    </div>
  );
}

export function RunGraphFocusPanel({
  node,
  resolve,
  fetchFn,
  bridge,
  onClose,
  onOpenTool,
  onOpenRecord,
  onRecordSaved,
  onUpdateAnnotation,
  proposalStore,
  onResolveProposal,
  signalResult,
  signalError,
  onPinSignalRows,
  linkedTool,
  collectionBinding,
  onPinnedFromCollection,
  selectedRecordId,
  selectedRecordEpoch = 0,
  toolLabel,
  onAskAgent,
}: {
  node: RunGraphNode;
  resolve?: RunGraphResolveResult;
  fetchFn: RunGraphFetch;
  bridge?: AppBridge;
  onClose: () => void;
  onOpenTool?: (node: RunGraphNode) => void;
  onOpenRecord?: (objectApiName: string, recordId: string) => void;
  onRecordSaved?: (node: RunGraphNode) => void;
  onUpdateAnnotation?: (nodeId: string, text: string) => Promise<void>;
  proposalStore?: ProposalStagingStore;
  onResolveProposal?: (status: "applied" | "rejected") => Promise<void>;
  signalResult?: RunGraphSignalResult;
  signalError?: string;
  onPinSignalRows?: (result: RunGraphSignalResult) => Promise<number>;
  linkedTool?: RunGraphNode;
  collectionBinding?: RunGraphBinding;
  onPinnedFromCollection?: (nodeId?: string) => void;
  selectedRecordId?: string | null;
  selectedRecordEpoch?: number;
  toolLabel?: string;
  onAskAgent?: (prompt: string) => void;
}) {
  const label = runGraphNodeLabel(node, resolve, toolLabel);
  const annotation = node.kind === "insight" || node.kind === "question";
  const [text, setText] = useState(annotation ? node.text ?? "" : "");
  const [savingText, setSavingText] = useState(false);
  const [annotationError, setAnnotationError] = useState("");

  useEffect(() => {
    setText(annotation ? node.text ?? "" : "");
    setAnnotationError("");
  }, [annotation, node.id, node.text]);

  const saveAnnotation = async () => {
    const next = text.trim();
    if (!next || !onUpdateAnnotation) return;
    setSavingText(true);
    setAnnotationError("");
    try {
      await onUpdateAnnotation(node.id, next);
    } catch (reason) {
      setAnnotationError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setSavingText(false);
    }
  };

  return (
    <aside className={`run-graph-focus crm-detail${node.kind === "collection" ? " is-collection" : ""}`} data-testid="run-graph-focus" aria-label="Graph focus">
      <header className="run-graph-focus-header">
        <div>
          <p className="run-graph-focus-kicker">{runGraphKindLabel(node.kind)}</p>
          {node.kind !== "record" ? <h3>{label.title}</h3> : null}
        </div>
        <button type="button" className="icon-btn" aria-label="Close graph focus" onClick={onClose}>×</button>
      </header>
      {node.kind === "record" ? (
        <RunGraphRecordFocus
          node={node}
          resolve={resolve}
          fetchFn={fetchFn}
          bridge={bridge}
          onOpenRecord={onOpenRecord}
          onRecordSaved={onRecordSaved}
        />
      ) : node.kind === "collection" ? (
        <RunGraphCollectionFocus
          node={node}
          binding={collectionBinding}
          bridge={bridge}
          onPinnedToGraph={onPinnedFromCollection}
          selectedRecordId={selectedRecordId}
          selectedRecordEpoch={selectedRecordEpoch}
        />
      ) : node.kind === "proposal" ? (
        <RunGraphProposalFocus
          node={node}
          fetchFn={fetchFn}
          proposalStore={proposalStore}
          onResolveProposal={onResolveProposal}
        />
      ) : node.kind === "signal" ? (
        <RunGraphSignalFocus
          result={signalResult}
          error={signalError}
          onPinRows={onPinSignalRows}
        />
      ) : annotation ? (
        <div className="run-graph-annotation-editor">
          <label className="composer-label">
            Annotation
            <textarea maxLength={4096} rows={7} value={text} onChange={(event) => setText(event.target.value)} />
          </label>
          <Button variant="primary" busy={savingText} disabled={!text.trim() || text.trim() === (node.text ?? "").trim()} onClick={() => void saveAnnotation()}>
            Save annotation
          </Button>
          {annotationError ? <p className="error" role="alert">{annotationError}</p> : null}
        </div>
      ) : node.kind === "tool" ? (
        <div className="run-graph-tool-sheet" data-testid="run-graph-tool-sheet">
          <RunToolPanel
            apiName={node.toolRef?.toolSpecApiName ?? toolLabel ?? "Tool"}
            label={toolLabel || node.toolRef?.toolSpecApiName || node.toolRef?.workingToolId || "Tool"}
            fetchFn={fetchFn}
            sessionToolId={node.toolRef?.workingToolId}
            onAskAgent={onAskAgent}
            variant="embedded"
          />
          {onOpenTool ? (
            <Button variant="ghost" onClick={() => onOpenTool(node)}>Open as board</Button>
          ) : null}
        </div>
      ) : node.kind === "person" ? (
        <>
          <p className="muted">{label.detail || "Person or contact"}</p>
          {node.ref?.contactRecordId && onOpenRecord ? (
            <Button variant="secondary" onClick={() => onOpenRecord("Contact", node.ref!.contactRecordId!)}>
              Open linked Contact
            </Button>
          ) : null}
        </>
      ) : (
        <p className="muted">{label.detail || "Connected item"}</p>
      )}
      {linkedTool && node.kind !== "tool" && onOpenTool ? (
        <div className="run-graph-opens-tool" data-testid="run-graph-opens-tool">
          <p className="muted">
            Opens · {linkedTool.toolRef?.toolSpecApiName ?? linkedTool.toolRef?.workingToolId ?? "Tool"}
          </p>
          <Button variant="secondary" onClick={() => onOpenTool(linkedTool)}>
            Open linked Tool
          </Button>
        </div>
      ) : null}
    </aside>
  );
}
