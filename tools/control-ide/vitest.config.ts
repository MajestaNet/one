import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      // Keep renderer imports stable in tests without Electron plugins.
      "@": path.resolve(import.meta.dirname, "src"),
      // monaco's package exports break Vite resolve in jsdom — stub for unit tests.
      "monaco-editor": path.resolve(import.meta.dirname, "src/test-stubs/monaco-editor.ts"),
    },
  },
  test: {
    dir: "src",
    environment: "jsdom",
    setupFiles: [path.resolve(import.meta.dirname, "vitest.setup.ts")],
    include: ["**/*.test.{ts,tsx}"],
    exclude: ["**/*.integration.test.{ts,tsx}"],
    coverage: {
      provider: "v8",
      reporter: ["text", "lcov"],
      // Trust-boundary policy (main + preload contracts) is inside the gate (CIDE-15 / Phase 4).
      // Electron lifecycle wiring in main.ts / preload.ts is exercised by smoke:electron in CI.
      include: ["src/main/**/*.{ts,tsx}", "src/preload/**/*.{ts,tsx}", "src/renderer/**/*.{ts,tsx}"],
      exclude: [
        "src/**/*.test.{ts,tsx}",
        "src/**/*.integration.test.{ts,tsx}",
        "src/renderer/main.tsx",
        "src/main/main.ts",
        "src/preload/preload.ts",
        "src/test-stubs/**",
        // Unused workspace canvas (not mounted); omit from global gate until wired.
        "src/renderer/workspace/AgentsCanvas.tsx",
      ],
      thresholds: {
        // Vitest 4 V8 uses AST remapping (stricter than v3 v8-to-istanbul). Keep the
        // gate at the measured post-upgrade floor rather than the inflated v3 numbers.
        lines: 76,
        functions: 72,
        branches: 64,
        statements: 73,
      },
    },
  },
});
