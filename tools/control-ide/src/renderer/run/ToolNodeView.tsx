import type { ReactNode } from "react";
import { Tooltip } from "@base-ui/react/tooltip";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import { Button } from "../ui";
import { KeyValueList } from "../ui/KeyValueList";
import { MarkdownNoteNode } from "./nodes/MarkdownNoteNode";
import { PipelineLaneNode } from "./nodes/PipelineLaneNode";
import { RecordTableNode } from "./nodes/RecordTableNode";
import { StatNode } from "./nodes/StatNode";
import {
  automationApiNameFromChip,
  isAutomationRunChip,
  operationsFromMutationNode,
  parseActionChips,
  type ActionChip,
  type ToolNode,
} from "./types";

function asString(value: unknown, fallback = ""): string {
  if (value == null) return fallback;
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return fallback;
}

function asRows(raw: unknown): Record<string, unknown>[] {
  if (!Array.isArray(raw)) return [];
  return raw.filter((r): r is Record<string, unknown> => !!r && typeof r === "object" && !Array.isArray(r));
}

const reviewSchema = z.object({
  note: z.string().max(500).optional(),
});

type ReviewForm = z.infer<typeof reviewSchema>;

export function ToolNodeView({
  node,
  selected,
  onSelect,
  onEnqueuePrompt,
  onInvokeAutomation,
  busyChipKey,
  onAddExcerptToChat,
  objectApiName,
}: {
  node: ToolNode;
  selected?: boolean;
  onSelect?: () => void;
  onEnqueuePrompt?: (prompt: string, node: ToolNode) => void;
  onInvokeAutomation?: (chip: ActionChip, node: ToolNode) => void;
  busyChipKey?: string | null;
  onAddExcerptToChat?: (excerpt: import("../workspace/contextExcerpt").ContextExcerpt) => void;
  objectApiName?: string;
}) {
  const shellClass = ["canvas-node", selected ? "is-selected" : ""].filter(Boolean).join(" ");

  const wrap = (body: ReactNode, extraClass = "") => (
    <article
      className={`${shellClass} ${extraClass}`.trim()}
      data-testid={`canvas-node-${node.id}`}
      data-kind={node.kind}
      onClick={(e) => {
        e.stopPropagation();
        onSelect?.();
      }}
    >
      {body}
    </article>
  );

  switch (node.kind) {
    case "sectionHeader":
      return wrap(
        <>
          <h3 className="canvas-node-title">{node.title ?? asString(node.props.subtitle, "Section")}</h3>
          {typeof node.props.subtitle === "string" && node.title ? (
            <p className="muted canvas-node-subtitle">{node.props.subtitle}</p>
          ) : null}
        </>,
        "canvas-section-header",
      );
    case "stat":
      return wrap(
        <StatNode
          id={node.id}
          label={asString(node.props.label, node.title ?? "Stat")}
          value={node.props.value}
          series={node.props.series}
        />,
        "canvas-stat",
      );
    case "markdownNote":
      return wrap(
        <MarkdownNoteNode title={node.title} text={asString(node.props.text)} />,
        "canvas-note",
      );
    case "recordTable":
      return wrap(
        <>
          {node.title ? <h4 className="canvas-node-title">{node.title}</h4> : null}
          <RecordTableNode
            columns={node.props.columns}
            rows={node.props.rows}
            emptyLabel="No rows in Tool table"
            testId={`run-table-${node.id}`}
            selectable={Boolean(onAddExcerptToChat)}
            objectApiName={
              objectApiName ??
              (typeof node.props.objectApiName === "string" ? node.props.objectApiName : undefined)
            }
            onAddExcerptToChat={onAddExcerptToChat}
          />
        </>,
        "canvas-table",
      );
    case "recordCard": {
      const fields =
        node.props.fields && typeof node.props.fields === "object" && !Array.isArray(node.props.fields)
          ? (node.props.fields as Record<string, unknown>)
          : {};
      const items = Object.entries(fields).map(([label, value]) => ({
        label,
        value: value == null ? "—" : typeof value === "object" ? JSON.stringify(value) : String(value),
      }));
      return wrap(
        <>
          <header className="canvas-node-header">
            <h4 className="canvas-node-title">{node.title ?? asString(node.props.recordId, "Record")}</h4>
            <p className="muted mono">
              {asString(node.props.objectApiName, "Object")} · {asString(node.props.recordId, "—")}
            </p>
          </header>
          <KeyValueList items={items} />
        </>,
        "canvas-record-card",
      );
    }
    case "queryResult": {
      const ids = Array.isArray(node.props.recordIds)
        ? node.props.recordIds.filter((id): id is string => typeof id === "string")
        : [];
      return wrap(
        <>
          <header className="canvas-node-header">
            <h4 className="canvas-node-title">{node.title ?? "Query result"}</h4>
            <p className="muted">{asString(node.props.objectApiName, "Object")}</p>
          </header>
          {ids.length === 0 ? (
            <p className="muted">No record Ids</p>
          ) : (
            <ul className="canvas-id-list">
              {ids.map((id) => (
                <li key={id} className="mono">
                  {id}
                </li>
              ))}
            </ul>
          )}
        </>,
        "canvas-query-result",
      );
    }
    case "relatedList":
      return wrap(
        <>
          <header className="canvas-node-header">
            <h4 className="canvas-node-title">{node.title ?? "Related records"}</h4>
            <p className="muted">
              {asString(node.props.objectApiName, "Object")} · {asString(node.props.relationship, "lookup")}
            </p>
          </header>
          <RecordTableNode
            columns={
              node.props.columns ?? [
                { key: "Name", label: "Name" },
                { key: "Id", label: "Id", mono: true },
              ]
            }
            rows={node.props.records}
            emptyLabel="No related records"
            selectable={Boolean(onAddExcerptToChat)}
            objectApiName={
              typeof node.props.objectApiName === "string" ? node.props.objectApiName : undefined
            }
            onAddExcerptToChat={onAddExcerptToChat}
          />
        </>,
        "canvas-related-list",
      );
    case "mutationProposal":
      return wrap(<MutationProposalBody node={node} />, "canvas-mutation-proposal");
    case "pipelineLane":
      return wrap(
        <PipelineLaneNode
          title={node.title}
          stage={typeof node.props.stage === "string" ? node.props.stage : undefined}
          cards={node.props.cards}
        />,
        "canvas-pipeline-lane",
      );
    case "actionChipGroup": {
      const actions = parseActionChips(node.props.actions);
      return wrap(
        <>
          {node.title ? <h4 className="canvas-node-title">{node.title}</h4> : null}
          <Tooltip.Provider>
            <div className="canvas-action-chips">
              {actions.length === 0 ? (
                <p className="muted">No actions</p>
              ) : (
                actions.map((chip) => {
                  const key = `${node.id}:${chip.label}`;
                  const automation = isAutomationRunChip(chip);
                  const apiName = automationApiNameFromChip(chip);
                  const tooltip = automation
                    ? apiName
                      ? `automationRun · ${apiName}`
                      : "automationRun (missing apiName)"
                    : chip.prompt || chip.type || chip.label;
                  return (
                  <Tooltip.Root key={chip.label}>
                    <Tooltip.Trigger
                      render={
                        <Button
                          type="button"
                          variant="secondary"
                          className="canvas-action-chip"
                          data-testid={`canvas-action-chip-${node.id}-${chip.label}`}
                          data-action-type={automation ? "automationRun" : chip.prompt ? "prompt" : "action"}
                          busy={busyChipKey === key}
                          disabled={busyChipKey === key}
                          onClick={(e) => {
                            e.stopPropagation();
                            if (automation) {
                              onInvokeAutomation?.(chip, node);
                              return;
                            }
                            if (chip.prompt) onEnqueuePrompt?.(chip.prompt, node);
                          }}
                        >
                          {chip.label}
                        </Button>
                      }
                    />
                    <Tooltip.Portal>
                      <Tooltip.Positioner sideOffset={6}>
                        <Tooltip.Popup className="run-tool-tooltip">
                          {tooltip}
                        </Tooltip.Popup>
                      </Tooltip.Positioner>
                    </Tooltip.Portal>
                  </Tooltip.Root>
                  );
                })
              )}
            </div>
          </Tooltip.Provider>
        </>,
        "canvas-action-chips-node",
      );
    }
    case "messageThread": {
      const messages = asRows(node.props.messages);
      return wrap(
        <>
          <header className="canvas-node-header">
            <h4 className="canvas-node-title">{node.title ?? "Messages"}</h4>
          </header>
          {messages.length === 0 ? (
            <p className="muted">No messages</p>
          ) : (
            <ul className="canvas-message-thread">
              {messages.map((m, i) => (
                <li key={i}>
                  <span className="muted">{asString(m.author, "user")}</span>: {asString(m.body, "")}
                </li>
              ))}
            </ul>
          )}
        </>,
        "canvas-message-thread-node",
      );
    }
    default:
      return null;
  }
}

