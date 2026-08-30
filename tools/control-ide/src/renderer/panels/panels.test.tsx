import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AppBridge } from "../App";
import { EnvPanel } from "./EnvPanel";
import { HostingPanel } from "./HostingPanel";
import { ClientPanel } from "./ClientPanel";
import { CrmPanel } from "./CrmPanel";
import { DeployPanel } from "./DeployPanel";
import { IntegrationsPanel } from "./IntegrationsPanel";
import { PermissionsPanel } from "./PermissionsPanel";
import { UsersPanel } from "./UsersPanel";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function bridge(fetchImpl?: AppBridge["fetch"], session: AppBridge["session"] = { baseUrl: "http://api", token: "jwt" }): AppBridge {
  return {
    session,
    setSession: vi.fn().mockResolvedValue(undefined),
    fetch: fetchImpl ?? vi.fn(),
  };
}

describe("EnvPanel", () => {
  it("loads environment card", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn().mockResolvedValue({ customerId: "acme", installId: "dev" });
    render(<EnvPanel bridge={bridge(fetch)} />);
    await user.click(screen.getAllByRole("button", { name: /Refresh environment/i })[0]);
    await waitFor(() => expect(screen.getByTestId("env-card")).toBeTruthy());
    expect(screen.getByText("acme")).toBeTruthy();
    expect(fetch).toHaveBeenCalledWith("/deploy/v1/environment");
    expect(screen.queryByTestId("do-cloud-section")).toBeNull();
  });

  it("does not embed cloud hosting admin (moved to Settings → Hosting)", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn().mockResolvedValue({
      customerId: "acme",
      installId: "prod",
      cloudHost: "digitalocean",
      capabilities: { cloud: true, digitaloceanCloud: true },
    });
    render(<EnvPanel bridge={bridge(fetch)} />);
    await user.click(screen.getAllByRole("button", { name: /Refresh environment/i })[0]);
    await waitFor(() => expect(screen.getByTestId("env-card")).toBeTruthy());
    expect(screen.queryByTestId("do-cloud-section")).toBeNull();
    expect(screen.queryByTestId("do-scale-save")).toBeNull();
  });

  it("maps customerRepoUrl and copies clone URL", async () => {
    const user = userEvent.setup();
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    const setSession = vi.fn().mockResolvedValue(undefined);
    const fetch = vi.fn().mockResolvedValue({
      customerId: "acme",
      installId: "prod-1",
      installRole: "prod",
      customerRepoUrl: "https://git.example/acme.git",
      capabilities: ["promote"],
    });
    const b = bridge(fetch);
    b.setSession = setSession;
    render(<EnvPanel bridge={b} />);
    await user.click(screen.getAllByRole("button", { name: /Refresh environment/i })[0]);
    await waitFor(() => expect(screen.getByTestId("env-card")).toBeTruthy());
    expect(screen.getByText("https://git.example/acme.git")).toBeTruthy();
    await user.click(screen.getByTestId("copy-clone-url"));
    expect(writeText).toHaveBeenCalledWith("https://git.example/acme.git");
    expect(setSession).toHaveBeenCalledWith(
      expect.objectContaining({ customerRepoUrl: "https://git.example/acme.git" }),
    );
  });

  it("maps customerRepoUrl from Deploy environment payload", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn().mockResolvedValue({
      customerId: "acme",
      installId: "prod-1",
      installRole: "prod",
      customerRepoUrl: "https://git.example/acme.git",
      capabilities: ["promote"],
    });
    render(<EnvPanel bridge={bridge(fetch)} />);
    await user.click(screen.getAllByRole("button", { name: /Refresh environment/i })[0]);
    await waitFor(() => expect(screen.getByTestId("env-card")).toBeTruthy());
    expect(screen.getByText("https://git.example/acme.git")).toBeTruthy();
    expect(screen.getByText("prod")).toBeTruthy();
  });

  it("loads peer topology into session", async () => {
    const user = userEvent.setup();
    const setSession = vi.fn().mockResolvedValue(undefined);
    const fetch = vi
      .fn()
      .mockResolvedValueOnce({
        customerId: "acme",
        installId: "dev",
        installRole: "test",
        customerRepoUrl: "https://git.example/acme.git",
      })
      .mockResolvedValueOnce({
        peers: [
          {
            installId: "prod",
            installRole: "prod",
            baseUrl: "https://prod.example",
            active: true,
          },
        ],
      });
    const b = bridge(fetch, {
      activeInstallId: "dev",
      environments: [
        { installId: "dev", installRole: "test", baseUrl: "http://api", token: "jwt" },
      ],
      baseUrl: "http://api",
      token: "jwt",
    });
    b.setSession = setSession;
    render(<EnvPanel bridge={b} />);
    await user.click(screen.getAllByRole("button", { name: /Refresh environment/i })[0]);
    await waitFor(() => expect(screen.getByTestId("peer-topology")).toBeTruthy());
    expect(setSession).toHaveBeenCalledWith(
      expect.objectContaining({
        environments: expect.arrayContaining([
          expect.objectContaining({ installId: "prod", baseUrl: "https://prod.example" }),
        ]),
      }),
    );
  });
});

