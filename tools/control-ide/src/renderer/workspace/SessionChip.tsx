import type { Session } from "../session";
import { sessionIdentity } from "../session";

export function SessionChip({
  session,
  onOpenAccount,
}: {
  session: Session | null;
  onOpenAccount: () => void;
}) {
  const identity = sessionIdentity(session);
  if (!identity) {
    return (
      <span className="session-chip" data-testid="session-chip">
        Not connected
      </span>
    );
  }
  const title = identity.email && identity.email !== identity.displayName
    ? `${identity.displayName} · ${identity.email}`
    : identity.displayName;
  return (
    <button
      type="button"
      className="session-chip connected session-chip-account"
      data-testid="session-chip"
      onClick={onOpenAccount}
      title={`${title} · Environments`}
      aria-label={`Account: ${identity.displayName}`}
    >
      <span className="session-avatar" aria-hidden>
        {identity.initials}
      </span>
      <span className="session-chip-name">{identity.displayName}</span>
    </button>
  );
}
