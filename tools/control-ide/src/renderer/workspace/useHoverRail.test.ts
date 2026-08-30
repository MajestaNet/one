import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useHoverRail } from "./useHoverRail";

describe("useHoverRail", () => {
  it("opens on pointer enter and closes after leave when not pinned", () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useHoverRail());
    expect(result.current.open).toBe(false);

    act(() => {
      result.current.rootPointerHandlers.onPointerEnter();
    });
    expect(result.current.open).toBe(true);

    act(() => {
      result.current.rootPointerHandlers.onPointerLeave();
      vi.advanceTimersByTime(250);
    });
    expect(result.current.open).toBe(false);
    vi.useRealTimers();
  });

  it("does not call onPinnedChange from inside the pin updater after toggle", () => {
    const onPinnedChange = vi.fn();
    const { result } = renderHook(() => useHoverRail({ onPinnedChange }));
    act(() => {
      result.current.togglePinned();
    });
    expect(onPinnedChange).toHaveBeenCalledTimes(1);
    expect(onPinnedChange).toHaveBeenCalledWith(true);
    expect(result.current.pinned).toBe(true);
    expect(result.current.docked).toBe(true);
  });

  it("setPinned(false) collapses hover and notifies the parent", () => {
    const onPinnedChange = vi.fn();
    const { result } = renderHook(() => useHoverRail({ onPinnedChange }));
    act(() => result.current.togglePinned());
    onPinnedChange.mockClear();
    act(() => result.current.setPinned(false));
    expect(onPinnedChange).toHaveBeenCalledWith(false);
    expect(result.current.pinned).toBe(false);
    expect(result.current.open).toBe(false);
  });

  it("forceExpanded docks the catalog without pinning", () => {
    const { result } = renderHook(() => useHoverRail({ forceExpanded: true }));
    expect(result.current.open).toBe(true);
    expect(result.current.docked).toBe(true);
    expect(result.current.pinned).toBe(false);
  });

  it("dismissNonce collapses hover and ignores further enter until leave", () => {
    const { result, rerender } = renderHook(
      ({ dismissNonce }: { dismissNonce: number }) => useHoverRail({ dismissNonce }),
      { initialProps: { dismissNonce: 0 } },
    );
    act(() => {
      result.current.rootPointerHandlers.onPointerEnter();
    });
    expect(result.current.open).toBe(true);

    rerender({ dismissNonce: 1 });
    expect(result.current.open).toBe(false);

    act(() => {
      result.current.rootPointerHandlers.onPointerEnter();
    });
    expect(result.current.open).toBe(false);

    act(() => {
      result.current.rootPointerHandlers.onPointerLeave();
      result.current.rootPointerHandlers.onPointerEnter();
    });
    expect(result.current.open).toBe(true);
  });

  it("honors controlled pinned without local drift on setPinned", () => {
    const onPinnedChange = vi.fn();
    const { result, rerender } = renderHook(
      ({ pinned }: { pinned: boolean }) => useHoverRail({ pinned, onPinnedChange }),
      { initialProps: { pinned: true } },
    );
    expect(result.current.pinned).toBe(true);
    act(() => result.current.setPinned(false));
    expect(onPinnedChange).toHaveBeenCalledWith(false);
    expect(result.current.pinned).toBe(true);
    rerender({ pinned: false });
    expect(result.current.pinned).toBe(false);
    expect(result.current.docked).toBe(false);
  });

  it("collapses hover when forceExpanded turns off so the overlay does not flicker back open", () => {
    const { result, rerender } = renderHook(
      ({ forceExpanded }: { forceExpanded: boolean }) => useHoverRail({ forceExpanded }),
      { initialProps: { forceExpanded: true } },
    );
    expect(result.current.open).toBe(true);
    rerender({ forceExpanded: false });
    expect(result.current.docked).toBe(false);
    expect(result.current.open).toBe(false);
    act(() => {
      result.current.rootPointerHandlers.onPointerEnter();
    });
    expect(result.current.open).toBe(false);
  });
});
