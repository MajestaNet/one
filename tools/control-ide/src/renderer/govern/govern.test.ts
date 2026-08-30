import { describe, expect, it, vi } from "vitest";
import {
  assignPermissionSet,
  assignRole,
  createCredential,
  createIntegration,
  createPermissionSet,
  createPrincipal,
  createRole,
  deleteIntegration,
  deleteRole,
  freezePrincipal,
  getIntegration,
  getPermissionSet,
  getPrincipal,
  getRole,
  listCredentials,
  listIntegrations,
  listPermissionSets,
  listPrincipals,
  listRoles,
  patchIntegration,
  patchPermissionSet,
  patchPrincipal,
  patchRole,
  revealIntegrationSecrets,
  revokeCredential,
  rotateIntegrationSecrets,
  setPrincipalPassword,
  unassignPermissionSet,
  unassignRole,
  unfreezePrincipal,
} from "./index";
import {
  cloudEnabled,
  digitaloceanCloudEnabled,
  getCloudApp,
  getCloudStatus,
  listCloudEnvironments,
  provisionCloudEnvironment,
  putCloudBinding,
  resizeCloudDatabase,
  scaleCloudApp,
} from "./cloud";

describe("cloud helpers", () => {
  it("detects capability from object or list", () => {
    expect(cloudEnabled({ capabilities: { cloud: true } })).toBe(true);
    expect(cloudEnabled({ cloudHost: "digitalocean" })).toBe(true);
    expect(cloudEnabled({ capabilities: { digitaloceanCloud: true } })).toBe(true);
    expect(digitaloceanCloudEnabled({ capabilities: { digitaloceanCloud: true } })).toBe(true);
    expect(digitaloceanCloudEnabled({ capabilities: ["promote", "digitaloceanCloud"] })).toBe(true);
    expect(digitaloceanCloudEnabled({ capabilities: { promote: true } })).toBe(false);
    expect(digitaloceanCloudEnabled({ capabilities: "nope" as unknown as string[] })).toBe(false);
    expect(digitaloceanCloudEnabled(null)).toBe(false);
    expect(cloudEnabled(null)).toBe(false);
  });

  it("calls host-free Deploy cloud routes", async () => {
    const fetchApi = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/deploy/v1/cloud/status") return { configured: true, host: "digitalocean" };
      if (path === "/deploy/v1/cloud/app") return { appResourceId: "a1", appId: "a1" };
      if (path === "/deploy/v1/cloud/binding" && init?.method === "PUT") {
        expect(JSON.parse(String(init.body))).toMatchObject({ appId: "a1", databaseId: "d1" });
        return { appResourceId: "a1", appId: "a1" };
      }
      if (path === "/deploy/v1/cloud/app/scale" && init?.method === "PATCH") {
        expect(JSON.parse(String(init.body))).toMatchObject({ apiInstanceCount: 2 });
        return { apiInstances: 2, apiInstanceCount: 2 };
      }
      if (path === "/deploy/v1/cloud/database/resize" && init?.method === "PATCH") {
        expect(JSON.parse(String(init.body))).toMatchObject({ size: "db-s-1vcpu-1gb", numNodes: 1 });
        return { ok: true };
      }
      if (path === "/deploy/v1/cloud/environments" && init?.method === "POST") {
        expect(JSON.parse(String(init.body))).toMatchObject({ installId: "dev-2" });
        return { appId: "app-1" };
      }
      if (path === "/deploy/v1/cloud/environments") return { provisionRuns: [] };
      return {};
    });
    expect(await getCloudStatus(fetchApi)).toEqual({ configured: true, host: "digitalocean" });
    expect(await getCloudApp(fetchApi)).toMatchObject({ appId: "a1", appResourceId: "a1" });
    expect(await putCloudBinding(fetchApi, { appId: "a1", databaseId: "d1", region: "nyc" })).toMatchObject({
      appId: "a1",
      appResourceId: "a1",
    });
    expect(await scaleCloudApp(fetchApi, { apiInstanceCount: 2 })).toMatchObject({ apiInstanceCount: 2 });
    expect(await resizeCloudDatabase(fetchApi, { size: "db-s-1vcpu-1gb", numNodes: 1 })).toEqual({
      ok: true,
    });
    expect(await listCloudEnvironments(fetchApi)).toEqual({ provisionRuns: [] });
    expect(await provisionCloudEnvironment(fetchApi, { installId: "dev-2" })).toEqual({ appId: "app-1" });
  });
});

