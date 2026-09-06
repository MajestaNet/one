import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AppBridge } from "../App";
import { AccountSettingsPanel } from "./AccountSettingsPanel";

afterEach(() => cleanup());

function bridge(): AppBridge {
  return {
    session: {
      activeInstallId: "prod",
      baseUrl: "https://api.example",
      token: "jwt",
      repoPath: "/repo",
      isAdmin: true,
      scopes: ["client", "metadata"],
      systemPermissions: ["ide.settings", "ide.settings.account"],
      environments: [
        {
          installId: "prod",
          installRole: "production",
          baseUrl: "https://api.example",
          token: "jwt",
        },
      ],
    },
    setSession: vi.fn().mockResolvedValue(undefined),
    fetch: vi.fn().mockResolvedValue({}),
  };
}

describe("AccountSettingsPanel", () => {
  it("uses the standard ToolSurface frame so the tool fills the workspace tile", () => {
    render(<AccountSettingsPanel bridge={bridge()} />);
    const root = screen.getByTestId("account-settings-panel");
    expect(root.className.split(/\s+/)).toEqual(expect.arrayContaining(["tool-surface", "account-settings-panel"]));
    expect(root.getAttribute("data-tool-surface")).toBe("true");
    expect(root.querySelector(".tool-surface-body")).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Account settings" })).toBeTruthy();
    expect(screen.getByText("Session active")).toBeTruthy();
    expect(screen.getByRole("heading", { name: "production" })).toBeTruthy();
    expect(screen.getByText("https://api.example")).toBeTruthy();
    expect(screen.getByTestId("account-caps").textContent).toContain("ide.settings.account");
    expect(screen.getByTestId("account-password")).toBeTruthy();
  });

  it("changes password and lists devices from Client APIs", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/devices") {
        return { devices: [{ deviceId: "dev-1", label: "Laptop" }] };
      }
      if (path === "/client/v1/me/password") return {};
      if (String(path).includes("/revoke")) return {};
      return {};
    });
    const b = bridge();
    b.fetch = fetch;
    render(<AccountSettingsPanel bridge={b} />);
    expect(await screen.findByTestId("account-devices")).toBeTruthy();
    expect(screen.getByText("Laptop")).toBeTruthy();
    await user.type(screen.getByTestId("account-password-current"), "old");
    await user.type(screen.getByTestId("account-password-new"), "newpw");
    await user.click(screen.getByTestId("account-password-save"));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/client/v1/me/password",
        expect.objectContaining({ method: "POST" }),
      ),
    );
    await user.click(screen.getByTestId("account-device-revoke-dev-1"));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith("/client/v1/devices/dev-1/revoke", { method: "POST" }),
    );
  });
});
