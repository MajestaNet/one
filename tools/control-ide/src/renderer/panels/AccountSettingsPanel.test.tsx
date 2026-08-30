import { cleanup, render, screen } from "@testing-library/react";
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
  });
});
