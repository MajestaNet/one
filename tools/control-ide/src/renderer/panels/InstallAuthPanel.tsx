import { useCallback, useEffect, useState } from "react";
import type { AppBridge } from "../App";
import { Button, EmptyState, PanelHeader, StatusBadge } from "../ui";
import { IconGovern } from "../icons/Icons";

export type InstallAuthSettings = {
  claimed?: boolean;
  ssoConfigured?: boolean;
  oidcIssuer?: string;
  oidcAudience?: string;
  oidcJwksUri?: string;
  oidcDisplayName?: string;
  oidcClientId?: string;
  oidcClientSecretSet?: boolean;
  jitProvisionUsers?: boolean;
  jitDefaultRole?: string;
  allowedEmailDomains?: string[];
  socialProviders?: string[];
  passwordLoginEnabled?: boolean;
};

export function InstallAuthPanel({ bridge }: { bridge: AppBridge }) {
  const connected = Boolean(bridge.session?.baseUrl && bridge.session?.token);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const [form, setForm] = useState({
    oidcIssuer: "",
    oidcAudience: "",
    oidcJwksUri: "",
    oidcDisplayName: "",
    oidcClientId: "",
    oidcClientSecret: "",
    clearOidcClientSecret: false,
    jitProvisionUsers: false,
    jitDefaultRole: "StandardUser",
    allowedEmailDomains: "",
    socialGoogle: false,
    socialApple: false,
    passwordLoginEnabled: true,
  });
  const [meta, setMeta] = useState<InstallAuthSettings | null>(null);

  const load = useCallback(async () => {
    if (!connected) return;
    setErr("");
    setBusy(true);
    try {
      const data = (await bridge.fetch("/metadata/v1/install/auth")) as InstallAuthSettings;
      setMeta(data);
      const social = data.socialProviders ?? [];
      setForm((f) => ({
        ...f,
        oidcIssuer: data.oidcIssuer ?? "",
        oidcAudience: data.oidcAudience ?? "",
        oidcJwksUri: data.oidcJwksUri ?? "",
        oidcDisplayName: data.oidcDisplayName ?? "",
        oidcClientId: data.oidcClientId ?? "",
        oidcClientSecret: "",
        clearOidcClientSecret: false,
        jitProvisionUsers: Boolean(data.jitProvisionUsers),
        jitDefaultRole: data.jitDefaultRole || "StandardUser",
        allowedEmailDomains: (data.allowedEmailDomains ?? []).join(", "),
        socialGoogle: social.includes("google"),
        socialApple: social.includes("apple"),
        passwordLoginEnabled: data.passwordLoginEnabled !== false,
      }));
    } catch (e) {
      setErr(String(e));
      setMeta(null);
    } finally {
      setBusy(false);
    }
  }, [bridge, connected]);

  useEffect(() => {
    void load();
  }, [load]);

  const save = async () => {
    setErr("");
    setSaved(false);
    setBusy(true);
    try {
      const socialProviders: string[] = [];
      if (form.socialGoogle) socialProviders.push("google");
      if (form.socialApple) socialProviders.push("apple");
      const domains = form.allowedEmailDomains
        .split(",")
        .map((d) => d.trim())
        .filter(Boolean);
      const body: Record<string, unknown> = {
        oidcIssuer: form.oidcIssuer.trim(),
        oidcAudience: form.oidcAudience.trim(),
        oidcJwksUri: form.oidcJwksUri.trim(),
        oidcDisplayName: form.oidcDisplayName.trim(),
        oidcClientId: form.oidcClientId.trim(),
        jitProvisionUsers: form.jitProvisionUsers,
        jitDefaultRole: form.jitDefaultRole.trim() || "StandardUser",
        allowedEmailDomains: domains,
        socialProviders,
        passwordLoginEnabled: form.passwordLoginEnabled,
      };
      if (form.clearOidcClientSecret) {
        body.clearOidcClientSecret = true;
      } else if (form.oidcClientSecret.trim()) {
        body.oidcClientSecret = form.oidcClientSecret.trim();
      }
      const data = (await bridge.fetch("/metadata/v1/install/auth", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      })) as InstallAuthSettings;
      setMeta(data);
      setSaved(true);
      setForm((f) => ({ ...f, oidcClientSecret: "", clearOidcClientSecret: false }));
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  if (!connected) {
    return (
      <EmptyState
        icon={<IconGovern size={28} />}
        title="Install auth"
        description="Connect to an install to configure SSO, JIT, and password login."
      />
    );
  }

  return (
    <section className="govern-section tool-surface" data-testid="install-auth-panel" data-tool-surface="true">
      <PanelHeader
        title="Install auth / SSO"
        subtitle="Configure the customer IdP, JIT provisioning, optional Google/Apple, and password login."
        actions={
          <StatusBadge tone={meta?.ssoConfigured ? "success" : "neutral"}>
            {meta?.ssoConfigured ? "SSO configured" : "SSO not set"}
          </StatusBadge>
        }
      />
      {err ? <p className="err">{err}</p> : null}
      {saved ? <p className="muted">Saved.</p> : null}

      <div className="env-card">
        <h3>SSO (OIDC)</h3>
        <div className="row">
          <label>
            Display name
            <input
              value={form.oidcDisplayName}
              onChange={(e) => setForm({ ...form, oidcDisplayName: e.target.value })}
              placeholder="Company SSO"
              data-testid="install-auth-display-name"
            />
          </label>
        </div>
        <div className="row">
          <label>
            Issuer
            <input
              value={form.oidcIssuer}
              onChange={(e) => setForm({ ...form, oidcIssuer: e.target.value })}
              placeholder="https://idp.example.com"
              data-testid="install-auth-issuer"
            />
          </label>
          <label>
            Audience / client id
            <input
              value={form.oidcAudience}
              onChange={(e) => setForm({ ...form, oidcAudience: e.target.value })}
              data-testid="install-auth-audience"
            />
          </label>
        </div>
        <div className="row">
          <label>
            OAuth client id
            <input
              value={form.oidcClientId}
              onChange={(e) => setForm({ ...form, oidcClientId: e.target.value })}
              data-testid="install-auth-client-id"
            />
          </label>
          <label>
            JWKS URI (optional)
            <input
              value={form.oidcJwksUri}
              onChange={(e) => setForm({ ...form, oidcJwksUri: e.target.value })}
            />
          </label>
        </div>
        <div className="row">
          <label>
            Client secret {meta?.oidcClientSecretSet ? "(set — leave blank to keep)" : ""}
            <input
              type="password"
              value={form.oidcClientSecret}
              onChange={(e) => setForm({ ...form, oidcClientSecret: e.target.value })}
              data-testid="install-auth-client-secret"
            />
          </label>
        </div>
        <label className="row">
          <input
            type="checkbox"
            checked={form.clearOidcClientSecret}
            onChange={(e) => setForm({ ...form, clearOidcClientSecret: e.target.checked })}
          />
          Clear stored client secret
        </label>
      </div>

      <div className="env-card">
        <h3>JIT provisioning</h3>
        <label className="row">
          <input
            type="checkbox"
            checked={form.jitProvisionUsers}
            onChange={(e) => setForm({ ...form, jitProvisionUsers: e.target.checked })}
            data-testid="install-auth-jit"
          />
          Auto-provision users on first SSO / social login
        </label>
        <div className="row">
          <label>
            Default role
            <input
              value={form.jitDefaultRole}
              onChange={(e) => setForm({ ...form, jitDefaultRole: e.target.value })}
            />
          </label>
          <label>
            Allowed email domains (comma-separated)
            <input
              value={form.allowedEmailDomains}
              onChange={(e) => setForm({ ...form, allowedEmailDomains: e.target.value })}
              placeholder="example.com"
              data-testid="install-auth-domains"
            />
          </label>
        </div>
      </div>

      <div className="env-card">
        <h3>Other login methods</h3>
        <label className="row">
          <input
            type="checkbox"
            checked={form.passwordLoginEnabled}
            onChange={(e) => setForm({ ...form, passwordLoginEnabled: e.target.checked })}
            data-testid="install-auth-password"
          />
          Password login enabled
        </label>
        <label className="row">
          <input
            type="checkbox"
            checked={form.socialGoogle}
            onChange={(e) => setForm({ ...form, socialGoogle: e.target.checked })}
            data-testid="install-auth-google"
          />
          Enable Google (requires deploy secrets)
        </label>
        <label className="row">
          <input
            type="checkbox"
            checked={form.socialApple}
            onChange={(e) => setForm({ ...form, socialApple: e.target.checked })}
            data-testid="install-auth-apple"
          />
          Enable Apple (requires deploy secrets)
        </label>
      </div>

      <Button variant="primary" busy={busy} onClick={() => void save()} data-testid="install-auth-save">
        Save install auth
      </Button>
    </section>
  );
}
