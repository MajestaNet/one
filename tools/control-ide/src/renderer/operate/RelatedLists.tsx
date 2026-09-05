import { useEffect, useState } from "react";
import type { AppBridge } from "../App";
import { EmptyState, Spinner } from "../ui";
import type { RelatedListDef } from "./types";
import { displayName, recordId } from "./types";
import { queryRecords } from "./recordClient";

export function RelatedLists({
  bridge,
  parentId,
  defs,
  onOpenRelated,
}: {
  bridge?: AppBridge;
  parentId: string;
  defs: RelatedListDef[];
  onOpenRelated?: (objectApiName: string, id: string) => void;
}) {
  const [active, setActive] = useState(defs[0]?.objectApiName ?? "");
  const [rows, setRows] = useState<Record<string, unknown>[]>([]);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  const def = defs.find((d) => d.objectApiName === active) ?? defs[0];

  useEffect(() => {
    if (!defs.length) return;
    if (!defs.some((d) => d.objectApiName === active)) {
      setActive(defs[0].objectApiName);
    }
  }, [defs, active]);

  useEffect(() => {
    if (!bridge?.session || !def || !parentId) {
      setRows([]);
      return;
    }
    let cancelled = false;
    void (async () => {
      setBusy(true);
      setErr("");
      try {
        const q = await queryRecords(bridge.fetch, {
          object: def.objectApiName,
          filters: [{ field: def.lookupField, op: "eq", value: parentId }],
          limit: 25,
        });
        if (!cancelled) setRows(q.records ?? []);
      } catch (e) {
        if (!cancelled) {
          setErr(String(e));
          setRows([]);
        }
      } finally {
        if (!cancelled) setBusy(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [bridge, def, parentId]);

  if (!defs.length) return null;

  return (
    <section className="crm-related" data-testid="crm-related-lists" aria-label="Related lists">
      <h4>Related</h4>
      <div className="crm-related-tabs" role="tablist">
        {defs.map((d) => (
          <button
            key={d.objectApiName}
            type="button"
            role="tab"
            className={`crm-tab ${active === d.objectApiName ? "active" : ""}`}
            aria-selected={active === d.objectApiName}
            onClick={() => setActive(d.objectApiName)}
          >
            {d.label}
          </button>
        ))}
      </div>
      {busy ? <Spinner /> : null}
      {err ? <p className="err">{err}</p> : null}
      {!busy && rows.length === 0 ? (
        <EmptyState title={`No ${def?.label ?? "related"} records`} description="Related rows appear when lookup filters match." />
      ) : (
        <ul className="crm-related-list">
          {rows.map((r) => {
            const id = recordId(r);
            return (
              <li key={id}>
                <button
                  type="button"
                  className="crm-row-btn"
                  onClick={() => onOpenRelated?.(def!.objectApiName, id)}
                >
                  <span className="crm-row-name">{displayName(r)}</span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
      {def && bridge?.session ? (
        <p className="muted crm-footnote">
          Filtered by {def.lookupField} = {parentId.slice(0, 8)}…
        </p>
      ) : null}
      {!bridge?.session ? (
        <p className="muted">Connect to load related lists via Client query.</p>
      ) : null}
    </section>
  );
}
