import { describe, expect, it, vi } from "vitest";
import { installWorkflowStarterPack, WORKFLOW_STARTER_PACKS } from "./starterPacks";

describe("workflow starter packs", () => {
  it("installs a ToolSpec, AgentSpec, and automation after conflict preflight", async () => {
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (!init) {
        if (path.endsWith("/tools")) return { tools: [] };
        if (path.endsWith("/playbooks")) return { playbooks: [] };
        if (path.endsWith("/automations")) return { automations: [] };
      }
      return { ok: true };
    });
    await installWorkflowStarterPack(fetchFn, WORKFLOW_STARTER_PACKS[0]);
    const posts = fetchFn.mock.calls.filter(([, init]) => init?.method === "POST");
    expect(posts.map(([path]) => path)).toEqual([
      "/metadata/v1/automations",
      "/metadata/v1/agents/playbooks",
      "/metadata/v1/tools",
    ]);
    const agentBody = JSON.parse(String(posts[1][1]?.body));
    expect(agentBody.allowedSkills).toEqual(["PreparePipelineFollowUps"]);
    expect(agentBody.objectScopes).toEqual(["Opportunity"]);
  });

  it("refuses a partial duplicate before writing", async () => {
    const pack = WORKFLOW_STARTER_PACKS[1];
    const fetchFn = vi.fn(async (path: string) => {
      if (path.endsWith("/tools")) return { tools: [{ apiName: pack.tool.apiName }] };
      if (path.endsWith("/playbooks")) return { playbooks: [] };
      return { automations: [] };
    });
    await expect(installWorkflowStarterPack(fetchFn, pack)).rejects.toThrow(/Starter already exists/);
    expect(fetchFn.mock.calls.some(([, init]) => init?.method === "POST")).toBe(false);
  });
});
