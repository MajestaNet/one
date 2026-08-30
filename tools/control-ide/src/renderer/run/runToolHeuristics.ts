import type { ToolBridgeCall } from "./toolContracts";
import type { ToolQueryBinding } from "./types";

const REFRESH_RE = /\b(refresh|reload|update)\b.*\b(list|table|tool|accounts?|records?)\b/i;
const TOP_N_RE = /\b(top|first|highest|best|lowest)\s+(\d+)\b/i;
const RANKING_RE = /\b(rank|ranking|score|priority)\b/i;

function parseTopN(goal: string): number | null {
  const topFirst = goal.match(TOP_N_RE);
  if (topFirst) {
    const n = Number(topFirst[2]);
    return Number.isFinite(n) && n > 0 ? Math.min(n, 500) : null;
  }
  const countAccounts = goal.match(/\b(\d+)\s+accounts?\b/i);
  if (countAccounts) {
    const n = Number(countAccounts[1]);
    return Number.isFinite(n) && n > 0 ? Math.min(n, 500) : null;
  }
  return null;
}

function guessSortField(goal: string): string {
  if (/account/i.test(goal)) {
    if (RANKING_RE.test(goal)) return "AnnualRevenue";
    return "Name";
  }
  if (RANKING_RE.test(goal)) return "Priority";
  return "Name";
}

/** Narrow safety net when playbook returns prose-only but user asked to refresh/filter a list. */
export function heuristicToolCallsFromGoal(
  goal: string,
  toolId: string,
  bindings: ToolQueryBinding[] = [],
): ToolBridgeCall[] | null {
  const trimmed = goal.trim();
  if (!trimmed || !toolId) return null;
  const wantsRefresh = REFRESH_RE.test(trimmed) || TOP_N_RE.test(trimmed) || RANKING_RE.test(trimmed);
  if (!wantsRefresh) return null;

  const topN = parseTopN(trimmed);
  const sortField = guessSortField(trimmed);
  const direction = /\b(lowest|asc|ascending)\b/i.test(trimmed) ? "asc" : "desc";

  const nextBindings = bindings.length
    ? bindings.map((b, i) => {
        if (i > 0) return b;
        const query = { ...(b.query ?? {}) };
        if (topN != null) query.limit = topN;
        query.sort = [{ field: sortField, direction }];
        return { ...b, query };
      })
    : [
        {
          id: "main",
          objectApiName: /opportunit/i.test(trimmed) ? "Opportunity" : "Account",
          query: {
            limit: topN ?? 50,
            sort: [{ field: sortField, direction }],
          },
        },
      ];

  return [
    { tool: "tool.update", input: { toolId, patch: { dataBindings: nextBindings } } },
    { tool: "tool.rerun", input: { toolId } },
  ];
}
