import { useEffect, useState } from "react";
import type { AppBridge } from "../App";
import {
  apiFetch,
  baseUrlOrigin,
  checkInstallBaseUrl,
  connectionFromActor,
  formatError,
  isLoopbackHost,
  normalizeSession,
  probeInstallVersion,
  upsertEnvironment,
  type Session,
} from "../api";
import {
  IDE_COMPAT_MANIFEST,
  compatCta,
  evaluateProductTestedAgainst,
  evaluateRevision,
  mergeCompatStatus,
  pinIntersection,
  selectPin,
  type CompatStatus,
} from "../compat";
import { apiRevisionFromPayload } from "../versionProbe";
import {
  CONTROL_IDE_INTEGRATION,
  DEFAULT_REDIRECT_URI,
  buildOneLoginUrl,
  createPkcePair,
  exchangeOneAuthorizationCode,
  exchangeOneIdToken,
  loadPendingPkce,
  parseOAuthCallbackUrl,
  randomOAuthState,
  statesMatch,
  storePendingPkce,
  takePendingPkce,
} from "../oauthPkce";
import { openExternalUrl } from "../external";
import { extractActorScopes } from "../scopes";
import { maskToken } from "../tokenDisplay";
import { Button, EmptyState, KeyValueList, PanelHeader, StatusBadge } from "../ui";
import { IconConnect } from "../icons/Icons";
import { envDisplayName } from "../session";
import { revokeRefreshToken } from "../refreshSession";

type MePayload = Record<string, unknown>;

function identityItems(me: MePayload | null): { label: string; value: string }[] {
  if (!me) return [];
  const items: { label: string; value: string }[] = [];
  const pick = (label: string, ...keys: string[]) => {
    for (const k of keys) {
      const v = me[k];
      if (v != null && v !== "") {
        items.push({ label, value: typeof v === "object" ? JSON.stringify(v) : String(v) });
        return;
      }
    }
  };
  pick("Principal", "principal", "sub", "name", "displayName", "id");
  pick("Type", "principalType", "principal_type", "type", "kind");
  pick("Scopes", "scopes", "scope", "api_scopes");
  pick("Roles", "roles", "role");
  pick("System permissions", "systemPermissions");
  pick("Azp", "azp");
  if (items.length === 0) {
    for (const [k, v] of Object.entries(me).slice(0, 6)) {
      items.push({ label: k, value: typeof v === "object" ? JSON.stringify(v) : String(v) });
    }
  }
  return items;
}

async function fetchEnvInfo(
  baseUrl: string,
  jwt: string,
  allowInsecureHttp?: boolean,
): Promise<Record<string, unknown> | null> {
  try {
    return (await apiFetch(baseUrl, jwt, "/deploy/v1/environment", {}, { allowInsecureHttp })) as Record<
      string,
      unknown
    >;
  } catch {
    return null;
  }
}

/**
 * Connection / credential UX hosted inside Settings → Environments.
 * Progressive disclosure: PKCE primary → client credentials → advanced JWT / device.
 */