describe("HostingPanel", () => {
  it("shows cloud not configured empty state", async () => {
    const fetch = vi.fn().mockResolvedValue({ customerId: "acme", installId: "dev" });
    render(<HostingPanel bridge={bridge(fetch)} />);
    await waitFor(() => expect(screen.getByTestId("hosting-panel")).toBeTruthy());
    await waitFor(() => expect(screen.getByTestId("do-cloud-section")).toBeTruthy());
    expect(screen.getByText(/Cloud hosting not configured/i)).toBeTruthy();
    expect(fetch).toHaveBeenCalledWith("/deploy/v1/environment");
  });

  it("shows hosting scale actions when cloud capability is on", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/deploy/v1/environment") {
        return {
          customerId: "acme",
          installId: "prod",
          cloudHost: "digitalocean",
          capabilities: { cloud: true, digitaloceanCloud: true },
        };
      }
      if (path === "/deploy/v1/cloud/status") {
        return { configured: true, host: "digitalocean", binding: { appId: "app-1", databaseId: "db-1", region: "nyc" } };
      }
      if (path === "/deploy/v1/cloud/app") {
        return {
          appId: "app-1",
          publicUrl: "https://app.example",
          apiInstanceCount: 1,
          apiInstanceSizeSlug: "apps-s-1vcpu-1gb",
          workerInstanceCount: 1,
          workerInstanceSizeSlug: "apps-s-1vcpu-1gb",
        };
      }
      if (path === "/deploy/v1/cloud/environments") {
        if (init?.method === "POST") return { appId: "peer-app", runId: "r1" };
        return { provisionRuns: [{ id: "r0", peerInstallId: "old", status: "complete" }] };
      }
      if (path === "/deploy/v1/cloud/binding" && init?.method === "PUT") {
        return { appId: "app-1", databaseId: "db-1" };
      }
      if (path === "/deploy/v1/cloud/app/scale" && init?.method === "PATCH") {
        return { apiInstanceCount: 2 };
      }
      if (path === "/deploy/v1/cloud/database/resize" && init?.method === "PATCH") {
        return { ok: true };
      }
      return {};
    });
    const b = bridge(fetch, { baseUrl: "http://api", token: "jwt", isAdmin: true });
    render(<HostingPanel bridge={b} />);
    await waitFor(() => expect(screen.getByTestId("do-scale-save")).toBeTruthy());
    expect(screen.getByTestId("do-db-resize")).toBeTruthy();
    expect(screen.getByTestId("do-prov-submit")).toBeTruthy();

    await user.clear(screen.getByTestId("do-bind-app-id"));
    await user.type(screen.getByTestId("do-bind-app-id"), "app-1");
    await user.clear(screen.getByTestId("do-bind-db-id"));
    await user.type(screen.getByTestId("do-bind-db-id"), "db-1");
    await user.click(screen.getByTestId("do-bind-save"));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/deploy/v1/cloud/binding",
        expect.objectContaining({ method: "PUT" }),
      ),
    );

    await user.clear(screen.getByTestId("do-scale-api-count"));
    await user.type(screen.getByTestId("do-scale-api-count"), "2");
    await user.click(screen.getByTestId("do-scale-save"));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/deploy/v1/cloud/app/scale",
        expect.objectContaining({ method: "PATCH" }),
      ),
    );

    await user.click(screen.getByTestId("do-db-resize"));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/deploy/v1/cloud/database/resize",
        expect.objectContaining({ method: "PATCH" }),
      ),
    );

    await user.type(screen.getByTestId("do-prov-install-id"), "dev-2");
    await user.type(screen.getByTestId("do-prov-api-keys"), "k+admin");
    await user.type(screen.getByTestId("do-prov-jwt"), "jwt-secret-long-enough");
    await user.click(screen.getByTestId("do-prov-submit"));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/deploy/v1/cloud/environments",
        expect.objectContaining({ method: "POST" }),
      ),
    );
  });
});

