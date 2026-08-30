import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { UpdateStatusBox } from "./UpdateStatusBox";

afterEach(() => {
  cleanup();
  // @ts-expect-error clear bridge
  delete window.one;
});

async function openPopover(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: /Application updates/i }));
  await screen.findByRole("dialog", { name: /Update status/i });
}

describe("UpdateStatusBox", () => {
  it("shows fallback when Electron bridge is missing", async () => {
    const user = userEvent.setup();
    render(<UpdateStatusBox />);
    expect(screen.getByTestId("update-status")).toBeTruthy();
    await openPopover(user);
    expect(screen.getByRole("dialog").textContent).toMatch(/UPDATE_FEED_URL|ADR-030/);
    expect((screen.getByRole("button", { name: /Check for updates/i }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("loads status and checks for updates when enabled", async () => {
    const user = userEvent.setup();
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      gitStatus: vi.fn(),
      listTree: vi.fn(),
      readText: vi.fn(),
      writeText: vi.fn(),
      getUpdateStatus: vi.fn().mockResolvedValue({ state: "idle", message: "Ready to check for updates." }),
      checkForUpdates: vi.fn().mockResolvedValue({ state: "not-available", message: "You are on the latest version." }),
      installUpdate: vi.fn(),
    };

    render(<UpdateStatusBox />);
    await openPopover(user);
    await waitFor(() => expect(screen.getByText(/Ready to check/i)).toBeTruthy());
    await user.click(screen.getByRole("button", { name: /Check for updates/i }));
    await waitFor(() => expect(screen.getByText(/latest version/i)).toBeTruthy());
    expect(window.one.checkForUpdates).toHaveBeenCalled();
  });

  it("enables restart when update is downloaded", async () => {
    const user = userEvent.setup();
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      gitStatus: vi.fn(),
      listTree: vi.fn(),
      readText: vi.fn(),
      writeText: vi.fn(),
      getUpdateStatus: vi.fn().mockResolvedValue({
        state: "downloaded",
        message: "Update ready — restart to install.",
        version: "0.2.0",
      }),
      checkForUpdates: vi.fn(),
      installUpdate: vi.fn().mockResolvedValue({ ok: true }),
    };

    render(<UpdateStatusBox />);
    await openPopover(user);
    await waitFor(() => expect(screen.getByText(/v0\.2\.0/)).toBeTruthy());
    const restart = screen.getByRole("button", { name: /Restart to update/i });
    expect((restart as HTMLButtonElement).disabled).toBe(false);
    await user.click(restart);
    expect(window.one.installUpdate).toHaveBeenCalled();
  });
});
