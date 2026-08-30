import { modeIcon } from "../icons/Icons";
import type { AppSection, WorkspaceMode } from "./types";
import { ACCOUNT_LAUNCHER, MODES } from "./types";

/**
 * Centered mode control. Hover animates; click opens the mode launcher overlay
 * so the left rail/submenu can stay gone. Account is the sixth launcher tile.
 */
export function ModeTitle({
  section,
  launcherOpen,
  onToggleLauncher,
}: {
  section: AppSection;
  launcherOpen: boolean;
  onToggleLauncher: () => void;
}) {
  const isAccount = section === "settings";
  const meta = !isAccount ? MODES.find((m) => m.id === (section as WorkspaceMode)) : undefined;
  const label = isAccount ? ACCOUNT_LAUNCHER.label : (meta?.label ?? section);
  return (
    <button
      type="button"
      className={`mode-title ${launcherOpen ? "open" : ""}`}
      data-testid="mode-title"
      aria-expanded={launcherOpen}
      aria-haspopup="dialog"
      title={isAccount ? "Switch mode — leave Account" : "Switch mode — hover & click to reopen mode tiles"}
      onClick={onToggleLauncher}
    >
      <span className="mode-title-icon" aria-hidden>
        {modeIcon(section, { size: 16 })}
      </span>
      <span className="mode-title-label" data-testid="active-mode">
        {label}
      </span>
      <span className="mode-title-hint muted">Switch mode</span>
    </button>
  );
}