describe("ClientPanel", () => {
  it("describes an object as a field list", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn().mockResolvedValue({
      apiName: "Account",
      fields: [{ apiName: "Name", type: "string" }],
    });
    render(<ClientPanel bridge={bridge(fetch)} />);
    await user.click(screen.getByRole("button", { name: /^Describe$/i }));
    await waitFor(() => expect(screen.getByTestId("field-list")).toBeTruthy());
    expect(screen.getByText("Name")).toBeTruthy();
    expect(fetch).toHaveBeenCalledWith("/client/v1/describe/Account");
  });

  it("queries with limit 5 into a table", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn().mockResolvedValue({ records: [{ id: "1", Name: "Acme" }] });
    render(<ClientPanel bridge={bridge(fetch)} />);
    await user.click(screen.getByRole("button", { name: /Query records/i }));
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(fetch).toHaveBeenCalledWith(
      "/client/v1/query",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ object: "Account", limit: 5 }),
      }),
    );
    expect(await screen.findByText("Acme")).toBeTruthy();
  });

  it("describes a custom object name and surfaces errors", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn().mockRejectedValue(new Error("404 /client/v1/describe/Contact"));
    render(<ClientPanel bridge={bridge(fetch)} />);
    const input = screen.getByDisplayValue("Account");
    await user.clear(input);
    await user.type(input, "Contact");
    await user.click(screen.getByRole("button", { name: /^Describe$/i }));
    expect(await screen.findByText(/404/)).toBeTruthy();
    expect(fetch).toHaveBeenCalledWith("/client/v1/describe/Contact");
  });

  it("surfaces query failures", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn().mockRejectedValue(new Error("401 query denied"));
    render(<ClientPanel bridge={bridge(fetch)} />);
    await user.click(screen.getByRole("button", { name: /Query records/i }));
    expect(await screen.findByText(/401 query denied/i)).toBeTruthy();
  });
});

