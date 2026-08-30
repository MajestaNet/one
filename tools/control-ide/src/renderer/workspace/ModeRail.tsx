import { MODES, type WorkspaceMode } from "./types";
import { IconHome, modeIcon } from "../icons/Icons";

export function ModeRail({
  mode,
  onModeChange,
  onHome,
}: {
  mode: WorkspaceMode;
  onModeChange: (m: WorkspaceMode) => void;
  onHome?: () => void;
}) {
  return (
    <aside className="mode-rail" aria-label="Workspace modes">
      {onHome && (
        <button type="button" className="mode-home" onClick={onHome} data-testid="mode-home" title="All modes">
          <IconHome size={14} /> All modes
        </button>
      )}
      <p className="mode-rail-title">Modes</p>
      {MODES.map((m) => (
        <button
          key={m.id}
          type="button"
          className={mode === m.id ? "mode-btn active" : "mode-btn"}
          onClick={() => onModeChange(m.id)}
          aria-pressed={mode === m.id}
        >
          {modeIcon(m.id, { size: 15 })}
          <span className="mode-label">{m.label}</span>
        </button>
      ))}
    </aside>
  );
}
