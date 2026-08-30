import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AppBridge } from "../App";
import { InstallAuthPanel } from "./InstallAuthPanel";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function bridge(overrides: Partial<AppBridge> = {}): AppBridge {
  return {
    session: null,
    setSession: vi.fn().mockResolvedValue(undefined),
    fetch: vi.fn(),
    ...overrides,
  };
}

const sampleAuth = {
  claimed: true,
  ssoConfigured: true,
  oidcIssuer: "https://idp.example.com",
  oidcAudience: "aud",
  oidcJwksUri: "https://idp.example.com/jwks",
  oidcDisplayName: "Acme SSO",
  oidcClientId: "cid",
  oidcClientSecretSet: true,
  jitProvisionUsers: true,
  jitDefaultRole: "StandardUser",
  allowedEmailDomains: ["example.com"],
  socialProviders: ["google"],
  passwordLoginEnabled: true,
};

describe("InstallAuthPanel", () => {
  it("shows empty state when disconnected", () => {
    render(<InstallAuthPanel bridge={bridge()} />);
    expect(screen.getByText("Install auth")).toBeTruthy();
    expect(screen.getByText(/Connect to an install/i)).toBeTruthy();
  });

  it("loads install auth settings into the form", async () => {
    const fetchFn = vi.fn().mockResolvedValue(sampleAuth);
    render(
      <InstallAuthPanel
        bridge={bridge({
          session: { baseUrl: "https://api.example", token: "t" },
          fetch: fetchFn,
        })}
      />,
    );

    expect(await screen.findByTestId("install-auth-panel")).toBeTruthy();
    await waitFor(() => expect(fetchFn).toHaveBeenCalledWith("/metadata/v1/install/auth"));
    expect(screen.getByTestId("install-auth-display-name")).toHaveProperty("value", "Acme SSO");
    expect(screen.getByTestId("install-auth-issuer")).toHaveProperty("value", "https://idp.example.com");
    expect(screen.getByTestId("install-auth-domains")).toHaveProperty("value", "example.com");
    expect((screen.getByTestId("install-auth-google") as HTMLInputElement).checked).toBe(true);
    expect((screen.getByTestId("install-auth-apple") as HTMLInputElement).checked).toBe(false);
    expect(screen.getByText("SSO configured")).toBeTruthy();
  });

  it("edits fields and saves with client secret and social toggles", async () => {
    const user = userEvent.setup();
    const fetchFn = vi
      .fn()
      .mockResolvedValueOnce(sampleAuth)
      .mockResolvedValueOnce({ ...sampleAuth, oidcDisplayName: "Updated SSO", socialProviders: ["google", "apple"] });

    render(
      <InstallAuthPanel
        bridge={bridge({
          session: { baseUrl: "https://api.example", token: "t" },
          fetch: fetchFn,
        })}
      />,
    );

    await screen.findByTestId("install-auth-panel");

    const typeInto = async (testId: string, value: string) => {
      const el = screen.getByTestId(testId);
      await user.clear(el);
      await user.type(el, value);
    };

    await typeInto("install-auth-display-name", "Updated SSO");
    await typeInto("install-auth-issuer", "https://idp2.example.com");
    await typeInto("install-auth-audience", "aud2");
    await typeInto("install-auth-client-id", "cid2");
    const jwks = screen.getByLabelText(/JWKS URI/i);
    await user.clear(jwks);
    await user.type(jwks, "https://idp2.example.com/jwks");
    await typeInto("install-auth-client-secret", "new-secret");
    await user.click(screen.getByTestId("install-auth-jit"));
    const role = screen.getByLabelText(/Default role/i);
    await user.clear(role);
    await user.type(role, "SystemAdmin");
    await typeInto("install-auth-domains", "acme.com, other.com");
    await user.click(screen.getByTestId("install-auth-password"));
    await user.click(screen.getByTestId("install-auth-apple"));
    await user.click(screen.getByTestId("install-auth-save"));

    await waitFor(() => expect(fetchFn).toHaveBeenCalledTimes(2));
    const putCall = fetchFn.mock.calls[1];
    expect(putCall[0]).toBe("/metadata/v1/install/auth");
    expect(putCall[1]).toMatchObject({ method: "PUT" });
    const body = JSON.parse(putCall[1].body as string);
    expect(body.oidcDisplayName).toBe("Updated SSO");
    expect(body.oidcIssuer).toBe("https://idp2.example.com");
    expect(body.oidcClientSecret).toBe("new-secret");
    expect(body.socialProviders).toEqual(["google", "apple"]);
    expect(body.allowedEmailDomains).toEqual(["acme.com", "other.com"]);
    expect(await screen.findByText("Saved.")).toBeTruthy();
  });

  it("sends clearOidcClientSecret when requested", async () => {
    const user = userEvent.setup();
    const fetchFn = vi.fn().mockResolvedValueOnce(sampleAuth).mockResolvedValueOnce(sampleAuth);
    render(
      <InstallAuthPanel
        bridge={bridge({
          session: { baseUrl: "https://api.example", token: "t" },
          fetch: fetchFn,
        })}
      />,
    );
    await screen.findByTestId("install-auth-panel");
    await user.click(screen.getByLabelText(/Clear stored client secret/i));
    await user.click(screen.getByTestId("install-auth-save"));
    await waitFor(() => expect(fetchFn).toHaveBeenCalledTimes(2));
    const body = JSON.parse(fetchFn.mock.calls[1][1].body as string);
    expect(body.clearOidcClientSecret).toBe(true);
    expect(body.oidcClientSecret).toBeUndefined();
  });

  it("shows load error from the API", async () => {
    const fetchFn = vi.fn().mockRejectedValue(new Error("forbidden"));
    render(
      <InstallAuthPanel
        bridge={bridge({
          session: { baseUrl: "https://api.example", token: "t" },
          fetch: fetchFn,
        })}
      />,
    );
    expect(await screen.findByText(/forbidden/i)).toBeTruthy();
  });

  it("shows save error from the API", async () => {
    const user = userEvent.setup();
    const fetchFn = vi.fn().mockResolvedValueOnce(sampleAuth).mockRejectedValueOnce(new Error("save failed"));
    render(
      <InstallAuthPanel
        bridge={bridge({
          session: { baseUrl: "https://api.example", token: "t" },
          fetch: fetchFn,
        })}
      />,
    );
    await screen.findByTestId("install-auth-panel");
    await user.click(screen.getByTestId("install-auth-save"));
    expect(await screen.findByText(/save failed/i)).toBeTruthy();
  });
});