describe("DeployPanel", () => {
  it("notifies pipeline after validate vs org with bundle id", async () => {
    const user = userEvent.setup();
    const onPipelineChange = vi.fn();
    const fetch = vi.fn().mockResolvedValue({
      bundleId: "bun_1",
      checksum: "abc",
      ok: true,
      validation: { ok: true, issues: [] },
      diff: { counts: { add: 0, change: 0, remove: 0, baseline: 0 }, entries: [] },
    });
    render(<DeployPanel bridge={bridge(fetch)} onPipelineChange={onPipelineChange} />);
    const bundleInput = screen.getByTestId("bundle-id");
    await user.clear(bundleInput);
    await user.type(bundleInput, "bun_1");
    await user.click(screen.getByTestId("validate-vs-org"));
    await waitFor(() => expect(screen.getByDisplayValue("bun_1")).toBeTruthy());
    await waitFor(() =>
      expect(onPipelineChange).toHaveBeenCalledWith(
        expect.arrayContaining([expect.objectContaining({ id: "validate", state: "passed" })]),
        expect.any(String),
      ),
    );
  });

  it("validates vs org via bundle id", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn().mockResolvedValue({
      bundleId: "bun_1",
      checksum: "abc",
      ok: true,
      validation: { ok: true },
    });
    render(<DeployPanel bridge={bridge(fetch)} />);
    await user.type(screen.getByTestId("bundle-id"), "bun_1");
    await user.click(screen.getByTestId("validate-vs-org"));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/deploy/v1/packages/validate-local",
        expect.objectContaining({ method: "POST", body: JSON.stringify({ bundleId: "bun_1" }) }),
      ),
    );
    expect(await screen.findByDisplayValue("abc")).toBeTruthy();
  });

  it("requires repo or bundle id before validate vs org", async () => {
    const user = userEvent.setup();
    render(<DeployPanel bridge={bridge(vi.fn())} />);
    await user.click(screen.getByTestId("validate-vs-org"));
    expect(await screen.findByText(/pack a bundle id first/i)).toBeTruthy();
  });

  it("surfaces API errors on validate vs org", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn().mockRejectedValue(new Error("403 forbidden"));
    render(<DeployPanel bridge={bridge(fetch)} />);
    await user.type(screen.getByTestId("bundle-id"), "bun_x");
    await user.click(screen.getByTestId("validate-vs-org"));
    expect(await screen.findByText(/403 forbidden/i)).toBeTruthy();
  });

  it("errors when packing zip without connection", async () => {
    const user = userEvent.setup();
    render(<DeployPanel bridge={bridge(vi.fn(), null)} />);
    const fileInput = screen.getByTestId("file-drop-input");
    const file = new File([new Uint8Array([1, 2, 3])], "customer.zip", { type: "application/zip" });
    await user.upload(fileInput, file);
    expect(await screen.findByText(/connect first/i)).toBeTruthy();
  });

  it("gates deploy until validate and customer tests are green, then deploys to org", async () => {
    const user = userEvent.setup();
    const fetch = vi
      .fn()
      .mockResolvedValueOnce({
        bundleId: "bun_2",
        checksum: "chk2",
        ok: true,
        validation: { ok: true },
      })
      .mockResolvedValueOnce({ run: { id: "r1", status: "passed" } })
      .mockResolvedValueOnce({ status: "accepted" });
    render(<DeployPanel bridge={bridge(fetch)} />);
    expect((screen.getByTestId("deploy-to-org") as HTMLButtonElement).disabled).toBe(true);
    await user.type(screen.getByTestId("bundle-id"), "bun_2");
    await user.click(screen.getByTestId("validate-vs-org"));
    await waitFor(() => expect(screen.getByTestId("release-readiness")).toBeTruthy());
    expect((screen.getByTestId("deploy-to-org") as HTMLButtonElement).disabled).toBe(true);
    await user.click(screen.getByTestId("run-tests"));
    await waitFor(() => expect((screen.getByTestId("deploy-to-org") as HTMLButtonElement).disabled).toBe(false));
    await user.click(screen.getByTestId("deploy-to-org"));
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(3));
    expect(fetch).toHaveBeenNthCalledWith(3, "/deploy/v1/promotions", expect.any(Object));
  });

  it("does not mark customer tests Passed on HTTP 200 when the suite failed", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn().mockResolvedValue({ status: "failed" });
    render(<DeployPanel bridge={bridge(fetch)} />);
    await user.click(screen.getByTestId("run-tests"));
    await waitFor(() => expect(screen.getByTestId("tests-step").textContent).toMatch(/Failed/));
    expect((screen.getByTestId("deploy-to-org") as HTMLButtonElement).disabled).toBe(true);
  });

  it("does not mark customer tests Passed on HTTP 200 without suite evidence", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn().mockResolvedValue({ runId: "r1" });
    render(<DeployPanel bridge={bridge(fetch)} />);
    await user.click(screen.getByTestId("run-tests"));
    await waitFor(() => expect(screen.getByTestId("tests-step").textContent).toMatch(/Failed/));
    expect(await screen.findByText(/did not report a passed suite/i)).toBeTruthy();
    expect((screen.getByTestId("deploy-to-org") as HTMLButtonElement).disabled).toBe(true);
  });

  it("polls /deploy/v1/work/{id} when a test run returns jobId", async () => {
    const user = userEvent.setup();
    const fetch = vi
      .fn()
      .mockResolvedValueOnce({
        bundleId: "bun_w",
        checksum: "chk",
        ok: true,
        validation: { ok: true },
      })
      .mockResolvedValueOnce({ jobId: "job-9", status: "queued", accepted: true })
      .mockResolvedValueOnce({ jobId: "job-9", status: "running" })
      .mockResolvedValueOnce({
        jobId: "job-9",
        status: "completed",
        result: { run: { status: "passed" } },
      });
    render(<DeployPanel bridge={bridge(fetch)} />);
    await user.type(screen.getByTestId("bundle-id"), "bun_w");
    await user.click(screen.getByTestId("validate-vs-org"));
    await waitFor(() => expect(screen.getByTestId("release-readiness")).toBeTruthy());
    await user.click(screen.getByTestId("run-tests"));
    await waitFor(
      () => expect((screen.getByTestId("deploy-to-org") as HTMLButtonElement).disabled).toBe(false),
      { timeout: 4000 },
    );
    expect(fetch).toHaveBeenCalledWith("/deploy/v1/work/job-9");
  });

  it("shows connected orgs and has no peer push controls", async () => {
    const user = userEvent.setup();
    const onMore = vi.fn();
    const fetch = vi.fn().mockResolvedValue({
      bundleId: "bun_p",
      checksum: "c",
      ok: true,
      validation: { ok: true },
    });
    const session = {
      activeInstallId: "dev",
      environments: [
        { installId: "dev", installRole: "test", baseUrl: "http://dev", token: "t1" },
        { installId: "uat", installRole: "staging", baseUrl: "http://uat", token: "t2" },
      ],
      baseUrl: "http://dev",
      token: "t1",
    };
    render(<DeployPanel bridge={bridge(fetch, session)} onMoreChanges={onMore} />);
    expect(screen.getByTestId("ship-stage-strip")).toBeTruthy();
    expect(screen.queryByTestId("promote-to-peer")).toBeNull();
    expect(screen.queryByTestId("deprecated-peer-promote")).toBeNull();
    await user.type(screen.getByTestId("bundle-id"), "bun_p");
    await user.click(screen.getByTestId("validate-vs-org"));
    await user.click(screen.getByTestId("run-tests"));
    await waitFor(() => expect((screen.getByTestId("deploy-to-org") as HTMLButtonElement).disabled).toBe(false));
    await user.click(screen.getByTestId("more-changes"));
    expect(onMore).toHaveBeenCalled();
  });
});

