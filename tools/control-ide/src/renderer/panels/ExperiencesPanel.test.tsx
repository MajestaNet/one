import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AppBridge } from "../App";
import { ExperiencesPanel } from "./ExperiencesPanel";
import { upsertEnvironment } from "../session";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function bridge(fetchImpl?: AppBridge["fetch"], connected = true): AppBridge {
  const session = connected
    ? upsertEnvironment(null, {
        installId: "dev",
        installRole: "test",
        baseUrl: "http://api",
        token: "jwt",
      })
    : null;
  return {
    session,
    setSession: vi.fn().mockResolvedValue(undefined),
    fetch: fetchImpl ?? vi.fn(),
  };
}

describe("ExperiencesPanel", () => {
  it("shows connect empty state when disconnected", () => {
    render(<ExperiencesPanel bridge={bridge(undefined, false)} />);
    expect(screen.getByText(/Not connected/i)).toBeTruthy();
  });

  it("lists experiences from metadata API", async () => {
    const fetch = vi.fn().mockResolvedValue({
      experiences: [
        {
          apiName: "portal",
          label: "Customer Portal",
          homeUrl: "https://portal.example",
          connectedAppApiName: "PortalApp",
          active: true,
        },
      ],
    });
    render(<ExperiencesPanel bridge={bridge(fetch)} />);
    expect(await screen.findByText("portal")).toBeTruthy();
    expect(screen.getByText("Customer Portal")).toBeTruthy();
    expect(screen.getByRole("link", { name: "https://portal.example" })).toBeTruthy();
    expect(screen.getByText("PortalApp")).toBeTruthy();
    expect(screen.getByText("Yes")).toBeTruthy();
    expect(fetch).toHaveBeenCalledWith("/metadata/v1/experiences");
  });

  it("shows empty state when install has no experiences", async () => {
    const fetch = vi.fn().mockResolvedValue({ experiences: [] });
    render(<ExperiencesPanel bridge={bridge(fetch)} />);
    expect(await screen.findByText(/No experiences/i)).toBeTruthy();
  });

  it("surfaces load errors", async () => {
    const fetch = vi.fn().mockRejectedValue(new Error("boom"));
    render(<ExperiencesPanel bridge={bridge(fetch)} />);
    await waitFor(() => expect(screen.getByText(/boom/)).toBeTruthy());
  });
});