describe("govern API helpers", () => {
  it("covers principal CRUD, freeze, credentials, and assignments", async () => {
    const fetchApi = vi.fn(async (path: string, init?: RequestInit) => {
      if (path.startsWith("/client/v1/principals?") || path === "/client/v1/principals") {
        if (init?.method === "POST") return { id: "2", email: "b@x.com" };
        if (path.includes("principalType=")) return [{ id: "1", email: "a@x.com" }];
        return { principals: [{ id: "1", email: "a@x.com" }] };
      }
      if (path === "/client/v1/principals/1") {
        if (init?.method === "PATCH") return { id: "1", displayName: "Ada" };
        return { id: "1", email: "a@x.com" };
      }
      if (path.endsWith("/freeze")) return { id: "1", frozenAt: "now" };
      if (path.endsWith("/unfreeze")) return { id: "1", isActive: true };
      if (path.endsWith("/credentials")) {
        if (init?.method === "POST") return { id: "c1", clientSecret: "once" };
        return { credentials: [{ id: "c1", label: "IDE" }] };
      }
      if (path.endsWith("/password") && init?.method === "POST") return { ok: true, userId: "1" };
      if (path.includes("/revoke")) return { ok: true };
      if (path === "/client/v1/roles/assign" || path === "/client/v1/roles/unassign") return { ok: true };
      if (path === "/client/v1/permissions/assign" || path === "/client/v1/permissions/unassign") {
        return { ok: true };
      }
      return {};
    });

    expect(await listPrincipals(fetchApi)).toEqual([{ id: "1", email: "a@x.com" }]);
    expect(await listPrincipals(fetchApi, { principalType: "user" })).toEqual([
      { id: "1", email: "a@x.com" },
    ]);
    expect(await getPrincipal(fetchApi, "1")).toMatchObject({ id: "1" });
    expect(await createPrincipal(fetchApi, { email: "b@x.com", roleApiName: "Operator" })).toMatchObject({
      id: "2",
    });
    expect(await patchPrincipal(fetchApi, "1", { displayName: "Ada" })).toMatchObject({
      displayName: "Ada",
    });
    expect(await freezePrincipal(fetchApi, "1", "review")).toMatchObject({ frozenAt: "now" });
    expect(await unfreezePrincipal(fetchApi, "1")).toMatchObject({ isActive: true });
    expect(await listCredentials(fetchApi, "1")).toEqual([{ id: "c1", label: "IDE" }]);
    expect(await createCredential(fetchApi, "1", "IDE")).toMatchObject({ clientSecret: "once" });
    expect(await revokeCredential(fetchApi, "1", "c1")).toEqual({ ok: true });
    expect(await setPrincipalPassword(fetchApi, "1", "new-password-ok")).toEqual({
      ok: true,
      userId: "1",
    });
    expect(await assignRole(fetchApi, "1", "Operator")).toEqual({ ok: true });
    expect(await unassignRole(fetchApi, "1", "Operator")).toEqual({ ok: true });
    expect(await assignPermissionSet(fetchApi, "1", "IdentityUsers")).toEqual({ ok: true });
    expect(await unassignPermissionSet(fetchApi, "1", "IdentityUsers")).toEqual({ ok: true });
  });

  it("covers integration CRUD and secrets", async () => {
    const fetchApi = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/integrations") {
        if (init?.method === "POST") return { apiName: "b", oneClientSecret: "s" };
        return { items: [{ apiName: "a" }] };
      }
      if (path === "/client/v1/integrations/a") {
        if (init?.method === "PATCH") return { apiName: "a", label: "A" };
        if (init?.method === "DELETE") return { ok: true, apiName: "a" };
        return { apiName: "a", label: "A" };
      }
      if (path.endsWith("/secrets/rotate")) {
        return { apiName: "a", oneClientSecret: "new" };
      }
      if (path.endsWith("/secrets/reveal")) {
        return { oneClientSecret: "rev" };
      }
      return {};
    });

    expect(await listIntegrations(fetchApi)).toEqual([{ apiName: "a" }]);
    expect(await getIntegration(fetchApi, "a")).toMatchObject({ apiName: "a" });
    expect(
      await createIntegration(fetchApi, { apiName: "b", oauthFlows: ["client_credentials"] }),
    ).toMatchObject({ apiName: "b" });
    expect(await patchIntegration(fetchApi, "a", { label: "A", isActive: true })).toMatchObject({
      label: "A",
    });
    expect(await rotateIntegrationSecrets(fetchApi, "a")).toMatchObject({ oneClientSecret: "new" });
    expect(await revealIntegrationSecrets(fetchApi, "a")).toMatchObject({ oneClientSecret: "rev" });
    expect(await deleteIntegration(fetchApi, "a")).toEqual({ ok: true, apiName: "a" });
  });

  it("covers roles and permission set helpers", async () => {
    const fetchApi = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/roles") {
        if (init?.method === "POST") return { apiName: "Custom", scopes: ["client"] };
        return { roles: [{ apiName: "Operator", scopes: ["client"] }] };
      }
      if (path.startsWith("/client/v1/roles/Operator")) {
        if (init?.method === "PATCH") return { apiName: "Operator", label: "Op" };
        if (init?.method === "DELETE") return { ok: true };
        return { apiName: "Operator", scopes: ["client"] };
      }
      if (path.startsWith("/metadata/v1/permissions/sets")) {
        if (init?.method === "POST") return { apiName: "CustomPS", label: "Custom" };
        if (path.includes("?")) {
          return { permissionSets: [{ apiName: "IdentityUsers", systemPermissions: ["identity.users"] }] };
        }
        if (path.endsWith("/IdentityUsers")) {
          if (init?.method === "PATCH") return { apiName: "IdentityUsers", label: "Users" };
          return { apiName: "IdentityUsers", label: "Users" };
        }
        return { permissionSets: [{ apiName: "IdentityUsers" }] };
      }
      return {};
    });

    expect(await listRoles(fetchApi)).toHaveLength(1);
    expect(await getRole(fetchApi, "Operator")).toMatchObject({ apiName: "Operator" });
    expect(await createRole(fetchApi, { apiName: "Custom", scopes: ["client"] })).toMatchObject({
      apiName: "Custom",
    });
    expect(await patchRole(fetchApi, "Operator", { label: "Op" })).toMatchObject({ label: "Op" });
    expect(await deleteRole(fetchApi, "Operator", true)).toEqual({ ok: true });

    expect(await listPermissionSets(fetchApi)).toHaveLength(1);
    expect(
      await listPermissionSets(fetchApi, { includeDataAccess: true, includeAutomationAccess: true }),
    ).toHaveLength(1);
    expect(await getPermissionSet(fetchApi, "IdentityUsers")).toMatchObject({ apiName: "IdentityUsers" });
    expect(
      await createPermissionSet(fetchApi, { apiName: "CustomPS", label: "Custom" }),
    ).toMatchObject({ apiName: "CustomPS" });
    expect(await patchPermissionSet(fetchApi, "IdentityUsers", { label: "Users" })).toMatchObject({
      label: "Users",
    });
  });
});
