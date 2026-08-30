import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { AppBridge } from "../../App";
import { Button, EmptyState, Spinner } from "../../ui";
import type { ContextExcerpt } from "../../workspace/contextExcerpt";
import type { OperateToolDragPayload } from "../../workspace/operateToolDrag";
import {
  getHomeRunGraph,
  patchRunGraph,
  putRunGraph,
  resolveRunGraphCards,
  type RunGraphFetch,
} from "./api";
import {
  describeCache,
  normalizeDescribeObject,
  normalizeGlobalObjects,
  type GlobalDescribeObject,
} from "../../operate/describeCache";
import { OperateSearch, type SearchHit } from "../../operate/OperateSearch";
import { executeGraphBridge } from "./agentGraphTools";
import { RunGraphFocusPanel } from "./RunGraphFocusPanel";
import { applyRunGraphLens, RUN_GRAPH_LENSES, type RunGraphLensId } from "./lenses";
import {
  markMyDayItemDoneEdges,
  myDayDoneEdgeIds,
  myDayPromotionEndpoints,
  promoteMyDayItemEdges,
  rankMyDayQueue,
  type MyDayQueueItem,
} from "./myDayQueue";
import { runGraphNodeLabel } from "./nodes/RunGraphNodeCard";
import { toolForOpenedNode } from "./opens";
import type { ProposalStagingStore } from "./proposalStaging";
import { graphSelectionToContextExcerpt } from "./selectionContext";
import {
  executeRunGraphSignalBinding,
  RUN_GRAPH_SIGNAL_TTL_MS,
  RunGraphSignalCache,
  type RunGraphSignalResult,
} from "./signalBindings";
import { RunGraphHydrateCache } from "./store";
import { RunGraphView } from "./RunGraphView";
import {
  RUN_GRAPH_EDGE_KINDS,
  type RunGraphDocument,
  type RunGraphEdge,
  type RunGraphEdgeKind,
  type RunGraphResolveResult,
} from "./types";
import type { RunGraphNode } from "./types";
import { publishRunGraphSubgraph } from "./publishSubgraph";
import { tidyPositions } from "./layoutBands";
import { mergeAccessibleObjectModel } from "./objectModel";
import { orphanDerivedRecordIds, tidyAttentionDocument } from "./hygiene";
import { runGraphEdgeLabel } from "./labels";

export type RunGraphMountableTool =
  | { id: string; kind: "published"; label: string; toolSpecApiName: string }
  | { id: string; kind: "working"; label: string; workingToolId: string };

const MY_DAY_CURATOR_PROMPT =
  "Act as the curator for my personal Run graph. Use graph.get, then maintain the My day cluster and rank attention with blocks, next, and watches edges; annotate any important question. End with visible graph.* topology writes only, never record fields or query rows.";