describe("UsersPanel", () => {
  it("lists principals and creates one", async () => {
    const user = userEvent.setup();
    const fetch = vi
      .fn()
      .mockResolvedValueOnce({ principals: [{ id: "u1", email: "a@x.com", displayName: "Ada", principalType: "user", isActive: true, roleApiNames: ["Operator"] }] })
      .mockResolvedValueOnce({ roles: [{ apiName: "Operator", label: "Operator", scopes: ["client"] }] })
      .mockResolvedValueOnce({ permissionSets: [] })
      .mockResolvedValueOnce({ credentials: [] })
      .mockResolvedValueOnce({ id: "u2", email: "b@x.com", displayName: "Bob", principalType: "user", roleApiNames: ["Operator"] })
      .mockResolvedValueOnce({ principals: [{ id: "u2", email: "b@x.com", displayName: "Bob", principalType: "user", isActive: true, roleApiNames: ["Operator"] }] })
      .mockResolvedValueOnce({ roles: [] })
      .mockResolvedValueOnce({ permissionSets: [] })
      .mockResolvedValueOnce({ credentials: [] });
    render(<UsersPanel bridge={bridge(fetch)} />);
    expect(await screen.findByTestId("user-row-u1")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /New principal/i }));
    await user.type(screen.getByTestId("users-email"), "b@x.com");
    await user.type(screen.getByTestId("users-role"), "Operator");
    await user.click(screen.getByTestId("users-create-btn"));
    await waitFor(() => expect(fetch).toHaveBeenCalledWith("/client/v1/principals", expect.objectContaining({ method: "POST" })));
  });

  it("selects a principal and saves profile, freezes, assigns, and creates a credential", async () => {
    const user = userEvent.setup();
    vi.spyOn(window, "confirm").mockReturnValue(true);
    const principal = {
      id: "u1",
      email: "a@x.com",
      displayName: "Ada",
      principalType: "user",
      isActive: true,
      roleApiNames: ["Operator"],
      permissionSetApiNames: ["IdentityUsers"],
      title: "",
      department: "",
    };
    const fetch = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/principals") {
        if (init?.method === "POST") return principal;
        return { principals: [principal] };
      }
      if (path === "/client/v1/roles") return { roles: [{ apiName: "Operator", scopes: ["client"] }] };
      if (String(path).startsWith("/metadata/v1/permissions/sets")) {
        return { permissionSets: [{ apiName: "IdentityUsers" }] };
      }
      if (path === "/client/v1/principals/u1/credentials") {
        if (init?.method === "POST") return { id: "c1", clientSecret: "once-secret", label: "Control IDE" };
        return { credentials: [{ id: "c1", label: "Control IDE" }] };
      }
      if (path === "/client/v1/principals/u1" && init?.method === "PATCH") {
        return { ...principal, displayName: "Ada Lovelace" };
      }
      if (path.endsWith("/freeze")) return { ...principal, frozenAt: "2026-01-01" };
      if (path === "/client/v1/roles/assign" || path === "/client/v1/roles/unassign") return { ok: true };
      if (path === "/client/v1/permissions/assign" || path === "/client/v1/permissions/unassign") {
        return { ok: true };
      }
      if (path.includes("/revoke")) return { ok: true };
      return {};
    });

    render(<UsersPanel bridge={bridge(fetch)} />);
    await user.click(await screen.findByTestId("user-row-u1"));
    expect(await screen.findByTestId("users-detail")).toBeTruthy();

    const display = screen.getByDisplayValue("Ada");
    await user.clear(display);
    await user.type(display, "Ada Lovelace");
    await user.click(screen.getByRole("button", { name: /Save profile/i }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/client/v1/principals/u1",
        expect.objectContaining({ method: "PATCH" }),
      ),
    );

    await user.type(screen.getByPlaceholderText(/Freeze reason/i), "review");
    await user.click(screen.getByRole("button", { name: /^Freeze$/i }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/client/v1/principals/u1/freeze",
        expect.objectContaining({ method: "POST" }),
      ),
    );

    await user.click(screen.getByRole("button", { name: /Create credential/i }));
    expect(await screen.findByTestId("secret-banner")).toBeTruthy();

    await user.click(screen.getAllByRole("button", { name: /^Unassign$/i })[0]);
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/client/v1/roles/unassign",
        expect.objectContaining({ method: "POST" }),
      ),
    );

    await user.click(screen.getByRole("button", { name: /^Revoke$/i }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        expect.stringContaining("/credentials/c1/revoke"),
        expect.objectContaining({ method: "POST" }),
      ),
    );
  });

  it("prompts to connect when disconnected", () => {
    render(<UsersPanel bridge={bridge(undefined, null)} />);
    expect(screen.getByText(/Connect first/i)).toBeTruthy();
  });
});

