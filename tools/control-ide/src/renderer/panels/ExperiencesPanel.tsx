import { useCallback, useEffect, useState } from "react";
import type { AppBridge } from "../App";
import { listExperiences, type Experience } from "../govern/experiences";
import { EmptyState, PanelHeader } from "../ui";

export function ExperiencesPanel({ bridge }: { bridge: AppBridge }) {
  const connected = Boolean(bridge.session?.baseUrl && bridge.session?.token);
  const [items, setItems] = useState<Experience[]>([]);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    if (!connected) return;
    setErr("");
    setBusy(true);
    try {
      setItems(await listExperiences(bridge.fetch));
    } catch (e) {
      setErr(String(e));
      setItems([]);
    } finally {
      setBusy(false);
    }
  }, [bridge.fetch, connected]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="panel experiences-panel tool-surface" data-tool-surface="true">
      <PanelHeader
        title="Experiences"
        subtitle="Registered Client Experience apps (config only — code hosted on customer infra)"
      />
      {!connected && (
        <EmptyState title="Not connected" description="Connect to an install in Settings → Environments." />
      )}
      {connected && err && <p className="panel-error">{err}</p>}
      {connected && busy && items.length === 0 && <p className="panel-muted">Loading…</p>}
      {connected && !busy && items.length === 0 && !err && (
        <EmptyState
          title="No experiences"
          description="Add metadata/experiences/*.yaml to the customer repo and deploy, or POST /metadata/v1/experiences."
        />
      )}
      {connected && items.length > 0 && (
        <table className="data-table">
          <thead>
            <tr>
              <th>API name</th>
              <th>Label</th>
              <th>Home URL</th>
              <th>Connected app</th>
              <th>Active</th>
            </tr>
          </thead>
          <tbody>
            {items.map((ex) => (
              <tr key={ex.apiName}>
                <td>{ex.apiName}</td>
                <td>{ex.label ?? "—"}</td>
                <td>
                  {ex.homeUrl ? (
                    <a href={ex.homeUrl} target="_blank" rel="noreferrer">
                      {ex.homeUrl}
                    </a>
                  ) : (
                    "—"
                  )}
                </td>
                <td>{ex.connectedAppApiName || "—"}</td>
                <td>{ex.active === false ? "No" : "Yes"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
