import type { ReactNode } from "react";

export function StatusBadge({
  tone = "neutral",
  children,
}: {
  tone?: "neutral" | "accent" | "success" | "warn" | "danger" | "info";
  children: ReactNode;
}) {
  return <span className={`status-badge tone-${tone}`}>{children}</span>;
}

export function CheckDot({ state }: { state: "running" | "passed" | "pending" | "failed" }) {
  return <span className={`check-dot ${state}`} aria-hidden />;
}
