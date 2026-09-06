import { useCallback, useEffect, useState } from "react";
import type { AppBridge } from "../App";
import { Button, EmptyState, PanelHeader, ToolSurface } from "../ui";
import { IconSettings } from "../icons/Icons";

type McpTool = {
  name?: string;
  description?: string;
  title?: string;
};

export function mcpUrl(baseUrl: string): string {
  return `${baseUrl.replace(/\/$/, "")}/mcp`;
}

export function McpCatalogPanel({ bridge }: { bridge: AppBridge }) {
  const connected = Boolean(bridge.session?.baseUrl && bridge.session?.token);
  const [tools, setTools] = useState<McpTool[]>([]);
  const [err, setErr] = useState("");
  const [copied, setCopied] = useState(false);
  const [busy, setBusy] = useState(false);
  const url = bridge.session?.baseUrl ? mcpUrl(bridge.session.baseUrl) : "";

  const load = useCallback(async () => {
    if (!connected) return;
    setErr("");
    setBusy(true);
    try {
      const row = (await bridge.fetch("/mcp/tools")) as { tools?: McpTool[] } | McpTool[];
      setTools(Array.isArray(row) ? row : (row.tools ?? []));
    } catch (e) {
      setErr(String(e));
      setTools([]);
    } finally {
      setBusy(false);
    }
  }, [bridge, connected]);

  useEffect(() => {
    void load();
  }, [load]);

  const copyUrl = async () => {
    if (!url) return;
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
    } catch {
      setCopied(false);
      setErr("Clipboard unavailable — copy the POST /mcp URL from the field.");
    }
  };

  if (!connected) {
    return (
      <EmptyState
        icon={<IconSettings size={28} />}
        title="Builder connect"
        description="Connect to an install to copy the MCP endpoint. Control IDE does not host MCP sessions (ADR-030)."
      />
    );
  }

  return (
    <ToolSurface testId="mcp-catalog-panel">
      <PanelHeader
        title="Builder connect"
        subtitle="Read-only MCP catalog for this install. Point Cursor or another MCP client at POST /mcp — do not host MCP inside Electron."
        actions={
          <Button variant="secondary" busy={busy} onClick={() => void load()}>
            Refresh
          </Button>
        }
      />
      {err ? <p className="panel-error">{err}</p> : null}
      <label>
        MCP URL
        <input readOnly value={url} data-testid="mcp-url" />
      </label>
      <div className="row">
        <Button variant="primary" onClick={() => void copyUrl()} data-testid="mcp-copy">
          Copy POST /mcp URL
        </Button>
        {copied ? <span className="muted">Copied</span> : null}
      </div>
      {tools.length === 0 && !busy && !err ? (
        <EmptyState title="No tools listed" description="GET /mcp/tools returned an empty catalog on this install." />
      ) : (
        <table className="data-table" data-testid="mcp-tools-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Description</th>
            </tr>
          </thead>
          <tbody>
            {tools.map((t) => (
              <tr key={t.name || t.title}>
                <td className="mono">{t.name || t.title || "—"}</td>
                <td>{t.description || "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </ToolSurface>
  );
}
