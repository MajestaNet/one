import { useEffect, useRef, useState } from "react";
import type { UpdateStatus } from "./api";
import { IconUpdate } from "./icons/Icons";
import { Button } from "./ui";

const FALLBACK: UpdateStatus = {
  state: "disabled",
  message: "Updates require the Electron shell with UPDATE_FEED_URL (see ADR-030).",
};

export function UpdateStatusBox() {
  const [status, setStatus] = useState<UpdateStatus>(FALLBACK);
  const [busy, setBusy] = useState(false);
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  const refresh = async () => {
    if (!window.one?.getUpdateStatus) {
      setStatus(FALLBACK);
      return;
    }
    setStatus(await window.one.getUpdateStatus());
  };

  useEffect(() => {
    void refresh();
  }, []);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  const check = async () => {
    if (!window.one?.checkForUpdates) return;
    setBusy(true);
    try {
      setStatus(await window.one.checkForUpdates());
    } finally {
      setBusy(false);
    }
  };

  const install = async () => {
    if (!window.one?.installUpdate) return;
    setBusy(true);
    try {
      const res = await window.one.installUpdate();
      if (!res.ok) {
        setStatus((s) => ({ ...s, state: "error", message: res.error ?? "Install failed" }));
      }
    } finally {
      setBusy(false);
    }
  };

  const disabled = status.state === "disabled";
  const canInstall = status.state === "downloaded";
  const hasUpdate = status.state === "downloaded" || status.state === "available" || status.state === "downloading";
  const title = disabled
    ? "Updates disabled (ADR-030)"
    : status.message + (status.version ? ` (v${status.version})` : "");

  return (
    <div className="update-box" data-testid="update-status" ref={rootRef}>
      <button
        type="button"
        className={`update-compact ${hasUpdate ? "has-update" : ""}`}
        aria-label="Application updates"
        title={title}
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
      >
        <IconUpdate size={15} />
        {hasUpdate ? <span className="update-dot" /> : null}
      </button>
      {open ? (
        <div className="update-popover" role="dialog" aria-label="Update status">
          <p className="status">
            {status.message}
            {status.version ? ` (v${status.version})` : ""}
            {typeof status.progress === "number" ? ` ${Math.round(status.progress)}%` : ""}
          </p>
          <div className="row">
            <Button variant="secondary" disabled={disabled || busy} busy={busy} onClick={() => void check()}>
              Check for updates
            </Button>
            <Button variant="primary" disabled={!canInstall || busy} busy={busy} onClick={() => void install()}>
              Restart to update
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
