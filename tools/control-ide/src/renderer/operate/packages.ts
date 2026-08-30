import type { ObjectTab, RelatedListDef } from "./types";

/** Core + package-gated Operate tabs (BP-019). */
export const BASE_OBJECT_TABS: ObjectTab[] = [
  { id: "Account", objectApiName: "Account", label: "Accounts" },
  { id: "Contact", objectApiName: "Contact", label: "Contacts" },
  {
    id: "Opportunity",
    objectApiName: "Opportunity",
    label: "Opportunities",
    packageName: "sales",
    boardField: "StageName",
  },
  { id: "Quote", objectApiName: "Quote", label: "Quotes", packageName: "sales" },
  {
    id: "Case",
    objectApiName: "Case",
    label: "Cases",
    packageName: "service",
    boardField: "Status",
  },
];

/** Offline draft still shows classic three tabs even without package probe. */
export const OFFLINE_OBJECT_TABS: ObjectTab[] = [
  { id: "Account", objectApiName: "Account", label: "Accounts" },
  { id: "Contact", objectApiName: "Contact", label: "Contacts" },
  {
    id: "Opportunity",
    objectApiName: "Opportunity",
    label: "Opportunities",
    packageName: "sales",
    boardField: "StageName",
  },
];

export const RELATED_LISTS: Record<string, RelatedListDef[]> = {
  Account: [
    { objectApiName: "Contact", lookupField: "AccountId", label: "Contacts" },
    { objectApiName: "Opportunity", lookupField: "AccountId", label: "Opportunities", packageName: "sales" },
    { objectApiName: "Case", lookupField: "AccountId", label: "Cases", packageName: "service" },
  ],
  Contact: [
    { objectApiName: "Opportunity", lookupField: "ContactId", label: "Opportunities", packageName: "sales" },
    { objectApiName: "Case", lookupField: "ContactId", label: "Cases", packageName: "service" },
  ],
  Opportunity: [
    { objectApiName: "Quote", lookupField: "OpportunityId", label: "Quotes", packageName: "sales" },
  ],
  Case: [
    { objectApiName: "WorkOrder", lookupField: "CaseId", label: "Work Orders", packageName: "service" },
  ],
};

export type PackageRow = {
  name?: string;
  enabled?: boolean;
  installedVersion?: string;
};

export function enabledPackageNames(packages: PackageRow[]): Set<string> {
  const set = new Set<string>(["core"]);
  for (const p of packages) {
    if (p.name && p.enabled === true) set.add(p.name);
  }
  return set;
}

export function tabsForPackages(enabled: Set<string> | null, connected: boolean): ObjectTab[] {
  if (!connected || !enabled) return OFFLINE_OBJECT_TABS;
  return BASE_OBJECT_TABS.filter((t) => !t.packageName || enabled.has(t.packageName));
}

export function relatedListsFor(
  objectApiName: string,
  enabled: Set<string> | null,
): RelatedListDef[] {
  const all = RELATED_LISTS[objectApiName] ?? [];
  if (!enabled) {
    return all.filter((r) => !r.packageName || r.packageName === "sales");
  }
  return all.filter((r) => !r.packageName || enabled.has(r.packageName));
}

export async function fetchEnabledPackages(
  fetchFn: (path: string, init?: RequestInit) => Promise<unknown>,
): Promise<Set<string>> {
  try {
    const res = (await fetchFn("/metadata/v1/packages")) as { packages?: PackageRow[] };
    return enabledPackageNames(res.packages ?? []);
  } catch {
    // Without metadata scope, assume core + sales so Opportunity tab remains (compat with thin board)
    return new Set(["core", "sales"]);
  }
}