describe("IntegrationsPanel", () => {
  it("lists and creates integrations", async () => {
    const user = userEvent.setup();
    const fetch = vi
      .fn()
      .mockResolvedValueOnce({ items: [{ apiName: "sales_bot", label: "Sales", isActive: true }] })
      .mockResolvedValueOnce({
        apiName: "svc_a",
        label: "Svc",
        oneClientSecret: "sec-once",
        isActive: true,
      })
      .mockResolvedValueOnce({ items: [{ apiName: "svc_a", label: "Svc", isActive: true }] });
    render(<IntegrationsPanel bridge={bridge(fetch)} />);
    expect(await screen.findByTestId("integration-row-sales_bot")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /New integration/i }));
    await user.type(screen.getByTestId("integrations-api-name"), "svc_a");
    await user.click(screen.getByTestId("integrations-create-btn"));
    expect(await screen.findByTestId("secret-banner")).toBeTruthy();
  });

  it("selects an integration and saves, rotates, reveals, and deletes", async () => {
    const user = userEvent.setup();
    const row = {
      apiName: "sales_bot",
      label: "Sales",
      description: "",
      isActive: true,
      hasOneSecret: true,
    };
    const fetch = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/integrations") {
        if (init?.method === "POST") return row;
        return { items: [row] };
      }
      if (path === "/client/v1/integrations/sales_bot") {
        if (init?.method === "PATCH") return { ...row, label: "Sales Bot" };
        if (init?.method === "DELETE") return { ok: true, apiName: "sales_bot" };
        return row;
      }
      if (path.endsWith("/secrets/rotate")) {
        return { ...row, oneClientSecret: "rotated" };
      }
      if (path.endsWith("/secrets/reveal")) {
        return { oneClientSecret: "revealed" };
      }
      return {};
    });
    vi.spyOn(window, "confirm").mockReturnValue(true);

    render(<IntegrationsPanel bridge={bridge(fetch)} />);
    await user.click(await screen.findByTestId("integration-row-sales_bot"));
    expect(await screen.findByTestId("integrations-detail")).toBeTruthy();

    const label = screen.getByDisplayValue("Sales");
    await user.clear(label);
    await user.type(label, "Sales Bot");
    await user.click(screen.getByRole("button", { name: /^Save$/i }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/client/v1/integrations/sales_bot",
        expect.objectContaining({ method: "PATCH" }),
      ),
    );

    await user.click(screen.getByRole("button", { name: /Rotate secrets/i }));
    expect(await screen.findByTestId("secret-banner")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /Dismiss/i }));

    await user.click(screen.getByRole("button", { name: /Reveal secrets/i }));
    expect(await screen.findByText(/Majesta One secret: revealed/i)).toBeTruthy();

    await user.click(screen.getByRole("button", { name: /^Delete$/i }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/client/v1/integrations/sales_bot",
        expect.objectContaining({ method: "DELETE" }),
      ),
    );
  });

  it("prompts to connect when disconnected", () => {
    render(<IntegrationsPanel bridge={bridge(undefined, null)} />);
    expect(screen.getByText(/Connect first/i)).toBeTruthy();
  });
});

