import { useCallback, useEffect, useRef, useState } from "react";

const HOVER_LEAVE_MS = 250;

export type HoverRailOptions = {
  /** Controlled pin. Omit to manage pin locally. */
  pinned?: boolean;
  onPinnedChange?: (pinned: boolean) => void;
  /** Dock the catalog in-flow (empty workspace browse mode). */
  forceExpanded?: boolean;
  /** Increment to collapse a hover overlay immediately after a selection. */
  dismissNonce?: number;
};

/** Shared hover/pin flyout behavior for left and right workspace rails. */
export function useHoverRail(options: HoverRailOptions = {}) {
  const { pinned: pinnedProp, onPinnedChange, forceExpanded = false, dismissNonce } = options;
  const [hoverOpen, setHoverOpen] = useState(false);
  const [uncontrolledPinned, setUncontrolledPinned] = useState(false);
  const leaveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const suppressHover = useRef(false);

  const pinned = pinnedProp ?? uncontrolledPinned;
  const docked = pinned || forceExpanded;
  const open = docked || hoverOpen;

  const clearLeave = useCallback(() => {
    if (leaveTimer.current) {
      clearTimeout(leaveTimer.current);
      leaveTimer.current = null;
    }
  }, []);

  const collapseHover = useCallback(() => {
    suppressHover.current = true;
    clearLeave();
    setHoverOpen(false);
  }, [clearLeave]);

  const setPinned = useCallback(
    (next: boolean) => {
      if (pinnedProp === undefined) setUncontrolledPinned(next);
      onPinnedChange?.(next);
      if (!next) collapseHover();
    },
    [pinnedProp, onPinnedChange, collapseHover],
  );

  const togglePinned = useCallback(() => {
    setPinned(!pinned);
  }, [pinned, setPinned]);

  const openFlyout = useCallback(() => {
    if (suppressHover.current) return;
    clearLeave();
    setHoverOpen(true);
  }, [clearLeave]);

  const scheduleClose = useCallback(() => {
    suppressHover.current = false;
    if (docked) return;
    clearLeave();
    leaveTimer.current = setTimeout(() => setHoverOpen(false), HOVER_LEAVE_MS);
  }, [docked, clearLeave]);

  useEffect(() => () => clearLeave(), [clearLeave]);

  const wasDocked = useRef(docked);
  useEffect(() => {
    const prev = wasDocked.current;
    wasDocked.current = docked;
    if (prev && !docked) collapseHover();
  }, [docked, collapseHover]);

  const dismissNonceRef = useRef(dismissNonce);
  useEffect(() => {
    if (dismissNonce === undefined) return;
    if (dismissNonceRef.current === dismissNonce) return;
    dismissNonceRef.current = dismissNonce;
    collapseHover();
  }, [dismissNonce, collapseHover]);

  const rootPointerHandlers = {
    onPointerEnter: openFlyout,
    onPointerLeave: scheduleClose,
  };

  return {
    open,
    pinned,
    docked,
    setPinned,
    togglePinned,
    collapseHover,
    rootPointerHandlers,
  };
}
