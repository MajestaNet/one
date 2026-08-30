import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";

/**
 * Live-API contract suite. Skips when ONE_JWT (or mintable ONE_API_KEY) is unset.
 * Run with: npm run test:integration (API must be reachable).
 */
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "src"),
      "monaco-editor": path.resolve(import.meta.dirname, "src/test-stubs/monaco-editor.ts"),
    },
  },
  test: {
    dir: "src",
    environment: "node",
    include: ["**/*.integration.test.{ts,tsx}"],
    fileParallelism: false,
  },
});
