import type { InputHTMLAttributes } from "react";
import { IconSearch } from "../icons/Icons";

/**
 * Standard in-tool search field. Use on the right of `ToolToolbar`.
 * Wrapper is a `div` so global `label` column/min-width rules cannot stack the
 * icon above the input. Do not invent a second search chrome in a workspace tool.
 */
export function SearchField({
  value,
  onChange,
  placeholder = "Search…",
  label = "Search",
  testId,
  className,
  ...rest
}: Omit<InputHTMLAttributes<HTMLInputElement>, "onChange" | "value"> & {
  value: string;
  onChange: (value: string) => void;
  label?: string;
  testId?: string;
}) {
  return (
    <div className={["tool-search", className].filter(Boolean).join(" ")}>
      <IconSearch size={14} aria-hidden />
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        aria-label={label}
        data-testid={testId}
        {...rest}
      />
    </div>
  );
}
