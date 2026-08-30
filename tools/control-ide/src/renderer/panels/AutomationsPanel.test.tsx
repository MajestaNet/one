import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AppBridge } from "../App";
import { AutomationsPanel } from "./AutomationsPanel";

afterEach(() => {
  cleanup();
  Reflect.deleteProperty(window, "one");
});

function bridge(session: AppBridge["session"] = { baseUrl: "http://x", token: "t" }): AppBridge {
  return {
    session,
    setSession: vi.fn().mockResolvedValue(undefined),
    fetch: vi.fn().mockResolvedValue({
      automations: [
        {
          apiName: "CreateAccount_From_Contact",
          label: "Create Account from Contact",
          objectApiName: "Contact",
          triggerEvent: "create",
          runtime: "code",
          entryFile: "src/automations/create_account_from_contact.ts",
        },
      ],
    }),
  };
}

describe("AutomationsPanel", () => {
  it("shows connect empty state without session", () => {
    render(<AutomationsPanel bridge={bridge(null)} />);
    expect(screen.getByText(/Connect to build automations/i)).toBeTruthy();
  });

  it("lists automations from Metadata API", async () => {
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      gitStatus: vi.fn(),
      listTree: vi.fn(),
      readText: vi.fn().mockResolvedValue("export default async function run() { return { ok: true }; }"),
      writeText: vi.fn(),
    };
    render(<AutomationsPanel bridge={bridge({ baseUrl: "http://x", token: "t", repoPath: "/tmp/r" })} />);
    await waitFor(() => expect(screen.getByTestId("automations-list")).toBeTruthy());
    expect(screen.getByTestId("automation-CreateAccount_From_Contact")).toBeTruthy();
  });

  it("creates a code automation via Metadata API", async () => {
    const user = userEvent.setup();
    const fetch = vi
      .fn()
      .mockResolvedValueOnce({ automations: [] })
      .mockResolvedValueOnce({})
      .mockResolvedValue({
        automations: [
          {
            apiName: "MyAuto",
            label: "My Auto",
            objectApiName: "Contact",
            runtime: "code",
            entryFile: "src/automations/my_auto.ts",
          },
        ],
      });
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      gitStatus: vi.fn(),
      listTree: vi.fn(),
      readText: vi.fn().mockRejectedValue(new Error("missing")),
      writeText: vi.fn().mockResolvedValue(true),
    };
    render(<AutomationsPanel bridge={{ session: { baseUrl: "http://x", token: "t", repoPath: "/tmp/r" }, setSession: vi.fn(), fetch }} />);
    await user.click(screen.getByTestId("automations-new"));
    await user.type(screen.getByTestId("automations-api-name"), "MyAuto");
    await user.type(screen.getByPlaceholderText(/Create Account from Contact/i), "My Auto");
    await user.click(screen.getByTestId("automations-create-btn"));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/metadata/v1/automations",
        expect.objectContaining({ method: "POST" }),
      ),
    );
  });
});
