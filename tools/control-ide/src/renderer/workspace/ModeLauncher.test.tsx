import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ModeLauncher } from "./ModeLauncher";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("ModeLauncher overlay", () => {
  it("dismisses on backdrop pointerdown", () => {
    const onDismiss = vi.fn();
    render(<ModeLauncher overlay onSelect={vi.fn()} onDismiss={onDismiss} />);
    fireEvent.pointerDown(screen.getByTestId("mode-launcher-overlay"));
    expect(onDismiss).toHaveBeenCalled();
  });

  it("exposes a stay-in-mode dismiss control", async () => {
    const user = userEvent.setup();
    const onDismiss = vi.fn();
    render(<ModeLauncher overlay onSelect={vi.fn()} onDismiss={onDismiss} />);
    await user.click(screen.getByRole("button", { name: /stay in current mode/i }));
    expect(onDismiss).toHaveBeenCalled();
  });
});
