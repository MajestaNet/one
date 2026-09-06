import { useCallback, useEffect, useState } from "react";
import type { AppBridge } from "../App";
import { changeMyPassword, listDevices, revokeDevice, type DeviceRow } from "../account/self";
import { Button, KeyValueList, StatusBadge, ToolSurface } from "../ui";
import { IconSettings } from "../icons/Icons";
import { envDisplayName } from "../session";

/** Account control center: session, access, password, and enrolled devices. */
export function AccountSettingsPanel({ bridge }: { bridge?: AppBridge }) {
  const session = bridge?.session;
  const connected = Boolean(session?.token);
  const activeEnv = session?.environments?.find((env) => env.installId === session.activeInstallId);
  const scopes = session?.scopes ?? [];
  const capabilities = session?.systemPermissions ?? [];
  const [devices, setDevices] = useState<DeviceRow[]>([]);
  const [devicesErr, setDevicesErr] = useState("");
  const [pwd, setPwd] = useState({ currentPassword: "", newPassword: "" });
  const [pwdMsg, setPwdMsg] = useState("");
  const [pwdErr, setPwdErr] = useState("");
  const [busy, setBusy] = useState(false);

  const loadDevices = useCallback(async () => {
    if (!bridge?.fetch || !connected) {
      setDevices([]);
      setDevicesErr("");
      return;
    }
    try {
      setDevices(await listDevices(bridge.fetch));
      setDevicesErr("");
    } catch (e) {
      setDevices([]);
      const msg = String(e);
      setDevicesErr(/403|FORBIDDEN|CAPABILITY/i.test(msg) ? "This session cannot list devices." : msg);
    }
  }, [bridge, connected]);

  useEffect(() => {
    void loadDevices();
  }, [loadDevices]);

  const onChangePassword = async () => {
    if (!bridge?.fetch) return;
    setPwdErr("");
    setPwdMsg("");
    setBusy(true);
    try {
      await changeMyPassword(bridge.fetch, pwd.currentPassword, pwd.newPassword);
      setPwd({ currentPassword: "", newPassword: "" });
      setPwdMsg("Password updated on this install.");
    } catch (e) {
      const msg = String(e);
      setPwdErr(/403|FORBIDDEN/i.test(msg) ? "Password change was forbidden for this principal." : msg);
    } finally {
      setBusy(false);
    }
  };

  const onRevoke = async (deviceId: string) => {
    if (!bridge?.fetch) return;
    setBusy(true);
    try {
      await revokeDevice(bridge.fetch, deviceId);
      await loadDevices();
    } catch (e) {
      setDevicesErr(String(e));
    } finally {
      setBusy(false);
    }
  };

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
          <p className="muted account-card-copy">Product capabilities</p>
          <div className="account-chip-list" data-testid="account-caps">
            {capabilities.length ? capabilities.map((cap) => <span key={cap}>{cap}</span>) : <span>Inherited / not reported</span>}
          </div>
        </section>

        <section className="account-card">
          <div className="account-card-heading"><div><p className="eyebrow">Security</p><h3>Password</h3></div></div>
          <p className="muted account-card-copy">POST /client/v1/me/password — AuthZ stays on the install.</p>
          {connected ? (
            <div className="om-form" data-testid="account-password">
              <label>
                Current password
                <input
                  type="password"
                  value={pwd.currentPassword}
                  onChange={(e) => setPwd((p) => ({ ...p, currentPassword: e.target.value }))}
                  data-testid="account-password-current"
                />
              </label>
              <label>
                New password
                <input
                  type="password"
                  value={pwd.newPassword}
                  onChange={(e) => setPwd((p) => ({ ...p, newPassword: e.target.value }))}
                  data-testid="account-password-new"
                />
              </label>
              <Button
                variant="primary"
                busy={busy}
                disabled={!pwd.currentPassword || !pwd.newPassword}
                onClick={() => void onChangePassword()}
                data-testid="account-password-save"
              >
                Update password
              </Button>
              {pwdMsg ? <p className="muted">{pwdMsg}</p> : null}
              {pwdErr ? <p className="err">{pwdErr}</p> : null}
            </div>
          ) : (
            <p className="muted">Connect to change the password for this principal.</p>
          )}
        </section>

        <section className="account-card">
          <div className="account-card-heading"><div><p className="eyebrow">Devices</p><h3>Enrolled devices</h3></div></div>
          <p className="muted account-card-copy">GET /client/v1/devices. Enroll from Settings → Environments (Connect).</p>
          {devicesErr ? <p className="err">{devicesErr}</p> : null}
          {devices.length === 0 && !devicesErr ? (
            <p className="muted">{connected ? "No devices enrolled." : "Connect to list devices."}</p>
          ) : (
            <ul className="govern-list" data-testid="account-devices">
              {devices.map((d) => (
                <li key={d.deviceId} className="row" style={{ justifyContent: "space-between" }}>
                  <span>
                    <strong>{d.label || d.deviceId}</strong>
                    <span className="muted"> {d.revokedAt ? "revoked" : d.deviceId}</span>
                  </span>
                  {!d.revokedAt ? (
                    <Button
                      variant="ghost"
                      busy={busy}
                      onClick={() => void onRevoke(d.deviceId)}
                      data-testid={`account-device-revoke-${d.deviceId}`}
                    >
                      Revoke
                    </Button>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
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
