import type { ReactNode } from "react";

export function PanelHeader({
  title,
  subtitle,
  actions,
}: {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
}) {
  return (
    <header className="panel-header">
      <div className="panel-header-text">
        <h2>{title}</h2>
        {subtitle ? <p className="muted panel-subtitle">{subtitle}</p> : null}
      </div>
      {actions ? <div className="panel-header-actions">{actions}</div> : null}
    </header>
  );
}
