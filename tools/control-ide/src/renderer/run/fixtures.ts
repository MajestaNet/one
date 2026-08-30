import type { ToolDocument } from "./types";
import { TOOL_DOCUMENT_API_VERSION } from "./types";

/** Hand-authored demo for offline Run smoke (sections + table). */
export function demoAccountsTool(): ToolDocument {
  return {
    apiVersion: TOOL_DOCUMENT_API_VERSION,
    id: "demo-top-accounts",
    title: "Top Accounts (demo)",
    layout: {
      mode: "sections",
      sections: [
        { id: "summary", title: "Summary", nodeIds: ["hdr", "stat-open", "note"] },
        { id: "records", title: "Records", nodeIds: ["table", "card-acme", "query"] },
      ],
    },
    dataBindings: [{ id: "bind-accounts", objectApiName: "Account", query: { limit: 10 } }],
    nodes: [
      {
        id: "hdr",
        kind: "sectionHeader",
        title: "Run Tool",
        props: { subtitle: "Phase 3 fixture — allowlisted node kinds only" },
      },
      {
        id: "stat-open",
        kind: "stat",
        title: "Open accounts",
        bindingId: "bind-accounts",
        props: { value: 3, label: "Accounts in view" },
      },
      {
        id: "note",
        kind: "markdownNote",
        props: {
          text: "Hand-authored **ToolDocument**. Unknown kinds never render.",
        },
      },
      {
        id: "table",
        kind: "recordTable",
        title: "Accounts",
        bindingId: "bind-accounts",
        props: {
          columns: [
            { key: "Name", label: "Name" },
            { key: "Id", label: "Id", mono: true },
          ],
          rows: [
            { Name: "Acme Corp", Id: "acc-1" },
            { Name: "Globex", Id: "acc-2" },
            { Name: "Initech", Id: "acc-3" },
          ],
        },
      },
      {
        id: "card-acme",
        kind: "recordCard",
        title: "Acme Corp",
        props: {
          objectApiName: "Account",
          recordId: "acc-1",
          fields: {
            Name: "Acme Corp",
            Industry: "Manufacturing",
            Website: "https://acme.example",
          },
        },
      },
      {
        id: "query",
        kind: "queryResult",
        title: "Ranked from last run",
        props: {
          objectApiName: "Account",
          recordIds: ["acc-1", "acc-2", "acc-3"],
        },
      },
    ],
    meta: { updatedAt: new Date(0).toISOString() },
  };
}

/** Spatial pipeline demo (React Flow shell + pipeline lanes). */
export function demoPipelineTool(): ToolDocument {
  return {
    apiVersion: TOOL_DOCUMENT_API_VERSION,
    id: "demo-pipeline-spatial",
    title: "Open Opportunities (spatial demo)",
    layout: {
      mode: "spatial",
      positions: {
        hdr: { x: 20, y: 20, w: 640, h: 60 },
        "lane-prospect": { x: 20, y: 100, w: 220, h: 240 },
        "lane-negotiation": { x: 280, y: 100, w: 220, h: 240 },
        mutations: { x: 540, y: 100, w: 240, h: 200 },
        actions: { x: 20, y: 380, w: 320, h: 120 },
        thread: { x: 380, y: 380, w: 400, h: 140 },
      },
    },
    nodes: [
      {
        id: "hdr",
        kind: "sectionHeader",
        title: "Pipeline board",
        props: { subtitle: "Spatial shell · React Flow" },
      },
      {
        id: "lane-prospect",
        kind: "pipelineLane",
        title: "Prospect",
        props: {
          stage: "Prospect",
          cards: [{ title: "Acme renewal", id: "c1" }, { title: "Globex pilot", id: "c2" }],
        },
      },
      {
        id: "lane-negotiation",
        kind: "pipelineLane",
        title: "Negotiation",
        props: {
          stage: "Negotiation",
          cards: [{ title: "Initech expansion", id: "c3" }],
        },
      },
      {
        id: "actions",
        kind: "actionChipGroup",
        title: "Next steps",
        props: {
          actions: [
            { label: "Email champion", prompt: "Draft a follow-up email to the Acme champion" },
            { label: "Schedule demo", prompt: "Propose a demo time for open Opportunities" },
            {
              label: "Enrich accounts",
              type: "automationRun",
              automationApiName: "Demo_Enrich_Accounts",
              input: { source: "run-demo-tool" },
            },
          ],
        },
      },
      {
        id: "thread",
        kind: "messageThread",
        title: "Activity",
        props: {
          messages: [
            { author: "agent", body: "Grouped open Opportunities by stage." },
            { author: "user", body: "Move Acme to Negotiation." },
          ],
        },
      },
      {
        id: "mutations",
        kind: "mutationProposal",
        title: "Stage updates",
        props: {
          status: "needs_review",
          operations: [
            {
              op: "update",
              object: "Opportunity",
              id: "opp-1",
              objectApiName: "Opportunity",
              recordId: "opp-1",
              data: { StageName: "Negotiation" },
            },
          ],
        },
      },
    ],
    meta: { updatedAt: new Date(0).toISOString() },
  };
}
