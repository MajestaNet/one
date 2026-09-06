import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AppBridge } from "../App";
import { SharingPanel } from "./SharingPanel";
import { upsertEnvironment } from "../session";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function bridge(fetchImpl: AppBridge["fetch"]): AppBridge {
  return {
    session: upsertEnvironment(null, {
      installId: "dev",
      installRole: "demo",
      baseUrl: "http://api",
      token: "jwt",
    }),
    setSession: vi.fn().mockResolvedValue(undefined),
    fetch: fetchImpl,
  };
}

describe("SharingPanel", () => {
  it("enables sharing then lists objects from Metadata", async () => {
    const user = userEvent.setup();
    const fetchImpl = vi
      .fn()
      .mockResolvedValueOnce({ recordSharingEnabled: false })
      .mockResolvedValueOnce({ recordSharingEnabled: true })
      .mockResolvedValueOnce({
        objects: [{ objectApiName: "Account", defaultAccess: "public_read" }],
      });
    render(<SharingPanel bridge={bridge(fetchImpl)} />);
    expect(await screen.findByText(/Record sharing off/i)).toBeTruthy();
    await user.click(screen.getByTestId("sharing-enable"));
    await waitFor(() => expect(screen.getByText("Account")).toBeTruthy());
    expect(fetchImpl).toHaveBeenCalledWith(
      "/metadata/v1/sharing/enable",
      expect.objectContaining({ method: "POST" }),
    );
  });
});
