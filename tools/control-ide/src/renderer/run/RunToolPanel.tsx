import { useEffect, useState } from "react";
import { Spinner } from "../ui";
import { RunToolErrorBoundary } from "./RunToolErrorBoundary";
import { ToolAutomationStatusBanner } from "./ToolAutomationStatus";
import { ToolDocumentView } from "./ToolDocumentView";
import { loadToolStore } from "./store";
import { resolveToolDocumentBindings, stripBakedRecordPayloads } from "./resolveBindings";
import { getClientTool, toolSpecToDocument, type FetchFn, type ToolSpecPayload } from "./tools";
import { useRunToolActions } from "./useRunToolActions";
import type { ToolDocument } from "./types";
import { validateToolDocument } from "./validate";
import { useAgentExcerptBridge } from "../workspace/AgentExcerptContext";

function RunToolDocumentHost({
  document,
  fetchFn,
  onAskAgent,
  onAddExcerptToChat,
  canInteract = true,
}: {
  document: ToolDocument;
  fetchFn?: FetchFn;
  onAskAgent?: (prompt: string) => void;
  onAddExcerptToChat?: (excerpt: import("../workspace/contextExcerpt").ContextExcerpt) => void;
  canInteract?: boolean;
}) {
  const {
    automationStatus,
    busyChipKey,
    dismissAutomationStatus,
    handleEnqueuePrompt,
    handleInvokeAutomation,
  } = useRunToolActions({ fetchFn, onAskAgent });

  return (
    <>
      {automationStatus ? (
        <ToolAutomationStatusBanner status={automationStatus} onDismiss={dismissAutomationStatus} />
      ) : null}
      <ToolDocumentView
        document={document}
        onEnqueuePrompt={canInteract ? (prompt) => handleEnqueuePrompt(prompt) : undefined}
        onInvokeAutomation={canInteract ? handleInvokeAutomation : undefined}
        busyChipKey={busyChipKey}
        onAddExcerptToChat={onAddExcerptToChat}
      />
    </>
  );
}

function withoutBakedRecords(doc: ToolDocument): ToolDocument {
  return { ...doc, nodes: doc.nodes.map(stripBakedRecordPayloads) };
}

/** Live ToolSpec or session working Tool panel (BP-050 + P0 AuthZ bind). */
export function RunToolPanel({
  apiName,
  label,
  description,
  fetchFn,
  sessionToolId,
  storeEpoch = 0,
  onAskAgent,
  variant = "panel",
}: {
  apiName: string;
  label: string;
  description?: string;
  fetchFn?: FetchFn;
  sessionToolId?: string | null;
  storeEpoch?: number;
  onAskAgent?: (prompt: string) => void;
  variant?: "panel" | "embedded";
}) {
  const excerptBridge = useAgentExcerptBridge();
  const [document, setDocument] = useState<ToolDocument | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [bindingWarnings, setBindingWarnings] = useState<string[]>([]);
  const [canInteract, setCanInteract] = useState(true);
  const [loading, setLoading] = useState(Boolean(fetchFn || sessionToolId));

  useEffect(() => {
    let cancelled = false;

    void (async () => {
      setLoading(true);
      setError(null);
      setBindingWarnings([]);
      setCanInteract(Boolean(sessionToolId));

      try {
        let base: ToolDocument | null = null;

        if (sessionToolId) {
          base = loadToolStore().documents.find((d) => d.id === sessionToolId) ?? null;
          if (!base) {
            if (!cancelled) {
              setError(`Session Tool not found: ${sessionToolId}`);
              setDocument(null);
            }
            return;
          }
        } else if (fetchFn && apiName) {
          const spec: ToolSpecPayload = await getClientTool(fetchFn, apiName);
          if (cancelled) return;
          setCanInteract(Boolean(spec.permissions?.canInteract));
          base = toolSpecToDocument(spec);
          if (!base) {
            setError("ToolSpec failed validation — unknown kinds or invalid layout.");
            setDocument(null);
            return;
          }
        } else {
          if (!cancelled) {
            setDocument(null);
            setError(null);
          }
          return;
        }

        if (fetchFn) {
          const { document: hydrated, errors } = await resolveToolDocumentBindings(base, fetchFn);
          if (cancelled) return;
          const validated = validateToolDocument(hydrated);
          if (!validated.ok) {
            setError("Tool failed validation after Client binding resolve.");
            setDocument(null);
            return;
          }
          setDocument(validated.document);
          setBindingWarnings(errors);
        } else {
          // No Client session: strip baked rows so metadata snapshots never look like live AuthZ.
          setDocument(withoutBakedRecords(base));
        }
      } catch (e) {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : "Failed to load ToolSpec");
        setDocument(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [apiName, fetchFn, sessionToolId, storeEpoch]);

  const panelTitle = document?.title || label || apiName;
  const panelSubtitle = sessionToolId
    ? `Session Tool · ${sessionToolId}`
    : `ToolSpec · ${apiName}`;

  return (
    <div
      className={`panel run-tool-panel tool-surface${variant === "embedded" ? " is-embedded" : ""}`}
      data-testid="run-tool-panel"
      data-tool-surface="true"
      data-tool-api-name={sessionToolId ? undefined : apiName}
      data-session-tool-id={sessionToolId ?? undefined}
    >
      {variant === "panel" ? (
        <header className="panel-header">
          <h2>{panelTitle}</h2>
          <p className="muted">{panelSubtitle}</p>
        </header>
      ) : null}
      <div className="panel-body run-tool-panel-body">
        {description ? <p className="muted">{description}</p> : null}
        {loading ? (
          <p className="muted" data-testid="run-tool-loading">
            <Spinner /> Loading Tool…
          </p>
        ) : null}
        {error ? (
          <p className="error" data-testid="run-tool-error">
            {error}
          </p>
        ) : null}
        {bindingWarnings.length > 0 ? (
          <div className="run-tool-binding-warnings" data-testid="run-tool-binding-warnings" role="status">
            {bindingWarnings.map((w) => (
              <p key={w} className="error">
                {w}
              </p>
            ))}
          </div>
        ) : null}
        {!loading && !error && document ? (
          <RunToolErrorBoundary label={panelTitle}>
            {!canInteract && !sessionToolId ? (
              <p className="muted" data-testid="run-tool-read-only">
                Read-only Tool · ask-agent and automation actions require can_interact.
              </p>
            ) : null}
            <RunToolDocumentHost
              document={document}
              fetchFn={fetchFn}
              onAskAgent={onAskAgent}
              onAddExcerptToChat={excerptBridge?.addExcerptToOpenChat}
              canInteract={canInteract}
            />
          </RunToolErrorBoundary>
        ) : null}
        {!loading && !error && !document && !fetchFn && !sessionToolId ? (
          <p className="muted">Connect with Metadata + Client scope to load this Tool.</p>
        ) : null}
      </div>
    </div>
  );
}
