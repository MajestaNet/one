import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AppBridge } from "../App";
import { DigitalOceanCloudSection } from "./DigitalOceanCloudSection";

afterEach(() => {
  cleanup();
});

function bridge(
  fetchImpl: AppBridge["fetch"],
  session: AppBridge["session"],
): AppBridge {
  return {
    session,
    setSession: vi.fn().mockResolvedValue(undefined),
    fetch: fetchImpl,
  };
}

describe("DigitalOceanCloudSection", () => {
  it("returns null without a session baseUrl", () => {
    const { container } = render(
      <DigitalOceanCloudSection
        bridge={bridge(vi.fn(), null)}
        env={{ capabilities: { digitaloceanCloud: true } }}
      />,
    );
    expect(container.querySelector("[data-testid=do-cloud-section]")).toBeNull();
  });

  it("shows console fallback when capability is off", () => {
    render(
      <DigitalOceanCloudSection
        bridge={bridge(vi.fn(), { baseUrl: "http://api", token: "t" })}
        env={{ capabilities: {} }}
      />,
    );
    expect(screen.getByText(/Cloud hosting not configured/i)).toBeTruthy();
  });

  it("surfaces status errors and non-admin copy", async () => {
    const fetch = vi.fn(async () => {
      throw new Error("token rejected");
    });
    render(
      <DigitalOceanCloudSection
        bridge={bridge(fetch, { baseUrl: "http://api", token: "t", isAdmin: false })}
        env={{ capabilities: { digitaloceanCloud: true } }}
      />,
    );
    await waitFor(() => expect(screen.getByText(/token rejected/i)).toBeTruthy());
    expect(screen.getByText(/without admin/i)).toBeTruthy();
  });

  it("tolerates app/list failures and refreshes", async () => {
    const user = userEvent.setup();
    let n = 0;
    const fetch = vi.fn(async (path: string) => {
      n += 1;
      if (path.endsWith("/status")) {
        return { configured: true, reachable: false, binding: { appId: "a", databaseId: "d" } };
      }
      if (path.endsWith("/app")) throw new Error("app down");
      if (path.endsWith("/environments")) throw new Error("envs down");
      return {};
    });
    render(
      <DigitalOceanCloudSection
        bridge={bridge(fetch, { baseUrl: "http://api", token: "t", isAdmin: true })}
        env={{ capabilities: { digitaloceanCloud: true } }}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("do-scale-save")).toBeTruthy());
    const before = n;
    await user.click(screen.getByRole("button", { name: /Refresh hosting status/i }));
    await waitFor(() => expect(n).toBeGreaterThan(before));
  });

  it("shows mutation errors from scale", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn(async (path: string, init?: RequestInit) => {
      if (path.endsWith("/status")) {
        return { configured: true, reachable: true, binding: { appId: "a", databaseId: "d" } };
      }
      if (path.endsWith("/app") && init?.method !== "PATCH") {
        return { apiInstanceCount: 1, apiInstanceSizeSlug: "apps-s-1vcpu-1gb" };
      }
      if (path.endsWith("/environments")) return { provisionRuns: [] };
      if (path.endsWith("/app/scale")) throw new Error("scale denied");
      return {};
    });
    render(
      <DigitalOceanCloudSection
        bridge={bridge(fetch, { baseUrl: "http://api", token: "t", isAdmin: true })}
        env={{ capabilities: { digitaloceanCloud: true } }}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("do-scale-save")).toBeTruthy());
    await user.click(screen.getByTestId("do-scale-save"));
    await waitFor(() => expect(screen.getByText(/scale denied/i)).toBeTruthy());
  });
});
