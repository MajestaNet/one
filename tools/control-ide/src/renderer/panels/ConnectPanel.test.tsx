import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AppBridge } from "../App";
import { ConnectPanel } from "./ConnectPanel";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const API_REVISION = { min: 1, current: 1 };

function jsonRes(body: Record<string, unknown>, ok = true, status = 200) {
  return {
    ok,
    status,
    text: async () => JSON.stringify(body),
    json: async () => body,
  };
}

/** URL-aware fetch mock so Connect handshake (/version, /me, /environment) stays in-window. */
function mockConnectFetch(opts?: {
  me?: Record<string, unknown>;
  token?: Record<string, unknown>;
  status?: Record<string, unknown>;
  version?: Record<string, unknown>;
  environment?: Record<string, unknown>;
  tokenOk?: boolean;
  meOk?: boolean;
}) {
  return vi.fn().mockImplementation(async (input: RequestInfo) => {
    const url = String(input);
    if (url.includes("/auth/v1/install/status")) {
      return jsonRes(opts?.status ?? { claimed: true });
    }
    if (url.includes("/auth/v1/revoke")) {
      return jsonRes({});
    }
    if (url.includes("/version")) {
      return jsonRes(opts?.version ?? { productVersion: "0.1.0", apiRevision: API_REVISION });
    }
    if (url.includes("/deploy/v1/environment")) {
      return jsonRes(opts?.environment ?? { apiRevision: API_REVISION, productVersion: "0.1.0" });
    }
    if (url.includes("/auth/v1/token")) {
      const ok = opts?.tokenOk ?? true;
      return jsonRes(opts?.token ?? { access_token: "minted-jwt" }, ok, ok ? 200 : 400);
    }
    if (url.includes("/client/v1/me")) {
      const ok = opts?.meOk ?? true;
      return jsonRes(
        {
          apiRevision: API_REVISION,
          principal: "admin",
          systemPermissions: ["identity.users"],
          ...opts?.me,
        },
        ok,
        ok ? 200 : 401,
      );
    }
    return jsonRes({ apiRevision: API_REVISION });
  });
}

function bridge(overrides: Partial<AppBridge> = {}): AppBridge {
  return {
    session: null,
    setSession: vi.fn().mockResolvedValue(undefined),
    fetch: vi.fn(),
    ...overrides,
  };
}

