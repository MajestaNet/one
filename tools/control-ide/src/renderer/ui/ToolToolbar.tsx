import type { ReactNode } from "react";

/**
 * Standard tool action row: primary/secondary actions on the left, search on
 * the right. Aligns to `PanelHeader` width — do not mix `.row` + ad-hoc search.
 */
export function ToolToolbar({
  actions,
  search,
  meta,
}: {
  actions?: ReactNode;
  /** Prefer `SearchField`. */
  search?: ReactNode;
  meta?: ReactNode;
}) {
  return (
    <div className="tool-toolbar" data-testid="tool-toolbar">
      <div className="tool-toolbar-start">
        {actions}
        {meta}
      </div>
      {search ? <div className="tool-toolbar-end">{search}</div> : null}
    </div>
  );
}
