import { useEffect, useMemo, useState } from "react";
import type { AppBridge } from "../App";
import { Button, EmptyState, PanelHeader, SearchField, Spinner, ToolSurface, ToolToolbar } from "../ui";
import { IconExplorer } from "../icons/Icons";
import {
  describeCache,
  normalizeDescribeObject,
  normalizeGlobalObjects,
  type GlobalDescribeObject,
} from "./describeCache";
import { layoutObjectGraph, type GraphEdge, type GraphNode, type ObjectGraph } from "./objectGraph";
import type { DescribeObject } from "./types";
import {
  EXPLORER_ALL_PACKAGES,
  catalogDescribes,
  explorerPackageOptions,
  mergeDescribes,
  mergeExplorerObjects,
  normalizePackageCatalog,
  type PackageCatalogRow,
} from "./explorerCatalog";

const DESCRIBE_BATCH = 12;

export function ExplorerPanel({
  bridge,
  refreshKey = 0,
  onOpenInQuery,
  onAskAgent,
}: {
  bridge: AppBridge;
  refreshKey?: number;
  /** Optional hook — parent can open Query with this object. */
  onOpenInQuery?: (objectApiName: string) => void;
  /** Route a prompt about the hovered object into the primary Operate chat. */
  onAskAgent?: (prompt: string) => void;
}) {
  const installId = bridge.session?.activeInstallId ?? bridge.session?.baseUrl ?? "default";
  const connected = Boolean(bridge.session?.token && bridge.session?.baseUrl);

  const [objects, setObjects] = useState<GlobalDescribeObject[]>([]);
  const [catalog, setCatalog] = useState<PackageCatalogRow[]>([]);
  const [describes, setDescribes] = useState<Map<string, DescribeObject>>(new Map());
  const [search, setSearch] = useState("");
  const [packageFilter, setPackageFilter] = useState<string>("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [hoverNode, setHoverNode] = useState<GraphNode | null>(null);
  const [hoverEdge, setHoverEdge] = useState<GraphEdge | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [scale, setScale] = useState(1);

  useEffect(() => {
    describeCache.invalidateInstall(installId);
    setDescribes(new Map());
    setObjects([]);
    setCatalog([]);
  }, [installId, refreshKey]);

  useEffect(() => {
    if (!connected) return;
    let cancelled = false;
    void (async () => {
      setBusy(true);
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

        try {
          const rawPackages = await bridge.fetch("/metadata/v1/packages");
          if (!cancelled) setCatalog(normalizePackageCatalog(rawPackages));
        } catch {
          if (!cancelled) setCatalog([]);
        }

        const next = new Map<string, DescribeObject>();
        const batch = list.slice(0, DESCRIBE_BATCH);
        await Promise.all(
          batch.map(async (o) => {
            let desc = describeCache.getObject(installId, o.apiName);
            if (!desc) {
              try {
                const raw = await bridge.fetch(`/client/v1/describe/${encodeURIComponent(o.apiName)}`);
                desc = normalizeDescribeObject(raw, o.apiName);
                describeCache.setObject(installId, o.apiName, desc);
              } catch {
                desc = { apiName: o.apiName, fields: [] };
              }
            }
            next.set(o.apiName, desc);
          }),
        );
        if (!cancelled) setDescribes(next);
      } catch (e) {
        if (!cancelled) setErr(String(e));
      } finally {
        if (!cancelled) setBusy(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [bridge, connected, installId, refreshKey]);

  const packageOptions = useMemo(
    () => explorerPackageOptions(objects, catalog),
    [objects, catalog],
  );

  const graphObjects = useMemo(
    () => mergeExplorerObjects(objects, catalog, { packageFilter }),
    [objects, catalog, packageFilter],
  );

  const graphDescribes = useMemo(
    () => mergeDescribes(catalogDescribes(catalog), describes),
    [catalog, describes],
  );

  const graph: ObjectGraph = useMemo(() => {
    return layoutObjectGraph(graphObjects, graphDescribes, {
      search,
      maxNodes: 80,
    });
  }, [graphObjects, graphDescribes, search]);

  return (
    <ToolSurface className="operate-explorer-panel" testId="operate-explorer-panel">
      <PanelHeader
        title="Explorer"
        subtitle="Object relationship graph across packages. Use the package selector to include catalog objects that are not enabled on this install yet."
        actions={
          <>
            <Button variant="ghost" onClick={() => setScale((s) => Math.min(2, s + 0.1))}>
              Zoom in
            </Button>
            <Button variant="ghost" onClick={() => setScale((s) => Math.max(0.5, s - 0.1))}>
              Zoom out
            </Button>
            <Button variant="ghost" onClick={() => setScale(1)}>
              Fit
            </Button>
          </>
        }
      />

      {!connected ? (
        <EmptyState
          icon={<IconExplorer size={28} />}
          title="Connect to explore"
          description="Connect an environment to load the tenancy object graph from describe metadata."
        />
      ) : (
        <>
          <ToolToolbar
            actions={
              <>
                <label>
                  Package
                  <select
                    data-testid="explorer-package"
                    value={packageFilter}
                    onChange={(e) => setPackageFilter(e.target.value)}
                  >
                    <option value="">Enabled</option>
                    <option value={EXPLORER_ALL_PACKAGES}>All packages</option>
                    {packageOptions.map((p) => (
                      <option key={p.name} value={p.name}>
                        {p.enabled ? p.name : `${p.name} (not enabled)`}
                      </option>
                    ))}
                  </select>
                </label>
                {busy ? <Spinner /> : null}
                <span className="muted">
                  {graph.nodes.length} objects · {graph.edges.length} relationships
                </span>
              </>
            }
            search={
              <SearchField
                value={search}
                onChange={setSearch}
                placeholder="apiName or label"
                label="Search objects"
                testId="explorer-search"
              />
            }
          />

          {err ? <p className="err">{err}</p> : null}

          <div className="operate-explorer-canvas" data-testid="explorer-canvas">
            {graph.nodes.length === 0 ? (
              <EmptyState
                icon={<IconExplorer size={24} />}
                title="No objects in view"
                description="Adjust search/package filters or choose All packages to include catalog objects not enabled yet."
              />
            ) : (
              <svg
                className="operate-explorer-svg"
                viewBox={`0 0 ${graph.width} ${graph.height}`}
                width={graph.width * scale}
                height={graph.height * scale}
                role="img"
                aria-label="Object relationship graph"
              >
                {graph.edges.map((e) => {
                  const pts =
                    e.points.length > 1
                      ? e.points.map((p) => `${p.x},${p.y}`).join(" ")
                      : (() => {
                          const a = graph.nodes.find((n) => n.id === e.from);
                          const b = graph.nodes.find((n) => n.id === e.to);
                          if (!a || !b) return "";
                          return `${a.x},${a.y} ${b.x},${b.y}`;
                        })();
                  return (
                    <polyline
                      key={e.id}
                      className={`operate-explorer-edge ${hoverEdge?.id === e.id ? "is-hover" : ""}`}
                      points={pts}
                      fill="none"
                      data-testid="explorer-edge"
                      onMouseEnter={() => setHoverEdge(e)}
                      onMouseLeave={() => setHoverEdge(null)}
                    />
                  );
                })}
                {graph.nodes.map((n) => {
                  const x = n.x - n.width / 2;
                  const y = n.y - n.height / 2;
                  return (
                    <g
                      key={n.id}
                      className={`operate-explorer-node ${selected === n.id ? "is-selected" : ""} ${n.enabled ? "" : "is-catalog"}`}
                      transform={`translate(${x},${y})`}
                      data-testid={`explorer-node-${n.apiName}`}
                      onMouseEnter={() => setHoverNode(n)}
                      onMouseLeave={() => setHoverNode(null)}
                      onClick={() => setSelected(n.id)}
                    >
                      <rect width={n.width} height={n.height} rx={8} />
                      <text x={n.width / 2} y={n.height / 2 - 4} textAnchor="middle">
                        {n.label}
                      </text>
                      <text
                        className="operate-explorer-node-sub"
                        x={n.width / 2}
                        y={n.height / 2 + 12}
                        textAnchor="middle"
                      >
                        {n.apiName}
                      </text>
                    </g>
                  );
                })}
              </svg>
            )}

            {hoverNode ? (
              <div className="operate-explorer-popover" data-testid="explorer-node-popover">
                <strong>{hoverNode.label}</strong>
                <div className="mono">{hoverNode.apiName}</div>
                <div className="muted">Package: {hoverNode.packageName ?? "core"}</div>
                <div className="muted">Fields: {hoverNode.fieldCount}</div>
                {!hoverNode.enabled ? (
                  <div className="muted" data-testid="explorer-node-not-enabled">
                    Not enabled on this install
                  </div>
                ) : null}
                {onOpenInQuery && hoverNode.enabled ? (
                  <Button variant="secondary" onClick={() => onOpenInQuery(hoverNode.apiName)}>
                    Open in Query
                  </Button>
                ) : null}
                {onAskAgent ? (
                  <Button
                    variant="ghost"
                    data-testid={`explorer-ask-agent-${hoverNode.apiName}`}
                    onClick={() => onAskAgent(`Tell me about the ${hoverNode.label ?? hoverNode.apiName} object and its key fields.`)}
                  >
                    Ask agent
                  </Button>
                ) : null}
              </div>
            ) : null}
            {hoverEdge && !hoverNode ? (
              <div className="operate-explorer-popover" data-testid="explorer-edge-popover">
                <strong>{hoverEdge.fieldApiName}</strong>
                <div className="muted">
                  {hoverEdge.from} → {hoverEdge.to} ({hoverEdge.relationshipType})
                </div>
              </div>
            ) : null}
          </div>
        </>
      )}
    </ToolSurface>
  );
}
