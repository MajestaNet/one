import { IconSpinner } from "../icons/Icons";

export function Spinner({ label = "Loading", size = 18 }: { label?: string; size?: number }) {
  return (
    <span className="ui-spinner" role="status" aria-label={label}>
      <IconSpinner size={size} />
    </span>
  );
}
