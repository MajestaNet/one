import type { ButtonHTMLAttributes, ReactNode } from "react";
import { IconSpinner } from "../icons/Icons";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";

export function Button({
  variant = "secondary",
  busy = false,
  children,
  className,
  disabled,
  type = "button",
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  busy?: boolean;
  children?: ReactNode;
}) {
  const classes = ["btn", `btn-${variant}`, className].filter(Boolean).join(" ");
  return (
    <button type={type} className={classes} disabled={disabled || busy} aria-busy={busy || undefined} {...rest}>
      {busy ? <IconSpinner size={14} /> : null}
      <span>{children}</span>
    </button>
  );
}
