import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useCallback, useRef, useState } from "react";
import {
  rowsToContextExcerpt,
  setExcerptDragData,
  type ContextExcerpt,
} from "../../workspace/contextExcerpt";

export type TableColumn = { key: string; label: string; mono?: boolean };

function asColumns(raw: unknown): TableColumn[] {
  if (!Array.isArray(raw)) return [];
  const out: TableColumn[] = [];
  for (const c of raw) {
    if (!c || typeof c !== "object") continue;
    const obj = c as Record<string, unknown>;
    if (typeof obj.key !== "string") continue;
    out.push({
      key: obj.key,
      label: typeof obj.label === "string" ? obj.label : obj.key,
      mono: Boolean(obj.mono),
    });
  }
  return out;
}

function asRows(raw: unknown): Record<string, unknown>[] {
  if (!Array.isArray(raw)) return [];
  return raw.filter((r): r is Record<string, unknown> => !!r && typeof r === "object" && !Array.isArray(r));
}

function rowKey(row: Record<string, unknown>, index: number): string {
  const id = row.id ?? row.Id;
  if (typeof id === "string" && id) return id;
  return `row-${index}`;
}

/** TanStack Table + Virtual record grid for ToolSpec recordTable / relatedList. */
export function RecordTableNode({
  columns: columnsRaw,
  rows: rowsRaw,
  emptyLabel = "No rows",
  testId,
  selectable = false,
  objectApiName,
  onAddExcerptToChat,
}: {
  columns: unknown;
  rows: unknown;
  emptyLabel?: string;
  testId?: string;
  /** Enable row selection + drag-to-chat for agent context. */
  selectable?: boolean;
  objectApiName?: string;
  onAddExcerptToChat?: (excerpt: ContextExcerpt) => void;
}) {
  const columns = asColumns(columnsRaw);
  const data = asRows(rowsRaw);
  const parentRef = useRef<HTMLDivElement>(null);
  const [selectedKeys, setSelectedKeys] = useState<Set<string>>(new Set());

  const toggleRow = useCallback((key: string) => {
    setSelectedKeys((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }, []);

  const selectedRows = data.filter((row, i) => selectedKeys.has(rowKey(row, i)));

  const buildExcerpt = useCallback(
    (rows: Record<string, unknown>[]) =>
      rowsToContextExcerpt({
        rows,
        columns,
        objectApiName,
        source: "tool_rows",
      }),
    [columns, objectApiName],
  );

  const handleAddSelection = useCallback(() => {
    if (!onAddExcerptToChat || selectedRows.length === 0) return;
    onAddExcerptToChat(buildExcerpt(selectedRows));
    setSelectedKeys(new Set());
  }, [buildExcerpt, onAddExcerptToChat, selectedRows]);

  const handleDragStart = useCallback(
    (e: React.DragEvent, rows: Record<string, unknown>[]) => {
      if (rows.length === 0) return;
      setExcerptDragData(e.dataTransfer, buildExcerpt(rows));
    },
    [buildExcerpt],
  );

  const columnDefs: ColumnDef<Record<string, unknown>>[] = columns.map((c) => ({
    accessorKey: c.key,
    header: c.label,
    cell: (info) => {
      const raw = info.getValue();
      const text = raw == null ? "—" : typeof raw === "object" ? JSON.stringify(raw) : String(raw);
      return <span className={c.mono ? "mono" : undefined}>{text}</span>;
    },
  }));

  const table = useReactTable({
    data,
    columns: columnDefs,
    getCoreRowModel: getCoreRowModel(),
  });

  const rows = table.getRowModel().rows;
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 32,
    overscan: 8,
  });

  if (columns.length === 0) {
    return <p className="muted">{emptyLabel}</p>;
  }
  if (data.length === 0) {
    return <p className="muted data-table-empty">{emptyLabel}</p>;
  }

  return (
    <div className="run-tool-table-wrap" data-testid={testId}>
      {selectable && onAddExcerptToChat ? (
        <div className="run-tool-table-actions">
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            disabled={selectedRows.length === 0}
            data-testid="run-tool-table-add-to-chat"
            onClick={handleAddSelection}
          >
            Add {selectedRows.length > 0 ? selectedRows.length : ""} to chat
          </button>
        </div>
      ) : null}
      <div ref={parentRef} className="run-tool-table-scroll">
        <table className="data-table run-tool-table">
          <thead>
            {table.getHeaderGroups().map((hg) => (
              <tr key={hg.id}>
                {selectable ? <th className="run-tool-table-select-col" aria-label="Select" /> : null}
                {hg.headers.map((h) => (
                  <th key={h.id}>{flexRender(h.column.columnDef.header, h.getContext())}</th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody style={{ height: `${virtualizer.getTotalSize()}px`, position: "relative" }}>
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const row = rows[virtualRow.index];
              const dataRow = row.original;
              const key = rowKey(dataRow, virtualRow.index);
              const isSelected = selectedKeys.has(key);
              return (
                <tr
                  key={row.id}
                  className={isSelected ? "is-selected" : undefined}
                  draggable={selectable && isSelected && selectedRows.length > 0}
                  onDragStart={(e) => {
                    if (!selectable || !isSelected) return;
                    const dragRows = selectedRows.length > 0 ? selectedRows : [dataRow];
                    handleDragStart(e, dragRows);
                  }}
                  style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    width: "100%",
                    transform: `translateY(${virtualRow.start}px)`,
                    display: "table",
                    tableLayout: "fixed",
                  }}
                  onClick={() => {
                    if (selectable) toggleRow(key);
                  }}
                >
                  {selectable ? (
                    <td className="run-tool-table-select-col">
                      <input
                        type="checkbox"
                        checked={isSelected}
                        readOnly
                        aria-label="Select row"
                        data-testid={`run-tool-row-select-${key}`}
                      />
                    </td>
                  ) : null}
                  {row.getVisibleCells().map((cell) => (
                    <td key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</td>
                  ))}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
