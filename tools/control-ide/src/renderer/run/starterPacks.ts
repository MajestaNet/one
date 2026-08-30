import { mirrorPlaybookYaml, mirrorToolSpecYaml } from "../metadataMirror";
import type { FetchFn } from "./tools";
import type { ToolLayout, ToolNode, ToolQueryBinding } from "./types";

export type WorkflowStarterPack = {
  id: string;
  label: string;
  description: string;
  objectApiNames: string[];
  tool: {
    apiName: string;
    label: string;
    description: string;
    sortOrder: number;
    layout: ToolLayout;
    nodes: ToolNode[];
    dataBindings: ToolQueryBinding[];
  };
  agent: {
    apiName: string;
    label: string;
    goalTemplate: string;
    instructions: string;
  };
  automation: {
    apiName: string;
    label: string;
    objectApiName: string;
  };
};

function sections(nodeIds: string[]): ToolLayout {
  return { mode: "sections", sections: [{ id: "main", title: "Workspace", nodeIds }] };
}

export const WORKFLOW_STARTER_PACKS: WorkflowStarterPack[] = [
  {
    id: "open-pipeline",
    label: "Open pipeline",
    description: "Live opportunity lanes, pipeline totals, and follow-up actions.",
    objectApiNames: ["Opportunity"],
    tool: {
      apiName: "OpenPipeline",
      label: "Open pipeline",
      description: "Cloneable sales workspace generated from live Opportunity records.",
      sortOrder: 100,
      layout: sections(["header", "open-count", "prospecting", "negotiation", "actions"]),
      dataBindings: [
        { id: "open", objectApiName: "Opportunity", query: { filters: [{ field: "IsClosed", op: "eq", value: false }], limit: 100 } },
        { id: "prospecting", objectApiName: "Opportunity", query: { filters: [{ field: "StageName", op: "eq", value: "Prospecting" }], limit: 50 } },
        { id: "negotiation", objectApiName: "Opportunity", query: { filters: [{ field: "StageName", op: "eq", value: "Negotiation" }], limit: 50 } },
      ],
      nodes: [
        { id: "header", kind: "sectionHeader", title: "Open pipeline", props: { subtitle: "Live Client API data · sharing and FLS enforced" } },
        { id: "open-count", kind: "stat", title: "Open opportunities", bindingId: "open", props: { label: "Open opportunities", value: null } },
        { id: "prospecting", kind: "pipelineLane", title: "Prospecting", bindingId: "prospecting", props: { stage: "Prospecting" } },
        { id: "negotiation", kind: "pipelineLane", title: "Negotiation", bindingId: "negotiation", props: { stage: "Negotiation" } },
        { id: "actions", kind: "actionChipGroup", title: "Pipeline actions", props: { actions: [{ label: "Prepare follow-ups", type: "automationRun", automationApiName: "PreparePipelineFollowUps", input: { source: "OpenPipeline" } }, { label: "Ask pipeline coach", prompt: "Summarize risk and next actions across the open pipeline" }] } },
      ],
    },
    agent: {
      apiName: "RunPipelineCoach",
      label: "Pipeline coach",
      goalTemplate: "Review the open pipeline and recommend the highest-value next actions.",
      instructions: "Use live Opportunity data, explain evidence, and ask before proposing record mutations.",
    },
    automation: { apiName: "PreparePipelineFollowUps", label: "Prepare pipeline follow-ups", objectApiName: "Opportunity" },
  },
  {
    id: "case-triage",
    label: "Case queue + triage",
    description: "A service queue with priority context and a safe triage action.",
    objectApiNames: ["Case"],
    tool: {
      apiName: "CaseQueueTriage",
      label: "Case queue + triage",
      description: "Cloneable support queue generated from live Case records.",
      sortOrder: 110,
      layout: sections(["header", "open-count", "queue", "actions"]),
      dataBindings: [{ id: "cases", objectApiName: "Case", query: { filters: [{ field: "Status", op: "ne", value: "Closed" }], sort: [{ field: "Priority", direction: "asc" }], limit: 100 } }],
      nodes: [
        { id: "header", kind: "sectionHeader", title: "Case queue", props: { subtitle: "Triage against live service records" } },
        { id: "open-count", kind: "stat", title: "Open cases", bindingId: "cases", props: { label: "Open cases", value: null } },
        { id: "queue", kind: "recordTable", title: "Cases needing attention", bindingId: "cases", props: { columns: [{ key: "CaseNumber", label: "Case" }, { key: "Subject", label: "Subject" }, { key: "Priority", label: "Priority" }, { key: "Status", label: "Status" }] } },
        { id: "actions", kind: "actionChipGroup", title: "Triage", props: { actions: [{ label: "Prepare triage", type: "automationRun", automationApiName: "PrepareCaseTriage", input: { source: "CaseQueueTriage" } }, { label: "Ask service coach", prompt: "Triage the visible cases by urgency and customer impact" }] } },
      ],
    },
    agent: {
      apiName: "RunServiceCoach",
      label: "Service coach",
      goalTemplate: "Triage the open case queue and explain priority recommendations.",
      instructions: "Use live Case records, preserve auditability, and require approval for mutations.",
    },
    automation: { apiName: "PrepareCaseTriage", label: "Prepare case triage", objectApiName: "Case" },
  },
  {
    id: "quote-follow-up",
    label: "Quote follow-up",
    description: "A quote worklist with owner-ready follow-up actions.",
    objectApiNames: ["Quote"],
    tool: {
      apiName: "QuoteFollowUp",
      label: "Quote follow-up",
      description: "Cloneable quote follow-up workspace generated from live Quote records.",
      sortOrder: 120,
      layout: sections(["header", "quote-count", "quotes", "actions"]),
      dataBindings: [{ id: "quotes", objectApiName: "Quote", query: { limit: 100 } }],
      nodes: [
        { id: "header", kind: "sectionHeader", title: "Quote follow-up", props: { subtitle: "Keep commercial follow-up visible and reviewable" } },
        { id: "quote-count", kind: "stat", title: "Quotes", bindingId: "quotes", props: { label: "Quotes in view", value: null } },
        { id: "quotes", kind: "recordTable", title: "Quote worklist", bindingId: "quotes", props: { columns: [{ key: "Name", label: "Quote" }, { key: "Status", label: "Status" }, { key: "TotalPrice", label: "Total" }, { key: "ExpirationDate", label: "Expires" }] } },
        { id: "actions", kind: "actionChipGroup", title: "Follow up", props: { actions: [{ label: "Prepare follow-ups", type: "automationRun", automationApiName: "PrepareQuoteFollowUps", input: { source: "QuoteFollowUp" } }, { label: "Draft owner brief", prompt: "Draft a concise owner follow-up brief for the visible quotes" }] } },
      ],
    },
    agent: {
      apiName: "RunQuoteCoach",
      label: "Quote coach",
      goalTemplate: "Review active quotes and recommend timely owner follow-up.",
      instructions: "Use live Quote data and distinguish evidence from recommendations.",
    },
    automation: { apiName: "PrepareQuoteFollowUps", label: "Prepare quote follow-ups", objectApiName: "Quote" },
  },
];

