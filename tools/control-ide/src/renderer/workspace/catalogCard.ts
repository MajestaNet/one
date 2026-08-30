/**
 * Catalog cards (tool rail + agent dock) show one title. A second line is only
 * a kicker when it is actually different from the title (apiName vs label).
 */

export function catalogKey(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, "");
}

/** True when two labels are the same name with different punctuation/casing. */
export function isDuplicateCatalogTitle(primary: string, secondary?: string): boolean {
  const a = catalogKey(primary);
  const b = catalogKey(secondary ?? "");
  if (!a || !b) return true;
  return a === b || a.includes(b) || b.includes(a);
}

export function catalogCardCopy(
  title: string,
  secondary?: string,
): { title: string; kicker?: string } {
  const trimmedTitle = title.trim() || (secondary ?? "").trim();
  const trimmedSecondary = (secondary ?? "").trim();
  if (!trimmedSecondary || isDuplicateCatalogTitle(trimmedTitle, trimmedSecondary)) {
    return { title: trimmedTitle };
  }
  return { title: trimmedTitle, kicker: trimmedSecondary };
}
