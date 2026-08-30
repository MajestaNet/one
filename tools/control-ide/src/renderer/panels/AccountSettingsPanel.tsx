import type { AppBridge } from "../App";
import { KeyValueList, StatusBadge, ToolSurface } from "../ui";
import { IconSettings } from "../icons/Icons";
import { envDisplayName } from "../session";

/** Account control center: session, access, workspace, and local-product preferences. */
export function AccountSettingsPanel({ bridge }: { bridge?: AppBridge }) {
  const session = bridge?.session;
  const activeEnv = session?.environments?.find((env) => env.installId === session.activeInstallId);
  const scopes = session?.scopes ?? [];
  const capabilities = session?.systemPermissions ?? [];
  return (
    <ToolSurface
      className="account-settings-panel"
      testId="account-settings-panel"
      title="Account settings"
      subtitle="Your Majesta One Control session, workspace context, and effective access at a glance."
    >
      <section className="account-hero">
        <div className="account-hero-icon"><IconSettings size={24} /></div>
        <div>
          <p className="eyebrow">Authenticated workspace</p>
          <h3>{activeEnv ? envDisplayName(activeEnv) : session?.activeInstallId || "Current install"}</h3>
          <p className="muted">Credentials stay encrypted on this device; effective authorization is resolved by the active install.</p>
        </div>
        <StatusBadge tone={session?.token ? "success" : "neutral"}>{session?.token ? "Session active" : "Not connected"}</StatusBadge>
      </section>

      <div className="account-card-grid">
        <section className="account-card">
          <div className="account-card-heading"><div><p className="eyebrow">Workspace</p><h3>Active context</h3></div><StatusBadge tone="accent">{activeEnv?.installRole || "install"}</StatusBadge></div>
          <KeyValueList items={[
            { label: "Install", value: session?.activeInstallId || "—" },
            { label: "API", value: session?.baseUrl || "—" },
            { label: "Customer repo", value: session?.repoPath || "Not linked" },
            { label: "Environments", value: String(session?.environments?.length ?? 0) },
          ]} />
        </section>

        <section className="account-card">
          <div className="account-card-heading"><div><p className="eyebrow">Authorization</p><h3>Effective access</h3></div><StatusBadge tone={session?.isAdmin ? "accent" : "neutral"}>{session?.isAdmin ? "Admin" : "Role scoped"}</StatusBadge></div>
          <p className="muted account-card-copy">Family scopes</p>
          <div className="account-chip-list">{scopes.length ? scopes.map((scope) => <span key={scope}>{scope}</span>) : <span>Not reported</span>}</div>
          <p className="muted account-card-copy">IDE capabilities</p>
          <div className="account-chip-list is-compact">{capabilities.length ? capabilities.slice(0, 12).map((cap) => <span key={cap}>{cap}</span>) : <span>Inherited / not reported</span>}</div>
          {capabilities.length > 12 ? <p className="muted">+{capabilities.length - 12} more capabilities</p> : null}
        </section>

        <section className="account-card account-card-wide">
          <div className="account-card-heading"><div><p className="eyebrow">Productivity</p><h3>Desktop shortcuts</h3></div></div>
          <div className="shortcut-grid">
            <div><kbd>⌘/Ctrl</kbd><span>+</span><kbd>Shift</kbd><span>+</span><kbd>L</kbd><p>Toggle light / dark theme</p></div>
            <div><kbd>Esc</kbd><p>Dismiss the mode launcher</p></div>
            <div><kbd>Enter</kbd><p>Send a chat message</p></div>
          </div>
        </section>
      </div>
    </ToolSurface>
  );
}
