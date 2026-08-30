/** Drag-and-drop / selection context for agent chat (Agentic Run uplift). */

export const CONTEXT_EXCERPT_MIME = "application/x-one-context-excerpt";

export type ContextExcerptSource = "selection" | "tool_rows";

export type ContextExcerptStructured = {
  objectApiName?: string;
  records: Record<string, unknown>[];
  columns?: Array<{ key: string; label: string }>;
};

export type ContextExcerpt = {
  id: string;
  mime: string;
  label: string;
  text: string;
  source: ContextExcerptSource;
  structured?: ContextExcerptStructured;
};

export function createContextExcerptId(): string {
  return `excerpt-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

export function rowsToContextExcerpt(opts: {
  rows: Record<string, unknown>[];
  columns?: Array<{ key: string; label: string }>;
  objectApiName?: string;
  label?: string;
  source?: ContextExcerptSource;
}): ContextExcerpt {
  const { rows, columns, objectApiName, source = "tool_rows" } = opts;
  const label =
    opts.label ??
    (objectApiName
      ? `${rows.length} ${objectApiName} record${rows.length === 1 ? "" : "s"}`
      : `${rows.length} selected row${rows.length === 1 ? "" : "s"}`);
  const text = formatExcerptText(rows, columns);
  return {
    id: createContextExcerptId(),
    mime: CONTEXT_EXCERPT_MIME,
    label,
    text,
    source,
    structured: {
      objectApiName,
      records: rows,
      columns,
    },
  };
}

export function formatExcerptText(
  rows: Record<string, unknown>[],
  columns?: Array<{ key: string; label: string }>,
): string {
  if (rows.length === 0) return "";
  const keys =
    columns?.map((c) => c.key) ??
    Object.keys(rows[0]).filter((k) => k !== "data" && !k.startsWith("_"));
  const header = keys.join(" | ");
  const lines = rows.map((row) =>
    keys
      .map((k) => {
        const v = row[k];
        if (v == null) return "—";
        return typeof v === "object" ? JSON.stringify(v) : String(v);
      })
      .join(" | "),
  );
  return [header, ...lines].join("\n");
}

export function serializeContextExcerpt(excerpt: ContextExcerpt): string {
  return JSON.stringify(excerpt);
}

export function parseContextExcerpt(raw: string): ContextExcerpt | null {
  try {
    const parsed = JSON.parse(raw) as Partial<ContextExcerpt>;
    if (!parsed || typeof parsed.label !== "string" || typeof parsed.text !== "string") return null;
    return {
      id: parsed.id ?? createContextExcerptId(),
      mime: CONTEXT_EXCERPT_MIME,
      label: parsed.label,
      text: parsed.text,
      source: parsed.source === "selection" ? "selection" : "tool_rows",
      structured: parsed.structured,
    };
  } catch {
    return null;
  }
}

export function excerptFromPlainText(text: string): ContextExcerpt {
  const trimmed = text.trim();
  const label = trimmed.length > 48 ? `${trimmed.slice(0, 45)}…` : trimmed || "Selection";
  return {
    id: createContextExcerptId(),
    mime: CONTEXT_EXCERPT_MIME,
    label,
    text: trimmed,
    source: "selection",
  };
}

/** Attach drag payload for a context excerpt. */
export function setExcerptDragData(dataTransfer: DataTransfer, excerpt: ContextExcerpt): void {
  const json = serializeContextExcerpt(excerpt);
  dataTransfer.setData(CONTEXT_EXCERPT_MIME, json);
  dataTransfer.setData("text/plain", excerpt.text);
  dataTransfer.effectAllowed = "copy";
}

export function readExcerptFromDataTransfer(dataTransfer: DataTransfer): ContextExcerpt | null {
  const raw = dataTransfer.getData(CONTEXT_EXCERPT_MIME);
  if (raw) {
    const parsed = parseContextExcerpt(raw);
    if (parsed) return parsed;
  }
  const plain = dataTransfer.getData("text/plain").trim();
  if (plain) return excerptFromPlainText(plain);
  return null;
}

export function isContextExcerptDrag(dataTransfer: DataTransfer): boolean {
  return [...dataTransfer.types].includes(CONTEXT_EXCERPT_MIME) || dataTransfer.types.includes("text/plain");
}
