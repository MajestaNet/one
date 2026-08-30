import Markdown from "react-markdown";

/** Markdown note — raw HTML disabled (ADR-021). */
export function MarkdownNoteNode({ text, title }: { text: string; title?: string }) {
  return (
    <div className="run-tool-markdown" data-testid="run-tool-markdown">
      {title ? <h4 className="canvas-node-title">{title}</h4> : null}
      <div className="canvas-note-text">
        <Markdown skipHtml disallowedElements={["script", "iframe", "object", "embed"]}>
          {text}
        </Markdown>
      </div>
    </div>
  );
}