describe("PermissionsPanel", () => {
  it("lists roles and creates a role", async () => {
    const user = userEvent.setup();
    const fetch = vi
      .fn()
      .mockImplementation(async (path: string, init?: RequestInit) => {
        if (path === "/client/v1/roles" && init?.method === "POST") {
          return { apiName: "Custom", label: "Custom", scopes: ["client"] };
        }
        if (path === "/client/v1/roles") return { roles: [{ apiName: "Operator", label: "Operator", scopes: ["client"] }] };
        if (String(path).startsWith("/metadata/v1/permissions/sets")) return { permissionSets: [] };
        return {};
      });
    render(<PermissionsPanel bridge={bridge(fetch)} />);
    expect(await screen.findByTestId("permissions-tabs")).toBeTruthy();
    expect(screen.getByRole("tab", { name: /Permission sets/i })).toBeTruthy();
    expect(await screen.findByTestId("role-row-Operator")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /New role/i }));
    await user.type(screen.getByTestId("role-api-name"), "Custom");
    await user.click(screen.getByTestId("role-create-btn"));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith("/client/v1/roles", expect.objectContaining({ method: "POST" })),
    );
  });

  it("edits a role and manages permission sets", async () => {
    const user = userEvent.setup();
    const fetch = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/roles") {
        if (init?.method === "POST") return { apiName: "Custom", scopes: ["client"] };
        return { roles: [{ apiName: "Operator", label: "Operator", scopes: ["client"], isSystem: false }] };
      }
      if (path.startsWith("/client/v1/roles/Operator")) {
        if (init?.method === "PATCH") return { apiName: "Operator", label: "Ops", scopes: ["client"] };
        if (init?.method === "DELETE") return { ok: true };
        return { apiName: "Operator", label: "Operator", scopes: ["client"] };
      }
      if (path === "/metadata/v1/permissions/sets" || path.startsWith("/metadata/v1/permissions/sets?")) {
        if (init?.method === "POST") {
          return { apiName: "CustomPS", label: "Custom", systemPermissions: ["identity.users"] };
        }
        return {
          permissionSets: [
            {
              apiName: "IdentityUsers",
              label: "Identity Users",
              description: "",
              systemPermissions: ["identity.users"],
              isSystem: false,
            },
          ],
        };
      }
      if (path.endsWith("/IdentityUsers") && init?.method === "PATCH") {
        return {
          apiName: "IdentityUsers",
          label: "Identity Users Pack",
          systemPermissions: ["identity.users"],
        };
      }
      return {};
    });
    vi.spyOn(window, "confirm").mockReturnValue(true);

    render(<PermissionsPanel bridge={bridge(fetch)} />);
    await user.click(await screen.findByTestId("role-row-Operator"));
    const roleLabel = screen.getByDisplayValue("Operator");
    await user.clear(roleLabel);
    await user.type(roleLabel, "Ops");
    await user.click(screen.getByRole("button", { name: /^Save$/i }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/client/v1/roles/Operator",
        expect.objectContaining({ method: "PATCH" }),
      ),
    );

    await user.click(screen.getByRole("tab", { name: /Permission sets/i }));
    expect(await screen.findByTestId("ps-row-IdentityUsers")).toBeTruthy();
    await user.click(screen.getByTestId("ps-row-IdentityUsers"));
    expect(screen.getByTestId("ps-edit-btn")).toBeTruthy();
    await user.click(screen.getByTestId("ps-edit-btn"));
    expect(await screen.findByTestId("sets-edit")).toBeTruthy();
    const psLabel = screen.getByTestId("ps-label");
    await user.clear(psLabel);
    await user.type(psLabel, "Identity Users Pack");
    await user.click(screen.getByTestId("ps-wizard-next"));
    // Exercise ide.* checkbox toggles on caps step.
    await user.click(screen.getByTestId("ide-cap-ide.govern"));
    await user.click(screen.getByTestId("ide-cap-ide.govern.users"));
    await user.click(screen.getByTestId("ps-wizard-next")); // tools
    await user.click(screen.getByTestId("ps-wizard-next")); // data
    await user.click(screen.getByTestId("ps-wizard-next")); // review
    await user.click(screen.getByTestId("ps-create-btn"));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/metadata/v1/permissions/sets/IdentityUsers",
        expect.objectContaining({ method: "PATCH" }),
      ),
    );

    await user.click(screen.getByRole("button", { name: /New permission set/i }));
    expect(await screen.findByTestId("sets-create")).toBeTruthy();
    await user.type(screen.getByTestId("ps-api-name"), "CustomPS");
    await user.type(screen.getByTestId("ps-label"), "Custom");
    await user.click(screen.getByTestId("ps-wizard-next"));
    await user.click(screen.getByTestId("ps-wizard-next"));
    await user.click(screen.getByTestId("ps-wizard-next"));
    await user.click(screen.getByTestId("ps-wizard-next"));
    await user.click(screen.getByTestId("ps-create-btn"));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/metadata/v1/permissions/sets",
        expect.objectContaining({ method: "POST" }),
      ),
    );
  });

  it("prompts to connect when disconnected", () => {
    render(<PermissionsPanel bridge={bridge(undefined, null)} />);
    expect(screen.getByText(/Connect first/i)).toBeTruthy();
  });
});

