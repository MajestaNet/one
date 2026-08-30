import { useEffect, useRef, useState } from "react";
import type { Session } from "../session";
import { envDisplayName } from "../session";

export function EnvSwitcher({
  session,
  onSwitch,
  onAddEnvironment,
}: {
  session: Session | null;
  onSwitch: (installId: string) => void;
  onAddEnvironment: () => void;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  if (!session?.environments?.length) {
    return (
      <button
        type="button"
        className="env-switcher env-switcher-empty"
        data-testid="env-switcher"
        onClick={onAddEnvironment}
        title="Connect an environment"
      >
        No env
      </button>
    );
  }

  const active =
    session.environments.find((e) => e.installId === session.activeInstallId) ??
    session.environments[0];

  return (
    <div className="env-switcher-wrap" ref={rootRef} data-testid="env-switcher">
      <button
        type="button"
        className="env-switcher"
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        title={`${envDisplayName(active)} · ${active.baseUrl}`}
      >
        <span className="env-switcher-role">{envDisplayName(active)}</span>
        {active.compatStatus && active.compatStatus !== "ok" ? (
          <span className="env-switcher-compat muted" data-testid="env-compat-badge">
            compat:{active.compatStatus}
          </span>
        ) : null}
        <span className="env-switcher-id muted">{active.installId}</span>
        <span className="env-switcher-caret" aria-hidden>
          ▾
        </span>
      </button>
      {open ? (
        <ul className="env-switcher-menu" role="listbox" data-testid="env-switcher-menu">
          {session.environments.map((env) => {
            const connected = Boolean(env.token);
            const isActive = env.installId === session.activeInstallId;
            return (
              <li key={env.installId} role="option" aria-selected={isActive}>
                <button
                  type="button"
                  className={`env-switcher-item ${isActive ? "active" : ""} ${connected ? "" : "disconnected"}`}
                  disabled={!connected}
                  title={
                    connected
                      ? env.baseUrl
                      : "Connect credentials for this peer first (Govern → Connect)"
                  }
                  onClick={() => {
                    if (!connected) return;
                    onSwitch(env.installId);
                    setOpen(false);
                  }}
                >
                  <span className="env-switcher-item-role">{envDisplayName(env)}</span>
                  <span className="env-switcher-item-meta muted">
                    {env.installId}
                    {!connected ? " · needs connect" : ""}
                  </span>
                </button>
              </li>
            );
          })}
          <li>
            <button
              type="button"
              className="env-switcher-item env-switcher-add"
              data-testid="env-switcher-add"
              onClick={() => {
                setOpen(false);
                onAddEnvironment();
              }}
            >
              Add environment…
            </button>
          </li>
        </ul>
      ) : null}
    </div>
  );
}
