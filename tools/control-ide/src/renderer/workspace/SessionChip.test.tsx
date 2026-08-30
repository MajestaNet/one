import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SessionChip } from "./SessionChip";
import { upsertEnvironment } from "../session";

afterEach(() => {
  cleanup();
});

describe("SessionChip", () => {
  it("shows Not connected when there is no session", () => {
    render(<SessionChip session={null} onOpenAccount={() => undefined} />);
    expect(screen.getByTestId("session-chip").textContent).toMatch(/Not connected/);
  });

  it("shows initials and username, not JWT labels", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const session = upsertEnvironment(null, {
      installId: "local",
      installRole: "local",
      baseUrl: "http://localhost:8080",
      token: "jwt",
      displayName: "Ada Lovelace",
      email: "ada@example.com",
    });
    render(<SessionChip session={session} onOpenAccount={onOpen} />);
    const chip = screen.getByTestId("session-chip");
    expect(chip.textContent).toContain("AL");
    expect(chip.textContent).toContain("Ada Lovelace");
    expect(chip.textContent).not.toMatch(/JWT/i);
    expect(chip.textContent).not.toMatch(/ephemeral/i);
    await user.click(chip);
    expect(onOpen).toHaveBeenCalled();
  });

  it("falls back to Signed in when the actor has no name", () => {
    const session = upsertEnvironment(null, {
      installId: "local",
      installRole: "local",
      baseUrl: "http://localhost:8080",
      token: "jwt",
    });
    render(<SessionChip session={session} onOpenAccount={() => undefined} />);
    expect(screen.getByTestId("session-chip").textContent).toMatch(/Signed in/);
  });
});
