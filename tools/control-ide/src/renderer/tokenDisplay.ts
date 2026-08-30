/** Mask a JWT (or similar secret) for display until the operator clicks Reveal (CIDE-18). */
export function maskToken(value: string): string {
  const t = value.trim();
  if (!t) return "";
  if (t.length <= 12) return "•".repeat(t.length);
  return `${t.slice(0, 6)}…${t.slice(-4)} (${t.length} chars)`;
}
