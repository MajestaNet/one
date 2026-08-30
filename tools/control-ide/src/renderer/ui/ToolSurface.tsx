import type { ReactNode } from "react";
import { PanelHeader } from "./PanelHeader";

/**
 * Standard workspace tool frame (every left-rail tool except Operate graph).
 *
 * Prefer passing `title` so the header stays put and `children` scroll. When
 * migrating an existing panel, wrapping the current root is enough — keep the
 * inner `PanelHeader` until you switch to the title prop.
 */
export function ToolSurface({
  title,
  subtitle,
  actions,
  toolbar,
  children,
  testId,
  className,
}: {
  title?: string;
  subtitle?: string;
  actions?: ReactNode;
  toolbar?: ReactNode;
  children?: ReactNode;
  testId?: string;
  className?: string;
}) {
  const hasChrome = Boolean(title) || Boolean(toolbar);
  return (
    <div
      className={["tool-surface", className].filter(Boolean).join(" ")}
      data-testid={testId}
      data-tool-surface="true"
    >
      {title ? <PanelHeader title={title} subtitle={subtitle} actions={actions} /> : null}
      {toolbar}
      {hasChrome ? <div className="tool-surface-body">{children}</div> : children}
    </div>
  );
}
