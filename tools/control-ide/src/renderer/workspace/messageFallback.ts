/**
 * Defensive plain-text extraction when assistant-ui has a message without a Majesta One store
 * payload. Never route through markdown / MessagePrimitive.Parts (CIDE-16).
 */
export function plainTextFromMessageContent(content: unknown): string {
  const parts = Array.isArray(content) ? content : [];
  return parts
    .filter((p): p is { type: "text"; text: string } =>
      Boolean(p && typeof p === "object" && (p as { type?: unknown }).type === "text" && typeof (p as { text?: unknown }).text === "string"),
    )
    .map((p) => p.text)
    .join("");
}
