import type { AppSection } from "../workspace/types";
import { ACCOUNT_LAUNCHER, MODES } from "../workspace/types";

/** BP-053 primary section = four launcher tiles + settings. */
export const PRIMARY_SECTIONS: AppSection[] = ["operate", "build", "govern", "settings"];

export function sectionLabel(section: AppSection): string {
  if (section === "settings") return ACCOUNT_LAUNCHER.label;
  return MODES.find((m) => m.id === section)?.label ?? section;
}

export function sectionTagline(section: AppSection): string {
  if (section === "settings") return ACCOUNT_LAUNCHER.tagline;
  return MODES.find((m) => m.id === section)?.tagline ?? "";
}

/**
 * Resolve the IDE dock home for a stored primarySection (+ optional harness id).
 * Canonical launcher sections: operate | build | govern | settings.
 */
export function dockSectionForPrimarySection(
  primarySection?: string | null,
  harnessId?: string | null,
): AppSection {
  const s = (primarySection || "").trim().toLowerCase();
  const harness = (harnessId || "").trim();
  if (s === "settings" || s === "govern") return s;
  if (s === "build" || s === "ship") return "build";
  if (s === "run") return "operate";
  if (s === "operate") {
    // Graph agents (harness.run.*) dock on Operate; legacy chat/query → Build (BP-057).
    if (harness.startsWith("harness.run.")) return "operate";
    return "build";
  }
  return "operate";
}

/** Map Client/Metadata primarySection onto dock modes (single home). */
export function modesFromPrimarySection(
  primarySection?: string | null,
  harnessId?: string | null,
): AppSection[] {
  return [dockSectionForPrimarySection(primarySection, harnessId)];
}

export function slugApiName(label: string): string {
  const trimmed = label.trim();
  if (!trimmed) return "";
  const withoutSuffix = trimmed.replace(/__c$/i, "");
  const cleaned = withoutSuffix.replace(/[^a-zA-Z0-9]+/g, "");
  if (!cleaned) return "";
  const capped = cleaned.charAt(0).toUpperCase() + cleaned.slice(1);
  return `${capped}__c`;
}

export const SECTION_INSTRUCTION_STUBS: Record<AppSection, string> = {
  operate:
    "You help users curate personal graph attention and compose declarative Tools. As Curator, maintain My day cluster membership, live signals, blocks, weighted next, and watches with graph.* topology writes. Pin object collections with graph.pinCollection when the user needs a list home on the graph; do not dump query rows as record nodes. As Doer, stage proposals for human Apply and never silently mutate CRM through graph state. When runGraph or refs-only selection context is present, end each substantive turn with graph.pin, graph.pinCollection, graph.link, graph.annotate, or a staged proposal; if no topology change is appropriate, explain why honestly. After a personal workflow proves reusable, suggest graph.publishSubgraph as an org ToolSpec playbook; never imply the personal graph is shared. Never persist hydrated record fields. Prefer query before writes; high-risk mutations need approval.",
  build:
    "You help design customer-owned metadata, validate and ship changes, and inspect the active install. Prefer query before writes; deep-link humans to Build panels for privileged deploy actions.",
  govern:
    "You help with principals, roles, permission sets, and install policy. Prefer read before write; identity changes always require approval.",
  settings:
    "You orient users on Account, Hosting, Inference, and Environments. Prefer read-only guidance. Never echo Hosting secrets or API keys.",
};

export type AgentHarness = {
  id: string;
  section: AppSection | string;
  version: string;
  label: string;
  job: string;
  systemPreamble?: string;
  toolFloor?: string[];
  requireApprovalDefault?: boolean;
  contextPackHints?: string[];
  chromeHints?: string[];
};

export async function listAgentHarnesses(
  fetchFn: (path: string, init?: RequestInit) => Promise<unknown>,
): Promise<AgentHarness[]> {
  const row = (await fetchFn("/metadata/v1/agents/harnesses")) as { harnesses?: AgentHarness[] };
  return row.harnesses ?? [];
}
