import { ACCOUNT_LAUNCHER, MODES, type LauncherTileId, type WorkspaceMode } from "./types";
import { modeIcon } from "../icons/Icons";
import { BrandMark } from "../ui/BrandMark";

export function ModeLauncher({
  onSelect,
  overlay = false,
  onDismiss,
  allowedModes,
  allowAccount = true,
}: {
  onSelect: (section: LauncherTileId) => void;
  /** When true, renders as animated overlay over the workspace. */
  overlay?: boolean;
  onDismiss?: () => void;
  /** Capability-filtered modes; defaults to all. */
  allowedModes?: WorkspaceMode[];
  /** When false, hide the Settings launcher tile (capability gate). */
  allowAccount?: boolean;
}) {
  const modes = allowedModes?.length
    ? MODES.filter((m) => allowedModes.includes(m.id))
    : MODES;
  const tiles = allowAccount ? [...modes, ACCOUNT_LAUNCHER] : modes;

  const body = (
    <section
      className={`mode-launcher ${overlay ? "mode-launcher-overlay-inner" : ""}`}
      data-testid="mode-launcher"
      aria-label="Choose a workspace mode"
    >
      <div className="mode-launcher-intro">
        <BrandMark variant="lockup" />
        <h1>{overlay ? "Switch mode" : "Customer IDE"}</h1>
        <p className="muted">
          {overlay
            ? "Pick another persona workspace. Your agent stream stays docked."
            : "Pick how you want to work. Business process change should feel like a feature change — intent, review, act, then a thread of record."}
        </p>
      </div>
      <div className="mode-launcher-grid">
        {tiles.map((m) => (
          <button
            key={m.id}
            type="button"
            className="mode-launch-card"
            data-testid={`mode-launch-${m.id}`}
            onClick={() => onSelect(m.id)}
          >
            <span className="mode-launch-icon">{modeIcon(m.id, { size: 22 })}</span>
            <span className="mode-launch-label">{m.label}</span>
            <span className="mode-launch-tagline">{m.tagline}</span>
            <span className="mode-launch-desc">{m.description}</span>
          </button>
        ))}
      </div>
      {overlay && onDismiss ? (
        <button type="button" className="secondary mode-launcher-dismiss" onClick={onDismiss}>
          Stay in current mode
        </button>
      ) : null}
    </section>
  );

  if (!overlay) return body;

  return (
    <div
      className="mode-launcher-backdrop"
      data-testid="mode-launcher-overlay"
      role="dialog"
      aria-modal="true"
      aria-label="Switch mode"
      onPointerDown={(e) => {
        if (e.target === e.currentTarget) onDismiss?.();
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) onDismiss?.();
      }}
    >
      {body}
    </div>
  );
}
