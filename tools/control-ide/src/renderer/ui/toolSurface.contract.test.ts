import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const rendererRoot = resolve("src/renderer");

const GRAPH_EXEMPT = new Set([
  "run/graph/RunGraphHome.tsx",
  "run/graph/RunGraphFocusPanel.tsx",
]);

function walk(dir: string, acc: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    const st = statSync(p);
    if (st.isDirectory()) walk(p, acc);
    else if (name.endsWith("Panel.tsx") || name === "RunGraphHome.tsx") {
      acc.push(p);
    }
  }
  return acc;
}

describe("tool surface contract", () => {
  it("workspace tools (except Operate graph) opt into ToolSurface", () => {
    const files = walk(rendererRoot)
      .map((abs) => relative(rendererRoot, abs))
      .filter((rel) => !rel.includes(".test."));
    const missing: string[] = [];
    for (const rel of files) {
      if (GRAPH_EXEMPT.has(rel)) continue;
      const src = readFileSync(join(rendererRoot, rel), "utf8");
      if (!src.includes("<") || src.trim().startsWith("export {")) continue;
      if (!src.includes("ToolSurface") && !src.includes("data-tool-surface")) {
        missing.push(rel);
      }
    }
    expect(missing).toEqual([]);
  });

  it("does not force Operate graph onto ToolSurface", () => {
    const src = readFileSync(join(rendererRoot, "run/graph/RunGraphHome.tsx"), "utf8");
    expect(src).not.toMatch(/ToolSurface/);
    expect(src).not.toMatch(/data-tool-surface/);
  });

  it("does not cap Account settings width like a settings page", () => {
    const css = readFileSync(join(rendererRoot, "styles.css"), "utf8");
    const blocks = [...css.matchAll(/\.account-settings-panel\s*\{([^}]*)\}/g)].map((m) => m[1]);
    expect(blocks.length).toBeGreaterThan(0);
    for (const body of blocks) {
      expect(body).not.toMatch(/max-width\s*:\s*(?!none)\S+/);
    }
  });
});
