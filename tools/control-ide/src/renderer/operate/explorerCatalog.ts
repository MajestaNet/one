import type { DescribeField, DescribeObject } from "./types";
import type { GlobalDescribeObject } from "./describeCache";

/** Package filter value: installed objects only (current default). */
export const EXPLORER_ENABLED_PACKAGES = "";
/** Package filter value: installed objects plus catalog objects not yet enabled. */
export const EXPLORER_ALL_PACKAGES = "*";

export type CatalogLookupField = {
  apiName: string;
  label?: string;
  fieldType?: string;
  referenceTo?: string | null;
};

export type CatalogObjectRow = {
  apiName: string;
  label?: string;
  pluralLabel?: string;
  fieldCount?: number;
  fields?: CatalogLookupField[];
};

export type PackageCatalogRow = {
  name: string;
  label?: string;
  enabled?: boolean;
  optional?: boolean;
  objectApiNames?: string[];
  objects?: CatalogObjectRow[];
};

export type ExplorerPackageOption = {
  name: string;
  label: string;
  enabled: boolean;
};

export type ExplorerGraphObject = GlobalDescribeObject & {
  enabled: boolean;
  fieldCount?: number;
};

export function normalizePackageCatalog(raw: unknown): PackageCatalogRow[] {
  const body = raw as { packages?: unknown };
  const list = Array.isArray(body?.packages)
    ? body.packages
    : Array.isArray(raw)
      ? raw
      : [];
  const out: PackageCatalogRow[] = [];
  for (const item of list) {
    const p = item as PackageCatalogRow;
    const name = String(p?.name ?? "").trim();
    if (!name) continue;
    const objects = Array.isArray(p.objects)
      ? p.objects
          .map((o) => ({
            apiName: String(o.apiName ?? "").trim(),
            label: o.label,
            pluralLabel: o.pluralLabel,
            fieldCount: typeof o.fieldCount === "number" ? o.fieldCount : undefined,
            fields: Array.isArray(o.fields) ? o.fields.filter((f) => f?.apiName) : [],
          }))
          .filter((o) => o.apiName)
      : [];
    const objectApiNames = Array.isArray(p.objectApiNames)
      ? p.objectApiNames.map((n) => String(n).trim()).filter(Boolean)
      : [];
    out.push({
      name,
      label: typeof p.label === "string" ? p.label : undefined,
      enabled: p.enabled === true,
      optional: p.optional === true,
      objectApiNames,
      objects,
    });
  }
  return out;
}

function catalogObjectsForPackage(p: PackageCatalogRow): CatalogObjectRow[] {
  if (p.objects?.length) return p.objects;
  return (p.objectApiNames ?? []).map((apiName) => ({ apiName, label: apiName }));
}

function packageHasObjects(p: PackageCatalogRow): boolean {
  return catalogObjectsForPackage(p).length > 0;
}

export function explorerPackageOptions(
  installed: GlobalDescribeObject[],
  catalog: PackageCatalogRow[],
): ExplorerPackageOption[] {
  const byName = new Map<string, ExplorerPackageOption>();
  for (const p of catalog) {
    if (!packageHasObjects(p)) continue;
    byName.set(p.name, {
      name: p.name,
      label: p.label || p.name,
      enabled: p.enabled === true,
    });
  }
  for (const o of installed) {
    const name = o.packageName ?? "core";
    const existing = byName.get(name);
    if (!existing) {
      byName.set(name, { name, label: name, enabled: true });
    }
  }
  return [...byName.values()].sort((a, b) => a.name.localeCompare(b.name));
}

export function mergeExplorerObjects(
  installed: GlobalDescribeObject[],
  catalog: PackageCatalogRow[],
  opts?: { packageFilter?: string },
): ExplorerGraphObject[] {
  const filter = opts?.packageFilter ?? EXPLORER_ENABLED_PACKAGES;
  const byApi = new Map<string, ExplorerGraphObject>();

  const add = (o: ExplorerGraphObject) => {
    const existing = byApi.get(o.apiName);
    if (!existing) {
      byApi.set(o.apiName, o);
      return;
    }
    if (!existing.enabled && o.enabled) {
      byApi.set(o.apiName, { ...o, fieldCount: o.fieldCount ?? existing.fieldCount });
    }
  };

  for (const o of installed) {
    const pkg = o.packageName ?? "core";
    if (filter && filter !== EXPLORER_ALL_PACKAGES && pkg !== filter) continue;
    add({
      apiName: o.apiName,
      label: o.label,
      pluralLabel: o.pluralLabel,
      packageName: o.packageName,
      enabled: true,
    });
  }

  const includeCatalog = filter === EXPLORER_ALL_PACKAGES || Boolean(filter);
  if (includeCatalog) {
    for (const p of catalog) {
      // Enabled packages already appear via Client describe (AuthZ-filtered).
      if (p.enabled === true) continue;
      if (filter && filter !== EXPLORER_ALL_PACKAGES && p.name !== filter) continue;
      for (const obj of catalogObjectsForPackage(p)) {
        add({
          apiName: obj.apiName,
          label: obj.label ?? obj.apiName,
          pluralLabel: obj.pluralLabel,
          packageName: p.name,
          enabled: false,
          fieldCount: obj.fieldCount,
        });
      }
    }
  }

  return [...byApi.values()];
}

export function catalogDescribes(catalog: PackageCatalogRow[]): Map<string, DescribeObject> {
  const out = new Map<string, DescribeObject>();
  for (const p of catalog) {
    for (const obj of catalogObjectsForPackage(p)) {
      const incoming: DescribeField[] = (obj.fields ?? [])
        .filter((f) => f.apiName)
        .map((f) => ({
          apiName: f.apiName,
          label: f.label,
          fieldType: f.fieldType,
          referenceTo: f.referenceTo ?? null,
          relationshipName: null,
          length: null,
        }));
      const existing = out.get(obj.apiName);
      if (!existing) {
        out.set(obj.apiName, {
          apiName: obj.apiName,
          label: obj.label,
          pluralLabel: obj.pluralLabel,
          fields: incoming,
        });
        continue;
      }
      const seen = new Set((existing.fields ?? []).map((f) => f.apiName));
      const fields = [...(existing.fields ?? [])];
      for (const f of incoming) {
        if (!seen.has(f.apiName)) fields.push(f);
      }
      out.set(obj.apiName, { ...existing, fields });
    }
  }
  return out;
}

export function mergeDescribes(
  catalog: Map<string, DescribeObject>,
  installed: Map<string, DescribeObject>,
): Map<string, DescribeObject> {
  const out = new Map(catalog);
  for (const [apiName, desc] of installed) {
    out.set(apiName, desc);
  }
  return out;
}
