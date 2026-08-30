import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { buildCsp } from "./src/main/security.ts";

/** Browser-only preview (no Electron) for localhost UI review. */
export default defineConfig(({ command }) => ({
  root: ".",
  base: "./",
  plugins: [
    react(),
    {
      name: "one-csp",
      transformIndexHtml: {
        order: "pre" as const,
        handler: (html: string) =>
          html.replace("%ONE_CSP%", buildCsp(command === "serve" ? "development" : "production")),
      },
    },
  ],
  build: {
    outDir: "dist",
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    // Loopback only — this preview serves the full renderer (CIDE-17).
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
  },
}));
