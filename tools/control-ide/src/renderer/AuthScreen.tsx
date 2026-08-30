import type { AppBridge } from "./App";
import { ConnectSection } from "./govern/ConnectSection";

/**
 * Full-shell sign-in when no Majesta One JWT is present.
 * Reuses ConnectSection so every auth path (claim, browser SSO, client credentials, JWT) is available.
 */
export function AuthScreen({
  bridge,
  prefillBaseUrl,
  onPrefillConsumed,
}: {
  bridge: AppBridge;
  prefillBaseUrl?: string;
  onPrefillConsumed?: () => void;
}) {
  return (
    <section className="auth-screen" data-testid="auth-screen" aria-label="Sign in to Majesta One">
      <div className="auth-screen-inner">
        <div className="auth-screen-intro">
          <p className="mode-launcher-brand">
            Majesta One <span>Control</span>
          </p>
          <h1>Sign in</h1>
          <p className="muted">
            Connect to an install to unlock Operate, Build, Govern, and Settings. AuthZ stays
            on the server — your session is encrypted on this device.
          </p>
        </div>
        <div className="auth-screen-panel">
          <ConnectSection
            bridge={bridge}
            prefillBaseUrl={prefillBaseUrl}
            onPrefillConsumed={onPrefillConsumed}
          />
        </div>
      </div>
    </section>
  );
}