export function ConnectSection({
  bridge,
  prefillBaseUrl,
  onPrefillConsumed,
  focusConnect,
  onFocusConnectConsumed,
}: {
  bridge: AppBridge;
  prefillBaseUrl?: string;
  onPrefillConsumed?: () => void;
  /** When true, open advanced JWT section and scroll into view. */
  focusConnect?: boolean;
  onFocusConnectConsumed?: () => void;
}) {
  const [baseUrl, setBaseUrl] = useState(
    prefillBaseUrl || bridge.session?.baseUrl || "http://localhost:8080",
  );
  const [token, setToken] = useState(bridge.session?.token ?? "");
  const [clientId, setClientId] = useState("");
  const [clientSecret, setClientSecret] = useState("");
  const [oauthClientId, setOauthClientId] = useState(CONTROL_IDE_INTEGRATION);
  const [authCode, setAuthCode] = useState("");
  const [idTokenPaste, setIdTokenPaste] = useState("");
  const [pkceVerifier, setPkceVerifier] = useState(() => loadPendingPkce()?.verifier ?? "");
  const [me, setMe] = useState<MePayload | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [advancedOpen, setAdvancedOpen] = useState(Boolean(focusConnect));
  const [status, setStatus] = useState("");
  const [showToken, setShowToken] = useState(false);
  const [allowInsecureHttp, setAllowInsecureHttp] = useState(false);
  const [claimToken, setClaimToken] = useState("");
  const [claimEmail, setClaimEmail] = useState("");
  const [claimPassword, setClaimPassword] = useState("");
  const [claimDisplayName, setClaimDisplayName] = useState("");
  const [installClaimed, setInstallClaimed] = useState<boolean | null>(null);
  /**
   * Origin that arrived from Deploy peer metadata rather than from the operator. A hostile or
   * mistaken peer record must not be able to collect a token on one click (CIDE-08).
   */
  const [unverifiedPeerOrigin, setUnverifiedPeerOrigin] = useState("");
  const [encryptionAvailable, setEncryptionAvailable] = useState<boolean | null>(null);
  const [compatOverride, setCompatOverride] = useState(false);
  const [compatBanner, setCompatBanner] = useState("");
  const [apiRevisionPin, setApiRevisionPin] = useState<number | undefined>(undefined);
  const [pinRange, setPinRange] = useState<{ minPin: number; maxPin: number } | null>(null);

  const connected = Boolean(bridge.session?.baseUrl && bridge.session?.token);
  const knownEnvs = bridge.session?.environments ?? [];
  const activeConn =
    bridge.session?.environments?.find((e) => e.installId === bridge.session?.activeInstallId) ??
    bridge.session?.environments?.[0];

  useEffect(() => {
    if (prefillBaseUrl) {
      setBaseUrl(prefillBaseUrl);
      setUnverifiedPeerOrigin(baseUrlOrigin(prefillBaseUrl));
      setAllowInsecureHttp(false);
      onPrefillConsumed?.();
    }
  }, [prefillBaseUrl, onPrefillConsumed]);

  useEffect(() => {
    void (async () => {
      if (!window.one?.isSessionEncryptionAvailable) {
        setEncryptionAvailable(null);
        return;
      }
      try {
        const v = await window.one.isSessionEncryptionAvailable();
        setEncryptionAvailable(Boolean(v));
      } catch {
        setEncryptionAvailable(null);
      }
    })();
  }, []);

  useEffect(() => {
    if (activeConn?.apiRevisionPin != null) {
      setApiRevisionPin(activeConn.apiRevisionPin);
      if (activeConn.apiRevisionMin != null && activeConn.apiRevisionCurrent != null) {
        setPinRange(
          pinIntersection(IDE_COMPAT_MANIFEST, {
            min: activeConn.apiRevisionMin,
            current: activeConn.apiRevisionCurrent,
          }),
        );
      }
    }
  }, [activeConn?.apiRevisionPin, activeConn?.apiRevisionMin, activeConn?.apiRevisionCurrent]);

  const updateSessionPin = async (pin: number) => {
    if (!bridge.session || !activeConn) return;
    const window = {
      min: activeConn.apiRevisionMin ?? pin,
      current: activeConn.apiRevisionCurrent ?? pin,
    };
    const revisionEval = evaluateRevision(pin, window, false);
    if (revisionEval.status === "block") {
      setErr(revisionEval.message ?? "API revision pin is outside the install window");
      return;
    }
    const conn = { ...activeConn, apiRevisionPin: pin, compatStatus: revisionEval.status as CompatStatus };
    const next = upsertEnvironment(bridge.session, conn, {
      makeActive: true,
      repoPath: bridge.session.repoPath,
      customerRepoUrl: bridge.session.customerRepoUrl,
      deviceId: bridge.session.deviceId,
    });
    await bridge.setSession({ ...next, allowInsecureHttp: bridge.session.allowInsecureHttp || undefined });
    setApiRevisionPin(pin);
  };

  useEffect(() => {
    if (focusConnect) {
      setAdvancedOpen(true);
      const el = document.getElementById("govern-connect-section");
      if (el && typeof el.scrollIntoView === "function") {
        el.scrollIntoView({ behavior: "smooth", block: "start" });
      }
      onFocusConnectConsumed?.();
    }
  }, [focusConnect, onFocusConnectConsumed]);

  const resolveCompatHandshake = async (
    url: string,
    actor: MePayload,
    envInfo?: Record<string, unknown> | null,
  ) => {
    const probe = await probeInstallVersion(url);
    const window =
      apiRevisionFromPayload(actor) ??
      probe.apiRevision ??
      apiRevisionFromPayload(envInfo ?? {}) ??
      null;
    if (!window) {
      throw new Error("Install did not advertise apiRevision — cannot negotiate wire compatibility");
    }
    const productVersion =
      (typeof actor.productVersion === "string" && actor.productVersion) ||
      probe.productVersion ||
      (typeof envInfo?.productVersion === "string" ? envInfo.productVersion : undefined);

    const loopback = isLoopbackHost(url);
    const pinChoice = selectPin(IDE_COMPAT_MANIFEST, window);
    if ("block" in pinChoice) {
      if (!compatOverride) {
        const cta = compatCta(pinChoice.code);
        throw new Error(`${pinChoice.message}${cta ? ` — ${cta}` : ""}`);
      }
    }
    const pin = "pin" in pinChoice ? pinChoice.pin : window.min;
    const revisionEval = evaluateRevision(pin, window, compatOverride);
    if (revisionEval.status === "block") {
      const cta = compatCta(revisionEval.code);
      throw new Error(`${revisionEval.message ?? "API revision blocked"}${cta ? ` — ${cta}` : ""}`);
    }
    const productEval = evaluateProductTestedAgainst(IDE_COMPAT_MANIFEST, productVersion, loopback);
    const merged = mergeCompatStatus(revisionEval, productEval);
    const compatStatus: CompatStatus = merged.status;
    const compatCode = merged.code;
    if (merged.status === "warn") {
      const cta = compatCta(merged.code);
      setCompatBanner(merged.message ? `${merged.message}${cta ? ` — ${cta}` : ""}` : cta);
    } else {
      setCompatBanner("");
    }
    setApiRevisionPin(pin);
    setPinRange(pinIntersection(IDE_COMPAT_MANIFEST, window));
    return {
      productVersion,
      apiRevisionPin: pin,
      apiRevisionMin: window.min,
      apiRevisionCurrent: window.current,
      compatStatus,
      compatCode,
    };
  };

  const persistActor = async (
    url: string,
    jwt: string,
    actor: MePayload,
    tokenMeta?: { refreshToken?: string; expiresIn?: number },
  ) => {
    const { scopes, isAdmin } = extractActorScopes(actor);
    const actorWithScopes = { ...actor, scopes, isAdmin };
    setMe(actor);
    const envInfo = await fetchEnvInfo(url, jwt, allowInsecureHttp);
    const compat = await resolveCompatHandshake(url, actor, envInfo);
    const conn = connectionFromActor(url, jwt, actorWithScopes, envInfo, tokenMeta);
    conn.scopes = scopes;
    conn.isAdmin = isAdmin;
    conn.systemPermissions = Array.isArray(actor.systemPermissions)
      ? (actor.systemPermissions as string[])
      : undefined;
    conn.productVersion = compat.productVersion;
    conn.apiRevisionPin = compat.apiRevisionPin;
    conn.apiRevisionMin = compat.apiRevisionMin;
    conn.apiRevisionCurrent = compat.apiRevisionCurrent;
    conn.compatStatus = compat.compatStatus;
    conn.compatCode = compat.compatCode;
    // Access-only paths (paste JWT / client_credentials) omit tokenMeta so any
    // previous refresh token for this env is cleared.
    conn.refreshToken = tokenMeta?.refreshToken;
    conn.accessExpiresAt =
      tokenMeta?.expiresIn != null ? Date.now() + tokenMeta.expiresIn * 1000 : undefined;

    const customerRepoUrl =
      (typeof envInfo?.customerRepoUrl === "string" && envInfo.customerRepoUrl) ||
      bridge.session?.customerRepoUrl;

    const next = upsertEnvironment(bridge.session, conn, {
      makeActive: true,
      repoPath: bridge.session?.repoPath,
      customerRepoUrl,
      deviceId: bridge.session?.deviceId,
    });
    // Carry the plaintext-HTTP acknowledgement onto the session so later apiFetch calls work.
    await bridge.setSession({ ...next, allowInsecureHttp: allowInsecureHttp || undefined });
    setUnverifiedPeerOrigin("");
  };

  /**
   * Resolve the install this action should talk to, refusing anything that would put the JWT
   * on an unvetted host. Called by every path that transmits a credential.
   */
  const resolveTargetBaseUrl = (): string => {
    const verdict = checkInstallBaseUrl(baseUrl, { allowInsecureHttp });
    if (!verdict.ok) throw new Error(verdict.error);
    if (unverifiedPeerOrigin && baseUrlOrigin(verdict.url) === unverifiedPeerOrigin) {
      throw new Error(
        `${unverifiedPeerOrigin} came from peer metadata, not from you. Confirm you trust it before sending a token.`,
      );
    }
    return verdict.url;
  };

  const save = async () => {
    setErr("");
    setBusy(true);
    try {
      const url = resolveTargetBaseUrl();
      const jwt = token.trim();
      if (!jwt) throw new Error("Paste a Majesta One JWT, or use Sign in");
      const actor = (await apiFetch(
        url,
        jwt,
        "/client/v1/me",
        {},
        { deviceId: bridge.session?.deviceId, allowInsecureHttp },
      )) as MePayload;
      await persistActor(url, jwt, actor);
    } catch (e) {
      setErr(formatError(e));
      setMe(null);
    } finally {
      setBusy(false);
    }
  };

  const exchangeClientCredentials = async () => {
    setErr("");
    setBusy(true);
    try {
      const url = resolveTargetBaseUrl();
      const body = new URLSearchParams({
        grant_type: "client_credentials",
        client_id: clientId,
        client_secret: clientSecret,
      });
      const res = await fetch(`${url}/auth/v1/token`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body,
      });
      const json = (await res.json()) as { access_token?: string; error?: string };
      if (!res.ok || !json.access_token) throw new Error(json.error ?? `Token request failed (${res.status})`);
      setToken(json.access_token);
      setShowToken(false);
      const actor = (await apiFetch(url, json.access_token, "/client/v1/me", {}, { allowInsecureHttp })) as MePayload;
      await persistActor(url, json.access_token, actor);
    } catch (e) {
      setErr(formatError(e));
      setMe(null);
    } finally {
      setBusy(false);
    }
  };

  const completePkce = async (code: string, expectedState?: string) => {
    // Single-use: consume the pending flow so a replayed deep link finds nothing (CIDE-11).
    const pending = takePendingPkce();
    const verifier = pending?.verifier || pkceVerifier;
    const client = pending?.clientId || oauthClientId.trim() || CONTROL_IDE_INTEGRATION;
    const redirect = pending?.redirectUri || DEFAULT_REDIRECT_URI;
    const apiBase = pending?.baseUrl || resolveTargetBaseUrl();
    if (!verifier) {
      throw new Error("Start Sign in first, then complete the browser login");
    }
    if (expectedState !== undefined && !statesMatch(pending?.state, expectedState)) {
      throw new Error("OAuth state mismatch — start Sign in again");
    }
    const one = await exchangeOneAuthorizationCode({
      baseUrl: apiBase,
      clientId: client,
      redirectUri: redirect,
      code: code.trim(),
      codeVerifier: verifier,
    });
    setToken(one.access_token);
    setShowToken(false);
    setPkceVerifier("");
    setAuthCode("");
    setStatus("Signed in");
    const actor = (await apiFetch(
      apiBase,
      one.access_token,
      "/client/v1/me",
      {},
      { allowInsecureHttp },
    )) as MePayload;
    await persistActor(apiBase, one.access_token, actor, {
      refreshToken: one.refresh_token,
      expiresIn: one.expires_in,
    });
  };

  useEffect(() => {
    const unsub = bridge.onOAuthCallback?.((url) => {
      void (async () => {
        // Any local process can fire a one-control:// deep link; ignore one unless this
        // window actually started a sign-in (CIDE-11).
        if (!loadPendingPkce()) return;
        setErr("");
        setBusy(true);
        try {
          const parsed = parseOAuthCallbackUrl(url);
          if (!parsed) throw new Error("OAuth callback missing an authorization code or state");
          await completePkce(parsed.code, parsed.state);
        } catch (e) {
          setErr(formatError(e));
          setMe(null);
        } finally {
          setBusy(false);
        }
      })();
    });
    return () => {
      unsub?.();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- bridge identity + one subscription
  }, [bridge]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const url = baseUrl.trim().replace(/\/$/, "");
        if (!url) return;
        const res = await fetch(`${url}/auth/v1/install/status`);
        if (!res.ok) return;
        const raw = await res.text();
        const json = JSON.parse(raw || "{}") as { claimed?: boolean };
        if (!cancelled) setInstallClaimed(Boolean(json.claimed));
      } catch {
        if (!cancelled) setInstallClaimed(null);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [baseUrl]);

  const claimInstall = async () => {
    setErr("");
    setBusy(true);
    try {
      const url = resolveTargetBaseUrl();
      const res = await fetch(`${url}/auth/v1/install/claim`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          token: claimToken.trim(),
          email: claimEmail.trim(),
          password: claimPassword,
          displayName: claimDisplayName.trim() || undefined,
        }),
      });
      const json = (await res.json()) as {
        access_token?: string;
        refresh_token?: string;
        expires_in?: number;
        refresh_expires_in?: number;
        error?: string;
        message?: string;
      };
      if (!res.ok || !json.access_token) {
        throw new Error(json.message ?? json.error ?? `Claim failed (${res.status})`);
      }
      setToken(json.access_token);
      setShowToken(false);
      setInstallClaimed(true);
      setStatus("Install claimed");
      const actor = (await apiFetch(url, json.access_token, "/client/v1/me", {}, { allowInsecureHttp })) as MePayload;
      await persistActor(url, json.access_token, actor, {
        refreshToken: json.refresh_token,
        expiresIn: json.expires_in,
      });
    } catch (e) {
      setErr(formatError(e));
      setMe(null);
    } finally {
      setBusy(false);
    }
  };

  const startPkceLogin = async () => {
    setErr("");
    setStatus("");
    setBusy(true);
    try {
      const client = oauthClientId.trim() || CONTROL_IDE_INTEGRATION;
      const apiBase = resolveTargetBaseUrl();
      const { verifier, challenge } = await createPkcePair();
      setPkceVerifier(verifier);
      const state = randomOAuthState();
      const redirectUri = DEFAULT_REDIRECT_URI;
      storePendingPkce({
        verifier,
        state,
        baseUrl: apiBase,
        clientId: client,
        redirectUri,
      });
      const url = buildOneLoginUrl({
        baseUrl: apiBase,
        clientId: client,
        redirectUri,
        codeChallenge: challenge,
        state,
      });
      setStatus("Complete sign-in in your browser…");
      await openExternalUrl(url);
    } catch (e) {
      setErr(formatError(e));
    } finally {
      setBusy(false);
    }
  };

  const finishPkceWithCode = async () => {
    setErr("");
    setBusy(true);
    try {
      if (!authCode.trim()) {
        throw new Error("Paste the authorization code from the callback URL, or use Sign in and finish in-browser");
      }
      // Manual paste still requires a pending flow (verifier); state is not on the pasted code.
      await completePkce(authCode.trim());
    } catch (e) {
      setErr(formatError(e));
      setMe(null);
    } finally {
      setBusy(false);
    }
  };

  const exchangeIdToken = async () => {
    setErr("");
    setBusy(true);
    try {
      const url = resolveTargetBaseUrl();
      const one = await exchangeOneIdToken(url, idTokenPaste.trim());
      setToken(one.access_token);
      setShowToken(false);
      const actor = (await apiFetch(url, one.access_token, "/client/v1/me", {}, { allowInsecureHttp })) as MePayload;
      await persistActor(url, one.access_token, actor, {
        refreshToken: one.refresh_token,
        expiresIn: one.expires_in,
      });
    } catch (e) {
      setErr(formatError(e));
      setMe(null);
    } finally {
      setBusy(false);
    }
  };

  const enrollDevice = async () => {
    setErr("");
    setBusy(true);
    try {
      const url = resolveTargetBaseUrl();
      const jwt = token.trim() || bridge.session?.token;
      if (!jwt) throw new Error("Connect first, then enroll a device");
      const deviceId = bridge.session?.deviceId || crypto.randomUUID();
      const res = (await apiFetch(
        url,
        jwt,
        "/client/v1/devices/enroll",
        {
          method: "POST",
          body: JSON.stringify({ deviceId, label: "Control IDE" }),
        },
        { allowInsecureHttp, apiRevisionPin: activeConn?.apiRevisionPin },
      )) as { deviceId?: string };
      const id = res.deviceId || deviceId;
      const base = bridge.session ? normalizeSession(bridge.session) : null;
      if (!base) {
        const next: Session = upsertEnvironment(
          null,
          {
            installId: "local",
            installRole: "local",
            baseUrl: url,
            token: jwt,
          },
          { deviceId: id },
        );
        await bridge.setSession({ ...next, deviceId: id, allowInsecureHttp: allowInsecureHttp || undefined });
      } else {
        await bridge.setSession({ ...base, deviceId: id });
      }
    } catch (e) {
      setErr(formatError(e));
    } finally {
      setBusy(false);
    }
  };

  const clear = async () => {
    const rt = bridge.session?.refreshToken || activeConn?.refreshToken;
    const url = bridge.session?.baseUrl;
    if (rt && url) {
      await revokeRefreshToken(url, rt, {
        allowInsecureHttp: allowInsecureHttp || bridge.session?.allowInsecureHttp,
      });
    }
    setMe(null);
    setToken("");
    setShowToken(false);
    await bridge.setSession(null);
  };

  const baseUrlVerdict = checkInstallBaseUrl(baseUrl, { allowInsecureHttp });
  const peerNeedsConfirm =
    Boolean(unverifiedPeerOrigin) && baseUrlOrigin(baseUrl) === unverifiedPeerOrigin;

  return (
    <section className="govern-section" id="govern-connect-section" data-testid="connect-section">
      <PanelHeader
        title="Connection"
        subtitle="Add or refresh credentials for an install. AuthZ stays on the server — your session is encrypted on this device."
        actions={
          <StatusBadge tone={connected ? "success" : "neutral"}>
            {connected ? "Connected" : "Not connected"}
          </StatusBadge>
        }
      />

      {encryptionAvailable === false ? (
        <p className="muted" data-testid="connect-ephemeral-hint">
          OS keyring is unavailable on this host. The session is still saved on this device with a
          local encrypted file — you will not need to sign in again after closing the app.
        </p>
      ) : null}

      {!connected && !me ? (
        <EmptyState
          icon={<IconConnect size={28} />}
          title="Not connected"
          description="Claim an unclaimed install (token + email + password), or Sign in via the Majesta One login page (customer SSO / password). Client credentials and JWT paste remain available."
        />
      ) : null}

      {knownEnvs.length > 0 ? (
        <div className="env-card" data-testid="known-environments">
          <p className="muted">Known environments in this session:</p>
          <ul className="repo-file-list">
            {knownEnvs.map((e) => (
              <li key={e.installId}>
                <strong>{envDisplayName(e)}</strong> · {e.installId}
                {!e.token
                  ? " (needs connect)"
                  : e.installId === bridge.session?.activeInstallId
                    ? " (active)"
                    : ""}
                <span className="muted"> — {e.baseUrl}</span>
              </li>
            ))}
          </ul>
        </div>
      ) : null}

      <div className="row">
        <label>
          Install base URL
          <input
            value={baseUrl}
            onChange={(e) => {
              setBaseUrl(e.target.value);
              // Typing a different host clears the peer-origin hold; typing the same keeps it.
              if (unverifiedPeerOrigin && baseUrlOrigin(e.target.value) !== unverifiedPeerOrigin) {
                setUnverifiedPeerOrigin("");
              }
            }}
            data-testid="connect-base-url"
          />
        </label>
      </div>

      {peerNeedsConfirm ? (
        <div className="env-card" data-testid="connect-peer-confirm">
          <p className="err">
            <strong>{unverifiedPeerOrigin}</strong> came from peer metadata on another install, not from
            you. Confirm you trust this host before Majesta One will send a token there.
          </p>
          <div className="row">
            <Button
              variant="primary"
              data-testid="connect-peer-trust"
              onClick={() => setUnverifiedPeerOrigin("")}
            >
              I trust this host
            </Button>
            <Button
              variant="secondary"
              onClick={() => {
                setBaseUrl(bridge.session?.baseUrl || "http://localhost:8080");
                setUnverifiedPeerOrigin("");
              }}
            >
              Use my current install URL
            </Button>
          </div>
        </div>
      ) : null}

      {!baseUrlVerdict.ok && baseUrlVerdict.needsInsecureAck ? (
        <label className="row" data-testid="connect-insecure-ack" style={{ alignItems: "flex-start" }}>
          <input
            type="checkbox"
            checked={allowInsecureHttp}
            onChange={(e) => setAllowInsecureHttp(e.target.checked)}
            style={{ marginTop: "0.35rem" }}
          />
          <span>
            Send my Majesta One token to <strong>{baseUrlOrigin(baseUrl) || baseUrl}</strong> over plaintext
            HTTP. Prefer HTTPS.
          </span>
        </label>
      ) : null}

      {installClaimed === false ? (
        <div className="env-card" data-testid="connect-claim">
          <PanelHeader
            title="Claim install"
            subtitle="Day-0: create the first SystemAdmin with INSTALL_CLAIM_TOKEN + email + password (also works via curl)."
          />
          <div className="row">
            <label>
              Claim token
              <input
                type="password"
                value={claimToken}
                onChange={(e) => setClaimToken(e.target.value)}
                aria-label="claim_token"
                data-testid="connect-claim-token"
              />
            </label>
          </div>
          <div className="row">
            <label>
              Admin email
              <input
                type="email"
                value={claimEmail}
                onChange={(e) => setClaimEmail(e.target.value)}
                aria-label="claim_email"
                data-testid="connect-claim-email"
              />
            </label>
            <label>
              Password (min 10)
              <input
                type="password"
                value={claimPassword}
                onChange={(e) => setClaimPassword(e.target.value)}
                aria-label="claim_password"
                data-testid="connect-claim-password"
              />
            </label>
          </div>
          <div className="row">
            <label>
              Display name (optional)
              <input
                value={claimDisplayName}
                onChange={(e) => setClaimDisplayName(e.target.value)}
                aria-label="claim_display_name"
              />
            </label>
          </div>
          <Button
            variant="primary"
            busy={busy}
            onClick={() => void claimInstall()}
            data-testid="connect-claim-submit"
          >
            Claim install
          </Button>
        </div>
      ) : null}

      <div className="env-card">
        <PanelHeader
          title="Sign in"
          subtitle="Opens the Majesta One login page in your system browser. The page shows customer SSO (if configured), password, or optional social providers — not a Google-only default."
        />
        <div className="row">
          <Button variant="primary" busy={busy} onClick={() => void startPkceLogin()} data-testid="connect-sign-in">
            Sign in
          </Button>
        </div>
        {status ? <p className="muted">{status}</p> : null}
        <details className="details-block">
          <summary>Manual code paste (if the browser callback did not return)</summary>
          <div className="row">
            <label>
              Authorization code
              <input value={authCode} onChange={(e) => setAuthCode(e.target.value)} aria-label="auth_code" />
            </label>
          </div>
          <Button variant="secondary" busy={busy} onClick={() => void finishPkceWithCode()}>
            Finish with code → Majesta One JWT
          </Button>
          <p className="muted">Or paste an external IdP ID token (Okta/Entra adapter):</p>
          <div className="row">
            <label>
              IdP ID token
              <textarea
                rows={2}
                value={idTokenPaste}
                onChange={(e) => setIdTokenPaste(e.target.value)}
                aria-label="idp_id_token"
              />
            </label>
          </div>
          <Button variant="secondary" busy={busy} onClick={() => void exchangeIdToken()}>
            Exchange ID token
          </Button>
          <label>
            Connected App client id
            <input
              value={oauthClientId}
              onChange={(e) => setOauthClientId(e.target.value)}
              placeholder={CONTROL_IDE_INTEGRATION}
              aria-label="oauth_client_id"
            />
          </label>
        </details>
      </div>

      <div className="env-card">
        <PanelHeader
          title="Client credentials"
          subtitle="Machine / service login."
        />
        <div className="row">
          <label>
            Client ID
            <input value={clientId} onChange={(e) => setClientId(e.target.value)} aria-label="client_id" />
          </label>
          <label>
            Client secret
            <input
              type="password"
              value={clientSecret}
              onChange={(e) => setClientSecret(e.target.value)}
              aria-label="client_secret"
            />
          </label>
        </div>
        <p className="muted">Uses POST /auth/v1/token under the hood.</p>
        <Button variant="secondary" busy={busy} onClick={() => void exchangeClientCredentials()}>
          Sign in with client credentials
        </Button>
      </div>

      <details
        className="govern-advanced details-block"
        open={advancedOpen}
        onToggle={(e) => setAdvancedOpen((e.target as HTMLDetailsElement).open)}
        data-testid="connect-advanced"
      >
        <summary>Advanced — paste JWT &amp; device enroll</summary>
        <p className="muted">
          Verifies against /client/v1/me after connect. Integration: {CONTROL_IDE_INTEGRATION}
        </p>
        <div className="row">
          <label style={{ flex: 1 }}>
            Majesta One JWT
            {showToken ? (
              <textarea
                rows={3}
                value={token}
                onChange={(e) => setToken(e.target.value)}
                aria-label="Majesta One JWT"
                data-testid="connect-jwt-input"
              />
            ) : (
              <input
                readOnly
                value={token ? maskToken(token) : ""}
                placeholder="Paste a JWT (click Reveal)"
                aria-label="Majesta One JWT (masked)"
                data-testid="connect-jwt-masked"
                onFocus={() => setShowToken(true)}
              />
            )}
          </label>
        </div>
        <div className="row">
          <Button
            variant="ghost"
            data-testid="connect-jwt-reveal"
            onClick={() => setShowToken((v) => !v)}
          >
            {showToken ? "Hide token" : "Reveal / paste token"}
          </Button>
          <Button variant="primary" busy={busy} onClick={() => void save()} data-testid="connect-jwt-save">
            Connect with JWT
          </Button>
          <Button variant="secondary" onClick={() => void clear()} data-testid="connect-clear-session">
            Clear session
          </Button>
          <Button variant="secondary" busy={busy} onClick={() => void enrollDevice()}>
            Enroll device
          </Button>
        </div>
      </details>

      {compatBanner ? (
        <p className="warn" data-testid="connect-compat-warn">{compatBanner}</p>
      ) : null}

      {pinRange && connected ? (
        <div className="env-card" data-testid="connect-api-revision">
          <PanelHeader
            title="API revision pin"
            subtitle={`Wire contract for ${activeConn?.compatStatus ?? "ok"} — sends One-API-Revision on every call.`}
          />
          <label>
            Pin ({pinRange.minPin}–{pinRange.maxPin})
            <input
              type="number"
              min={pinRange.minPin}
              max={pinRange.maxPin}
              value={apiRevisionPin ?? pinRange.minPin}
              onChange={(e) => {
                const n = Number(e.target.value);
                if (Number.isFinite(n)) setApiRevisionPin(n);
              }}
              data-testid="connect-api-revision-pin"
            />
          </label>
          <Button
            variant="secondary"
            onClick={() => {
              if (apiRevisionPin != null) void updateSessionPin(apiRevisionPin);
            }}
          >
            Apply pin
          </Button>
        </div>
      ) : null}

      <label className="row" data-testid="connect-compat-override" style={{ alignItems: "flex-start" }}>
        <input
          type="checkbox"
          checked={compatOverride}
          onChange={(e) => setCompatOverride(e.target.checked)}
          style={{ marginTop: "0.35rem" }}
        />
        <span>
          Connect anyway (break-glass compat override). Still sends an API revision pin when possible.
        </span>
      </label>

      {err && <p className="err">{err}</p>}
      {(me || connected) && (
        <div className="env-card identity-card" data-testid="identity-card">
          <PanelHeader title="Active identity" subtitle="Verified principal for this session." />
          <KeyValueList
            items={
              me
                ? identityItems(me)
                : [
                    { label: "Base URL", value: bridge.session?.baseUrl ?? "" },
                    { label: "Install", value: bridge.session?.activeInstallId ?? "" },
                    {
                      label: "Scopes",
                      value: (bridge.session?.scopes ?? []).join(", ") || "—",
                    },
                  ]
            }
          />
          {me ? (
            <details className="details-block">
              <summary>Raw /me response</summary>
              <pre className="log">{JSON.stringify(me, null, 2)}</pre>
            </details>
          ) : null}
        </div>
      )}
    </section>
  );
}
