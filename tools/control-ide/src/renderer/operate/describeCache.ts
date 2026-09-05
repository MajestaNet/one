import type { DescribeField, DescribeObject } from "./types";

export type GlobalDescribeObject = {
  apiName: string;
  label?: string;
  pluralLabel?: string;
  packageName?: string;
  storageMode?: string;
};

export type GlobalDescribe = {
  objects?: GlobalDescribeObject[];
};

type CacheEntry<T> = { value: T; at: number };

/** In-memory describe cache keyed by install + object. Never written to disk. */
export class DescribeCache {
  private global = new Map<string, CacheEntry<GlobalDescribeObject[]>>();
  private objects = new Map<string, CacheEntry<DescribeObject>>();

  constructor(private ttlMs = 60_000) {}

  private installKey(installId: string): string {
    return installId || "default";
  }

  private objectKey(installId: string, objectApiName: string): string {
    return `${this.installKey(installId)}::${objectApiName}`;
  }

  clear(): void {
    this.global.clear();
    this.objects.clear();
  }

  invalidateInstall(installId: string): void {
    const prefix = `${this.installKey(installId)}::`;
    this.global.delete(this.installKey(installId));
    for (const key of [...this.objects.keys()]) {
      if (key.startsWith(prefix)) this.objects.delete(key);
    }
  }

  getGlobal(installId: string): GlobalDescribeObject[] | null {
    const hit = this.global.get(this.installKey(installId));
    if (!hit) return null;
    if (Date.now() - hit.at > this.ttlMs) {
      this.global.delete(this.installKey(installId));
      return null;
    }
    return hit.value;
  }

  setGlobal(installId: string, objects: GlobalDescribeObject[]): void {
    this.global.set(this.installKey(installId), { value: objects, at: Date.now() });
  }

  getObject(installId: string, objectApiName: string): DescribeObject | null {
    const hit = this.objects.get(this.objectKey(installId, objectApiName));
    if (!hit) return null;
    if (Date.now() - hit.at > this.ttlMs) {
      this.objects.delete(this.objectKey(installId, objectApiName));
      return null;
    }
    return hit.value;
  }

  setObject(installId: string, objectApiName: string, desc: DescribeObject): void {
    this.objects.set(this.objectKey(installId, objectApiName), { value: desc, at: Date.now() });
  }
}

export const describeCache = new DescribeCache();

export function normalizeGlobalObjects(raw: unknown): GlobalDescribeObject[] {
  const body = raw as {
    objects?: Array<Record<string, unknown>>;
    sobjects?: Array<Record<string, unknown>>;
  };
  const list = Array.isArray(body?.objects)
    ? body.objects
    : Array.isArray(body?.sobjects)
      ? body.sobjects
      : [];
  return list
    .map((o) => ({
      apiName: String(o.apiName ?? o.name ?? "").trim(),
      label: typeof o.label === "string" ? o.label : undefined,
      pluralLabel:
        typeof o.pluralLabel === "string"
          ? o.pluralLabel
          : typeof o.labelPlural === "string"
            ? o.labelPlural
            : undefined,
      packageName:
        typeof o.packageName === "string"
          ? o.packageName
          : o.packageName == null
            ? undefined
            : String(o.packageName),
      storageMode:
        typeof o.storageMode === "string"
          ? o.storageMode
          : typeof o.storage_mode === "string"
            ? o.storage_mode
            : undefined,
    }))
    .filter((o) => o.apiName);
}

export function normalizeDescribeObject(raw: unknown, fallbackApiName: string): DescribeObject {
  const d = (raw ?? {}) as DescribeObject;
  const fields = Array.isArray(d.fields) ? d.fields : [];
  return {
    apiName: d.apiName ?? fallbackApiName,
    label: d.label,
    pluralLabel: d.pluralLabel,
    storageMode: d.storageMode,
    fields: fields
      .map((f) => normalizeField(f))
      .filter((f): f is DescribeField => Boolean(f?.apiName)),
  };
}

function normalizeField(f: DescribeField | Record<string, unknown>): DescribeField | null {
  const apiName = String((f as DescribeField).apiName ?? (f as { name?: string }).name ?? "").trim();
  if (!apiName) return null;
  return {
    apiName,
    label: (f as DescribeField).label,
    fieldType: (f as DescribeField).fieldType ?? (f as { type?: string }).type,
    required: (f as DescribeField).required,
    filterable: (f as DescribeField).filterable,
    sortable: (f as DescribeField).sortable,
    picklistValues: (f as DescribeField).picklistValues,
    referenceTo: (f as DescribeField).referenceTo ?? null,
    relationshipName: (f as DescribeField).relationshipName ?? null,
    length: (f as DescribeField).length ?? null,
  };
}

export const FILTER_OPS = [
  "eq",
  "ne",
  "gt",
  "gte",
  "lt",
  "lte",
  "like",
  "in",
  "is_null",
  "is_not_null",
] as const;
