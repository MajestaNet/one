export type KeyValueItem = { label: string; value: string };

export function KeyValueList({ items }: { items: KeyValueItem[] }) {
  if (items.length === 0) return null;
  return (
    <dl className="kv-list">
      {items.map((item) => (
        <div key={item.label} className="kv-row">
          <dt>{item.label}</dt>
          <dd className="mono">{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}
