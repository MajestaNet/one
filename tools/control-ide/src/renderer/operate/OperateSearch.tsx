import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from "react";
import type { AppBridge } from "../App";

export type SearchHit = {
  id: string;
  object: string;
  title?: string;
  subtitle?: string;
  updatedAt?: string;
  score?: number;
};

export type GraphSearchMatch = {
  nodeId: string;
  title: string;
  detail?: string;
};

type SearchResponse = {
  query?: string;
  hits?: SearchHit[];
};

function minQueryHint(q: string): string | null {
  const trimmed = q.trim();
  if (!trimmed) return "Type at least 2 characters";
  const digitsOnly = /^[\d\s]+$/.test(trimmed) && /\d/.test(trimmed);
  if (digitsOnly) {
    const digits = trimmed.replace(/\D/g, "");
    if (digits.length < 3) return "Type at least 3 digits";
  } else if ([...trimmed].length < 2) {
    return "Type at least 2 characters";
  }
  return null;
}

function hitLabel(hit: SearchHit): string {
  const title = (hit.title || "").trim();
  if (title) return title;
  const short = hit.id.length > 8 ? hit.id.slice(0, 8) : hit.id;
  return `${hit.object} ${short}`;
}

export function OperateSearch({
  fetchFn,
  onOpenHit,
  onPinHit,
  graphMatches = [],
  onOpenGraphMatch,
  isOnGraph,
  placeholder = "Find records, tools, objects…",
}: {
  fetchFn: AppBridge["fetch"];
  onOpenHit: (hit: SearchHit) => void;
  onPinHit?: (hit: SearchHit) => void;
  graphMatches?: readonly GraphSearchMatch[];
  onOpenGraphMatch?: (match: GraphSearchMatch) => void;
  isOnGraph?: (hit: SearchHit) => boolean;
  placeholder?: string;
}) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const seqRef = useRef(0);
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [hits, setHits] = useState<SearchHit[]>([]);
  const [activeIndex, setActiveIndex] = useState(0);
  const hint = minQueryHint(q);

  const grouped = useMemo(() => {
    const order: string[] = [];
    const byObject = new Map<string, SearchHit[]>();
    for (const hit of hits) {
      const key = hit.object || "Record";
      if (!byObject.has(key)) {
        order.push(key);
        byObject.set(key, []);
      }
      byObject.get(key)!.push(hit);
    }
    return order.map((object) => ({ object, hits: byObject.get(object)! }));
  }, [hits]);

  const flatHits = useMemo(() => grouped.flatMap((g) => g.hits), [grouped]);
  const matchingGraph = useMemo(() => {
    const needle = q.trim().toLowerCase();
    if ([...needle].length < 2) return [];
    return graphMatches
      .filter((match) => `${match.title} ${match.detail ?? ""}`.toLowerCase().includes(needle))
      .slice(0, 8);
  }, [graphMatches, q]);
  const options = useMemo(
    () => [
      ...matchingGraph.map((match) => ({ kind: "graph" as const, match })),
      ...flatHits.map((hit) => ({ kind: "record" as const, hit })),
    ],
    [flatHits, matchingGraph],
  );

  const runSearch = useCallback(
    async (needle: string) => {
      const blocked = minQueryHint(needle);
      if (blocked) {
        setHits([]);
        setBusy(false);
        setErr("");
        return;
      }
      const seq = ++seqRef.current;
      setBusy(true);
      setErr("");
      try {
        const raw = (await fetchFn("/client/v1/search", {
          method: "POST",
          body: JSON.stringify({ q: needle.trim(), limit: 20 }),
        })) as SearchResponse;
        if (seq !== seqRef.current) return;
        setHits(Array.isArray(raw?.hits) ? raw.hits : []);
        setActiveIndex(0);
      } catch (e) {
        if (seq !== seqRef.current) return;
        setHits([]);
        setErr(String(e));
      } finally {
        if (seq === seqRef.current) setBusy(false);
      }
    },
    [fetchFn],
  );

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    if (hint) {
      setHits([]);
      setBusy(false);
      setErr("");
      return;
    }
    debounceRef.current = setTimeout(() => {
      void runSearch(q);
    }, 200);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [q, hint, runSearch]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        inputRef.current?.focus();
        setOpen(true);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const choose = (hit: SearchHit) => {
    onOpenHit(hit);
    setOpen(false);
  };

  const chooseGraph = (match: GraphSearchMatch) => {
    onOpenGraphMatch?.(match);
    setOpen(false);
  };

  const onKeyDown = (e: ReactKeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Escape") {
      setOpen(false);
      inputRef.current?.blur();
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setOpen(true);
      setActiveIndex((i) => Math.min(i + 1, Math.max(options.length - 1, 0)));
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((i) => Math.max(i - 1, 0));
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      const option = options[activeIndex];
      if (option?.kind === "graph") chooseGraph(option.match);
      else if (option?.kind === "record") choose(option.hit);
    }
  };

  let optionIndex = -1;

  return (
    <div className="operate-search">
      <label className="visually-hidden" htmlFor="operate-global-search">
        Search records
      </label>
      <input
        ref={inputRef}
        id="operate-global-search"
        data-testid="operate-global-search"
        role="combobox"
        aria-expanded={open}
        aria-controls="operate-search-listbox"
        aria-autocomplete="list"
        aria-activedescendant={options[activeIndex]
          ? options[activeIndex].kind === "graph"
            ? `operate-search-graph-${options[activeIndex].match.nodeId}`
            : `operate-search-opt-${options[activeIndex].hit.id}`
          : undefined}
        placeholder={placeholder}
        value={q}
        onChange={(e) => {
          setQ(e.target.value);
          setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        onBlur={() => {
          window.setTimeout(() => setOpen(false), 120);
        }}
        onKeyDown={onKeyDown}
      />
      <span className="operate-search-kbd" aria-hidden="true">
        ⌘K
      </span>
      {open ? (
        <div className="operate-search-popover" id="operate-search-listbox" role="listbox" data-testid="operate-search-results">
          {hint && !q.trim() ? <p className="muted">{hint}</p> : null}
          {hint && q.trim() ? <p className="muted">{hint}</p> : null}
          {!hint && busy ? <p className="muted">Searching…</p> : null}
          {!hint && !busy && err ? <p className="error">{err}</p> : null}
          {!hint && !busy && !err && hits.length === 0 && q.trim() ? (
            matchingGraph.length === 0 ? <p className="muted">No matching records or graph items</p> : null
          ) : null}
          {!hint && matchingGraph.length > 0 ? (
            <div className="operate-search-group">
              <p className="operate-search-group-label">On this graph</p>
              {matchingGraph.map((match, index) => (
                <button
                  key={match.nodeId}
                  type="button"
                  id={`operate-search-graph-${match.nodeId}`}
                  role="option"
                  aria-selected={index === activeIndex}
                  className={index === activeIndex ? "operate-search-hit is-active" : "operate-search-hit"}
                  data-testid={`operate-search-graph-${match.nodeId}`}
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => chooseGraph(match)}
                >
                  <span className="operate-search-hit-title">{match.title}</span>
                  {match.detail ? <span className="muted">{match.detail}</span> : null}
                </button>
              ))}
            </div>
          ) : null}
          {!hint &&
            grouped.map((group) => (
              <div key={group.object} className="operate-search-group">
                <p className="operate-search-group-label">{group.object}</p>
                {group.hits.map((hit) => {
                  optionIndex += 1;
                  const graphOffset = matchingGraph.length;
                  const idx = graphOffset + optionIndex;
                  return (
                    <div key={`${hit.object}-${hit.id}`} className="operate-search-hit-row">
                      <button
                        type="button"
                        id={`operate-search-opt-${hit.id}`}
                        role="option"
                        aria-selected={idx === activeIndex}
                        className={idx === activeIndex ? "operate-search-hit is-active" : "operate-search-hit"}
                        data-testid={`operate-search-hit-${hit.id}`}
                        onMouseDown={(event) => event.preventDefault()}
                        onClick={() => choose(hit)}
                      >
                        <span className="operate-search-hit-title">{hitLabel(hit)}</span>
                        <span className="operate-search-hit-meta">
                          <span className="operate-search-badge">{hit.object}</span>
                          {isOnGraph?.(hit) ? <span className="operate-search-badge is-on-graph">On graph</span> : null}
                          {hit.subtitle ? <span className="muted">{hit.subtitle}</span> : null}
                        </span>
                      </button>
                      {onPinHit ? (
                        <button
                          type="button"
                          className="operate-search-pin"
                          aria-label={`Pin ${hitLabel(hit)} to graph`}
                          title="Pin record to graph"
                          onMouseDown={(event) => event.preventDefault()}
                          onClick={() => {
                            onPinHit(hit);
                            setOpen(false);
                          }}
                        >
                          Pin
                        </button>
                      ) : null}
                    </div>
                  );
                })}
              </div>
            ))}
        </div>
      ) : null}
    </div>
  );
}
