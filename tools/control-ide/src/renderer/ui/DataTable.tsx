export type DataColumn = {
  key: string;
  label: string;
  mono?: boolean;
};

export function DataTable({
  columns,
  rows,
  emptyLabel = "No rows",
}: {
  columns: DataColumn[];
  rows: Record<string, unknown>[];
  emptyLabel?: string;
}) {
  if (rows.length === 0) {
    return <p className="muted data-table-empty">{emptyLabel}</p>;
  }
  return (
    <div className="data-table-wrap">
      <table className="data-table">
        <thead>
          <tr>
            {columns.map((c) => (
              <th key={c.key}>{c.label}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, i) => (
            <tr key={i}>
              {columns.map((c) => {
                const raw = row[c.key];
                const text = raw == null ? "—" : typeof raw === "object" ? JSON.stringify(raw) : String(raw);
                return (
                  <td key={c.key} className={c.mono ? "mono" : undefined}>
                    {text}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