function MutationProposalBody({ node }: { node: ToolNode }) {
  const ops = operationsFromMutationNode(node);
  const status = asString(node.props.status, "pending");
  const form = useForm<ReviewForm>({
    resolver: zodResolver(reviewSchema),
    defaultValues: { note: "" },
  });

  return (
    <>
      <header className="canvas-node-header">
        <h4 className="canvas-node-title">{node.title ?? "Staged mutations"}</h4>
        <p className="muted" data-testid={`canvas-mutation-status-${node.id}`}>
          {status}
        </p>
      </header>
      {ops.length === 0 ? (
        <p className="muted">No operations staged</p>
      ) : (
        <ul className="canvas-mutation-list">
          {ops.map((op, i) => (
            <li key={i} className="mono">
              {op.op} {op.object} {op.id ?? ""}
            </li>
          ))}
        </ul>
      )}
      <form
        className="run-tool-mutation-form"
        data-testid={`run-mutation-form-${node.id}`}
        onSubmit={form.handleSubmit(() => {
          /* Apply lands in a later phase — form validates note via RHF+Zod. */
        })}
        onClick={(e) => e.stopPropagation()}
      >
        <label className="muted">
          Review note
          <input type="text" className="run-tool-input" {...form.register("note")} />
        </label>
        <Button type="submit" variant="secondary">
          Validate note
        </Button>
      </form>
    </>
  );
}
