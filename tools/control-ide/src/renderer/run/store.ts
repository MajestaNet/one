import type { ToolDocument } from "./types";
import { validateToolDocument } from "./validate";

const STORAGE_KEY = "one.control.run.tools";

export type ToolStoreSnapshot = {
  documents: ToolDocument[];
  activeId: string | null;
};

function readRaw(): unknown {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    return JSON.parse(raw) as unknown;
  } catch {
    return null;
  }
}

export function loadToolStore(): ToolStoreSnapshot {
  const parsed = readRaw();
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    return { documents: [], activeId: null };
  }
  const obj = parsed as Record<string, unknown>;
  const documents: ToolDocument[] = [];
  if (Array.isArray(obj.documents)) {
    for (const item of obj.documents) {
      const result = validateToolDocument(item);
      if (result.ok) documents.push(result.document);
    }
  }
  const activeId = typeof obj.activeId === "string" ? obj.activeId : null;
  return {
    documents,
    activeId: activeId && documents.some((d) => d.id === activeId) ? activeId : documents[0]?.id ?? null,
  };
}

export function saveToolStore(snapshot: ToolStoreSnapshot): void {
  try {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        documents: snapshot.documents,
        activeId: snapshot.activeId,
      }),
    );
  } catch {
    // Quota / private mode — in-memory only
  }
}

export function upsertToolDocument(
  snapshot: ToolStoreSnapshot,
  document: ToolDocument,
  options?: { skipSave?: boolean },
): ToolStoreSnapshot {
  const rest = snapshot.documents.filter((d) => d.id !== document.id);
  const next: ToolStoreSnapshot = {
    documents: [...rest, { ...document, meta: { ...document.meta, updatedAt: new Date().toISOString() } }],
    activeId: document.id,
  };
  if (!options?.skipSave) saveToolStore(next);
  return next;
}

export function removeToolDocument(snapshot: ToolStoreSnapshot, id: string): ToolStoreSnapshot {
  const documents = snapshot.documents.filter((d) => d.id !== id);
  const next: ToolStoreSnapshot = {
    documents,
    activeId: snapshot.activeId === id ? documents[0]?.id ?? null : snapshot.activeId,
  };
  saveToolStore(next);
  return next;
}

export function setActiveToolId(snapshot: ToolStoreSnapshot, id: string | null): ToolStoreSnapshot {
  const next = { ...snapshot, activeId: id };
  saveToolStore(next);
  return next;
}