describe("CrmPanel", () => {
  it("shows an Offline stub badge and seed rows when disconnected", async () => {
    const user = userEvent.setup();
    render(<CrmPanel />);
    expect(screen.getByTestId("crm-panel")).toBeTruthy();
    expect(await screen.findByTestId("crm-offline-stub")).toBeTruthy();
    await user.click(screen.getByTestId("crm-tab-opportunities"));
    expect(screen.getAllByText(/Acme — Renewal/i).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByRole("button", { name: /^Save$/i })).toHaveProperty("disabled", true);
    expect(screen.getByRole("button", { name: /^New$/i })).toHaveProperty("disabled", true);
  });

  it("applies filters and shows what-to-do from handoff", async () => {
    const user = userEvent.setup();
    render(
      <CrmPanel
        handoff={{
          source: "run",
          runId: "run-abc",
          objectApiName: "Account",
          recordIds: ["a1"],
          rationale: "Call these accounts",
          suggestions: [{ id: "open", label: "Open ranked records", action: "focus_ids" }],
        }}
      />,
    );
    expect(await screen.findByTestId("crm-what-to-do")).toBeTruthy();
    expect(screen.getByTestId("crm-intent-chip")).toBeTruthy();
    expect(screen.getByTestId("crm-query-bar")).toBeTruthy();
    await user.click(screen.getByTestId("crm-what-to-do-dismiss"));
    expect(screen.queryByTestId("crm-what-to-do")).toBeNull();
  });

  it("filters the offline list", async () => {
    const user = userEvent.setup();
    render(<CrmPanel />);
    expect((await screen.findAllByText(/Acme Manufacturing/i)).length).toBeGreaterThanOrEqual(1);
    const value = screen.getByLabelText(/Filter value/i);
    await user.clear(value);
    await user.type(value, "Acme");
    await user.click(screen.getByRole("button", { name: /Apply filter/i }));
    expect(screen.getAllByText(/Acme Manufacturing/i).length).toBeGreaterThanOrEqual(1);
    expect(screen.queryByText(/Northwind Logistics/i)).toBeNull();
  });

  it("refuses offline mutate and still toggles pipeline", async () => {
    const user = userEvent.setup();
    render(<CrmPanel />);
    await user.click(await screen.findByTestId("crm-tab-opportunities"));
    await user.click(screen.getByTestId("crm-board-toggle"));
    expect(screen.getByTestId("crm-pipeline")).toBeTruthy();
    expect(screen.getByRole("button", { name: /^New$/i })).toHaveProperty("disabled", true);
    expect(screen.getByRole("button", { name: /^Save$/i })).toHaveProperty("disabled", true);
    expect(screen.getByRole("button", { name: /^Delete$/i })).toHaveProperty("disabled", true);
  });

  it("runs suggestion actions and clears intent", async () => {
    const user = userEvent.setup();
    const onStaged = vi.fn();
    render(
      <CrmPanel
        onStagedMutations={onStaged}
        handoff={{
          source: "run",
          runId: "run-xyz123",
          objectApiName: "Account",
          recordIds: ["a1", "a2"],
          proposedMutations: [{ op: "update", object: "Account", id: "a1", data: { Type: "Customer" } }],
          suggestions: [
            { id: "open", label: "Open ranked records", action: "focus_ids" },
            { id: "filter", label: "Filter customers", action: "filter_type_customer" },
          ],
        }}
      />,
    );
    expect(onStaged).toHaveBeenCalledWith(1);
    await user.click(screen.getByRole("button", { name: /Filter customers/i }));
    expect(screen.queryByTestId("crm-what-to-do")).toBeNull();
    await user.click(screen.getByLabelText(/Clear intent/i));
    expect(screen.queryByTestId("crm-intent-chip")).toBeNull();
  });

  it("loads live describe/query when connected", async () => {
    const fetch = vi.fn(async (path: string) => {
      if (path.includes("/describe/")) {
        return {
          apiName: "Account",
          fields: [
            { apiName: "Name", label: "Name", fieldType: "text", required: true, filterable: true, sortable: true },
            { apiName: "Type", label: "Type", fieldType: "picklist", picklistValues: ["Customer"], filterable: true },
          ],
        };
      }
      if (path === "/metadata/v1/packages") {
        return {
          packages: [
            { name: "core", enabled: true },
            { name: "sales", enabled: true },
            { name: "activities", enabled: true },
          ],
        };
      }
      if (path === "/client/v1/query") {
        return { records: [{ id: "live1", Name: "Live Co", Type: "Customer" }] };
      }
      return {};
    });
    render(<CrmPanel bridge={bridge(fetch)} />);
    expect((await screen.findAllByText(/Live Co/i)).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(/^Live$/)).toBeTruthy();
    expect(screen.queryByText(/Acme Manufacturing/i)).toBeNull();
  });

  it("does not fall back to seed rows when a connected fetch fails", async () => {
    const fetch = vi.fn(async (path: string) => {
      if (path === "/metadata/v1/packages") {
        return { packages: [{ name: "core", enabled: true }] };
      }
      throw new Error("503 describe failed");
    });
    render(<CrmPanel bridge={bridge(fetch)} />);
    expect(await screen.findByText(/503 describe failed/i)).toBeTruthy();
    expect(screen.queryByText(/Acme Manufacturing/i)).toBeNull();
    expect(screen.queryByText(/Northwind Logistics/i)).toBeNull();
    expect(screen.queryByTestId("crm-offline-stub")).toBeNull();
  });
});
