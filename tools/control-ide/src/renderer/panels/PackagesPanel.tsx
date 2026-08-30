import { useCallback, useEffect, useMemo, useState } from "react";
import type { AppBridge } from "../App";
import { Button, EmptyState, PanelHeader, StatusBadge, SearchField, ToolSurface, ToolToolbar } from "../ui";
import { IconMetadata } from "../icons/Icons";

export type PackageStatus = {
  name: string;
  label: string;
  description?: string;
  version?: string;
  installedVersion?: string;
  dependsOn?: string[];
  optional?: boolean;
  autoEnable?: boolean;
  enabled?: boolean;
  documentationPath?: string;
  objectApiNames?: string[];
};

function sortPackages(rows: PackageStatus[]): PackageStatus[] {
  return [...rows].sort((a, b) => {
    const la = (a.label || a.name).toLowerCase();
    const lb = (b.label || b.name).toLowerCase();
    return la.localeCompare(lb);
  });
}

function depsMet(pkg: PackageStatus, byName: Map<string, PackageStatus>): boolean {
  for (const dep of pkg.dependsOn ?? []) {
    const d = byName.get(dep);
    if (!d?.enabled) return false;
  }
  return true;
}

export function PackagesPanel({
  bridge,
  refreshKey = 0,
}: {
  bridge: AppBridge;
  refreshKey?: number;
}) {
  const [packages, setPackages] = useState<PackageStatus[]>([]);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [actionName, setActionName] = useState<string | null>(null);
  const [filter, setFilter] = useState("");
  const [expanded, setExpanded] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!bridge.session?.token) {
      setPackages([]);
      return;
    }
    setBusy(true);
    setErr("");
    try {
      const row = (await bridge.fetch("/metadata/v1/packages")) as { packages?: PackageStatus[] };
      setPackages(sortPackages(row.packages ?? []));
    } catch (e) {
      setErr(String(e));
      setPackages([]);
    } finally {
      setBusy(false);
    }
  }, [bridge]);

  useEffect(() => {
    void load();
    setExpanded(null);
  }, [load, refreshKey]);

  const byName = useMemo(() => {
    const m = new Map<string, PackageStatus>();
    for (const p of packages) m.set(p.name, p);
    return m;
  }, [packages]);

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return packages;
    return packages.filter((p) => {
      const objects = (p.objectApiNames ?? []).join(" ").toLowerCase();
      return (
        p.name.toLowerCase().includes(q) ||
        (p.label || "").toLowerCase().includes(q) ||
        (p.description || "").toLowerCase().includes(q) ||
        objects.includes(q)
      );
    });
  }, [packages, filter]);

  const enablePackage = async (name: string) => {
    setErr("");
    setActionName(name);
    try {
      await bridge.fetch(`/metadata/v1/packages/${encodeURIComponent(name)}/enable`, {
        method: "POST",
        body: "{}",
      });
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setActionName(null);
    }
  };

  const disablePackage = async (name: string) => {
    setErr("");
    setActionName(name);
    try {
      await bridge.fetch(`/metadata/v1/packages/${encodeURIComponent(name)}/disable`, {
        method: "POST",
        body: "{}",
      });
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setActionName(null);
    }
  };

  if (!bridge.session?.token) {
    return (
      <ToolSurface testId="packages-panel">
        <PanelHeader
          title="Packages"
          subtitle="Enable optional managed packages (Notes, Sales, Service, …) on the active environment."
        />
        <EmptyState
          icon={<IconMetadata size={28} />}
          title="Connect an environment"
          description="Packages are managed via the Metadata API on the active install. Use the top-bar env switcher."
        />
      </ToolSurface>
    );
  }

  return (
    <ToolSurface className="packages-panel" testId="packages-panel">
      <PanelHeader
        title="Packages"
        subtitle="Optional managed packages ship objects and fields in the product image. Enable them when you are ready to use them (for example Notes or Sales)."
      />
      {err && <p className="err">{err}</p>}
      <ToolToolbar
        actions={
          <Button variant="secondary" busy={busy} onClick={() => void load()}>
            Refresh
          </Button>
        }
        meta={
          <span className="muted om-count">
            {filtered.length === packages.length
              ? `${packages.length} package${packages.length === 1 ? "" : "s"}`
              : `${filtered.length} of ${packages.length}`}
          </span>
        }
        search={
          <SearchField
            value={filter}
            onChange={setFilter}
            placeholder="Search packages…"
            label="Search packages"
            testId="pkg-search"
          />
        }
      />
      <div className="data-table-wrap om-table-wrap" data-testid="pkg-list">
        <table className="data-table">
          <thead>
            <tr>
              <th>Package</th>
              <th>Objects</th>
              <th>Depends on</th>
              <th>Status</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((p) => {
              const isCore = !p.optional;
              const ready = depsMet(p, byName);
              const canEnable = Boolean(p.optional && !p.enabled && !p.autoEnable && ready);
              const canDisable = Boolean(p.optional && p.enabled && !p.autoEnable);
              const objects = p.objectApiNames ?? [];
              const open = expanded === p.name;
              return (
                <tr key={p.name} data-testid={`pkg-row-${p.name}`}>
                  <td>
                    <button
                      type="button"
                      className="pkg-name-btn"
                      onClick={() => setExpanded(open ? null : p.name)}
                      aria-expanded={open}
                    >
                      <strong>{p.label || p.name}</strong>
                      <span className="muted mono"> {p.name}</span>
                    </button>
                    {p.description ? <p className="muted pkg-desc">{p.description}</p> : null}
                    {open ? (
                      <div className="pkg-detail" data-testid={`pkg-detail-${p.name}`}>
                        <p className="muted">
                          Version {p.version || "—"}
                          {p.installedVersion ? ` · installed ${p.installedVersion}` : ""}
                        </p>
                        {objects.length ? (
                          <ul className="pkg-object-list">
                            {objects.map((o) => (
                              <li key={o} className="mono">
                                {o}
                              </li>
                            ))}
                          </ul>
                        ) : (
                          <p className="muted">
                            {p.autoEnable
                              ? "Bridge package — field extensions only (no owned objects)."
                              : "No objects listed."}
                          </p>
                        )}
                      </div>
                    ) : null}
                  </td>
                  <td>
                    {objects.length ? (
                      <span className="mono">{objects.join(", ")}</span>
                    ) : (
                      <span className="muted">—</span>
                    )}
                  </td>
                  <td>
                    {(p.dependsOn ?? []).length ? (
                      <span className="mono">{(p.dependsOn ?? []).join(", ")}</span>
                    ) : (
                      <span className="muted">—</span>
                    )}
                  </td>
                  <td>
                    {isCore ? (
                      <StatusBadge tone="neutral">Always on</StatusBadge>
                    ) : p.autoEnable ? (
                      <StatusBadge tone={p.enabled ? "accent" : "neutral"}>
                        {p.enabled ? "Auto-enabled" : "Auto when deps ready"}
                      </StatusBadge>
                    ) : p.enabled ? (
                      <StatusBadge tone="accent">Enabled</StatusBadge>
                    ) : ready ? (
                      <StatusBadge tone="neutral">Available</StatusBadge>
                    ) : (
                      <StatusBadge tone="neutral">Needs dependencies</StatusBadge>
                    )}
                  </td>
                  <td>
                    {canEnable ? (
                      <Button
                        variant="primary"
                        busy={actionName === p.name}
                        data-testid={`pkg-enable-${p.name}`}
                        onClick={() => void enablePackage(p.name)}
                      >
                        Enable
                      </Button>
                    ) : null}
                    {canDisable ? (
                      <Button
                        variant="secondary"
                        busy={actionName === p.name}
                        data-testid={`pkg-disable-${p.name}`}
                        onClick={() => void disablePackage(p.name)}
                      >
                        Disable
                      </Button>
                    ) : null}
                    {!canEnable && !canDisable ? <span className="muted">—</span> : null}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
        {!filtered.length && !busy ? (
          <p className="data-table-empty muted">No packages match.</p>
        ) : null}
      </div>
    </ToolSurface>
  );
}
