import { useCallback, useEffect, useState } from "react";
import type { AppBridge } from "../App";
import { EmptyState, Spinner } from "../ui";

export type ActivityFeedItem = {
  kind?: string;
  objectApiName?: string;
  id?: string;
  occurredAt?: string;
  subject?: string;
  summary?: string;
  channel?: string;
  direction?: string;
  status?: string;
};

function itemKey(it: ActivityFeedItem, idx: number): string {
  return `${it.objectApiName ?? "row"}:${it.id ?? idx}`;
}

function kindLabel(it: ActivityFeedItem): string {
  return it.objectApiName || "Activity";
}

export function ActivityFeed({
  bridge,
  parentType,
  parentId,
  activitiesEnabled,
}: {
  bridge?: AppBridge;
  parentType: string;
  parentId: string;
  activitiesEnabled: boolean;
}) {
  const [items, setItems] = useState<ActivityFeedItem[]>([]);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  const load = useCallback(async () => {
    if (!activitiesEnabled || !bridge?.session || !parentId) {
      setItems([]);
      return;
    }
    setBusy(true);
    setErr("");
    try {
      const q = new URLSearchParams({
        parentType,
        parentId,
        limit: "50",
      });
      const res = (await bridge.fetch(`/client/v1/activity-feed?${q.toString()}`)) as {
        items?: ActivityFeedItem[];
      };
      setItems(res.items ?? []);
    } catch (e) {
      setErr(String(e));
      setItems([]);
    } finally {
      setBusy(false);
    }
  }, [activitiesEnabled, bridge, parentId, parentType]);

  useEffect(() => {
    void load();
  }, [load]);

  if (!activitiesEnabled) return null;

  return (
    <section className="crm-messages" data-testid="crm-activity-feed" aria-label="Activity feed">
      <h4>Activity feed</h4>
      {!bridge?.session ? (
        <p className="muted">Connect to load the Activity feed.</p>
      ) : (
        <>
          {busy ? <Spinner /> : null}
          {err ? <p className="err">{err}</p> : null}
          {items.length === 0 && !busy ? (
            <EmptyState title="No activity" description="Create Tasks, Appointments, Phone Calls, or Emails on this record." />
          ) : (
            <ul className="crm-message-list">
              {items.map((it, idx) => (
                <li key={itemKey(it, idx)} className="crm-message-item" data-kind={it.kind}>
                  <p className="crm-message-meta muted">
                    {kindLabel(it)}
                    {it.status ? ` · ${it.status}` : ""}
                    {it.direction ? ` · ${it.direction}` : ""}
                  </p>
                  <p className="crm-message-subject">{it.subject || it.objectApiName || "Untitled"}</p>
                  {it.summary ? <p className="crm-message-body">{it.summary}</p> : null}
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </section>
  );
}

/** @deprecated Prefer ActivityFeed — kept for import compatibility. */
export function MessageTimeline(props: {
  bridge?: AppBridge;
  parentType: string;
  parentId: string;
  enabled: boolean;
}) {
  return (
    <ActivityFeed
      bridge={props.bridge}
      parentType={props.parentType}
      parentId={props.parentId}
      activitiesEnabled={props.enabled}
    />
  );
}
