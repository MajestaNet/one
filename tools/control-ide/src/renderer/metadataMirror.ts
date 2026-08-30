/** Minimal YAML emitters for one/v1 object/field dual-write. */

function yamlScalar(value: unknown): string {
  if (value == null) return "null";
  if (typeof value === "boolean" || typeof value === "number") return String(value);
  const s = String(value);
  if (s === "" || /[:#\n'"{}[\],&*?|<>=!%@`]/.test(s) || /^\s|\s$/.test(s)) {
    return JSON.stringify(s);
  }
  return s;
}

export type ObjectMirror = {
  apiName: string;
  label: string;
  pluralLabel?: string;
  storageMode?: string;
  ownership?: string;
  features?: Record<string, boolean>;
};

export type FieldMirror = {
  objectApiName: string;
  apiName: string;
  label: string;
  fieldType: string;
  required?: boolean;
  uniqueField?: boolean;
  indexed?: boolean;
  filterable?: boolean;
  sortable?: boolean;
  length?: number | null;
  ownership?: string;
  picklistValues?: string[];
  referenceTo?: string | null;
};

export function objectToYaml(obj: ObjectMirror): string {
  const lines = [
    `apiName: ${yamlScalar(obj.apiName)}`,
    `label: ${yamlScalar(obj.label)}`,
    `pluralLabel: ${yamlScalar(obj.pluralLabel || `${obj.label}s`)}`,
    `storageMode: ${yamlScalar(obj.storageMode || "flexible")}`,
    `ownership: ${yamlScalar(obj.ownership || "custom")}`,
  ];
  if (obj.features && Object.keys(obj.features).length) {
    lines.push("features:");
    for (const [k, v] of Object.entries(obj.features)) {
      lines.push(`  ${k}: ${v ? "true" : "false"}`);
    }
  }
  return `${lines.join("\n")}\n`;
}

export function fieldToYaml(field: FieldMirror): string {
  const lines = [
    `objectApiName: ${yamlScalar(field.objectApiName)}`,
    `apiName: ${yamlScalar(field.apiName)}`,
    `label: ${yamlScalar(field.label)}`,
    `fieldType: ${yamlScalar(field.fieldType)}`,
    `required: ${field.required ? "true" : "false"}`,
    `uniqueField: ${field.uniqueField ? "true" : "false"}`,
    `indexed: ${field.indexed ? "true" : "false"}`,
    `filterable: ${field.filterable !== false ? "true" : "false"}`,
    `sortable: ${field.sortable !== false ? "true" : "false"}`,
    `ownership: ${yamlScalar(field.ownership || "custom")}`,
  ];
  if (field.length != null) lines.push(`length: ${field.length}`);
  if (field.referenceTo) lines.push(`referenceTo: ${yamlScalar(field.referenceTo)}`);
  if (field.picklistValues?.length) {
    lines.push("picklistValues:");
    for (const v of field.picklistValues) lines.push(`  - ${yamlScalar(v)}`);
  }
  return `${lines.join("\n")}\n`;
}

export function objectYamlPath(apiName: string): string {
  return `metadata/objects/${apiName}.yaml`;
}

export function fieldYamlPath(objectApiName: string, apiName: string): string {
  return `metadata/fields/${objectApiName}/${apiName}.yaml`;
}

export type PlaybookMirror = {
  apiName: string;
  label: string;
  goalTemplate?: string;
  instructions?: string;
  primarySection?: string;
  harnessId?: string;
  harnessVersion?: string;
  allowedTools?: string[];
  objectScopes?: string[];
  allowedSkills?: string[];
  requireApproval?: boolean;
  active?: boolean;
  ownership?: string;
  packageName?: string;
};

export function playbookYamlPath(apiName: string): string {
  return `metadata/agents/playbooks/${apiName}.yaml`;
}

export function playbookToYaml(pb: PlaybookMirror): string {
  const lines = [
    `apiName: ${yamlScalar(pb.apiName)}`,
    `label: ${yamlScalar(pb.label)}`,
    `goalTemplate: ${yamlScalar(pb.goalTemplate || "")}`,
    `instructions: ${yamlScalar(pb.instructions || "")}`,
    `requireApproval: ${pb.requireApproval !== false ? "true" : "false"}`,
    `active: ${pb.active !== false ? "true" : "false"}`,
    `ownership: ${yamlScalar(pb.ownership || "custom")}`,
    `packageName: ${yamlScalar(pb.packageName || "customer.default")}`,
  ];
  if (pb.primarySection) lines.push(`primarySection: ${yamlScalar(pb.primarySection)}`);
  if (pb.harnessId) lines.push(`harnessId: ${yamlScalar(pb.harnessId)}`);
  if (pb.harnessVersion) lines.push(`harnessVersion: ${yamlScalar(pb.harnessVersion)}`);
  const tools = pb.allowedTools?.length ? pb.allowedTools : ["sobjects.read", "sobjects.write", "query"];
  lines.push("allowedTools:");
  for (const t of tools) lines.push(`  - ${yamlScalar(t)}`);
  lines.push("objectScopes:");
  for (const s of pb.objectScopes ?? []) lines.push(`  - ${yamlScalar(s)}`);
  if (pb.allowedSkills?.length) {
    lines.push("allowedSkills:");
    for (const s of pb.allowedSkills) lines.push(`  - ${yamlScalar(s)}`);
  }
  return `${lines.join("\n")}\n`;
}

export async function mirrorPlaybookYaml(
  repoPath: string | undefined,
  pb: PlaybookMirror,
): Promise<string | null> {
  if (!repoPath || !window.one?.writeText) return null;
  if (pb.ownership === "managed") return null;
  try {
    await window.one.writeText(repoPath, playbookYamlPath(pb.apiName), playbookToYaml(pb));
    return null;
  } catch (e) {
    return `YAML mirror failed: ${String(e)}`;
  }
}

export type ToolSpecMirror = {
  apiName: string;
  label: string;
  description?: string;
  icon?: string;
  sortOrder?: number;
  layout: unknown;
  nodes: unknown;
  dataBindings?: unknown;
  active?: boolean;
  ownership?: string;
  packageName?: string;
};

export function toolSpecYamlPath(apiName: string): string {
  return `metadata/tools/${apiName}.yaml`;
}

export function toolSpecToYaml(spec: ToolSpecMirror): string {
  const lines = [
    `apiVersion: one.tool/v1`,
    `kind: ToolSpec`,
    `apiName: ${yamlScalar(spec.apiName)}`,
    `label: ${yamlScalar(spec.label)}`,
    `description: ${yamlScalar(spec.description || "")}`,
    `active: ${spec.active !== false ? "true" : "false"}`,
    `ownership: ${yamlScalar(spec.ownership || "custom")}`,
    `packageName: ${yamlScalar(spec.packageName || "customer.default")}`,
  ];
  if (spec.icon) lines.push(`icon: ${yamlScalar(spec.icon)}`);
  if (spec.sortOrder != null) lines.push(`sortOrder: ${spec.sortOrder}`);
  lines.push("layout:");
  for (const l of JSON.stringify(spec.layout, null, 2).split("\n")) lines.push(`  ${l}`);
  lines.push("nodes:");
  for (const l of JSON.stringify(spec.nodes, null, 2).split("\n")) lines.push(`  ${l}`);
  if (spec.dataBindings !== undefined) {
    lines.push("dataBindings:");
    for (const l of JSON.stringify(spec.dataBindings, null, 2).split("\n")) lines.push(`  ${l}`);
  }
  return `${lines.join("\n")}\n`;
}

export async function mirrorToolSpecYaml(
  repoPath: string | undefined,
  spec: ToolSpecMirror,
): Promise<string | null> {
  if (!repoPath || !window.one?.writeText) return null;
  if (spec.ownership === "managed") return null;
  try {
    await window.one.writeText(repoPath, toolSpecYamlPath(spec.apiName), toolSpecToYaml(spec));
    return null;
  } catch (e) {
    return `YAML mirror failed: ${String(e)}`;
  }
}

/** Best-effort mirror to local customer repo. Returns warning string on failure. */
export async function mirrorObjectYaml(
  repoPath: string | undefined,
  obj: ObjectMirror,
): Promise<string | null> {
  if (!repoPath || !window.one?.writeText) return null;
  if (obj.ownership === "managed") {
    return null; // managed defs live under .one/baseline (read-only)
  }
  try {
    await window.one.writeText(repoPath, objectYamlPath(obj.apiName), objectToYaml(obj));
    return null;
  } catch (e) {
    return `YAML mirror failed: ${String(e)}`;
  }
}

export async function mirrorFieldYaml(
  repoPath: string | undefined,
  field: FieldMirror,
): Promise<string | null> {
  if (!repoPath || !window.one?.writeText) return null;
  if (field.ownership === "managed") {
    return null;
  }
  try {
    await window.one.writeText(
      repoPath,
      fieldYamlPath(field.objectApiName, field.apiName),
      fieldToYaml(field),
    );
    return null;
  } catch (e) {
    return `YAML mirror failed: ${String(e)}`;
  }
}
