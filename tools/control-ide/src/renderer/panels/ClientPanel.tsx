import { useState } from "react";
import type { AppBridge } from "../App";
import { Button, DataTable, EmptyState, PanelHeader } from "../ui";
import { IconRecords } from "../icons/Icons";

type DescribeResult = {
  apiName?: string;
  fields?: Array<{
    apiName?: string;
    name?: string;
    fieldType?: string;
    type?: string;
    dataType?: string;
  }>;
};

type QueryResult = {
  records?: Array<Record<string, unknown>>;
};

export function ClientPanel({ bridge }: { bridge: AppBridge }) {
  const [objectName, setObjectName] = useState("Account");
  const [describe, setDescribe] = useState<DescribeResult | null>(null);
  const [records, setRecords] = useState<Record<string, unknown>[] | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState<"describe" | "query" | null>(null);

  const runDescribe = async () => {
    setErr("");
    setBusy("describe");
    try {
      const d = (await bridge.fetch(`/client/v1/describe/${encodeURIComponent(objectName)}`)) as DescribeResult;
      setDescribe(d);
    } catch (e) {
      setErr(String(e));
      setDescribe(null);
    } finally {
      setBusy(null);
    }
  };

  const runQuery = async () => {
    setErr("");
    setBusy("query");
    try {
      const q = (await bridge.fetch("/client/v1/query", {
        method: "POST",
        body: JSON.stringify({
          object: objectName,
          limit: 5,
        }),
      })) as QueryResult;
      setRecords(q.records ?? []);
    } catch (e) {
      setErr(String(e));
      setRecords(null);
    } finally {
      setBusy(null);
    }
  };

  const fields = describe?.fields ?? [];
  const recordColumns =
    records && records.length > 0
      ? Object.keys(records[0])
          .slice(0, 6)
          .map((key) => ({ key, label: key, mono: key === "id" }))
      : [{ key: "id", label: "id", mono: true }];

  return (
    <div className="panel tool-surface" data-tool-surface="true">
      <PanelHeader
        title="Client"
        subtitle="Browse object fields and sample records after a change. Thin Client API explorer for Operate."
      />
      <div className="row">
        <label>
          Object API name
          <input value={objectName} onChange={(e) => setObjectName(e.target.value)} />
        </label>
      </div>
      <div className="row">
        <Button variant="primary" busy={busy === "describe"} onClick={() => void runDescribe()}>
          Describe
        </Button>
        <Button variant="secondary" busy={busy === "query"} onClick={() => void runQuery()}>
          Query records
        </Button>
      </div>
      <p className="muted">Query uses limit 5 for a quick smoke sample.</p>
      {err && <p className="err">{err}</p>}
      {!describe && !records && !err ? (
        <EmptyState
          icon={<IconRecords size={28} />}
          title="No object loaded"
          description="Describe an object to list fields, or run a query to preview records."
        />
      ) : null}
      {describe && (
        <div className="env-card">
          <h3>{describe.apiName ?? objectName} fields</h3>
          {fields.length === 0 ? (
            <p className="muted">No fields returned.</p>
          ) : (
            <ul className="field-list" data-testid="field-list">
              {fields.map((f, i) => (
                <li key={f.apiName ?? f.name ?? i}>
                  <span className="mono">{f.apiName ?? f.name ?? "—"}</span>
                  <span className="muted">{f.fieldType ?? f.type ?? f.dataType ?? ""}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
      {records && (
        <div className="env-card">
          <h3>Records</h3>
          <DataTable columns={recordColumns} rows={records} emptyLabel="No records returned." />
        </div>
      )}
    </div>
  );
}
