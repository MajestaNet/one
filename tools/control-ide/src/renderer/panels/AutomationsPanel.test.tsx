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
          ownership: "custom",
          description: "Creates an Account from a Contact.",
          active: true,
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

  it("runs a callable automation and patches active", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/metadata/v1/automations" && !init?.method) {
        return {
          automations: [
            {
              apiName: "CreateAccount_From_Contact",
              label: "Create Account from Contact",
              objectApiName: "Contact",
              triggerEvent: "create",
              runtime: "code",
              active: true,
              entryFile: "src/automations/create_account_from_contact.ts",
            },
          ],
        };
      }
      if (path === "/metadata/v1/automations/CreateAccount_From_Contact" && init?.method === "PATCH") {
        return { apiName: "CreateAccount_From_Contact", active: false };
      }
      if (path === "/client/v1/automations/CreateAccount_From_Contact/runs") {
        return { id: "run-1", status: "completed", automationApiName: "CreateAccount_From_Contact" };
      }
      return {};
    });
    render(
      <AutomationsPanel
        bridge={{ session: { baseUrl: "http://x", token: "t" }, setSession: vi.fn(), fetch }}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("automations-list")).toBeTruthy());
    await user.click(screen.getByTestId("automation-CreateAccount_From_Contact"));
    expect(await screen.findByTestId("automations-run")).toBeTruthy();
    await user.click(screen.getByTestId("automations-run"));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/client/v1/automations/CreateAccount_From_Contact/runs",
        expect.objectContaining({ method: "POST" }),
      ),
    );
    expect(await screen.findByTestId("automations-run-status")).toBeTruthy();
    await user.click(screen.getByTestId("automations-active"));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/metadata/v1/automations/CreateAccount_From_Contact",
        expect.objectContaining({
          method: "PATCH",
          body: JSON.stringify({ active: false }),
        }),
      ),
    );
  });

  it("shows description, ownership, and filters package vs custom", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn(async (path: string) => {
      if (path === "/metadata/v1/automations") {
        return {
          automations: [
            {
              apiName: "CreateAccount_From_Contact",
              label: "Create Account from Contact",
              objectApiName: "Contact",
              ownership: "custom",
              description: "Creates an Account from a Contact.",
              active: true,
            },
            {
              apiName: "Lead_ConvertOnConvertedStatus",
              label: "Convert lead on Converted",
              objectApiName: "Lead",
              ownership: "managed",
              packageName: "lead_marketing",
              description: "When Lead.Status becomes Converted, runs lead.convert.",
              active: true,
              triggerEvent: "update",
              execution: "sync",
              source: "export default async function run(ctx) { await ctx.invokeAction({ apiName: 'lead.convert' }); }",
            },
          ],
        };
      }
      if (path === "/metadata/v1/automations/Lead_ConvertOnConvertedStatus") {
        return {
          apiName: "Lead_ConvertOnConvertedStatus",
          label: "Convert lead on Converted",
          objectApiName: "Lead",
          ownership: "managed",
          packageName: "lead_marketing",
          description: "When Lead.Status becomes Converted, runs lead.convert.",
          active: true,
          triggerEvent: "update",
          execution: "sync",
          source: "export default async function run(ctx) { await ctx.invokeAction({ apiName: 'lead.convert' }); }",
        };
      }
      return {};
    });
    render(
      <AutomationsPanel
        bridge={{ session: { baseUrl: "http://x", token: "t" }, setSession: vi.fn(), fetch }}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("automations-list")).toBeTruthy());
    expect(screen.getByTestId("automation-desc-Lead_ConvertOnConvertedStatus").textContent).toMatch(
      /lead\.convert/i,
    );
    expect(screen.getAllByText("managed").length).toBeGreaterThan(0);
    expect(screen.getAllByText("custom").length).toBeGreaterThan(0);
    await user.click(screen.getByTestId("automations-filter-package"));
    expect(screen.getByTestId("automation-Lead_ConvertOnConvertedStatus")).toBeTruthy();
    expect(screen.queryByTestId("automation-CreateAccount_From_Contact")).toBeNull();
    await user.click(screen.getByTestId("automations-filter-custom"));
    expect(screen.getByTestId("automation-CreateAccount_From_Contact")).toBeTruthy();
    expect(screen.queryByTestId("automation-Lead_ConvertOnConvertedStatus")).toBeNull();
  });

  it("toggles managed active via PATCH and keeps source read-only", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/metadata/v1/automations" && !init?.method) {
        return {
          automations: [
            {
              apiName: "Lead_ConvertOnConvertedStatus",
              label: "Convert lead on Converted",
              objectApiName: "Lead",
              ownership: "managed",
              packageName: "lead_marketing",
              description: "When Lead.Status becomes Converted, runs lead.convert.",
              active: true,
              triggerEvent: "update",
              execution: "sync",
            },
          ],
        };
      }
      if (path === "/metadata/v1/automations/Lead_ConvertOnConvertedStatus" && !init?.method) {
        return {
          apiName: "Lead_ConvertOnConvertedStatus",
          ownership: "managed",
          packageName: "lead_marketing",
          description: "When Lead.Status becomes Converted, runs lead.convert.",
          active: true,
          triggerEvent: "update",
          execution: "sync",
          source: "export default async function run(ctx) { await ctx.invokeAction({ apiName: 'lead.convert' }); }",
        };
      }
      if (path === "/metadata/v1/automations/Lead_ConvertOnConvertedStatus" && init?.method === "PATCH") {
        return { apiName: "Lead_ConvertOnConvertedStatus", active: false, ownership: "managed" };
      }
      return {};
    });
    render(
      <AutomationsPanel
        bridge={{ session: { baseUrl: "http://x", token: "t" }, setSession: vi.fn(), fetch }}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("automations-list")).toBeTruthy());
    await user.click(screen.getByTestId("automation-Lead_ConvertOnConvertedStatus"));
    expect(await screen.findByTestId("automations-managed-source")).toBeTruthy();
    expect(screen.queryByTestId("automations-save")).toBeNull();
    expect(screen.queryByRole("button", { name: /Open YAML/i })).toBeNull();
    await user.click(screen.getByTestId("automations-active"));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/metadata/v1/automations/Lead_ConvertOnConvertedStatus",
        expect.objectContaining({
          method: "PATCH",
          body: JSON.stringify({ active: false }),
        }),
      ),
    );
  });

  it("surfaces 409 PACKAGE_NOT_ENABLED on run now", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/metadata/v1/automations" && !init?.method) {
        return {
          automations: [
            {
              apiName: "Lead_ConvertOnConvertedStatus",
              label: "Convert lead on Converted",
              objectApiName: "Lead",
              ownership: "managed",
              packageName: "lead_marketing",
              active: true,
            },
          ],
        };
      }
      if (path === "/metadata/v1/automations/Lead_ConvertOnConvertedStatus") {
        return {
          apiName: "Lead_ConvertOnConvertedStatus",
          ownership: "managed",
          active: true,
          source: "export default async function run() { return { ok: true }; }",
        };
      }
      if (path === "/client/v1/automations/Lead_ConvertOnConvertedStatus/runs") {
        throw new Error('409 /client/v1/automations/Lead_ConvertOnConvertedStatus/runs: {"error":"PACKAGE_NOT_ENABLED"}');
      }
      return {};
    });
    render(
      <AutomationsPanel
        bridge={{ session: { baseUrl: "http://x", token: "t" }, setSession: vi.fn(), fetch }}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("automations-list")).toBeTruthy());
    await user.click(screen.getByTestId("automation-Lead_ConvertOnConvertedStatus"));
    await user.click(await screen.findByTestId("automations-run"));
    await waitFor(() => expect(screen.getByText(/PACKAGE_NOT_ENABLED/i)).toBeTruthy());
  });
});
