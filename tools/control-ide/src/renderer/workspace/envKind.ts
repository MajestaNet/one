/** Operator convention on installRole / label — not a topology API. */
export type EnvKind = "demo" | "mock" | "prod";

export function envKindFromLabels(role?: string, label?: string): EnvKind | null {
  const s = `${role ?? ""} ${label ?? ""}`.toLowerCase();
  if (/\bmock\b/.test(s)) return "mock";
  if (/\b(demo|sandbox|lab|dev)\b/.test(s)) return "demo";
  if (/\b(prod|production)\b/.test(s)) return "prod";
  return null;
}

export function envKindLabel(kind: EnvKind): string {
  if (kind === "mock") return "Mock";
  if (kind === "prod") return "Prod";
  return "Demo";
}
