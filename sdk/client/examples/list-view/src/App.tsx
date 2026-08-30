import { useCallback, useEffect, useMemo, useState } from "react";
import { createOneAuthClient, exchangeAuthorizationCode, generatePKCE } from "@one/auth";
import { createOneClient } from "@one/client";

const baseUrl = import.meta.env.VITE_ONE_BASE_URL ?? "http://127.0.0.1:8080";
const clientId = import.meta.env.VITE_ONE_CLIENT_ID ?? "";
const redirectUri = "http://127.0.0.1:5174/oauth/callback";

const PKCE_KEY = "one.pkce";
const TOKEN_KEY = "one.accessToken";

export function App() {
  const [token, setToken] = useState<string | null>(() => sessionStorage.getItem(TOKEN_KEY));
  const [rows, setRows] = useState<unknown[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const auth = useMemo(
    () =>
      createOneAuthClient({
        baseUrl,
        clientId,
        redirectUri,
        scopes: ["client"],
      }),
    [],
  );

  const client = useMemo(
    () =>
      createOneClient({
        baseUrl,
        getAccessToken: () => token ?? "",
      }),
    [token],
  );

  const startLogin = useCallback(async () => {
    setError("");
    const { verifier, challenge } = await generatePKCE();
    sessionStorage.setItem(PKCE_KEY, verifier);
    const state = crypto.randomUUID();
    sessionStorage.setItem("one.oauth.state", state);
    window.location.href = auth.buildAuthorizeUrl(state, challenge);
  }, [auth]);

  useEffect(() => {
    const path = window.location.pathname;
    if (path !== "/oauth/callback") return;
    const params = new URLSearchParams(window.location.search);
    const code = params.get("code");
    const state = params.get("state");
    const savedState = sessionStorage.getItem("one.oauth.state");
    const verifier = sessionStorage.getItem(PKCE_KEY);
    if (!code || !verifier || state !== savedState) {
      setError("OAuth callback missing code or state mismatch");
      return;
    }
    void (async () => {
      try {
        const tok = await exchangeAuthorizationCode(auth.config, code, verifier);
        sessionStorage.setItem(TOKEN_KEY, tok.access_token);
        setToken(tok.access_token);
        window.history.replaceState({}, "", "/");
      } catch (e) {
        setError(String(e));
      }
    })();
  }, [auth.config]);

  const loadAccounts = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    setError("");
    try {
      const res = await client.query({
        object: "Account",
        select: ["Name"],
        limit: 25,
      });
      setRows(res.records ?? []);
    } catch (e) {
      setError(String(e));
      setRows([]);
    } finally {
      setLoading(false);
    }
  }, [client, token]);

  useEffect(() => {
    if (token) void loadAccounts();
  }, [token, loadAccounts]);

  return (
    <main style={{ fontFamily: "system-ui", margin: "2rem", maxWidth: 720 }}>
      <h1>Majesta One Client Experience — List view</h1>
      <p>Sample Account list via <code>/client/v1/query</code> (Client scope only).</p>
      {!token ? (
        <button type="button" onClick={() => void startLogin()} disabled={!clientId}>
          Sign in with Majesta One
        </button>
      ) : (
        <>
          <button type="button" onClick={() => void loadAccounts()} disabled={loading}>
            Refresh
          </button>
          <button
            type="button"
            onClick={() => {
              sessionStorage.removeItem(TOKEN_KEY);
              setToken(null);
              setRows([]);
            }}
            style={{ marginLeft: 8 }}
          >
            Sign out
          </button>
        </>
      )}
      {!clientId && <p style={{ color: "crimson" }}>Set VITE_ONE_CLIENT_ID in .env</p>}
      {error && <pre style={{ color: "crimson" }}>{error}</pre>}
      {loading && <p>Loading…</p>}
      <ul>
        {rows.map((row, i) => (
          <li key={i}>
            <code>{JSON.stringify(row)}</code>
          </li>
        ))}
      </ul>
    </main>
  );
}
