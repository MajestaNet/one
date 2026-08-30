import { Bar, BarChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

function asNumber(value: unknown): number | null {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string" && value.trim() && !Number.isNaN(Number(value))) return Number(value);
  return null;
}

/** Stat KPI — optional Recharts spark when `props.series` is present. */
export function StatNode({
  id,
  label,
  value,
  series,
}: {
  id: string;
  label: string;
  value: unknown;
  series?: unknown;
}) {
  const n = asNumber(value);
  const chartData = Array.isArray(series)
    ? series
        .map((pt, i) => {
          if (typeof pt === "number") return { i, v: pt };
          if (pt && typeof pt === "object" && typeof (pt as { v?: unknown }).v === "number") {
            return { i, v: (pt as { v: number }).v };
          }
          return null;
        })
        .filter((x): x is { i: number; v: number } => x != null)
    : [];

  return (
    <div className="run-tool-stat">
      <p className="muted canvas-stat-label">{label}</p>
      <p className="canvas-stat-value" data-testid={`canvas-stat-value-${id}`}>
        {n != null ? String(n) : value == null ? "—" : String(value)}
      </p>
      {chartData.length > 1 ? (
        <div className="run-tool-stat-chart" data-testid={`run-stat-chart-${id}`}>
          <ResponsiveContainer width="100%" height={48}>
            <BarChart data={chartData}>
              <XAxis dataKey="i" hide />
              <YAxis hide />
              <Tooltip />
              <Bar dataKey="v" fill="var(--accent)" radius={2} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      ) : null}
    </div>
  );
}