function includesApiName(raw: unknown, key: string, apiName: string): boolean {
  const rows = (raw as Record<string, unknown>)?.[key];
  return Array.isArray(rows) && rows.some((row) => row && typeof row === "object" && (row as { apiName?: string }).apiName === apiName);
}

async function mirrorAutomation(repoPath: string | undefined, pack: WorkflowStarterPack): Promise<string | null> {
  if (!repoPath || !window.one?.writeText) return null;
  const a = pack.automation;
  const yaml = [
    `apiName: ${a.apiName}`,
    `label: ${a.label}`,
    `objectApiName: ${a.objectApiName}`,
    "triggerEvent: manual",
    "active: true",
    "runtime: actions",
    "execution: async",
    "ownership: custom",
    "packageName: customer.default",
    "actions: []",
    "",
  ].join("\n");
  try {
    await window.one.writeText(repoPath, `metadata/automations/${a.apiName}.yaml`, yaml);
    return null;
  } catch (error) {
    return `Automation YAML mirror failed: ${String(error)}`;
  }
}

export async function installWorkflowStarterPack(
  fetchFn: FetchFn,
  pack: WorkflowStarterPack,
  repoPath?: string,
): Promise<{ warnings: string[] }> {
  const [tools, agents, automations] = await Promise.all([
    fetchFn("/metadata/v1/tools"),
    fetchFn("/metadata/v1/agents/playbooks"),
    fetchFn("/metadata/v1/automations"),
  ]);
  const conflicts = [
    includesApiName(tools, "tools", pack.tool.apiName) && `ToolSpec ${pack.tool.apiName}`,
    includesApiName(agents, "playbooks", pack.agent.apiName) && `AgentSpec ${pack.agent.apiName}`,
    includesApiName(automations, "automations", pack.automation.apiName) && `Automation ${pack.automation.apiName}`,
  ].filter(Boolean);
  if (conflicts.length) throw new Error(`Starter already exists: ${conflicts.join(", ")}`);

  const created: Array<{ path: string }> = [];
  try {
    await fetchFn("/metadata/v1/automations", {
      method: "POST",
      body: JSON.stringify({ ...pack.automation, triggerEvent: "manual", active: true, runtime: "actions", execution: "async", actions: [] }),
    });
    created.push({ path: `/metadata/v1/automations/${encodeURIComponent(pack.automation.apiName)}` });

    await fetchFn("/metadata/v1/agents/playbooks", {
      method: "POST",
      body: JSON.stringify({ ...pack.agent, allowedTools: ["query", "describe", "sobjects.read"], objectScopes: pack.objectApiNames, allowedSkills: [pack.automation.apiName], requireApproval: true, active: true }),
    });
    created.push({ path: `/metadata/v1/agents/playbooks/${encodeURIComponent(pack.agent.apiName)}` });

    await fetchFn("/metadata/v1/tools", {
      method: "POST",
      body: JSON.stringify({ ...pack.tool, active: true }),
    });
    created.push({ path: `/metadata/v1/tools/${encodeURIComponent(pack.tool.apiName)}` });
  } catch (error) {
    await Promise.allSettled(created.reverse().map((item) => fetchFn(item.path, { method: "DELETE" })));
    throw error;
  }

  const warnings = (await Promise.all([
    mirrorToolSpecYaml(repoPath, { ...pack.tool, active: true, ownership: "custom", packageName: "customer.default" }),
    mirrorPlaybookYaml(repoPath, { ...pack.agent, allowedTools: ["query", "describe", "sobjects.read"], objectScopes: pack.objectApiNames, allowedSkills: [pack.automation.apiName], requireApproval: true, active: true, ownership: "custom", packageName: "customer.default" }),
    mirrorAutomation(repoPath, pack),
  ])).filter((warning): warning is string => Boolean(warning));
  return { warnings };
}
