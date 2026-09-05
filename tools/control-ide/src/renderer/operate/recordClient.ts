/** Client record helpers — kernel User is identity, not DataEngine query (ADR-026). */

import type { DescribeField, QueryFilter, SortSpec } from "./types";
import { SYSTEM_FIELD_SKIP } from "./types";

export type RecordFetch = (path: string, init?: RequestInit) => Promise<unknown>;

export type QueryRecordsInput = {
  object: string;
  select?: string[];
  filters?: QueryFilter[];
  sort?: SortSpec[];
  limit?: number;
};

const KERNEL_IDENTITY_OBJECTS = new Set(["User"]);

export function isKernelIdentityObject(apiName: string, storageMode?: string): boolean {
  if ((storageMode ?? "").toLowerCase() === "kernel") return true;
  return KERNEL_IDENTITY_OBJECTS.has(apiName.trim());
}

/** Prefer a filterable name-like field that actually exists on the object. */
export function listIdentityField(objectApiName: string): string {
  switch (objectApiName) {
    case "Contact":
    case "Lead":
      return "LastName";
    case "Case":
    case "Task":
    case "Event":
      return "Subject";
    case "User":
      return "DisplayName";
    default:
      return "Name";
  }
}

export function recordWritePayload(
  fields: DescribeField[],
  values: Record<string, unknown>,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const field of fields) {
    if (!field.apiName || SYSTEM_FIELD_SKIP.has(field.apiName)) continue;
    const value = values[field.apiName];
    if (value == null) continue;
    if (typeof value === "string" && value.trim() === "") continue;
    out[field.apiName] = value;
  }
  return out;
}

export function principalToRecord(raw: Record<string, unknown>): Record<string, unknown> {
  const name = (raw.name ?? {}) as Record<string, unknown>;
  const id = String(raw.id ?? raw.Id ?? "");
  const email = raw.email ?? raw.Email;
  const displayName = raw.displayName ?? raw.DisplayName;
  const given = raw.GivenName ?? name.givenName ?? raw.givenName;
  const family = raw.FamilyName ?? name.familyName ?? raw.familyName;
  const phones = raw.phoneNumbers;
  const phone =
    raw.Phone ??
    raw.phone ??
    (Array.isArray(phones) && phones[0] && typeof phones[0] === "object"
      ? (phones[0] as Record<string, unknown>).value
      : undefined);
  const rec: Record<string, unknown> = {
    Id: id,
    id,
    Email: email,
    DisplayName: displayName,
    Name: displayName || email,
    Username: raw.userName ?? raw.Username,
    GivenName: given,
    FamilyName: family,
    Phone: phone,
    Locale: raw.locale ?? raw.Locale,
    Timezone: raw.timezone ?? raw.Timezone,
    Title: raw.title ?? raw.Title,
    Department: raw.department ?? raw.Department,
    EmployeeNumber: raw.employeeNumber ?? raw.EmployeeNumber,
    ExternalId: raw.externalId ?? raw.ExternalId,
    IsActive: raw.isActive ?? raw.IsActive,
    PrincipalType: raw.principalType ?? raw.PrincipalType,
    IsAdmin: raw.isAdmin ?? raw.IsAdmin,
  };
  for (const [key, value] of Object.entries(raw)) {
    if (key in rec || value == null) continue;
    if (["name", "emails", "phoneNumbers", "roleApiNames", "permissionSetAPINames", "permissionSetApiNames"].includes(key)) {
      continue;
    }
    rec[key] = value;
  }
  return rec;
}

function recordToPrincipalBody(
  data: Record<string, unknown>,
  opts?: { creating?: boolean },
): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  const email = data.Email ?? data.email;
  if (typeof email === "string" && email.trim()) body.email = email.trim();
  const display = data.DisplayName ?? data.displayName ?? data.Name;
  if (typeof display === "string" && display.trim()) body.displayName = display.trim();
  const userName = data.Username ?? data.userName;
  if (typeof userName === "string" && userName.trim()) body.userName = userName.trim();
  const given = data.GivenName ?? data.givenName;
  const family = data.FamilyName ?? data.familyName;
  if ((typeof given === "string" && given.trim()) || (typeof family === "string" && family.trim())) {
    body.name = {
      givenName: typeof given === "string" ? given.trim() : "",
      familyName: typeof family === "string" ? family.trim() : "",
    };
  }
  const phone = data.Phone ?? data.phone;
  if (typeof phone === "string" && phone.trim()) body.phoneNumbers = [{ value: phone.trim() }];
  const locale = data.Locale ?? data.locale;
  if (typeof locale === "string" && locale.trim()) body.locale = locale.trim();
  const timezone = data.Timezone ?? data.timezone;
  if (typeof timezone === "string" && timezone.trim()) body.timezone = timezone.trim();
  const title = data.Title ?? data.title;
  if (typeof title === "string" && title.trim()) body.title = title.trim();
  const department = data.Department ?? data.department;
  if (typeof department === "string" && department.trim()) body.department = department.trim();
  const principalType = data.PrincipalType ?? data.principalType;
  if (typeof principalType === "string" && principalType.trim()) body.principalType = principalType.trim();
  const roles = data.roleApiNames ?? data.RoleApiNames;
  if (Array.isArray(roles) && roles.length) body.roleApiNames = roles;
  else if (opts?.creating) body.roleApiNames = ["StandardUser"];
  return body;
}

