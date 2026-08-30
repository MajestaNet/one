import { useEffect, useState } from "react";
import type { AppBridge } from "../App";
import { ConnectSection } from "../govern/ConnectSection";
import { mergePeerHints, envDisplayName, normalizeSession } from "../session";
import { Button, EmptyState, KeyValueList, PanelHeader, StatusBadge, ToolSurface } from "../ui";
import { IconEnv } from "../icons/Icons";

type EnvPayload = Record<string, unknown>;

function pickRepoUrl(env: EnvPayload): string {
  for (const k of ["customerRepoUrl", "customerRepo", "CUSTOMER_REPO_URL", "repoUrl"]) {
    const v = env[k];
    if (typeof v === "string" && v.trim()) return v.trim();
  }
  return "";
}

function envItems(env: EnvPayload): { label: string; value: string }[] {
  const items: { label: string; value: string }[] = [];
  const pick = (label: string, ...keys: string[]) => {
    for (const k of keys) {
      const v = env[k] ?? env[k.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`)];
      if (v != null && v !== "") {
        items.push({ label, value: typeof v === "object" ? JSON.stringify(v) : String(v) });
        return;
      }
    }
  };
  pick("Customer", "customerId", "customer_id", "CUSTOMER_ID");
  pick("Install", "installId", "install_id", "INSTALL_ID");
  pick("Role", "installRole", "install_role", "INSTALL_ROLE", "role");
  pick("Peer mode", "peerMode", "peer_mode");
  pick("Capabilities", "capabilities");
  pick("Repo", "customerRepoUrl", "customerRepo", "CUSTOMER_REPO_URL", "repoUrl");
  if (items.length === 0) {
    for (const [k, v] of Object.entries(env).slice(0, 8)) {
      items.push({ label: k, value: typeof v === "object" ? JSON.stringify(v) : String(v) });
    }
  }
  return items;
}

type PeerRow = {
  installId?: string;
  installRole?: string;
  label?: string;
  baseUrl?: string;
  active?: boolean;
};

export function EnvPanel({
  bridge,
  onSwitchEnv,
  onConnectPeer,
  prefillBaseUrl,
  onPrefillConsumed,
  focusConnect,
  onFocusConnectConsumed,
}: {
  bridge: AppBridge;
  onSwitchEnv?: (installId: string) => void;
  /** Scroll/focus the connection section (peer needs credentials). */
  onConnectPeer?: (baseUrl: string) => void;
  prefillBaseUrl?: string;
  onPrefillConsumed?: () => void;
  focusConnect?: boolean;
  onFocusConnectConsumed?: () => void;
}) {
  const [env, setEnv] = useState<EnvPayload | null>(null);
  const [peers, setPeers] = useState<PeerRow[]>([]);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState(false);
  const [peerForm, setPeerForm] = useState({ installId: "", label: "", installRole: "", baseUrl: "" });
  const connected = Boolean(bridge.session?.baseUrl && bridge.session?.token);

  const load = async () => {
    if (!connected) {
      setEnv(null);
      setPeers([]);
      return;
    }
    setErr("");
    setBusy(true);
    try {
      const row = (await bridge.fetch("/deploy/v1/environment")) as EnvPayload;
      setEnv(row);
      const url = pickRepoUrl(row);
      if (url && bridge.session) {
        await bridge.setSession({ ...bridge.session, customerRepoUrl: url });
      }

      try {
        const peerRes = (await bridge.fetch("/deploy/v1/peers")) as { peers?: PeerRow[] };
        const list = peerRes.peers ?? [];
        setPeers(list);
        const normalized = bridge.session ? normalizeSession(bridge.session) : null;
        if (normalized && list.length) {
          const merged = mergePeerHints(
            normalized,
            list.map((p) => ({
              installId: p.installId || "",
              installRole: p.installRole,
              label: p.label,
              baseUrl: p.baseUrl,
              active: p.active,
            })),
          );
          await bridge.setSession(url ? { ...merged, customerRepoUrl: url } : merged);
        } else if (url && bridge.session) {
          const n = normalizeSession({ ...bridge.session, customerRepoUrl: url });
          if (n) await bridge.setSession(n);
        }
      } catch {
        setPeers([]);
      }
    } catch (e) {
      setErr(String(e));
      setEnv(null);
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => {
    if (connected) void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- refresh when session connect flips
  }, [connected, bridge.session?.activeInstallId]);

  const copyCloneUrl = async () => {
    const url = env ? pickRepoUrl(env) : "";
    if (!url) return;
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setErr("Clipboard unavailable");
    }
  };

  const upsertPeer = async () => {
    setErr("");
    setBusy(true);
    try {
      await bridge.fetch("/deploy/v1/peers", {
        method: "POST",
        body: JSON.stringify({
          installId: peerForm.installId.trim(),
          label: peerForm.label.trim() || undefined,
          installRole: peerForm.installRole.trim() || undefined,
          baseUrl: peerForm.baseUrl.trim() || undefined,
          active: true,
        }),
      });
      setPeerForm({ installId: "", label: "", installRole: "", baseUrl: "" });
      await load();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const repoUrl = env ? pickRepoUrl(env) : "";
  const activeId = bridge.session?.activeInstallId;

  return (
    <ToolSurface testId="env-panel">
      <PanelHeader
        title="Environments"
        subtitle="Connect to an install, inspect identity, and manage peer topology for this customer."
      />

      <ConnectSection
        bridge={bridge}
        prefillBaseUrl={prefillBaseUrl}
        onPrefillConsumed={onPrefillConsumed}
        focusConnect={focusConnect}
        onFocusConnectConsumed={onFocusConnectConsumed}
      />

      <section className="govern-section" data-testid="env-install-section">
        <PanelHeader
          title="This install"
          subtitle="Reads GET /deploy/v1/environment. Stages are free-form install roles."
          actions={
            <Button variant="primary" busy={busy} disabled={!connected} onClick={() => void load()}>
              Refresh environment
            </Button>
          }
        />
        {!connected ? (
          <EmptyState
            icon={<IconEnv size={28} />}
            title="Connect first"
            description="Use the Connection section above to authenticate, then refresh install identity."
          />
        ) : null}
        {err && <p className="err">{err}</p>}
        {connected && !env && !err ? (
          <EmptyState
            icon={<IconEnv size={28} />}
            title="No environment loaded"
            description="Refresh to load install identity and peer topology for this connection."
            action={
              <Button variant="secondary" busy={busy} onClick={() => void load()}>
                Refresh environment
              </Button>
            }
          />
        ) : null}
        {env && (
          <div className="env-card" data-testid="env-card">
            <KeyValueList items={envItems(env)} />
            {repoUrl ? (
              <div className="row" style={{ marginTop: "0.75rem" }}>
                <Button variant="secondary" onClick={() => void copyCloneUrl()} data-testid="copy-clone-url">
                  {copied ? "Copied clone URL" : "Copy clone URL"}
                </Button>
                <p className="muted">Use Repo → Clone, or paste into git clone.</p>
              </div>
            ) : null}
            <details className="details-block">
              <summary>Raw response</summary>
              <pre className="log">{JSON.stringify(env, null, 2)}</pre>
            </details>
          </div>
        )}
      </section>

      <section className="govern-section" data-testid="env-peers-section">
        <PanelHeader title="Peers" subtitle="Registered peers and session connections." />
        {(peers.length > 0 || (bridge.session?.environments?.length ?? 0) > 0) && (
          <div className="env-card" data-testid="peer-topology">
            <ul className="stage-strip" data-testid="stage-strip">
              {(bridge.session?.environments ?? []).map((e) => (
                <li
                  key={e.installId}
                  className={`stage-chip ${e.installId === activeId ? "active" : ""} ${e.token ? "" : "disconnected"}`}
                >
                  <div className="stage-chip-role">{envDisplayName(e)}</div>
                  <div className="muted stage-chip-id">{e.installId}</div>
                  <StatusBadge tone={e.installId === activeId ? "accent" : e.token ? "success" : "neutral"}>
                    {e.installId === activeId ? "Active" : e.token ? "Connected" : "Needs connect"}
                  </StatusBadge>
                  <div className="row" style={{ marginTop: "0.35rem" }}>
                    {e.token && e.installId !== activeId && onSwitchEnv ? (
                      <Button variant="ghost" onClick={() => onSwitchEnv(e.installId)}>
                        Switch here
                      </Button>
                    ) : null}
                    {!e.token && e.baseUrl && onConnectPeer ? (
                      <Button variant="secondary" onClick={() => onConnectPeer(e.baseUrl)}>
                        Connect…
                      </Button>
                    ) : null}
                  </div>
                </li>
              ))}
            </ul>
            {peers.length > 0 ? (
              <details className="details-block">
                <summary>Raw peers ({peers.length})</summary>
                <pre className="log">{JSON.stringify(peers, null, 2)}</pre>
              </details>
            ) : null}
          </div>
        )}

        {connected ? (
          <div className="env-card" data-testid="peer-upsert">
            <p className="muted">Register or update a peer (requires deploy.promote).</p>
            <div className="row">
              <label>
                Install ID
                <input
                  value={peerForm.installId}
                  onChange={(e) => setPeerForm((p) => ({ ...p, installId: e.target.value }))}
                  data-testid="peer-install-id"
                />
              </label>
              <label>
                Label
                <input
                  value={peerForm.label}
                  onChange={(e) => setPeerForm((p) => ({ ...p, label: e.target.value }))}
                />
              </label>
            </div>
            <div className="row">
              <label>
                Install role
                <input
                  value={peerForm.installRole}
                  onChange={(e) => setPeerForm((p) => ({ ...p, installRole: e.target.value }))}
                />
              </label>
              <label>
                Base URL
                <input
                  value={peerForm.baseUrl}
                  onChange={(e) => setPeerForm((p) => ({ ...p, baseUrl: e.target.value }))}
                />
              </label>
            </div>
            <Button
              variant="secondary"
              busy={busy}
              disabled={!peerForm.installId.trim()}
              onClick={() => void upsertPeer()}
              data-testid="peer-upsert-btn"
            >
              Save peer
            </Button>
          </div>
        ) : null}
      </section>
    </ToolSurface>
  );
}
