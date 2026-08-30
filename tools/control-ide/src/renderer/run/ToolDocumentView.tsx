import { useState } from "react";
import { ToolNodeView } from "./ToolNodeView";
import { ToolSpatialView } from "./spatial/ToolSpatialView";
import type { ActionChip, ToolDocument, ToolNode } from "./types";

function nodesById(doc: ToolDocument): Map<string, ToolNode> {
  return new Map(doc.nodes.map((n) => [n.id, n]));
}

export function ToolDocumentView({
  document,
  onEnqueuePrompt,
  onInvokeAutomation,
  busyChipKey,
  onAddExcerptToChat,
}: {
  document: ToolDocument;
  onEnqueuePrompt?: (prompt: string, node: ToolNode) => void;
  onInvokeAutomation?: (chip: ActionChip, node: ToolNode) => void;
  busyChipKey?: string | null;
  onAddExcerptToChat?: (excerpt: import("../workspace/contextExcerpt").ContextExcerpt) => void;
}) {
  const [selectedNodeIds, setSelectedNodeIds] = useState<string[]>([]);
  const map = nodesById(document);
  const sections = document.layout.sections;
  const orderedIds =
    sections && sections.length > 0
      ? sections.flatMap((s) => s.nodeIds)
      : document.nodes.map((n) => n.id);
  const missing = orderedIds.filter((id) => !map.has(id));
  const spatial = document.layout.mode === "spatial";
  const selected = new Set(selectedNodeIds);

  const select = (nodeId: string) => setSelectedNodeIds([nodeId]);

  return (
    <div className="canvas-document" data-testid="canvas-document" data-canvas-id={document.id}>
      <header className="canvas-document-header">
        <h2 className="canvas-document-title">{document.title}</h2>
        <p className="muted mono canvas-document-meta">
          {document.apiVersion}
          {spatial ? " · spatial" : " · sections"}
        </p>
      </header>

      {spatial ? (
        <ToolSpatialView
          document={document}
          selectedNodeIds={selectedNodeIds}
          onSelectNode={select}
          onEnqueuePrompt={onEnqueuePrompt}
          onInvokeAutomation={onInvokeAutomation}
          busyChipKey={busyChipKey}
          onAddExcerptToChat={onAddExcerptToChat}
        />
      ) : sections && sections.length > 0 ? (
        sections.map((section) => (
          <section key={section.id} className="canvas-section" data-testid={`canvas-section-${section.id}`}>
            {section.title ? <h3 className="canvas-section-title">{section.title}</h3> : null}
            <div className="canvas-section-nodes">
              {section.nodeIds.map((id) => {
                const node = map.get(id);
                if (!node) {
                  return (
                    <p key={id} className="muted">
                      Missing node {id}
                    </p>
                  );
                }
                return (
                  <ToolNodeView
                    key={id}
                    node={node}
                    selected={selected.has(id)}
                    onSelect={() => select(id)}
                    onEnqueuePrompt={onEnqueuePrompt}
                    onInvokeAutomation={onInvokeAutomation}
                    busyChipKey={busyChipKey}
                    onAddExcerptToChat={onAddExcerptToChat}
                  />
                );
              })}
            </div>
          </section>
        ))
      ) : (
        <div className="canvas-section-nodes">
          {document.nodes.map((node) => (
            <ToolNodeView
              key={node.id}
              node={node}
              selected={selected.has(node.id)}
              onSelect={() => select(node.id)}
              onEnqueuePrompt={onEnqueuePrompt}
              onInvokeAutomation={onInvokeAutomation}
              busyChipKey={busyChipKey}
              onAddExcerptToChat={onAddExcerptToChat}
            />
          ))}
        </div>
      )}

      {missing.length > 0 ? (
        <p className="muted" data-testid="canvas-missing-nodes">
          Layout references missing nodes: {missing.join(", ")}
        </p>
      ) : null}
    </div>
  );
}
