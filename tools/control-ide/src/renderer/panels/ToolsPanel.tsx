import { useCallback, useEffect, useMemo, useState } from "react";
import type { AppBridge } from "../App";
import { mirrorToolSpecYaml } from "../metadataMirror";
import { sanitizeToolNodesForMetadata } from "../run/resolveBindings";
import type { ToolSpecPayload } from "../run/tools";
import { Button, PanelHeader, SearchField, StatusBadge, ToolSurface, ToolToolbar } from "../ui";
import { IconChevronLeft } from "../icons/Icons";
import { installWorkflowStarterPack, WORKFLOW_STARTER_PACKS } from "../run/starterPacks";

export type ToolSpec = ToolSpecPayload & {
  ownership?: string;
  active?: boolean;
  packageName?: string;
  layout?: unknown;
  nodes?: unknown[];
  dataBindings?: unknown[];
};

function sortSpecs(rows: ToolSpec[]): ToolSpec[] {
  return [...rows].sort((a, b) => {
    const la = (a.label || a.apiName).toLowerCase();
    const lb = (b.label || b.apiName).toLowerCase();
    return la.localeCompare(lb);
  });
}

const SEED_LAYOUT = `{
  "mode": "sections",
  "sections": [{ "id": "main", "title": "Main", "nodeIds": ["hdr"] }]
}`;

const SEED_NODES = `[
  {
    "id": "hdr",
    "kind": "sectionHeader",
    "title": "New tool",
    "props": { "subtitle": "Edit nodes after create" }
  }
]`;

