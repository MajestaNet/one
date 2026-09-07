/**
 * RFC 8252 loopback PKCE callback for Control IDE.
 *
 * The install already allowlists `http://127.0.0.1:5173/oauth/callback`. Completing
 * sign-in in the OS browser then hits this listener instead of `one-control://`, which
 * on unpackaged macOS would launch a second Electron.app.
 */

import http from "node:http";

/** Seeded Control IDE public-app callback (must match `internal/integration.EnsureControlIDE`). */
export const LOOPBACK_OAUTH_PORT = 5173;
export const LOOPBACK_OAUTH_PATH = "/oauth/callback";
export const LOOPBACK_REDIRECT_URI = `http://127.0.0.1:${LOOPBACK_OAUTH_PORT}${LOOPBACK_OAUTH_PATH}`;

export type LoopbackRequest = {
  method: string;
  url: string;
  host: string;
  port: number;
};

export type LoopbackResponse = {
  status: number;
  body: string;
  contentType: string;
  /** Absolute callback URL to broadcast to the renderer (only when code+state are present). */
  callbackUrl?: string;
};

const HTML = "text/html; charset=utf-8";

function allowedHosts(port: number): Set<string> {
  return new Set([
    `127.0.0.1:${port}`,
    `localhost:${port}`,
    "[::1]:" + String(port),
  ]);
}

export function oauthCallbackSuccessHtml(): string {
  return `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"/><title>Signed in · Majesta One Control</title></head>
<body style="font-family:system-ui,sans-serif;padding:2rem;max-width:32rem">
  <h1>You can return to Control IDE</h1>
  <p>Sign-in finished. This tab can be closed.</p>
</body>
</html>`;
}

export function oauthCallbackErrorHtml(kind: "method" | "not-found" | "invalid" | "missing"): string {
  const message =
    kind === "method"
      ? "Method not allowed."
      : kind === "missing"
        ? "This callback is missing an authorization code or state."
        : kind === "invalid"
          ? "Invalid callback URL."
          : "Not found.";
  return `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"/><title>Sign-in · Majesta One Control</title></head>
<body style="font-family:system-ui,sans-serif;padding:2rem;max-width:32rem">
  <h1>Sign-in did not complete</h1>
  <p>${message}</p>
</body>
</html>`;
}

/** Pure request policy for the loopback listener (unit-tested without binding a port). */
export function handleLoopbackRequest(req: LoopbackRequest): LoopbackResponse {
  const method = (req.method || "GET").toUpperCase();
  if (method !== "GET" && method !== "HEAD") {
    return { status: 405, body: oauthCallbackErrorHtml("method"), contentType: HTML };
  }
  const host = String(req.host ?? "")
    .trim()
    .toLowerCase();
  if (!allowedHosts(req.port).has(host)) {
    return { status: 404, body: oauthCallbackErrorHtml("not-found"), contentType: HTML };
  }

  let parsed: URL;
  try {
    parsed = new URL(req.url || "/", `http://127.0.0.1:${req.port}`);
  } catch {
    return { status: 400, body: oauthCallbackErrorHtml("invalid"), contentType: HTML };
  }
  if (parsed.pathname !== LOOPBACK_OAUTH_PATH) {
    return { status: 404, body: oauthCallbackErrorHtml("not-found"), contentType: HTML };
  }

  const code = parsed.searchParams.get("code") ?? "";
  const state = parsed.searchParams.get("state") ?? "";
  if (!code || !state) {
    return {
      status: 400,
      body: oauthCallbackErrorHtml("missing"),
      contentType: HTML,
    };
  }

  return {
    status: 200,
    body: oauthCallbackSuccessHtml(),
    contentType: HTML,
    callbackUrl: parsed.href,
  };
}

export type OAuthLoopbackServer = {
  redirectUri: string;
  close: () => Promise<void>;
};

/**
 * Bind `127.0.0.1:5173` for one PKCE round-trip. Callers must close after the
 * callback (or a timeout). Fails if Vite / another process already owns the port.
 */
export function startOAuthLoopback(
  onCallback: (url: string) => void,
  port: number = LOOPBACK_OAUTH_PORT,
): Promise<OAuthLoopbackServer> {
  return new Promise((resolve, reject) => {
    const server = http.createServer((req, res) => {
      const addr = server.address();
      const bound = typeof addr === "object" && addr ? addr.port : port;
      const result = handleLoopbackRequest({
        method: req.method ?? "GET",
        url: req.url ?? "/",
        host: req.headers.host ?? "",
        port: bound,
      });
      res.writeHead(result.status, { "Content-Type": result.contentType });
      if ((req.method ?? "GET").toUpperCase() === "HEAD") {
        res.end();
      } else {
        res.end(result.body);
      }
      if (result.callbackUrl) onCallback(result.callbackUrl);
    });
    server.once("error", reject);
    server.listen(port, "127.0.0.1", () => {
      const addr = server.address();
      const bound = typeof addr === "object" && addr ? addr.port : port;
      resolve({
        redirectUri: `http://127.0.0.1:${bound}${LOOPBACK_OAUTH_PATH}`,
        close: () =>
          new Promise((resClose, rejClose) => {
            server.close((err) => (err ? rejClose(err) : resClose()));
          }),
      });
    });
  });
}
