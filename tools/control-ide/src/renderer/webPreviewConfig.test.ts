import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const root = path.dirname(fileURLToPath(import.meta.url));

describe("vite.web.config loopback bind (CIDE-17)", () => {
  it("binds the browser preview to 127.0.0.1", () => {
    const src = readFileSync(path.join(root, "../../vite.web.config.ts"), "utf8");
    expect(src).toMatch(/host:\s*["']127\.0\.0\.1["']/);
    expect(src).not.toMatch(/host:\s*["']0\.0\.0\.0["']/);
  });
});