describe("ConnectPanel", () => {
  it("saves JWT session and shows identity", async () => {
    const user = userEvent.setup();
    const setSession = vi.fn().mockResolvedValue(undefined);
    const b = bridge({ setSession });

    vi.stubGlobal("fetch", mockConnectFetch());

    render(<ConnectPanel bridge={b} />);
    const url = screen.getByDisplayValue("http://localhost:8080");
    await user.clear(url);
    await user.type(url, "https://api.example");
    await user.click(screen.getByTestId("connect-jwt-reveal"));
    await user.type(screen.getByTestId("connect-jwt-input"), "eyJ.test");
    await user.click(screen.getByRole("button", { name: /Connect with JWT/i }));

    await waitFor(() => expect(setSession).toHaveBeenCalled());
    expect(setSession).toHaveBeenCalledWith(
      expect.objectContaining({
        baseUrl: "https://api.example",
        token: "eyJ.test",
        scopes: [],
        isAdmin: false,
        systemPermissions: ["identity.users"],
        environments: expect.arrayContaining([
          expect.objectContaining({
            apiRevisionPin: 1,
            apiRevisionMin: 1,
            apiRevisionCurrent: 1,
            compatStatus: "ok",
          }),
        ]),
      }),
    );
    expect(await screen.findByTestId("identity-card")).toBeTruthy();
    expect(screen.getByText("admin")).toBeTruthy();
    expect(setSession.mock.calls[0][0].refreshToken).toBeFalsy();
  });

  it("blocks connect when the install omits apiRevision", async () => {
    const user = userEvent.setup();
    const setSession = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal(
      "fetch",
      mockConnectFetch({
        me: { principal: "admin", apiRevision: undefined },
        version: { productVersion: "0.1.0" },
        environment: {},
      }),
    );

    render(<ConnectPanel bridge={bridge({ setSession })} />);
    await user.click(screen.getByTestId("connect-jwt-reveal"));
    await user.type(screen.getByTestId("connect-jwt-input"), "eyJ.test");
    await user.click(screen.getByRole("button", { name: /Connect with JWT/i }));

    expect(await screen.findByText(/did not advertise apiRevision/i)).toBeTruthy();
    expect(setSession).not.toHaveBeenCalled();
  });

  it("shows error when /me verify fails", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        text: async () => JSON.stringify({ error: "unauthorized" }),
        status: 401,
      }),
    );

    render(<ConnectPanel bridge={bridge()} />);
    await user.click(screen.getByTestId("connect-jwt-reveal"));
    await user.type(screen.getByTestId("connect-jwt-input"), "bad");
    await user.click(screen.getByRole("button", { name: /Connect with JWT/i }));
    expect(await screen.findByText(/401|unauthorized|Error/i)).toBeTruthy();
  });

  it("clears session", async () => {
    const user = userEvent.setup();
    const setSession = vi.fn().mockResolvedValue(undefined);
    render(<ConnectPanel bridge={bridge({ setSession })} />);
    await user.click(screen.getByRole("button", { name: /Clear session/i }));
    await waitFor(() => expect(setSession).toHaveBeenCalledWith(null));
  });

  it("revokes the refresh token before clearing a session", async () => {
    const user = userEvent.setup();
    const setSession = vi.fn().mockResolvedValue(undefined);
    const fetchMock = mockConnectFetch();
    vi.stubGlobal("fetch", fetchMock);
    render(
      <ConnectPanel
        bridge={bridge({
          setSession,
          session: {
            baseUrl: "http://localhost:8080",
            token: "sess-jwt",
            refreshToken: "rt-old",
            activeInstallId: "local",
            environments: [
              {
                installId: "local",
                installRole: "local",
                baseUrl: "http://localhost:8080",
                token: "sess-jwt",
                refreshToken: "rt-old",
              },
            ],
          },
        })}
      />,
    );
    await user.click(screen.getByRole("button", { name: /Clear session/i }));
    await waitFor(() => expect(setSession).toHaveBeenCalledWith(null));
    const revoke = fetchMock.mock.calls.find((c) => String(c[0]).includes("/auth/v1/revoke"));
    expect(revoke).toBeTruthy();
    expect(JSON.parse(String((revoke![1] as RequestInit).body))).toEqual({
      token: "rt-old",
      token_type_hint: "refresh_token",
    });
  });

  it("exchanges client credentials for a token", async () => {
    const user = userEvent.setup();
    const setSession = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("fetch", mockConnectFetch({ me: { principal: "svc" } }));

    render(<ConnectPanel bridge={bridge({ setSession })} />);
    await user.type(screen.getByLabelText("client_id"), "cid");
    await user.type(screen.getByLabelText("client_secret"), "sec");
    await user.click(screen.getByRole("button", { name: /Sign in with client credentials/i }));

    await waitFor(() => expect(setSession).toHaveBeenCalled());
    expect(setSession.mock.calls[0][0]).toMatchObject({ token: "minted-jwt" });
    expect(setSession.mock.calls[0][0].refreshToken).toBeFalsy();
    expect(await screen.findByText("svc")).toBeTruthy();
    expect(screen.queryByText(/ide_users/i)).toBeNull();
  });

  it("shows error when token exchange fails", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        json: async () => ({ error: "invalid_client" }),
      }),
    );

    render(<ConnectPanel bridge={bridge()} />);
    await user.type(screen.getByLabelText("client_id"), "cid");
    await user.type(screen.getByLabelText("client_secret"), "bad");
    await user.click(screen.getByRole("button", { name: /Sign in with client credentials/i }));
    expect(await screen.findByText(/invalid_client/i)).toBeTruthy();
  });

  it("opens Majesta One login page after Sign in", async () => {
    const user = userEvent.setup();
    const open = vi.fn();
    vi.stubGlobal("open", open);
    vi.stubGlobal("sessionStorage", {
      store: {} as Record<string, string>,
      getItem(k: string) {
        return this.store[k] ?? null;
      },
      setItem(k: string, v: string) {
        this.store[k] = v;
      },
      removeItem(k: string) {
        delete this.store[k];
      },
    });

    render(<ConnectPanel bridge={bridge()} />);
    await user.click(screen.getByTestId("connect-sign-in"));

    await waitFor(() => expect(open).toHaveBeenCalled());
    expect(String(open.mock.calls[0][0])).toContain("/auth/v1/login?");
    expect(String(open.mock.calls[0][0])).toContain("client_id=");
    expect(String(open.mock.calls[0][0])).toContain("one.controlIde");
    expect(String(open.mock.calls[0][0])).toContain("offline_access");
    expect(String(open.mock.calls[0][0])).not.toContain("provider=");
  });

  it("hands the login URL to the OS browser under Electron, not an in-app window", async () => {
    const user = userEvent.setup();
    const open = vi.fn();
    const openExternal = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("open", open);
    vi.stubGlobal("sessionStorage", {
      store: {} as Record<string, string>,
      getItem(k: string) {
        return this.store[k] ?? null;
      },
      setItem(k: string, v: string) {
        this.store[k] = v;
      },
      removeItem(k: string) {
        delete this.store[k];
      },
    });
    (window as unknown as { one: unknown }).one = { openExternal };

    try {
      render(<ConnectPanel bridge={bridge()} />);
      await user.click(screen.getByTestId("connect-sign-in"));

      await waitFor(() => expect(openExternal).toHaveBeenCalled());
      expect(String(openExternal.mock.calls[0][0])).toContain("/auth/v1/login?");
      // A renderer-created window would inherit the preload bridge (CIDE-01).
      expect(open).not.toHaveBeenCalled();
    } finally {
      delete (window as { one?: unknown }).one;
    }
  });

  it("exchanges pasted IdP ID token for Majesta One JWT", async () => {
    const user = userEvent.setup();
    const setSession = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal(
      "fetch",
      mockConnectFetch({
        token: { access_token: "exchanged-jwt", refresh_token: "rt-ex", expires_in: 3600 },
        me: { id: "u1", principalType: "user" },
      }),
    );

    render(<ConnectPanel bridge={bridge({ setSession })} />);
    await user.click(screen.getByText(/Manual code paste/i));
    await user.type(screen.getByLabelText("idp_id_token"), "idp.id");
    await user.click(screen.getByRole("button", { name: /Exchange ID token/i }));

    await waitFor(() => expect(setSession).toHaveBeenCalled());
    expect(setSession.mock.calls[0][0]).toMatchObject({
      token: "exchanged-jwt",
      refreshToken: "rt-ex",
    });
  });

  it("finishes PKCE with authorization code", async () => {
    const user = userEvent.setup();
    const setSession = vi.fn().mockResolvedValue(undefined);
    const open = vi.fn();
    vi.stubGlobal("open", open);
    vi.stubGlobal("sessionStorage", {
      store: {} as Record<string, string>,
      getItem(k: string) {
        return this.store[k] ?? null;
      },
      setItem(k: string, v: string) {
        this.store[k] = v;
      },
      removeItem(k: string) {
        delete this.store[k];
      },
    });

    vi.stubGlobal(
      "fetch",
      mockConnectFetch({
        token: { access_token: "one-from-pkce", refresh_token: "rt-pkce", expires_in: 3600 },
        me: { id: "u2" },
      }),
    );

    render(<ConnectPanel bridge={bridge({ setSession })} />);
    await user.click(screen.getByTestId("connect-sign-in"));
    await waitFor(() => expect(open).toHaveBeenCalled());

    await user.click(screen.getByText(/Manual code paste/i));
    await user.type(screen.getByLabelText("auth_code"), "the-code");
    await user.click(screen.getByRole("button", { name: /Finish with code/i }));

    await waitFor(() => expect(setSession).toHaveBeenCalled());
    expect(setSession.mock.calls[0][0]).toMatchObject({
      token: "one-from-pkce",
      refreshToken: "rt-pkce",
    });
  });

  it("enrolls a device after connect", async () => {
    const user = userEvent.setup();
    const setSession = vi.fn().mockResolvedValue(undefined);
    const b = bridge({
      setSession,
      session: {
        baseUrl: "http://localhost:8080",
        token: "sess-jwt",
      },
    });
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        text: async () => JSON.stringify({ deviceId: "device-uuid" }),
      }),
    );

    render(<ConnectPanel bridge={b} />);
    await user.click(screen.getByRole("button", { name: /Enroll device/i }));
    await waitFor(() => expect(setSession).toHaveBeenCalled());
    expect(setSession.mock.calls[0][0]).toMatchObject({
      token: "sess-jwt",
      deviceId: "device-uuid",
    });
  });

  it("requires connect before device enroll", async () => {
    const user = userEvent.setup();
    render(<ConnectPanel bridge={bridge()} />);
    await user.click(screen.getByRole("button", { name: /Enroll device/i }));
    expect(await screen.findByText(/Connect first/i)).toBeTruthy();
  });

  it("blocks Sign in to a peer-supplied host until the operator confirms it", async () => {
    const user = userEvent.setup();
    const open = vi.fn();
    vi.stubGlobal("open", open);
    vi.stubGlobal("sessionStorage", {
      store: {} as Record<string, string>,
      getItem(k: string) {
        return this.store[k] ?? null;
      },
      setItem(k: string, v: string) {
        this.store[k] = v;
      },
      removeItem(k: string) {
        delete this.store[k];
      },
    });

    render(
      <ConnectPanel
        bridge={bridge()}
        prefillBaseUrl="https://peer.example:8443"
      />,
    );

    expect(await screen.findByTestId("connect-peer-confirm")).toBeTruthy();
    await user.click(screen.getByTestId("connect-sign-in"));
    expect(open).not.toHaveBeenCalled();
    expect(screen.getAllByText(/came from peer metadata/i).length).toBeGreaterThan(0);

    await user.click(screen.getByTestId("connect-peer-trust"));
    await user.click(screen.getByTestId("connect-sign-in"));
    await waitFor(() => expect(open).toHaveBeenCalled());
    expect(String(open.mock.calls[0][0])).toContain("https://peer.example:8443");
  });

  it("masks the JWT until Reveal and never shows the raw token while masked (CIDE-18)", async () => {
    const user = userEvent.setup();
    const secret = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature";
    render(
      <ConnectPanel
        bridge={bridge({
          session: { baseUrl: "https://api.example", token: secret },
        })}
      />,
    );
    const masked = (await screen.findByTestId("connect-jwt-masked")) as HTMLInputElement;
    expect(masked.value).not.toContain("payload");
    expect(masked.value).toMatch(/…/);
    await user.click(screen.getByTestId("connect-jwt-reveal"));
    expect((screen.getByTestId("connect-jwt-input") as HTMLTextAreaElement).value).toBe(secret);
  });

  it("ignores an OAuth deep link when no PKCE flow is pending (CIDE-11)", async () => {
    let callback: ((url: string) => void) | undefined;
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("sessionStorage", {
      store: {} as Record<string, string>,
      getItem(k: string) {
        return this.store[k] ?? null;
      },
      setItem(k: string, v: string) {
        this.store[k] = v;
      },
      removeItem(k: string) {
        delete this.store[k];
      },
    });

    render(
      <ConnectPanel
        bridge={bridge({
          onOAuthCallback: (handler) => {
            callback = handler;
            return () => {
              callback = undefined;
            };
          },
        })}
      />,
    );

    await waitFor(() => expect(callback).toBeTypeOf("function"));
    callback!("one-control://oauth/callback?code=stolen&state=x");
    await new Promise((r) => setTimeout(r, 30));
    // Install status probe is allowed; OAuth token exchange must not run without pending PKCE.
    const oauthCalls = fetchMock.mock.calls.filter(
      (c) => !String(c[0]).includes("/auth/v1/install/status"),
    );
    expect(oauthCalls).toHaveLength(0);
  });

  it("shows claim form when install is unclaimed", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        text: async () => JSON.stringify({ claimed: false }),
      }),
    );
    render(<ConnectPanel bridge={bridge()} />);
    expect(await screen.findByTestId("connect-claim")).toBeTruthy();
    expect(screen.getByTestId("connect-claim-submit")).toBeTruthy();
  });

  it("captures refresh_token from install claim", async () => {
    const user = userEvent.setup();
    const setSession = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation(async (input: RequestInfo) => {
        const url = String(input);
        if (url.includes("/auth/v1/install/status")) return jsonRes({ claimed: false });
        if (url.includes("/auth/v1/install/claim")) {
          return jsonRes({
            access_token: "claimed-jwt",
            refresh_token: "rt-claim",
            expires_in: 3600,
          });
        }
        if (url.includes("/version")) {
          return jsonRes({ productVersion: "0.1.0", apiRevision: API_REVISION });
        }
        if (url.includes("/deploy/v1/environment")) {
          return jsonRes({ apiRevision: API_REVISION, productVersion: "0.1.0" });
        }
        if (url.includes("/client/v1/me")) {
          return jsonRes({ apiRevision: API_REVISION, principal: "admin" });
        }
        return jsonRes({ apiRevision: API_REVISION });
      }),
    );

    render(<ConnectPanel bridge={bridge({ setSession })} />);
    expect(await screen.findByTestId("connect-claim")).toBeTruthy();
    await user.type(screen.getByTestId("connect-claim-token"), "install-token");
    await user.type(screen.getByTestId("connect-claim-email"), "ada@example.com");
    await user.type(screen.getByTestId("connect-claim-password"), "password-ok");
    await user.click(screen.getByTestId("connect-claim-submit"));

    await waitFor(() => expect(setSession).toHaveBeenCalled());
    expect(setSession.mock.calls[0][0]).toMatchObject({
      token: "claimed-jwt",
      refreshToken: "rt-claim",
    });
  });

  it("requires an explicit acknowledgement before plaintext remote HTTP", async () => {
    const user = userEvent.setup();
    const open = vi.fn();
    vi.stubGlobal("open", open);
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        text: async () => JSON.stringify({ claimed: true }),
      }),
    );
    vi.stubGlobal("sessionStorage", {
      store: {} as Record<string, string>,
      getItem(k: string) {
        return this.store[k] ?? null;
      },
      setItem(k: string, v: string) {
        this.store[k] = v;
      },
      removeItem(k: string) {
        delete this.store[k];
      },
    });
    render(<ConnectPanel bridge={bridge()} />);
    const url = screen.getByDisplayValue("http://localhost:8080");
    await user.clear(url);
    await user.type(url, "http://api.example");
    expect(await screen.findByTestId("connect-insecure-ack")).toBeTruthy();
    await user.click(screen.getByTestId("connect-sign-in"));
    expect(open).not.toHaveBeenCalled();
    expect(screen.getByText(/accept the risk explicitly/i)).toBeTruthy();
  });
});
