import { sanitizeRunGraphDocument } from "./sanitize";
import {
  type RunGraphDocument,
  type RunGraphEnvelope,
  type RunGraphResolveRef,
  type RunGraphResolveResult,
} from "./types";
import { validateRunGraphDocument } from "./validate";

export type RunGraphFetch = (path: string, init?: RequestInit) => Promise<unknown>;

function asObject(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error("Run graph response must be an object");
  }
  return value as Record<string, unknown>;
}

function parseEnvelope(input: unknown): RunGraphEnvelope {
  const raw = asObject(input);
  const sanitized = sanitizeRunGraphDocument(raw.document);
  const validated = validateRunGraphDocument(sanitized);
  if (!validated.ok) {
    throw new Error(`Run graph failed validation: ${validated.issues[0]?.message ?? "invalid document"}`);
  }
  if (typeof raw.graphKey !== "string" || typeof raw.revision !== "number") {
    throw new Error("Run graph response is missing graphKey or revision");
  }
  return {
    id: typeof raw.id === "string" ? raw.id : validated.document.id,
    graphKey: raw.graphKey,
    title: typeof raw.title === "string" ? raw.title : validated.document.title,
    document: validated.document,
    revision: raw.revision,
    createdAt: typeof raw.createdAt === "string" ? raw.createdAt : undefined,
    updatedAt: typeof raw.updatedAt === "string" ? raw.updatedAt : undefined,
  };
}

export async function getHomeRunGraph(fetchFn: RunGraphFetch): Promise<RunGraphEnvelope> {
  return getRunGraph(fetchFn, "home");
}

export async function getRunGraph(
  fetchFn: RunGraphFetch,
  graphKey: string,
): Promise<RunGraphEnvelope> {
  const path = graphKey === "home"
    ? "/client/v1/run-graphs/home"
    : `/client/v1/run-graphs/${encodeURIComponent(graphKey)}`;
  return parseEnvelope(await fetchFn(path));
}

export async function putRunGraph(
  fetchFn: RunGraphFetch,
  graphKey: string,
  document: RunGraphDocument,
  expectedRevision?: number,
): Promise<RunGraphEnvelope> {
  const sanitized = sanitizeRunGraphDocument(document);
  const validated = validateRunGraphDocument(sanitized);
  if (!validated.ok) throw new Error(validated.issues[0]?.message ?? "Invalid Run graph");
  return parseEnvelope(
    await fetchFn(`/client/v1/run-graphs/${encodeURIComponent(graphKey)}`, {
      method: "PUT",
      headers: expectedRevision === undefined ? undefined : { "If-Match": `"${expectedRevision}"` },
      body: JSON.stringify(validated.document),
    }),
  );
}

export async function patchRunGraph(
  fetchFn: RunGraphFetch,
  graphKey: string,
  patch: Record<string, unknown>,
  expectedRevision?: number,
): Promise<RunGraphEnvelope> {
  return parseEnvelope(
    await fetchFn(`/client/v1/run-graphs/${encodeURIComponent(graphKey)}`, {
      method: "PATCH",
      headers: expectedRevision === undefined ? undefined : { "If-Match": `"${expectedRevision}"` },
      body: JSON.stringify(sanitizeRunGraphDocument(patch)),
    }),
  );
}

export async function resolveRunGraphCards(
  fetchFn: RunGraphFetch,
  nodes: RunGraphResolveRef[],
): Promise<RunGraphResolveResult[]> {
  if (!nodes.length) return [];
  const body = asObject(
    await fetchFn("/client/v1/run-graphs/resolve", {
      method: "POST",
      body: JSON.stringify({ nodes, projection: "card" }),
    }),
  );
  if (!Array.isArray(body.nodes)) throw new Error("Run graph resolve response is missing nodes");
  return body.nodes.flatMap((item) => {
    if (!item || typeof item !== "object" || Array.isArray(item)) return [];
    const result = item as Record<string, unknown>;
    if (typeof result.nodeId !== "string" || typeof result.ok !== "boolean") return [];
    return [
      {
        nodeId: result.nodeId,
        ok: result.ok,
        code:
          result.code === "FORBIDDEN" || result.code === "NOT_FOUND"
            ? result.code
            : undefined,
        record:
          result.ok && result.record && typeof result.record === "object" && !Array.isArray(result.record)
            ? (result.record as Record<string, unknown>)
            : undefined,
      } satisfies RunGraphResolveResult,
    ];
  });
}