export function ToolsPanel({
  bridge,
  refreshKey = 0,
  onToolsChanged,
  onStarterPackInstalled,
}: {
  bridge: AppBridge;
  refreshKey?: number;
  /** Notify shell to reload Run rail ToolSpecs. */
  onToolsChanged?: () => void;
  /** Refresh the agent dock after a pack creates its AgentSpec. */
  onStarterPackInstalled?: () => void;
}) {
  const [specs, setSpecs] = useState<ToolSpec[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [detail, setDetail] = useState<ToolSpec | null>(null);
  const [err, setErr] = useState("");
  const [warn, setWarn] = useState("");
  const [busy, setBusy] = useState(false);
  const [filter, setFilter] = useState("");
  const [showNew, setShowNew] = useState(false);
  const [packBusy, setPackBusy] = useState<string | null>(null);
  const [packNotice, setPackNotice] = useState("");
  const [form, setForm] = useState({
    apiName: "",
    label: "",
    description: "",
    icon: "",
    sortOrder: "0",
    layoutJson: SEED_LAYOUT,
    nodesJson: SEED_NODES,
    dataBindingsJson: "[]",
  });

  const loadList = useCallback(async () => {
    if (!bridge.session?.token) {
      setSpecs([]);
      return;
    }
    setBusy(true);
    setErr("");
    try {
      const row = (await bridge.fetch("/metadata/v1/tools")) as { tools?: ToolSpec[] };
      setSpecs(sortSpecs(row.tools ?? []));
    } catch (e) {
      setErr(String(e));
      setSpecs([]);
    } finally {
      setBusy(false);
    }
  }, [bridge]);

  const loadDetail = useCallback(
    async (apiName: string) => {
      setBusy(true);
      setErr("");
      try {
        const desc = (await bridge.fetch(
          `/metadata/v1/tools/${encodeURIComponent(apiName)}`,
        )) as ToolSpec;
        setDetail(desc);
        setSelected(apiName);
        setShowNew(false);
      } catch (e) {
        setErr(String(e));
        setDetail(null);
      } finally {
        setBusy(false);
      }
    },
    [bridge],
  );

  useEffect(() => {
    void loadList();
    setSelected(null);
    setDetail(null);
    setWarn("");
  }, [loadList, refreshKey]);

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return specs;
    return specs.filter(
      (s) =>
        s.apiName.toLowerCase().includes(q) ||
        (s.label ?? "").toLowerCase().includes(q) ||
        (s.description ?? "").toLowerCase().includes(q),
    );
  }, [specs, filter]);

  const parseBody = () => {
    const layout = JSON.parse(form.layoutJson) as unknown;
    const nodes = JSON.parse(form.nodesJson) as unknown;
    const dataBindings = JSON.parse(form.dataBindingsJson) as unknown;
    const sortOrder = Number.parseInt(form.sortOrder, 10);
    return {
      layout,
      nodes: sanitizeToolNodesForMetadata(nodes),
      dataBindings,
      icon: form.icon.trim() || undefined,
      sortOrder: Number.isFinite(sortOrder) ? sortOrder : 0,
    };
  };

  const createSpec = async () => {
    setErr("");
    setWarn("");
    setBusy(true);
    try {
      const body = parseBody();
      const created = (await bridge.fetch("/metadata/v1/tools", {
        method: "POST",
        body: JSON.stringify({
          apiName: form.apiName.trim(),
          label: form.label.trim(),
          description: form.description.trim(),
          ...body,
        }),
      })) as ToolSpec;
      const mirrorWarn = await mirrorToolSpecYaml(bridge.session?.repoPath, {
        apiName: created.apiName || form.apiName.trim(),
        label: created.label || form.label.trim(),
        description: created.description ?? form.description.trim(),
        icon: created.icon ?? body.icon,
        sortOrder: created.sortOrder ?? body.sortOrder,
        layout: created.layout ?? body.layout,
        nodes: created.nodes ?? body.nodes,
        dataBindings: created.dataBindings ?? body.dataBindings,
        active: created.active,
        ownership: created.ownership,
        packageName: created.packageName,
      });
      if (mirrorWarn) setWarn(mirrorWarn);
      setShowNew(false);
      onToolsChanged?.();
      await loadList();
      await loadDetail(created.apiName || form.apiName.trim());
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const saveSpec = async () => {
    if (!detail?.apiName || detail.ownership !== "custom") return;
    setErr("");
    setWarn("");
    setBusy(true);
    try {
      const patched = (await bridge.fetch(
        `/metadata/v1/tools/${encodeURIComponent(detail.apiName)}`,
        {
          method: "PATCH",
          body: JSON.stringify({
            label: detail.label,
            description: detail.description,
            icon: detail.icon,
            sortOrder: detail.sortOrder,
            layout: detail.layout,
            nodes: sanitizeToolNodesForMetadata(detail.nodes),
            dataBindings: detail.dataBindings ?? [],
            active: detail.active,
          }),
        },
      )) as ToolSpec;
      const mirrorWarn = await mirrorToolSpecYaml(bridge.session?.repoPath, {
        apiName: patched.apiName,
        label: patched.label,
        description: patched.description,
        icon: patched.icon,
        sortOrder: patched.sortOrder,
        layout: patched.layout,
        nodes: patched.nodes ?? [],
        dataBindings: patched.dataBindings,
        active: patched.active,
        ownership: patched.ownership,
        packageName: patched.packageName,
      });
      if (mirrorWarn) setWarn(mirrorWarn);
      onToolsChanged?.();
      await loadList();
      setDetail(patched);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const addStarterPack = async (packId: string) => {
    const pack = WORKFLOW_STARTER_PACKS.find((item) => item.id === packId);
    if (!pack) return;
    setPackBusy(pack.id);
    setErr("");
    setWarn("");
    setPackNotice("");
    try {
      const result = await installWorkflowStarterPack(bridge.fetch, pack, bridge.session?.repoPath);
      setPackNotice(`${pack.label} added: ToolSpec, AgentSpec, and automation are ready to review.`);
      if (result.warnings.length) setWarn(result.warnings.join(" · "));
      await loadList();
      onToolsChanged?.();
      onStarterPackInstalled?.();
    } catch (e) {
      setErr(String(e));
    } finally {
      setPackBusy(null);
    }
  };

  if (selected && detail) {
    const nodes = Array.isArray(detail.nodes) ? detail.nodes : [];
    return (
      <ToolSurface className="tools-panel" testId="tools-panel">
        <PanelHeader
          title={detail.label}
          subtitle={detail.apiName}
          actions={
            <>
              <Button variant="secondary" onClick={() => setSelected(null)}>
                <IconChevronLeft size={14} /> List
              </Button>
              {detail.ownership === "custom" ? (
                <Button variant="primary" busy={busy} onClick={() => void saveSpec()} data-testid="tool-spec-save">
                  Save
                </Button>
              ) : (
                <StatusBadge tone="neutral">Managed</StatusBadge>
              )}
            </>
          }
        />
        {warn ? <p className="panel-warn">{warn}</p> : null}
        {err ? (
          <p className="panel-error" role="alert">
            {err}
          </p>
        ) : null}
        <p className="muted">{detail.description}</p>
        <p className="muted">
          Nodes: {nodes.length} · Bindings: {detail.dataBindings?.length ?? 0}
          {detail.packageName ? ` · Package: ${detail.packageName}` : ""}
        </p>
        <pre className="canvas-spec-preview" data-testid="tool-spec-preview">
          {JSON.stringify(
            { layout: detail.layout, nodes: detail.nodes, dataBindings: detail.dataBindings },
            null,
            2,
          )}
        </pre>
      </ToolSurface>
    );
  }

  return (
    <ToolSurface className="tools-panel" testId="tools-panel">
      <PanelHeader
        title="Tools"
        subtitle="ToolSpec metadata — promote via Repo / Deploy; Run mode opens these on the left rail."
      />
      {err ? (
        <p className="panel-error" role="alert">
          {err}
        </p>
      ) : null}
      {warn ? <p className="panel-warn">{warn}</p> : null}
      <ToolToolbar
        actions={
          <>
            <Button
              variant="primary"
              onClick={() => setShowNew((v) => !v)}
              data-testid="tool-spec-new"
            >
              {showNew ? "Cancel" : "New ToolSpec"}
            </Button>
            <Button variant="secondary" busy={busy} onClick={() => void loadList()}>
              Refresh
            </Button>
          </>
        }
        meta={
          <span className="muted om-count">
            {filtered.length === specs.length
              ? `${specs.length} tool${specs.length === 1 ? "" : "s"}`
              : `${filtered.length} of ${specs.length}`}
          </span>
        }
        search={
          <SearchField
            value={filter}
            onChange={setFilter}
            placeholder="Search tools…"
            label="Search tools"
            testId="tool-spec-search"
          />
        }
      />
      {showNew ? (
        <div className="om-form om-new-object-form tools-new-form" data-testid="tool-spec-new-form">
          <label>
            Label
            <input
              value={form.label}
              onChange={(e) => setForm((f) => ({ ...f, label: e.target.value }))}
            />
          </label>
          <label>
            API name
            <input
              value={form.apiName}
              onChange={(e) => setForm((f) => ({ ...f, apiName: e.target.value }))}
              data-testid="tool-spec-api-name"
            />
          </label>
          <label>
            Description
            <input
              value={form.description}
              onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
            />
          </label>
          <label>
            Icon (optional)
            <input
              value={form.icon}
              onChange={(e) => setForm((f) => ({ ...f, icon: e.target.value }))}
            />
          </label>
          <label>
            sortOrder
            <input
              value={form.sortOrder}
              onChange={(e) => setForm((f) => ({ ...f, sortOrder: e.target.value }))}
            />
          </label>
          <label>
            layout (JSON)
            <textarea
              rows={5}
              value={form.layoutJson}
              onChange={(e) => setForm((f) => ({ ...f, layoutJson: e.target.value }))}
            />
          </label>
          <label>
            nodes (JSON)
            <textarea
              rows={8}
              value={form.nodesJson}
              onChange={(e) => setForm((f) => ({ ...f, nodesJson: e.target.value }))}
            />
          </label>
          <label>
            dataBindings (JSON)
            <textarea
              rows={4}
              value={form.dataBindingsJson}
              onChange={(e) => setForm((f) => ({ ...f, dataBindingsJson: e.target.value }))}
            />
          </label>
          <div className="om-new-object-actions">
            <Button
              variant="primary"
              busy={busy}
              disabled={!form.apiName.trim() || !form.label.trim()}
              onClick={() => void createSpec()}
              data-testid="tool-spec-create"
            >
              Create ToolSpec
            </Button>
          </div>
        </div>
      ) : null}
      <section className="starter-pack-section" aria-labelledby="starter-pack-title">
        <div className="starter-pack-heading">
          <div>
            <p className="eyebrow">Recipes</p>
            <h3 id="starter-pack-title">Starter packs</h3>
            <p className="muted">Clone a coherent ToolSpec + AgentSpec + automation, then tailor it like any customer metadata.</p>
          </div>
          <StatusBadge tone="neutral">3 curated</StatusBadge>
        </div>
        <div className="starter-pack-grid">
          {WORKFLOW_STARTER_PACKS.map((pack) => {
            const installed = specs.some((spec) => spec.apiName === pack.tool.apiName);
            return (
              <article className="starter-pack-card" key={pack.id}>
                <p className="eyebrow">{pack.objectApiNames.join(" + ")}</p>
                <h4>{pack.label}</h4>
                <p className="muted">{pack.description}</p>
                <Button
                  variant={installed ? "ghost" : "secondary"}
                  disabled={installed || Boolean(packBusy)}
                  busy={packBusy === pack.id}
                  onClick={() => void addStarterPack(pack.id)}
                  data-testid={`starter-pack-${pack.id}`}
                >
                  {installed ? "Added" : "Add starter"}
                </Button>
              </article>
            );
          })}
        </div>
        {packNotice ? <p className="panel-success" data-testid="starter-pack-notice">{packNotice}</p> : null}
      </section>
      <div className="data-table-wrap om-table-wrap" data-testid="tool-spec-list">
        <table className="data-table om-object-table">
          <thead>
            <tr>
              <th>Label</th>
              <th>API name</th>
              <th>Ownership</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((s) => (
              <tr
                key={s.apiName}
                className="om-object-row"
                data-testid={`tool-spec-item-${s.apiName}`}
                onClick={() => void loadDetail(s.apiName)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    void loadDetail(s.apiName);
                  }
                }}
                tabIndex={0}
                role="button"
                aria-label={`Open ${s.label || s.apiName}`}
              >
                <td>{s.label || s.apiName}</td>
                <td className="mono">{s.apiName}</td>
                <td>
                  {s.ownership ? (
                    <StatusBadge tone={s.ownership === "custom" ? "accent" : "neutral"}>
                      {s.ownership}
                    </StatusBadge>
                  ) : (
                    "—"
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!filtered.length && !busy ? (
          <p className="data-table-empty muted">No tools match.</p>
        ) : null}
      </div>
    </ToolSurface>
  );
}