function fieldValue(row: Record<string, unknown>, field: string): unknown {
  if (field in row) return row[field];
  const lower = field.charAt(0).toLowerCase() + field.slice(1);
  if (lower in row) return row[lower];
  return undefined;
}

function matchesFilter(row: Record<string, unknown>, filter: QueryFilter): boolean {
  const actual = fieldValue(row, filter.field);
  const actualStr = actual == null ? "" : String(actual);
  const want = filter.value == null ? "" : String(filter.value);
  switch (filter.op) {
    case "eq":
      return actualStr === want;
    case "ne":
      return actualStr !== want;
    case "like":
      return actualStr.toLowerCase().includes(want.toLowerCase());
    case "is_null":
      return actual == null || actualStr === "";
    case "is_not_null":
      return actual != null && actualStr !== "";
    default:
      return true;
  }
}

function sortRecords(rows: Record<string, unknown>[], sort: SortSpec[]): Record<string, unknown>[] {
  if (!sort[0]) return rows;
  const { field, direction } = sort[0];
  const mul = direction === "desc" ? -1 : 1;
  return [...rows].sort((a, b) => {
    const av = String(fieldValue(a, field) ?? "");
    const bv = String(fieldValue(b, field) ?? "");
    return mul * av.localeCompare(bv);
  });
}

function projectSelect(row: Record<string, unknown>, select?: string[]): Record<string, unknown> {
  if (!select?.length) return row;
  const out: Record<string, unknown> = { Id: row.Id ?? row.id, id: row.id ?? row.Id };
  for (const key of select) {
    const v = fieldValue(row, key);
    if (v !== undefined) out[key] = v;
  }
  return out;
}

async function queryKernelUser(
  fetchFn: RecordFetch,
  input: QueryRecordsInput,
): Promise<{ records: Record<string, unknown>[] }> {
  const raw = (await fetchFn("/client/v1/principals?principalType=user")) as
    | { principals?: Record<string, unknown>[] }
    | Record<string, unknown>[];
  const list = Array.isArray(raw) ? raw : (raw.principals ?? []);
  let rows = list.map(principalToRecord);
  for (const filter of input.filters ?? []) {
    rows = rows.filter((row) => matchesFilter(row, filter));
  }
  rows = sortRecords(rows, input.sort ?? []);
  const limit = Math.min(Math.max(1, Math.trunc(input.limit ?? 50)), 200);
  rows = rows.slice(0, limit).map((row) => projectSelect(row, input.select));
  return { records: rows };
}

/** Query flexible objects via DataEngine; kernel User via Client principals (ADR-026). */
export async function queryRecords(
  fetchFn: RecordFetch,
  input: QueryRecordsInput,
): Promise<{ records: Record<string, unknown>[] }> {
  if (isKernelIdentityObject(input.object)) {
    return queryKernelUser(fetchFn, input);
  }
  const raw = (await fetchFn("/client/v1/query", {
    method: "POST",
    body: JSON.stringify({
      object: input.object,
      select: input.select,
      filters: input.filters,
      sort: input.sort,
      limit: input.limit ?? 50,
    }),
  })) as { records?: Record<string, unknown>[] };
  return { records: raw.records ?? [] };
}

export async function getRecord(
  fetchFn: RecordFetch,
  objectApiName: string,
  id: string,
): Promise<Record<string, unknown>> {
  if (isKernelIdentityObject(objectApiName)) {
    const raw = (await fetchFn(`/client/v1/principals/${encodeURIComponent(id)}`)) as Record<string, unknown>;
    return principalToRecord(raw);
  }
  return (await fetchFn(
    `/client/v1/sobjects/${encodeURIComponent(objectApiName)}/${encodeURIComponent(id)}`,
  )) as Record<string, unknown>;
}

export async function createRecord(
  fetchFn: RecordFetch,
  objectApiName: string,
  data: Record<string, unknown>,
): Promise<Record<string, unknown>> {
  if (isKernelIdentityObject(objectApiName)) {
    const created = (await fetchFn("/client/v1/principals", {
      method: "POST",
      body: JSON.stringify(recordToPrincipalBody(data, { creating: true })),
    })) as Record<string, unknown>;
    return principalToRecord(created);
  }
  return (await fetchFn(`/client/v1/sobjects/${encodeURIComponent(objectApiName)}`, {
    method: "POST",
    body: JSON.stringify(data),
  })) as Record<string, unknown>;
}

export async function updateRecord(
  fetchFn: RecordFetch,
  objectApiName: string,
  id: string,
  data: Record<string, unknown>,
): Promise<unknown> {
  if (isKernelIdentityObject(objectApiName)) {
    return fetchFn(`/client/v1/principals/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify(recordToPrincipalBody(data)),
    });
  }
  return fetchFn(`/client/v1/sobjects/${encodeURIComponent(objectApiName)}/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}