export function RunGraphHome({
  fetchFn,
  bridge,
  refreshKey = 0,
  mountableTools = [],
  onOpenObjectHome,
  onAskRunAgent,
  onOpenTool,
  onPublishedTool,
  proposalStore,
  onProposalResolved,
  onSelectionContextChange,
  mountRequest,
}: {
  fetchFn: RunGraphFetch;
  /** Active app bridge for reused Operate related/activity primitives. */
  bridge?: AppBridge;
  refreshKey?: number;
  /** AuthZ-filtered ToolSpecs and local working Tools available for reference-only mounts. */
  mountableTools?: readonly RunGraphMountableTool[];
  /** Empty-state CTA: open Run List View to pin records. */
  onOpenObjectHome?: () => void;
  /** Empty My day CTA: ask a Run curator to rebuild attention topology. */
  onAskRunAgent?: (prompt: string) => void;
  onOpenTool?: (node: RunGraphNode) => void;
  onPublishedTool?: (apiName: string) => void;
  proposalStore?: ProposalStagingStore;
  onProposalResolved?: (status: "applied" | "rejected", pendingCount: number) => void;
  onSelectionContextChange?: (excerpt: ContextExcerpt | null) => void;
  /** Rail click while My graph is open: ensure/focus the Tool instead of swapping the canvas. */
  mountRequest?: { toolId: string; epoch: number } | null;
}) {
  const [document, setDocument] = useState<RunGraphDocument | null>(null);
  const [lens, setLens] = useState<RunGraphLensId>("all");
  const [resolved, setResolved] = useState<Record<string, RunGraphResolveResult>>({});
  const [signalResults, setSignalResults] = useState<Record<string, RunGraphSignalResult>>({});
  const [signalErrors, setSignalErrors] = useState<Record<string, string>>({});
  const [signalRefreshEpoch, setSignalRefreshEpoch] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedNodeIds, setSelectedNodeIds] = useState<string[]>([]);
  const [selectedEdgeId, setSelectedEdgeId] = useState<string | null>(null);
  const [wireKind, setWireKind] = useState<RunGraphEdgeKind>("relates");
  const [publishApiName, setPublishApiName] = useState("");
  const [publishLabel, setPublishLabel] = useState("");
  const [noteText, setNoteText] = useState("");
  const [annotationKind, setAnnotationKind] = useState<"insight" | "question">("insight");
  const [composerOpen, setComposerOpen] = useState(false);
  const [busyAction, setBusyAction] = useState<"add-note" | "mount-tool" | "compact" | "publish" | "tidy-layout" | null>(null);
  const [catalog, setCatalog] = useState<GlobalDescribeObject[]>([]);
  const [modelSyncing, setModelSyncing] = useState(false);
  const [connectSourceId, setConnectSourceId] = useState<string | null>(null);
  const [pulseNodeId, setPulseNodeId] = useState<string | null>(null);
  const [searchLanding, setSearchLanding] = useState<{ nodeId: string; recordId: string; epoch: number } | null>(null);
  const [queueBusy, setQueueBusy] = useState<{ action: "done" | "promote"; nodeId: string } | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const cache = useRef(new RunGraphHydrateCache());
  const signalCache = useRef(new RunGraphSignalCache());
  const revisionRef = useRef<number | null>(null);
  const graphWriteQueue = useRef<Promise<void>>(Promise.resolve());
  const modelSyncKeyRef = useRef("");
  const mountRequestEpochRef = useRef<number | null>(null);

  useEffect(() => {
    let cancelled = false;
    cache.current.clear();
    signalCache.current.clear();
    revisionRef.current = null;
    modelSyncKeyRef.current = "";
    setResolved({});
    setSignalResults({});
    setSignalErrors({});
    setLoading(true);
    setError(null);
    void getHomeRunGraph(fetchFn)
      .then((graph) => {
        if (!cancelled) {
          setDocument(graph.document);
          revisionRef.current = graph.revision;
          setSelectedNodeIds([]);
          setSelectedEdgeId(null);
        }
      })
      .catch((reason: unknown) => {
        if (!cancelled) {
          setDocument(null);
          setError(reason instanceof Error ? reason.message : "Failed to load Run graph");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [fetchFn, refreshKey]);

  const view = useMemo(
    () => (document ? applyRunGraphLens(document, lens) : { nodes: [], edges: [] }),
    [document, lens],
  );
  const myDayQueue = useMemo(() => (document ? rankMyDayQueue(document) : []), [document]);
  const selectionContext = useMemo(
    () => (document ? graphSelectionToContextExcerpt(document, selectedNodeIds) : null),
    [document, selectedNodeIds],
  );
  const visibleSignals = useMemo(
    () =>
      view.nodes.flatMap((node) => {
        if (node.kind !== "signal" || !node.bindingId) return [];
        const binding = document?.dataBindings?.find((candidate) => candidate.id === node.bindingId);
        return binding ? [{ node, binding }] : [];
      }),
    [document?.dataBindings, view.nodes],
  );
  const visibleSignalsKey = useMemo(
    () => visibleSignals.map(({ node, binding }) => `${node.id}:${JSON.stringify(binding)}`).join("|"),
    [visibleSignals],
  );
  const refsKey = useMemo(
    () =>
      view.nodes
        .filter((node) => node.kind === "record")
        .map((node) => `${node.id}:${node.ref?.objectApiName}:${node.ref?.recordId}`)
        .join("|"),
    [view.nodes],
  );

  useEffect(() => {
    onSelectionContextChange?.(selectionContext);
  }, [onSelectionContextChange, selectionContext]);

  useEffect(
    () => () => onSelectionContextChange?.(null),
    [onSelectionContextChange],
  );

  useEffect(() => {
    let cancelled = false;
    const refs = view.nodes.flatMap((node) => {
      if (node.kind !== "record" || !node.ref?.objectApiName || !node.ref.recordId) return [];
      return [{ nodeId: node.id, objectApiName: node.ref.objectApiName, recordId: node.ref.recordId }];
    });
    const cached: Record<string, RunGraphResolveResult> = {};
    const missing = refs.filter((ref) => {
      const hit = cache.current.get(ref.objectApiName, ref.recordId);
      if (hit) cached[ref.nodeId] = { ...hit, nodeId: ref.nodeId };
      return !hit;
    });
    setResolved((previous) => ({ ...previous, ...cached }));
    if (!missing.length) return;

    void resolveRunGraphCards(fetchFn, missing)
      .then((results) => {
        if (cancelled) return;
        const next: Record<string, RunGraphResolveResult> = {};
        for (const result of results) {
          const ref = missing.find((candidate) => candidate.nodeId === result.nodeId);
          if (!ref) continue;
          cache.current.set(ref.objectApiName, ref.recordId, result);
          next[result.nodeId] = result;
        }
        setResolved((previous) => ({ ...previous, ...next }));
      })
      .catch(() => {
        if (cancelled) return;
        cache.current.clear();
        setResolved((previous) => {
          const next = { ...previous };
          for (const ref of missing) next[ref.nodeId] = { nodeId: ref.nodeId, ok: false, code: "UNAVAILABLE" };
          return next;
        });
      });
    return () => {
      cancelled = true;
    };
  }, [document, fetchFn, refsKey, view.nodes]);

  useEffect(() => {
    let cancelled = false;
    const cached: Record<string, RunGraphSignalResult> = {};
    const missing = visibleSignals.filter(({ node, binding }) => {
      const hit = signalCache.current.get(node.id, binding);
      if (hit) cached[node.id] = hit;
      return !hit;
    });
    if (Object.keys(cached).length) {
      setSignalResults((previous) => ({ ...previous, ...cached }));
    }
    if (!missing.length) return;
    setSignalResults((previous) => {
      const next = { ...previous };
      for (const { node } of missing) delete next[node.id];
      return next;
    });
    setSignalErrors((previous) => {
      const next = { ...previous };
      for (const { node } of missing) delete next[node.id];
      return next;
    });

    void (async () => {
      const results: Array<
        | { ok: true; nodeId: string; binding: (typeof missing)[number]["binding"]; result: RunGraphSignalResult }
        | { ok: false; nodeId: string; binding: (typeof missing)[number]["binding"]; error: string }
      > = [];
      for (let index = 0; index < missing.length; index += 4) {
        const batch = await Promise.all(
          missing.slice(index, index + 4).map(async ({ node, binding }) => {
            try {
              const result = await executeRunGraphSignalBinding(fetchFn, node, binding);
              return { ok: true as const, nodeId: node.id, binding, result };
            } catch (reason) {
              return {
                ok: false as const,
                nodeId: node.id,
                binding,
                error: reason instanceof Error ? reason.message : String(reason),
              };
            }
          }),
        );
        results.push(...batch);
        if (cancelled) return;
      }
      if (cancelled) return;
      const nextResults: Record<string, RunGraphSignalResult> = {};
      const nextErrors: Record<string, string> = {};
      for (const row of results) {
        if (row.ok) {
          signalCache.current.set(row.nodeId, row.binding, row.result);
          nextResults[row.nodeId] = row.result;
        } else {
          nextErrors[row.nodeId] = row.error;
        }
      }
      setSignalResults((previous) => ({ ...previous, ...nextResults }));
      setSignalErrors((previous) => {
        const next = { ...previous, ...nextErrors };
        for (const nodeId of Object.keys(nextResults)) delete next[nodeId];
        return next;
      });
    })();
    return () => {
      cancelled = true;
    };
  }, [fetchFn, signalRefreshEpoch, visibleSignals]);

  useEffect(() => {
    if (!visibleSignals.length) return;
    const timer = window.setTimeout(
      () => setSignalRefreshEpoch((current) => current + 1),
      RUN_GRAPH_SIGNAL_TTL_MS + 50,
    );
    return () => window.clearTimeout(timer);
  }, [signalRefreshEpoch, visibleSignals.length, visibleSignalsKey]);

  const staleCount = Object.values(resolved).filter(
    (result) => !result.ok && (result.code === "NOT_FOUND" || result.code === "FORBIDDEN"),
  ).length;

  const reload = useCallback(async () => {
    const graph = await getHomeRunGraph(fetchFn);
    setDocument(graph.document);
    revisionRef.current = graph.revision;
    setSelectedNodeIds((current) => current.filter((id) => graph.document.nodes.some((node) => node.id === id)));
    setSelectedEdgeId((current) =>
      current && graph.document.edges.some((edge) => edge.id === current) ? current : null,
    );
  }, [fetchFn]);

  const graphLoaded = Boolean(document);

  useEffect(() => {
    if (!graphLoaded) return;
    let cancelled = false;
    const installId = bridge?.session?.activeInstallId ?? "default";
    const cached = describeCache.getGlobal(installId);
    void (async () => {
      try {
        const list = cached ?? normalizeGlobalObjects(await fetchFn("/client/v1/describe"));
        if (!cached) describeCache.setGlobal(installId, list);
        if (!cancelled) {
          setCatalog(list);
        }
      } catch {
        if (!cancelled) setCatalog([]);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [bridge?.session?.activeInstallId, fetchFn, graphLoaded, refreshKey]);

  const queueGraphWrite = useCallback((operation: () => Promise<void>) => {
    const queued = graphWriteQueue.current.then(operation, operation);
    graphWriteQueue.current = queued.catch(() => undefined);
    return queued;
  }, []);

  useEffect(() => {
    if (!document || !bridge?.session || !catalog.length || loading) return;
    const installId = bridge.session.activeInstallId ?? bridge.session.baseUrl ?? "default";
    const syncKey = `${installId}:${refreshKey}:${catalog.map((item) => item.apiName).join("|")}`;
    if (modelSyncKeyRef.current === syncKey) return;
    modelSyncKeyRef.current = syncKey;
    let cancelled = false;
    setModelSyncing(true);
    void queueGraphWrite(async () => {
      const describeEntries = await Promise.all(catalog.map(async (object) => {
        const cached = describeCache.getObject(installId, object.apiName);
        if (cached) return [object.apiName, cached] as const;
        try {
          const raw = await fetchFn(`/client/v1/describe/${encodeURIComponent(object.apiName)}`);
          const normalized = normalizeDescribeObject(raw, object.apiName);
          describeCache.setObject(installId, object.apiName, normalized);
          return [object.apiName, normalized] as const;
        } catch {
          return null;
        }
      }));
      if (cancelled) return;
      const current = await getHomeRunGraph(fetchFn);
      const merged = mergeAccessibleObjectModel(
        current.document,
        catalog,
        new Map(describeEntries.filter((entry): entry is NonNullable<typeof entry> => Boolean(entry))),
      );
      const saved = merged.changed
        ? await putRunGraph(fetchFn, "home", merged.document, current.revision)
        : current;
      if (cancelled) return;
      setDocument(saved.document);
      revisionRef.current = saved.revision;
      if (merged.addedObjects > 0) {
        setNotice(
          `Graph ready with ${catalog.length} accessible object${catalog.length === 1 ? "" : "s"}`
          + (merged.relationshipCount ? ` and ${merged.relationshipCount} model relationship${merged.relationshipCount === 1 ? "" : "s"}.` : "."),
        );
      }
    }).catch((reason: unknown) => {
      if (!cancelled) setError(reason instanceof Error ? reason.message : String(reason));
    }).finally(() => {
      if (!cancelled) setModelSyncing(false);
    });
    return () => {
      cancelled = true;
    };
  }, [bridge?.session, catalog, document, fetchFn, loading, queueGraphWrite, refreshKey]);

  const addNote = async () => {
    const text = noteText.trim();
    if (!text) return;
    setBusyAction("add-note"); setError(null); setNotice(null);
    try {
      const focusedId = selectedNodeIds[selectedNodeIds.length - 1];
      const result = await executeGraphBridge(
        "graph.annotate",
        { kind: annotationKind, text, ...(focusedId ? { linkToNodeId: focusedId } : {}) },
        { fetch: fetchFn },
      );
      if (!result.ok) throw new Error(String(result.error));
      await reload();
      setLens("all");
      setNoteText("");
      setComposerOpen(false);
      setSelectedEdgeId(null);
      setSelectedNodeIds([String(result.nodeId)]);
      pulse(String(result.nodeId));
      setNotice(`${annotationKind === "question" ? "Question" : "Note"} added${focusedId ? " to this work" : " to My graph"}.`);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally { setBusyAction(null); }
  };

  const pulse = useCallback((nodeId: string) => {
    setPulseNodeId(null);
    window.requestAnimationFrame(() => setPulseNodeId(nodeId));
    window.setTimeout(() => setPulseNodeId((current) => current === nodeId ? null : current), 700);
  }, []);

  const mountToolOnGraph = useCallback(async (
    tool: RunGraphMountableTool,
    position?: { x: number; y: number },
    targetNodeId?: string,
  ) => {
    const existed = document?.nodes.some((node) => node.kind === "tool" && (
      tool.kind === "published"
        ? node.toolRef?.toolSpecApiName === tool.toolSpecApiName
        : node.toolRef?.workingToolId === tool.workingToolId
    ));
    setBusyAction("mount-tool"); setError(null); setNotice(null);
    try {
      let nodeId = "";
      await queueGraphWrite(async () => {
        const input = tool.kind === "published"
          ? { toolSpecApiName: tool.toolSpecApiName }
          : { workingToolId: tool.workingToolId };
        const result = await executeGraphBridge("graph.mountTool", input, { fetch: fetchFn });
        if (!result.ok) throw new Error(String(result.error));
        nodeId = String(result.nodeId);
        if (position) {
          const layout = await executeGraphBridge(
            "graph.layout",
            { positions: { [nodeId]: position } },
            { fetch: fetchFn },
          );
          if (!layout.ok) throw new Error(String(layout.error));
        }
        if (targetNodeId && targetNodeId !== nodeId) {
          const linked = await executeGraphBridge(
            "graph.link",
            { from: nodeId, to: targetNodeId, kind: "opens" },
            { fetch: fetchFn },
          );
          if (!linked.ok) throw new Error(String(linked.error));
        }
        await reload();
      });
      setLens("all");
      setSelectedEdgeId(null);
      setSelectedNodeIds([nodeId]);
      pulse(nodeId);
      setNotice(existed ? `${tool.label} is already on your graph.` : `${tool.label} added to your graph.`);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setBusyAction(null);
    }
  }, [document?.nodes, fetchFn, pulse, queueGraphWrite, reload]);

  useEffect(() => {
    if (!mountRequest || mountRequestEpochRef.current === mountRequest.epoch) return;
    mountRequestEpochRef.current = mountRequest.epoch;
    const tool = mountableTools.find((candidate) => candidate.id === mountRequest.toolId);
    if (tool) void mountToolOnGraph(tool);
  }, [mountRequest, mountToolOnGraph, mountableTools]);

  const compact = async () => {
    setBusyAction("compact"); setError(null); setNotice(null);
    try {
      let removed = 0;
      await queueGraphWrite(async () => {
        const current = await getHomeRunGraph(fetchFn);
        const refs = current.document.nodes.flatMap((node) =>
          node.kind === "record" && node.ref?.objectApiName && node.ref.recordId
            ? [{ nodeId: node.id, objectApiName: node.ref.objectApiName, recordId: node.ref.recordId }]
            : [],
        );
        const resolveResults = await resolveRunGraphCards(fetchFn, refs);
        const stale = new Set(resolveResults.flatMap((result) =>
          !result.ok && (result.code === "NOT_FOUND" || result.code === "FORBIDDEN") ? [result.nodeId] : [],
        ));
        const tidy = tidyAttentionDocument(current.document, stale);
        removed = tidy.removed;
        if (!removed) return;
        const saved = await putRunGraph(fetchFn, "home", tidy.document, current.revision);
        setDocument(saved.document);
        revisionRef.current = saved.revision;
        setSelectedNodeIds((ids) => ids.filter((id) => saved.document.nodes.some((node) => node.id === id)));
      });
      setNotice(removed ? `Tidied ${removed} redundant or unavailable pin${removed === 1 ? "" : "s"}.` : "Your graph is already tidy.");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally { setBusyAction(null); }
  };

  const tidyLayout = async () => {
    if (!document?.nodes.length) return;
    setBusyAction("tidy-layout"); setError(null); setNotice(null);
    try {
      const result = await executeGraphBridge(
        "graph.layout",
        { positions: tidyPositions(document.nodes) },
        { fetch: fetchFn },
      );
      if (!result.ok) throw new Error(String(result.error));
      await reload();
      setNotice("Layout tidied into objects, Tools, and active work.");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setBusyAction(null);
    }
  };

  const publish = async () => {
    if (!document) return;
    setBusyAction("publish"); setError(null); setNotice(null);
    try {
      const result = await publishRunGraphSubgraph(
        fetchFn, document, selectedNodeIds, publishApiName.trim(), publishLabel.trim() || publishApiName.trim(),
      );
      setNotice(`Published ${result.nodeCount} node(s) as ${result.apiName}.`);
      onPublishedTool?.(result.apiName);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally { setBusyAction(null); }
  };

  const persistViewport = async (viewport: { x: number; y: number; zoom: number }) => {
    if (!document || revisionRef.current === null) return;
    await queueGraphWrite(async () => {
      const expectedRevision = revisionRef.current;
      if (expectedRevision === null) return;
      try {
        const graph = await patchRunGraph(fetchFn, "home", { viewport }, expectedRevision);
        revisionRef.current = graph.revision;
        setDocument(graph.document);
      } catch (reason) {
        setError(reason instanceof Error ? reason.message : "Viewport save conflicted; graph reloaded");
        await reload().catch(() => undefined);
      }
    });
  };

  const linkNodes = async (sourceId: string, targetId: string) => {
    setError(null);
    setNotice(null);
    try {
      await queueGraphWrite(async () => {
        const result = await executeGraphBridge(
          "graph.link",
          { from: sourceId, to: targetId, kind: wireKind },
          { fetch: fetchFn },
        );
        if (!result.ok) throw new Error(String(result.error));
        await reload();
        setSelectedNodeIds([]);
        setSelectedEdgeId(String(result.edgeId));
        setConnectSourceId(null);
      });
      const source = document?.nodes.find((node) => node.id === sourceId);
      const target = document?.nodes.find((node) => node.id === targetId);
      const sourceTitle = source ? runGraphNodeLabel(source, resolved[sourceId], toolLabels[sourceId]).title : "item";
      const targetTitle = target ? runGraphNodeLabel(target, resolved[targetId], toolLabels[targetId]).title : "item";
      setNotice(`Connected ${sourceTitle} to ${targetTitle}.`);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
      setConnectSourceId(null);
    }
  };

  const positionNode = async (nodeId: string, position: { x: number; y: number }) => {
    setError(null);
    try {
      await queueGraphWrite(async () => {
        const result = await executeGraphBridge(
          "graph.layout",
          { positions: { [nodeId]: position } },
          { fetch: fetchFn },
        );
        if (!result.ok) throw new Error(String(result.error));
        await reload();
      });
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    }
  };

  const unlinkEdge = async (edgeId: string) => {
    setError(null);
    setNotice(null);
    try {
      await queueGraphWrite(async () => {
        const result = await executeGraphBridge("graph.unlink", { edgeId }, { fetch: fetchFn });
        if (!result.ok) throw new Error(String(result.error));
        setSelectedEdgeId(null);
        await reload();
      });
      setNotice("Wire removed.");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    }
  };

  const markMyDayItemDone = async (item: MyDayQueueItem) => {
    setQueueBusy({ action: "done", nodeId: item.node.id });
    setError(null);
    setNotice(null);
    try {
      await queueGraphWrite(async () => {
        const current = await getHomeRunGraph(fetchFn);
        const edges = markMyDayItemDoneEdges(current.document, item.node.id);
        if (edges.length === current.document.edges.length) return;
        const saved = await patchRunGraph(fetchFn, "home", { edges }, current.revision);
        setDocument(saved.document);
        revisionRef.current = saved.revision;
      });
      setNotice("Marked done by removing next, blocks, watches, and My day membership wires.");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
      await reload().catch(() => undefined);
    } finally {
      setQueueBusy(null);
    }
  };

  const promoteMyDayItem = async (item: MyDayQueueItem) => {
    setQueueBusy({ action: "promote", nodeId: item.node.id });
    setError(null);
    setNotice(null);
    try {
      await queueGraphWrite(async () => {
        const current = await getHomeRunGraph(fetchFn);
        const currentItem = rankMyDayQueue(current.document)
          .find((candidate) => candidate.node.id === item.node.id);
        if (!currentItem) throw new Error("Work item is no longer in My day.");
        const edges = promoteMyDayItemEdges(current.document, currentItem);
        const saved = await patchRunGraph(fetchFn, "home", { edges }, current.revision);
        setDocument(saved.document);
        revisionRef.current = saved.revision;
      });
      setNotice("Promoted to next through graph topology.");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
      await reload().catch(() => undefined);
    } finally {
      setQueueBusy(null);
    }
  };

  const openMyDayItemFocus = (nodeId: string) => {
    setSelectedEdgeId(null);
    setSelectedNodeIds([nodeId]);
  };

  const changeEdgeKind = async (edge: RunGraphEdge, kind: RunGraphEdgeKind) => {
    if (edge.kind === kind) return;
    setError(null);
    setNotice(null);
    try {
      await queueGraphWrite(async () => {
        const linked = await executeGraphBridge(
          "graph.link",
          { from: edge.from, to: edge.to, kind },
          { fetch: fetchFn },
        );
        if (!linked.ok) throw new Error(String(linked.error));
        const unlinked = await executeGraphBridge("graph.unlink", { edgeId: edge.id }, { fetch: fetchFn });
        if (!unlinked.ok) throw new Error(String(unlinked.error));
        await reload();
        setSelectedEdgeId(String(linked.edgeId));
      });
      setNotice(`Connection changed to ${runGraphEdgeLabel(kind)}.`);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    }
  };

  const updateAnnotation = async (nodeId: string, text: string) => {
    await queueGraphWrite(async () => {
      const current = await getHomeRunGraph(fetchFn);
      const nextDocument: RunGraphDocument = {
        ...current.document,
        nodes: current.document.nodes.map((node) =>
          node.id === nodeId && (node.kind === "insight" || node.kind === "question")
            ? { ...node, text }
            : node,
        ),
      };
      const saved = await putRunGraph(fetchFn, "home", nextDocument, current.revision);
      setDocument(saved.document);
      revisionRef.current = saved.revision;
    });
    setNotice("Annotation updated.");
  };

  const resolveProposal = async (
    node: RunGraphNode,
    status: "applied" | "rejected",
  ) => {
    if (node.kind !== "proposal" || !node.proposalId) return;
    setError(null);
    setNotice(null);
    await queueGraphWrite(async () => {
      const current = await getHomeRunGraph(fetchFn);
      const saved = await patchRunGraph(
        fetchFn,
        "home",
        {
          nodes: current.document.nodes.filter((candidate) => candidate.id !== node.id),
          edges: current.document.edges.filter(
            (edge) => edge.from !== node.id && edge.to !== node.id,
          ),
        },
        current.revision,
      );
      proposalStore?.drop(node.proposalId!);
      cache.current.clear();
      setResolved({});
      setDocument(saved.document);
      revisionRef.current = saved.revision;
      setSelectedNodeIds([]);
      onProposalResolved?.(status, proposalStore?.size ?? 0);
    });
    setNotice(status === "applied" ? "Proposal applied through Client." : "Proposal rejected.");
  };

  const refreshRecordCard = useCallback(async (node: RunGraphNode) => {
    if (node.kind !== "record" || !node.ref?.objectApiName || !node.ref.recordId) return;
    try {
      const [result] = await resolveRunGraphCards(fetchFn, [{
        nodeId: node.id,
        objectApiName: node.ref.objectApiName,
        recordId: node.ref.recordId,
      }]);
      if (!result) return;
      cache.current.set(node.ref.objectApiName, node.ref.recordId, result);
      setResolved((current) => ({ ...current, [node.id]: result }));
    } catch {
      cache.current.clear();
    }
  }, [fetchFn]);

  const pinSignalSurvivors = async (signal: RunGraphSignalResult): Promise<number> => {
    const recordIds = [...new Set(signal.rows
      .map((row) => String(row.id ?? row.Id ?? "").trim())
      .filter(Boolean))];
    if (!recordIds.length) return 0;
    setError(null);
    setNotice(null);
    await queueGraphWrite(async () => {
      let ensured = 0;
      try {
        for (const recordId of recordIds) {
          const result = await executeGraphBridge(
            "graph.pin",
            { objectApiName: signal.objectApiName, recordId },
            { fetch: fetchFn },
          );
          if (!result.ok) throw new Error(String(result.error));
          ensured += 1;
        }
      } catch (reason) {
        throw new Error(
          `Ensured ${ensured} of ${recordIds.length} survivor pins before failure: ${
            reason instanceof Error ? reason.message : String(reason)
          }`,
          { cause: reason },
        );
      }
      await reload();
    });
    setNotice(`Ensured ${recordIds.length} live signal survivor pin${recordIds.length === 1 ? "" : "s"}.`);
    return recordIds.length;
  };

  const landSearchHit = async (hit: SearchHit) => {
    setError(null); setNotice(null);
    try {
      let collectionId = "";
      await queueGraphWrite(async () => {
        const result = await executeGraphBridge(
          "graph.pinCollection",
          {
            objectApiName: hit.object,
            label: catalog.find((object) => object.apiName === hit.object)?.pluralLabel
              || catalog.find((object) => object.apiName === hit.object)?.label
              || hit.object,
          },
          { fetch: fetchFn },
        );
        if (!result.ok) throw new Error(String(result.error));
        collectionId = String(result.nodeId);
        await reload();
      });
      setLens("all");
      setSelectedEdgeId(null);
      setSelectedNodeIds([collectionId]);
      setSearchLanding((current) => ({
        nodeId: collectionId,
        recordId: hit.id,
        epoch: (current?.epoch ?? 0) + 1,
      }));
      pulse(collectionId);
      setNotice(`Opened ${hit.object} in your graph.`);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    }
  };

  const pinSearchHit = async (hit: SearchHit) => {
    setError(null); setNotice(null);
    try {
      let collectionId = "";
      let recordNodeId = "";
      await queueGraphWrite(async () => {
        const collection = await executeGraphBridge(
          "graph.pinCollection",
          { objectApiName: hit.object, label: catalog.find((object) => object.apiName === hit.object)?.label || hit.object },
          { fetch: fetchFn },
        );
        if (!collection.ok) throw new Error(String(collection.error));
        collectionId = String(collection.nodeId);
        const pin = await executeGraphBridge(
          "graph.pin",
          { objectApiName: hit.object, recordId: hit.id, collectionId },
          { fetch: fetchFn },
        );
        if (!pin.ok) throw new Error(String(pin.error));
        recordNodeId = String(pin.nodeId);
        await reload();
      });
      setLens("all");
      setSelectedEdgeId(null);
      setSelectedNodeIds([recordNodeId]);
      pulse(recordNodeId);
      setNotice(`${hit.title || hit.object} pinned to your graph.`);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    }
  };

  const focusSearch = () => {
    setComposerOpen(false);
    window.requestAnimationFrame(() => {
      globalThis.document.querySelector<HTMLInputElement>('[data-testid="operate-global-search"]')?.focus();
    });
  };

  const toolLabels = useMemo(() => {
    if (!document) return {};
    return Object.fromEntries(document.nodes.flatMap((node) => {
      if (node.kind !== "tool") return [];
      const tool = mountableTools.find((candidate) =>
        candidate.kind === "published"
          ? candidate.toolSpecApiName === node.toolRef?.toolSpecApiName
          : candidate.workingToolId === node.toolRef?.workingToolId,
      );
      return tool ? [[node.id, tool.label]] : [];
    }));
  }, [document, mountableTools]);

  const graphMatches = useMemo(() => document?.nodes.map((node) => {
    const label = runGraphNodeLabel(node, resolved[node.id], toolLabels[node.id]);
    return { nodeId: node.id, title: label.title, detail: label.detail || node.kind };
  }) ?? [], [document, resolved, toolLabels]);

  const orphanCount = useMemo(
    () => document ? orphanDerivedRecordIds(document).size : 0,
    [document],
  );

  const focusedNode = selectedNodeIds.length > 0 && document
    ? document.nodes.find((node) => node.id === selectedNodeIds[selectedNodeIds.length - 1])
    : undefined;
  const selectedEdge = selectedEdgeId && document
    ? document.edges.find((edge) => edge.id === selectedEdgeId)
    : undefined;
  const selectedEdgeFromNode = selectedEdge
    ? document?.nodes.find((node) => node.id === selectedEdge.from)
    : undefined;
  const selectedEdgeToNode = selectedEdge
    ? document?.nodes.find((node) => node.id === selectedEdge.to)
    : undefined;
  const linkedTool = focusedNode && document
    ? toolForOpenedNode(document, focusedNode.id)
    : undefined;

  const openGraphRecord = async (objectApiName: string, recordId: string) => {
    const target = document?.nodes.find(
      (node) => node.kind === "record" && node.ref?.objectApiName === objectApiName && node.ref.recordId === recordId,
    );
    let nodeId = target?.id ?? "";
    if (!nodeId) {
      const result = await executeGraphBridge(
        "graph.pin",
        { objectApiName, recordId },
        { fetch: fetchFn },
      );
      if (!result.ok) {
        setError(String(result.error));
        return;
      }
      nodeId = String(result.nodeId);
      await reload();
      setNotice(`${objectApiName} record added to this graph.`);
    }
    setSelectedEdgeId(null);
    setSelectedNodeIds([nodeId]);
    pulse(nodeId);
  };

  const focusedTool = focusedNode?.kind === "tool"
    ? mountableTools.find((candidate) =>
        candidate.kind === "published"
          ? candidate.toolSpecApiName === focusedNode.toolRef?.toolSpecApiName
          : candidate.workingToolId === focusedNode.toolRef?.workingToolId,
      )
    : undefined;

  return (
    <div className="panel run-graph-home" data-testid="run-graph-home">
      <section className="run-graph-command" aria-label="My graph command bar">
        <h2 className="visually-hidden">{document?.title || "My graph"}</h2>
        <OperateSearch
          fetchFn={fetchFn}
          onOpenHit={(hit) => void landSearchHit(hit)}
          onPinHit={(hit) => void pinSearchHit(hit)}
          graphMatches={graphMatches}
          onOpenGraphMatch={(match) => {
            setLens("all");
            setSelectedEdgeId(null);
            setSelectedNodeIds([match.nodeId]);
            pulse(match.nodeId);
          }}
          isOnGraph={(hit) => Boolean(document?.nodes.some((node) =>
            node.kind === "record" && node.ref?.objectApiName === hit.object && node.ref.recordId === hit.id,
          ))}
        />
        <div className="run-graph-command-actions">
          <label className="run-graph-view-picker">
            <span>View</span>
            <select
              aria-label="Graph view"
              value={lens}
              onChange={(event) => {
                setLens(event.target.value as RunGraphLensId);
                setSelectedNodeIds([]);
                setSelectedEdgeId(null);
                setConnectSourceId(null);
              }}
            >
              {RUN_GRAPH_LENSES.map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
            </select>
          </label>
          <button type="button" className="secondary" onClick={() => setComposerOpen((current) => !current)}>
            + Add
          </button>
          <button
            type="button"
            className="secondary"
            disabled={!document?.nodes.length || busyAction !== null}
            onClick={() => void tidyLayout()}
          >
            {busyAction === "tidy-layout" ? "Tidying…" : "Tidy layout"}
          </button>
          {staleCount > 0 || orphanCount > 0 ? (
            <button
              type="button"
              className="secondary"
              disabled={busyAction !== null}
              onClick={() => void compact()}
            >
              {busyAction === "compact" ? "Tidying…" : `Tidy graph (${staleCount + orphanCount})`}
            </button>
          ) : null}
          <span className="run-graph-count" title="Personal graph node count">
            {modelSyncing ? "Syncing objects…" : `${document?.nodes.length ?? 0}/200`}
          </span>
        </div>
        {composerOpen ? (
          <div className="run-graph-composer" data-testid="run-graph-composer">
            <div className="run-graph-composer-kinds" role="group" aria-label="Annotation type">
              <button
                type="button"
                className={annotationKind === "insight" ? "active" : ""}
                aria-pressed={annotationKind === "insight"}
                onClick={() => setAnnotationKind("insight")}
              >
                Note
              </button>
              <button
                type="button"
                className={annotationKind === "question" ? "active" : ""}
                aria-pressed={annotationKind === "question"}
                onClick={() => setAnnotationKind("question")}
              >
                Question
              </button>
            </div>
            <textarea
              autoFocus
              aria-label="Note text"
              disabled={busyAction !== null}
              maxLength={4096}
              rows={3}
              placeholder={focusedNode ? "Add context to the selected node…" : "Capture a thought on your graph…"}
              value={noteText}
              onChange={(event) => setNoteText(event.target.value)}
              onKeyDown={(event) => {
                if ((event.metaKey || event.ctrlKey) && event.key === "Enter" && noteText.trim()) {
                  event.preventDefault();
                  void addNote();
                }
              }}
            />
            <div className="run-graph-composer-actions">
              <span className="muted">
                {focusedNode ? `Links to ${runGraphNodeLabel(focusedNode, resolved[focusedNode.id], toolLabels[focusedNode.id]).title}` : "Adds to My graph"}
              </span>
              <Button variant="ghost" onClick={focusSearch}>Pin a record</Button>
              <Button
                variant="primary"
                busy={busyAction === "add-note"}
                disabled={busyAction !== null || !noteText.trim()}
                onClick={() => void addNote()}
              >
                Add {annotationKind === "question" ? "question" : "note"}
              </Button>
            </div>
          </div>
        ) : null}
      </section>
      {document && lens === "my-day" ? (
        <section className="run-graph-my-day" aria-label="My day work queue" data-testid="run-graph-my-day-queue">
          <header className="run-graph-my-day-header">
            <div>
              <h3>My day queue</h3>
              <p className="muted">Blocked first · next by weight · watching · My day pins</p>
            </div>
            <div className="run-graph-my-day-header-actions">
              <span className="run-graph-my-day-count">{myDayQueue.length}</span>
              {onAskRunAgent ? (
                <Button
                  variant="secondary"
                  data-testid="my-day-rebuild"
                  onClick={() => onAskRunAgent(MY_DAY_CURATOR_PROMPT)}
                >
                  Rebuild My day
                </Button>
              ) : null}
            </div>
          </header>
          {myDayQueue.length ? (
            <ol className="run-graph-my-day-list">
              {myDayQueue.map((item, index) => {
                const label = runGraphNodeLabel(item.node, resolved[item.node.id]);
                const doneEdgeIds = myDayDoneEdgeIds(document, item.node.id);
                const canPromote = Boolean(myDayPromotionEndpoints(item));
                const busy = queueBusy?.nodeId === item.node.id;
                return (
                  <li key={item.node.id} className="run-graph-my-day-item">
                    <span className="run-graph-my-day-rank">{index + 1}</span>
                    <div className="run-graph-my-day-copy">
                      <span className={`run-graph-my-day-reason is-${item.reason}`}>
                        {item.reason === "my-day" ? "My day" : item.reason}
                      </span>
                      <strong>{label.title}</strong>
                      <small>
                        {label.detail || item.node.kind}
                        {item.reason === "next" && item.weight !== undefined ? ` · weight ${item.weight}` : ""}
                      </small>
                    </div>
                    <div className="run-graph-my-day-actions">
                      <Button
                        variant="ghost"
                        busy={busy && queueBusy?.action === "done"}
                        disabled={queueBusy !== null || doneEdgeIds.length === 0}
                        aria-label={`Mark ${label.title} done`}
                        onClick={() => void markMyDayItemDone(item)}
                      >
                        Mark done
                      </Button>
                      <Button
                        variant="ghost"
                        busy={busy && queueBusy?.action === "promote"}
                        disabled={queueBusy !== null || !canPromote}
                        aria-label={`Promote ${label.title} next`}
                        onClick={() => void promoteMyDayItem(item)}
                      >
                        Promote next
                      </Button>
                      <Button
                        variant="secondary"
                        aria-label={`Open ${label.title} focus`}
                        onClick={() => openMyDayItemFocus(item.node.id)}
                      >
                        Open focus
                      </Button>
                    </div>
                  </li>
                );
              })}
            </ol>
          ) : (
            <div className="run-graph-my-day-empty">
              <div>
                <strong>Nothing is queued yet</strong>
                <p className="muted">Pin work from Object Home or ask a Run curator to wire today with graph edges.</p>
              </div>
              <div className="row">
                {onOpenObjectHome ? (
                  <Button variant="primary" data-testid="my-day-open-object-home" onClick={onOpenObjectHome}>
                    Pin from Object Home
                  </Button>
                ) : null}
                {onAskRunAgent ? (
                  <Button
                    variant="secondary"
                    data-testid="my-day-ask-curator"
                    onClick={() => onAskRunAgent(MY_DAY_CURATOR_PROMPT)}
                  >
                    Ask Run curator
                  </Button>
                ) : null}
              </div>
            </div>
          )}
        </section>
      ) : null}
      {notice ? <p className="run-graph-notice" role="status">{notice}</p> : null}
      {error && document ? <p className="error run-graph-error" role="alert">{error}</p> : null}
      <div className="panel-body run-graph-body">
        {loading ? (
          <p className="muted" data-testid="run-graph-loading"><Spinner /> Loading personal graph…</p>
        ) : error && !document ? (
          <p className="error" data-testid="run-graph-error">{error}</p>
        ) : view.nodes.length === 0 ? (
          <EmptyState
            title={lens === "all" ? "Your graph is ready" : `No nodes in ${RUN_GRAPH_LENSES.find((item) => item.id === lens)?.label}`}
            description={modelSyncing
              ? "Adding the objects you can access and wiring their data-model relationships…"
              : "Find a record above, add a note, or drag a Tool here. Accessible objects appear automatically when connected."}
            action={
              <div className="row">
                <Button
                  variant="primary"
                  data-testid="run-graph-empty-add"
                  disabled={busyAction !== null}
                  onClick={() => setComposerOpen(true)}
                >
                  Add a note
                </Button>
                {onOpenObjectHome ? (
                  <Button variant="secondary" data-testid="run-graph-open-list-view" onClick={onOpenObjectHome}>
                    Open List View
                  </Button>
                ) : null}
              </div>
            }
          />
        ) : (
          <>
            <div className="run-graph-canvas-stage">
              <RunGraphView
                view={view}
                resolved={resolved}
                selectedNodeIds={selectedNodeIds}
                selectedEdgeId={selectedEdgeId}
                connectSourceId={connectSourceId}
                pulseNodeId={pulseNodeId}
                toolLabels={toolLabels}
                onSelectedNodeIdsChange={(ids) => {
                  setSelectedEdgeId(null);
                  setSelectedNodeIds(ids);
                  if (!ids.length) setConnectSourceId(null);
                }}
                onSelectedEdgeIdChange={(id) => {
                  setSelectedEdgeId(id);
                  if (id) setConnectSourceId(null);
                }}
                onStartConnect={(nodeId) => {
                  setSelectedEdgeId(null);
                  setSelectedNodeIds([nodeId]);
                  setConnectSourceId((current) => current === nodeId ? null : nodeId);
                }}
                onConnectNodes={(sourceId, targetId) => void linkNodes(sourceId, targetId)}
                onNodePositionChange={(nodeId, position) => void positionNode(nodeId, position)}
                onDropTool={(payload: OperateToolDragPayload, position, targetNodeId) => {
                  const tool = mountableTools.find((candidate) => candidate.id === payload.railId);
                  if (tool) void mountToolOnGraph(tool, position, targetNodeId);
                }}
                viewport={document?.viewport}
                onViewportChange={(next) => void persistViewport(next)}
              />
              {connectSourceId ? (
                <div className="run-graph-connect-hint" role="status">
                  Click another node to connect
                  <label>
                    as
                    <select
                      aria-label="Connection type"
                      value={wireKind}
                      onChange={(event) => setWireKind(event.target.value as RunGraphEdgeKind)}
                    >
                      {RUN_GRAPH_EDGE_KINDS.map((kind) => <option key={kind} value={kind}>{runGraphEdgeLabel(kind)}</option>)}
                    </select>
                  </label>
                  <button type="button" onClick={() => setConnectSourceId(null)}>Cancel</button>
                </div>
              ) : null}
              {selectedNodeIds.length > 0 ? (
                <div className="run-graph-context-bar" aria-label="Selected graph actions">
                  <button
                    type="button"
                    onClick={() => {
                      const sourceId = selectedNodeIds[selectedNodeIds.length - 1];
                      setConnectSourceId((current) => current === sourceId ? null : sourceId);
                    }}
                  >
                    Connect
                  </button>
                  <button type="button" onClick={() => setComposerOpen(true)}>Add context</button>
                  <details>
                    <summary>Share as Tool</summary>
                    <div className="run-graph-publish-popover">
                      <input aria-label="Published Tool API name" placeholder="Tool_API_Name" value={publishApiName} onChange={(event) => setPublishApiName(event.target.value)} />
                      <input aria-label="Published Tool label" placeholder="Tool label" value={publishLabel} onChange={(event) => setPublishLabel(event.target.value)} />
                      <Button variant="secondary" busy={busyAction === "publish"} disabled={!publishApiName.trim()} onClick={() => void publish()}>
                        Publish {selectedNodeIds.length} selected
                      </Button>
                    </div>
                  </details>
                </div>
              ) : null}
            </div>
            {focusedNode ? (
              <RunGraphFocusPanel
                node={focusedNode}
                resolve={resolved[focusedNode.id]}
                fetchFn={fetchFn}
                bridge={bridge}
                onClose={() => setSelectedNodeIds([])}
                onOpenTool={onOpenTool}
                linkedTool={linkedTool}
                onOpenRecord={openGraphRecord}
                onRecordSaved={(node) => void refreshRecordCard(node)}
                onUpdateAnnotation={updateAnnotation}
                proposalStore={proposalStore}
                onResolveProposal={(status) => resolveProposal(focusedNode, status)}
                signalResult={signalResults[focusedNode.id]}
                signalError={
                  signalErrors[focusedNode.id] ??
                  (focusedNode.kind === "signal" &&
                  !document?.dataBindings?.some((binding) => binding.id === focusedNode.bindingId)
                    ? `Signal binding ${focusedNode.bindingId ?? "(missing)"} is unavailable.`
                    : undefined)
                }
                onPinSignalRows={pinSignalSurvivors}
                collectionBinding={
                  focusedNode.kind === "collection" && focusedNode.bindingId
                    ? document?.dataBindings?.find((binding) => binding.id === focusedNode.bindingId)
                    : undefined
                }
                selectedRecordId={searchLanding?.nodeId === focusedNode.id ? searchLanding.recordId : undefined}
                selectedRecordEpoch={searchLanding?.nodeId === focusedNode.id ? searchLanding.epoch : 0}
                toolLabel={focusedTool?.label}
                onAskAgent={onAskRunAgent}
                onPinnedFromCollection={(nodeId) => {
                  void reload().then(() => {
                    if (nodeId) {
                      setSelectedNodeIds([nodeId]);
                      pulse(nodeId);
                    }
                  });
                }}
              />
            ) : selectedEdge ? (
              <aside className="run-graph-focus crm-detail" data-testid="run-graph-edge-focus" aria-label="Connection details">
                <header className="run-graph-focus-header">
                  <div>
                    <p className="run-graph-focus-kicker">Connection</p>
                    <h3>
                      <select
                        aria-label="Connection type"
                        value={selectedEdge.kind}
                        onChange={(event) => void changeEdgeKind(selectedEdge, event.target.value as RunGraphEdgeKind)}
                      >
                        {RUN_GRAPH_EDGE_KINDS.map((kind) => <option key={kind} value={kind}>{runGraphEdgeLabel(kind)}</option>)}
                      </select>
                    </h3>
                  </div>
                  <button type="button" className="icon-btn" aria-label="Close connection details" onClick={() => setSelectedEdgeId(null)}>×</button>
                </header>
                <dl className="run-graph-edge-detail">
                  <div>
                    <dt>From</dt>
                    <dd>{selectedEdgeFromNode ? runGraphNodeLabel(selectedEdgeFromNode, resolved[selectedEdge.from], toolLabels[selectedEdge.from]).title : "Unknown item"}</dd>
                  </div>
                  <div>
                    <dt>To</dt>
                    <dd>{selectedEdgeToNode ? runGraphNodeLabel(selectedEdgeToNode, resolved[selectedEdge.to], toolLabels[selectedEdge.to]).title : "Unknown item"}</dd>
                  </div>
                </dl>
                <Button variant="danger" onClick={() => void unlinkEdge(selectedEdge.id)}>Remove connection</Button>
              </aside>
            ) : null}
          </>
        )}
      </div>
    </div>
  );
}
