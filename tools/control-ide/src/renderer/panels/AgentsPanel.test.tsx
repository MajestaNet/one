import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AppBridge } from "../App";
import { AgentsPanel } from "./AgentsPanel";
import { upsertEnvironment } from "../session";

afterEach(() => {
  cleanup();
  Reflect.deleteProperty(window, "one");
  vi.restoreAllMocks();
});

function bridge(fetchImpl?: AppBridge["fetch"], repoPath?: string): AppBridge {
  let session = upsertEnvironment(null, {
    installId: "dev",
    installRole: "test",
    baseUrl: "http://api",
    token: "jwt",
  });
  if (repoPath) session = { ...session, repoPath };
  return {
    session,
    setSession: vi.fn().mockResolvedValue(undefined),
    fetch: fetchImpl ?? vi.fn(),
  };
}

const harnessCatalog = {
  harnesses: [
    {
      id: "harness.operate.query",
      section: "operate",
      version: "1",
      label: "Operate query",
      job: "Query / ask",
      toolFloor: ["sobjects.read", "query"],
      requireApprovalDefault: true,
    },
  ],
};

describe("AgentsPanel", () => {
  it("lists playbooks and opens detail", async () => {
    const user = userEvent.setup();
    const fetch = vi
      .fn()
      .mockResolvedValueOnce({
        playbooks: [
          {
            apiName: "MetadataBuilder",
            label: "Metadata builder",
            ownership: "custom",
            active: true,
            requireApproval: true,
            primarySection: "build",
            harnessId: "harness.build.metadata",
          },
        ],
      })
      .mockResolvedValueOnce(harnessCatalog)
      .mockResolvedValueOnce({
        apiName: "MetadataBuilder",
        label: "Metadata builder",
        ownership: "custom",
        instructions: "Build metadata",
        active: true,
        requireApproval: true,
        allowedTools: ["query"],
        primarySection: "build",
        harnessId: "harness.build.metadata",
      });
    render(<AgentsPanel bridge={bridge(fetch)} />);
    expect(await screen.findByTestId("agents-list")).toBeTruthy();
    const toolbar = screen.getByTestId("tool-toolbar");
    expect(within(toolbar).getByTestId("agents-new").textContent).toMatch(/New agent/i);
    expect(within(toolbar).getByRole("button", { name: /Refresh/i })).toBeTruthy();
    expect(within(toolbar).getByTestId("agents-search")).toBeTruthy();
    expect(within(screen.getByTestId("agents-list")).getByText("Metadata builder")).toBeTruthy();
    expect(within(screen.getByTestId("agents-list")).getByText("Build")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /Open Metadata builder/i }));
    expect(await screen.findByTestId("agents-detail")).toBeTruthy();
    expect(screen.getByDisplayValue("Build metadata")).toBeTruthy();
    expect(screen.getByText("harness.build.metadata")).toBeTruthy();
  });

  it("creates a playbook via section wizard and mirrors yaml", async () => {
    const user = userEvent.setup();
    const onCatalogChanged = vi.fn();
    const writeText = vi.fn().mockResolvedValue(true);
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      gitStatus: vi.fn(),
      listTree: vi.fn(),
      readText: vi.fn(),
      writeText,
    };
    const fetch = vi.fn().mockImplementation(async (path: string, init?: RequestInit) => {
      if (path === "/metadata/v1/agents/harnesses") return harnessCatalog;
      if (path === "/metadata/v1/agents/playbooks" && (!init || init.method === undefined || init.method === "GET")) {
        return { playbooks: [] };
      }
      if (path === "/metadata/v1/agents/playbooks" && init?.method === "POST") {
        const body = JSON.parse(String(init.body));
        expect(body.primarySection).toBe("operate");
        return {
          apiName: body.apiName,
          label: body.label,
          ownership: "custom",
          active: true,
          requireApproval: true,
          allowedTools: body.allowedTools,
          primarySection: "operate",
          harnessId: "harness.operate.query",
          harnessVersion: "1",
        };
      }
      if (String(path).includes("/agents/playbooks/")) {
        return {
          apiName: "Queryassistant__c",
          label: "Query assistant",
          ownership: "custom",
          instructions: "Help",
          active: true,
          allowedTools: ["query"],
          primarySection: "operate",
          harnessId: "harness.operate.query",
        };
      }
      return { playbooks: [{ apiName: "Queryassistant__c", label: "Query assistant", ownership: "custom", active: true, primarySection: "operate" }] };
    });
    render(<AgentsPanel bridge={bridge(fetch, "/tmp/repo")} onCatalogChanged={onCatalogChanged} />);
    await screen.findByTestId("agents-list");
    await user.click(screen.getByTestId("agents-new"));
    const form = await screen.findByTestId("agents-new-form");
    expect(form).toBeTruthy();
    await user.click(screen.getByTestId("agents-section-operate"));
    await user.click(screen.getByTestId("agents-wizard-next")); // harness
    await user.click(screen.getByTestId("agents-wizard-next")); // identity
    await user.type(screen.getByTestId("agents-wizard-label"), "Query assistant");
    await user.click(screen.getByTestId("agents-wizard-next")); // behavior
    await user.click(screen.getByTestId("agents-wizard-next")); // review
    await user.click(screen.getByTestId("agents-wizard-create"));
    await waitFor(() => expect(writeText).toHaveBeenCalled());
    expect(writeText.mock.calls[0][1]).toBe("metadata/agents/playbooks/Queryassistant__c.yaml");
    expect(String(writeText.mock.calls[0][2])).toContain("primarySection: operate");
    expect(onCatalogChanged).toHaveBeenCalled();
    expect(await screen.findByTestId("agents-create-toast")).toBeTruthy();
  });

  it("moves primary section after confirm", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn().mockImplementation(async (path: string, init?: RequestInit) => {
      if (path === "/metadata/v1/agents/harnesses") return harnessCatalog;
      if (path === "/metadata/v1/agents/playbooks" && (!init || !init.method || init.method === "GET")) {
        return {
          playbooks: [
            {
              apiName: "CustomAgent__c",
              label: "Custom",
              ownership: "custom",
              active: true,
              primarySection: "operate",
              harnessId: "harness.operate.query",
            },
          ],
        };
      }
      if (String(path).includes("/agents/playbooks/CustomAgent__c") && init?.method === "PATCH") {
        const body = JSON.parse(String(init.body));
        expect(body.primarySection).toBe("govern");
        return {
          apiName: "CustomAgent__c",
          label: "Custom",
          ownership: "custom",
          instructions: "x",
          active: true,
          primarySection: "govern",
          harnessId: "harness.govern.admin",
          harnessVersion: "1",
          allowedTools: ["sobjects.read", "query"],
        };
      }
      if (String(path).includes("/agents/playbooks/CustomAgent__c")) {
        return {
          apiName: "CustomAgent__c",
          label: "Custom",
          ownership: "custom",
          instructions: "x",
          active: true,
          primarySection: "operate",
          harnessId: "harness.operate.query",
          allowedTools: ["query"],
        };
      }
      return { playbooks: [] };
    });
    render(<AgentsPanel bridge={bridge(fetch)} />);
    await screen.findByTestId("agents-list");
    await user.click(screen.getByRole("button", { name: /Open Custom/i }));
    expect(await screen.findByTestId("agents-move-section")).toBeTruthy();
    await user.selectOptions(screen.getByTestId("agents-move-select"), "govern");
    expect((screen.getByTestId("agents-move-submit") as HTMLButtonElement).disabled).toBe(true);
    await user.click(screen.getByTestId("agents-move-confirm"));
    await user.click(screen.getByTestId("agents-move-submit"));
    await waitFor(() => expect(screen.getByText(/Moved to Govern dock/i)).toBeTruthy());
  });

  it("filters list by section chip", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn().mockImplementation(async (path: string) => {
      if (path === "/metadata/v1/agents/harnesses") return harnessCatalog;
      return {
        playbooks: [
          {
            apiName: "A__c",
            label: "Operate agent",
            ownership: "custom",
            active: true,
            primarySection: "operate",
          },
          {
            apiName: "B__c",
            label: "Build agent",
            ownership: "custom",
            active: false,
            primarySection: "build",
            requireApproval: true,
          },
        ],
      };
    });
    render(<AgentsPanel bridge={bridge(fetch)} />);
    await screen.findByTestId("agents-list");
    expect(screen.getByText("Operate agent")).toBeTruthy();
    expect(screen.getByText("Build agent")).toBeTruthy();
    await user.click(screen.getByTestId("agents-filter-build"));
    expect(screen.queryByText("Operate agent")).toBeNull();
    expect(screen.getByText("Build agent")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /Open Build agent/i }));
    expect(await screen.findByTestId("agents-detail")).toBeTruthy();
  });

  it("opens detail via keyboard and cancels wizard", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn().mockImplementation(async (path: string) => {
      if (path === "/metadata/v1/agents/harnesses") return harnessCatalog;
      if (String(path).includes("/agents/playbooks/A__c")) {
        return {
          apiName: "A__c",
          label: "Operate agent",
          ownership: "managed",
          instructions: "managed",
          active: true,
          primarySection: "operate",
          harnessId: "harness.operate.query",
          allowedTools: ["query"],
        };
      }
      return {
        playbooks: [
          {
            apiName: "A__c",
            label: "Operate agent",
            ownership: "managed",
            active: true,
            primarySection: "operate",
          },
        ],
      };
    });
    render(<AgentsPanel bridge={bridge(fetch)} />);
    await screen.findByTestId("agents-list");
    await user.click(screen.getByTestId("agents-new"));
    expect(await screen.findByTestId("agents-new-form")).toBeTruthy();
    await user.click(screen.getByTestId("agents-wizard-back")); // still on section, disabled-ish but clickable path
    await user.click(screen.getByTestId("agents-new")); // cancel
    expect(screen.queryByTestId("agents-new-form")).toBeNull();
    const row = screen.getByTestId("agents-row-A__c");
    row.focus();
    await user.keyboard("{Enter}");
    expect(await screen.findByTestId("agents-detail")).toBeTruthy();
    expect(screen.getByText(/read-only for managed/i)).toBeTruthy();
  });

  it("shows connect empty state without session", () => {
    render(
      <AgentsPanel bridge={{ session: null, setSession: vi.fn(), fetch: vi.fn() }} />,
    );
    expect(screen.getByText(/Connect an environment/i)).toBeTruthy();
  });
});
