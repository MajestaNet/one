import type { SavedView } from "./types";

const STORAGE_KEY = "one.control.operate.savedViews";

export function loadSavedViews(): SavedView[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(
      (v): v is SavedView =>
        Boolean(v) &&
        typeof v === "object" &&
        typeof (v as SavedView).id === "string" &&
        typeof (v as SavedView).name === "string" &&
        typeof (v as SavedView).objectApiName === "string",
    );
  } catch {
    return [];
  }
}

export function persistSavedViews(views: SavedView[]): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(views));
}

export function viewsForObject(views: SavedView[], objectApiName: string): SavedView[] {
  return views.filter((v) => v.objectApiName === objectApiName);
}

export function upsertSavedView(views: SavedView[], view: SavedView): SavedView[] {
  const next = views.filter((v) => v.id !== view.id);
  next.push(view);
  persistSavedViews(next);
  return next;
}

export function deleteSavedView(views: SavedView[], id: string): SavedView[] {
  const next = views.filter((v) => v.id !== id);
  persistSavedViews(next);
  return next;
}

/**
 * Upgrade path: IDE-local views first (this module). Prefer declarative list-view
 * metadata on the install when a kernel home exists — then sync instead of localStorage.
 */
export const SAVED_VIEWS_UPGRADE_NOTE =
  "Saved views are stored in this IDE (localStorage). A future metadata ListView type can replace local persistence without changing the Operate UX.";
