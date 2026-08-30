import { useEffect, useState } from "react";
import type { AppBridge } from "../App";
import { assertApiBaseUrl } from "../installUrl";
import { Button, EmptyState, PanelHeader, ToolSurface } from "../ui";
import { IconRepo } from "../icons/Icons";

export function RepoPanel({
  bridge,
}: {
  bridge: AppBridge;
  /** @deprecated Repo no longer opens dirty files inline. */
  onOpenRepoFile?: (rel: string) => void;
  /** @deprecated Prefer onOpenRepoFile */
  onOpenMetadataFile?: (rel: string) => void;
}) {
  const [repoPath, setRepoPath] = useState(bridge.session?.repoPath ?? "");
  const [orgName, setOrgName] = useState("");
  const [err, setErr] = useState("");
  const [info, setInfo] = useState("");
  const [busy, setBusy] = useState(false);

  const effectivePath = repoPath.trim() || bridge.session?.repoPath || "";

  useEffect(() => {
    setRepoPath(bridge.session?.repoPath ?? "");
  }, [bridge.session?.repoPath]);

  useEffect(() => {
    if (!bridge.session?.token || !bridge.fetch) return;
    void (async () => {
      try {
        const env = (await bridge.fetch("/deploy/v1/environment")) as {
          customerId?: string;
          customerRepoUrl?: string;
        };
        if (env?.customerId) setOrgName(env.customerId);
        if (env?.customerRepoUrl && bridge.session) {
          await bridge.setSession({ ...bridge.session, customerRepoUrl: env.customerRepoUrl });
        }
      } catch {
        /* non-fatal */
      }
    })();
  }, [bridge]);

  const persistPath = async (path: string, options: { register?: boolean } = {}) => {
    if (!bridge.session) {
      setErr("Connect first");
      return false;
    }
    if (options.register !== false && window.one?.registerRepoRoot && path) {
      const res = await window.one.registerRepoRoot(path);
      if (res.ok) {
        await bridge.setSession({ ...bridge.session, repoPath: res.path || path });
        setRepoPath(res.path || path);
        return true;
      }
      // Empty / not-yet-a-repo folders are allowed as session destinations.
      if (!/Not a Majesta One customer repo/i.test(res.error ?? "")) {
        setErr(res.error ?? "Repo path rejected");
        return false;
      }
    }
    await bridge.setSession({ ...bridge.session, repoPath: path });
    setRepoPath(path);
    return true;
  };

  const chooseFolder = async () => {
    setErr("");
    setInfo("");
    if (!bridge.session) {
      setErr("Connect first");
      return;
    }
    const chooser = window.one?.chooseLocalFolder ?? window.one?.chooseRepoRoot;
    if (!chooser) {
      setErr("Choosing a folder requires the Electron shell");
      return;
    }
    setBusy(true);
    try {
      const res = await chooser();
      if (res.canceled) return;
      if (!res.ok || !res.path) throw new Error(res.error ?? "Could not use that folder");
      // chooseLocalFolder already registered when markers exist; skip re-register.
      await persistPath(res.path, { register: false });
      setInfo(`Local folder: ${res.path}`);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const initializeRemote = async () => {
    setErr("");
    setInfo("");
    if (!bridge.session) {
      setErr("Connect to an install (admin + deploy) first");
      return;
    }
    if (!effectivePath) {
      setErr("Choose a local folder before Initialize");
      return;
    }
    setBusy(true);
    try {
      const res = (await bridge.fetch("/deploy/v1/packages/initialize-repo", {
        method: "POST",
        body: JSON.stringify({ confirm: true, force: false }),
      })) as { customerRepoUrl?: string; commitSha?: string };
      const nextUrl = res.customerRepoUrl || bridge.session.customerRepoUrl;
      if (nextUrl) {
        await bridge.setSession({
          ...bridge.session,
          repoPath: effectivePath,
          customerRepoUrl: nextUrl,
        });
      }
      // Prefer cloning the seeded remote into the chosen folder.
      if (nextUrl && window.one?.gitClone) {
        const clone = await window.one.gitClone(nextUrl, effectivePath);
        if (clone.ok) {
          const path = clone.path || effectivePath;
          await persistPath(path, { register: false });
          setInfo(
            `Remote initialized${res.commitSha ? ` @ ${res.commitSha.slice(0, 8)}` : ""} and cloned to ${path}`,
          );
          return;
        }
        // Folder may already have a clone — fall through to message.
        if (!/not empty|already exists|Destination not empty/i.test(clone.error ?? "")) {
          setErr(clone.error ?? "Clone after initialize failed");
          return;
        }
      }
      setInfo(
        `Remote initialized${res.commitSha ? ` @ ${res.commitSha.slice(0, 8)}` : ""}${
          nextUrl ? ` → ${nextUrl}` : ""
        }. Use Pull from org if the folder already has content.`,
      );
    } catch (e) {
      const msg = String(e);
      if (/REPO_ALREADY_INITIALIZED|already initialized/i.test(msg)) {
        setErr("");
        try {
          const env = (await bridge.fetch("/deploy/v1/environment")) as {
            customerId?: string;
            customerRepoUrl?: string;
          };
          if (env?.customerId) setOrgName(env.customerId);
          if (env?.customerRepoUrl) {
            await bridge.setSession({
              ...bridge.session,
              repoPath: effectivePath,
              customerRepoUrl: env.customerRepoUrl,
            });
          }
        } catch {
          /* ignore */
        }
        setInfo("Remote already initialized — Pull from org or Sync from Git remote.");
      } else {
        setErr(msg);
      }
    } finally {
      setBusy(false);
    }
  };

  const syncFromGit = async () => {
    setErr("");
    setInfo("");
    const url = bridge.session?.customerRepoUrl;
    if (!effectivePath) {
      setErr("Choose a local folder first");
      return;
    }
    if (!window.one) {
      setErr("Sync requires the Electron shell");
      return;
    }
    setBusy(true);
    try {
      if (window.one.registerRepoRoot && window.one.gitPull && window.one.gitStatus) {
        const reg = await window.one.registerRepoRoot(effectivePath);
        if (reg.ok) {
          const st = await window.one.gitStatus(effectivePath);
          if (st.ok) {
            const pull = await window.one.gitPull(effectivePath);
            if (!pull.ok) throw new Error(pull.error ?? "pull failed");
            await persistPath(effectivePath, { register: false });
            setInfo(`Pulled origin/${pull.branch || "main"} into ${effectivePath}`);
            return;
          }
        }
      }
      if (!url) {
        setErr("No customer repo URL — Initialize remote first");
        return;
      }
      if (!window.one.gitClone) {
        setErr("Clone requires the Electron shell");
        return;
      }
      const res = await window.one.gitClone(url, effectivePath);
      if (!res.ok) throw new Error(res.error ?? "clone failed");
      const path = res.path || effectivePath;
      await persistPath(path, { register: false });
      setInfo(`Cloned to ${path}`);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const pullFromOrg = async () => {
    setErr("");
    setInfo("");
    if (!bridge.session) {
      setErr("Connect first");
      return;
    }
    if (!effectivePath) {
      setErr("Choose a local folder first");
      return;
    }
    if (!window.one?.importExportZip) {
      setErr("Pull from org requires the Electron shell");
      return;
    }
    if (window.one.gitStatus) {
      // Dirty guard only when already a registered git repo.
      const st = await window.one.gitStatus(effectivePath);
      if (st.ok && (st.status || "").trim()) {
        setErr("Working tree dirty — commit or stash in your editor before Pull from org.");
        return;
      }
    }
    setBusy(true);
    try {
      const base = assertApiBaseUrl(bridge.session.baseUrl, { allowInsecureHttp: true });
      const res = await fetch(`${base}/deploy/v1/packages/export`, {
        headers: { Authorization: `Bearer ${bridge.session.token}` },
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `export failed (${res.status})`);
      }
      const buf = await res.arrayBuffer();
      const bytes = new Uint8Array(buf);
      let binary = "";
      for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]!);
      const base64 = btoa(binary);
      const imported = await window.one.importExportZip(effectivePath, base64);
      if (!imported.ok) throw new Error(imported.error ?? "import failed");
      const path = imported.path || effectivePath;
      await persistPath(path, { register: false });
      const label = orgName || "org";
      setInfo(`Pulled from org ${label} into ${path}`);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const openEditor = async () => {
    setErr("");
    setInfo("");
    if (!effectivePath) {
      setErr("Choose a local folder first");
      return;
    }
    if (!window.one?.openInEditor) {
      setErr("Open in editor requires the Electron shell");
      return;
    }
    setBusy(true);
    try {
      // Ensure registered when possible so editor IPC accepts the path.
      if (window.one.registerRepoRoot) {
        await window.one.registerRepoRoot(effectivePath);
      }
      const res = await window.one.openInEditor(effectivePath, "auto");
      if (!res.ok) throw new Error(res.error ?? "open failed");
      setInfo("Opened in editor");
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const newChange = async () => {
    setErr("");
    setInfo("");
    if (!effectivePath) {
      setErr("Choose a local folder first");
      return;
    }
    if (!window.one?.gitCreateBranch || !window.one?.writeText) {
      setErr("New change requires the Electron shell");
      return;
    }
    const raw = window.prompt("Change slug (becomes change/<slug>)", "my-change");
    if (raw == null) return;
    const slug = raw.trim().replace(/^change\//, "");
    if (!/^[a-z0-9][a-z0-9._-]{0,62}$/i.test(slug)) {
      setErr("Slug must be alphanumeric (dots/underscores/hyphens ok)");
      return;
    }
    setBusy(true);
    try {
      if (window.one.registerRepoRoot) {
        await window.one.registerRepoRoot(effectivePath);
      }
      const branch = `change/${slug}`;
      const br = await window.one.gitCreateBranch(effectivePath, branch);
      if (!br.ok) throw new Error(br.error ?? "branch failed");
      const yaml = [
        `title: "${slug}"`,
        "risk: low",
        "targetEnvs:",
        "  - staging",
        "  - prod",
        `summary: "Describe the metadata change"`,
        "",
      ].join("\n");
      await window.one.writeText(effectivePath, `changes/${slug}/CHANGE.yaml`, yaml);
      setInfo(`Created ${branch} and changes/${slug}/CHANGE.yaml — edit in your editor, then Ship → Validate vs org.`);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  if (!bridge.session?.token) {
    return (
      <ToolSurface testId="repo-panel">
        <PanelHeader title="Repo" subtitle="Bind a local customer repo folder to the active install." />
        <EmptyState
          icon={<IconRepo size={28} />}
          title="Connect an environment"
          description="Repo initialize and pull-from-org need a connected install with deploy scope."
        />
      </ToolSurface>
    );
  }

  const orgLabel = orgName || "org";

  return (
    <ToolSurface testId="repo-panel">
      <PanelHeader
        title="Repo"
        subtitle="Choose a local folder, initialize the remote from this install, then pull metadata from the org. Edit and commit in your editor."
      />

      <section className="govern-section" data-testid="repo-section-path">
        <h3 className="section-title">Local folder</h3>
        <p className="muted">
          Required before Initialize. Pick an empty folder for a new clone, or an existing Majesta One
          customer repo.
        </p>
        <div className="row" style={{ alignItems: "center" }}>
          <code data-testid="repo-path-display" className="repo-path-display">
            {effectivePath || "No folder selected"}
          </code>
          <Button variant="primary" busy={busy} onClick={() => void chooseFolder()} data-testid="repo-browse">
            Choose folder…
          </Button>
        </div>
        {bridge.session.customerRepoUrl ? (
          <p className="muted" data-testid="repo-clone-hint">
            Git remote: <code>{bridge.session.customerRepoUrl}</code>
          </p>
        ) : null}
      </section>

      <section className="govern-section" data-testid="repo-section-init">
        <h3 className="section-title">Initialize & sync</h3>
        <p className="muted">
          Initialize seeds the customer Git <code>main</code> from this install (admin + deploy). Pull
          from org overlays the connected install export into your local folder (repo → org workflow).
        </p>
        <div className="row">
          <Button
            variant="primary"
            busy={busy}
            disabled={!effectivePath}
            onClick={() => void initializeRemote()}
            data-testid="repo-initialize-remote"
          >
            Initialize remote
          </Button>
          <Button
            variant="secondary"
            busy={busy}
            disabled={!effectivePath}
            onClick={() => void pullFromOrg()}
            data-testid="repo-pull-org"
          >
            Pull from org {orgLabel}
          </Button>
          <Button
            variant="ghost"
            busy={busy}
            disabled={!effectivePath}
            onClick={() => void syncFromGit()}
            data-testid="repo-sync"
          >
            Sync from Git remote
          </Button>
        </div>
      </section>

      <section className="govern-section" data-testid="repo-section-editor">
        <h3 className="section-title">Edit in your tools</h3>
        <div className="row">
          <Button
            variant="secondary"
            busy={busy}
            disabled={!effectivePath}
            onClick={() => void newChange()}
            data-testid="repo-new-change"
          >
            New change…
          </Button>
          <Button
            variant="secondary"
            busy={busy}
            disabled={!effectivePath}
            onClick={() => void openEditor()}
            data-testid="repo-open-editor"
          >
            Open folder in editor
          </Button>
        </div>
        <p className="muted" data-testid="repo-editor-note">
          New change creates <code>change/&lt;slug&gt;</code> plus <code>changes/&lt;slug&gt;/CHANGE.yaml</code>.
          Commit and push from your editor, then Validate vs org in Ship.
        </p>
      </section>

      {info && <p className="muted" data-testid="repo-info">{info}</p>}
      {err && <p className="err">{err}</p>}
    </ToolSurface>
  );
}
