import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import electron from "vite-plugin-electron/simple";
import path from "node:path";
import { buildCsp } from "./src/main/security.ts";

/** Production builds must not emit source maps into desktop installers. */
const noSourcemap = { sourcemap: false as const };

/**
 * Substitute the CSP placeholder in index.html. Dev needs `unsafe-inline`/`unsafe-eval` for
 * HMR; the packaged build gets the strict policy.
 */
function cspPlugin(isDev: boolean) {
  return {
    name: "one-csp",
    transformIndexHtml: {
      order: "pre" as const,
      handler(html: string) {
        const csp = buildCsp(isDev ? "development" : "production", process.env.VITE_DEV_SERVER_URL);
        return html.replace("%ONE_CSP%", csp);
      },
    },
  };
}

export default defineConfig(({ command }) => ({
  root: ".",
  base: "./",
  plugins: [
    react(),
    cspPlugin(command === "serve"),
    electron({
      main: {
        entry: "src/main/main.ts",
        vite: {
          build: {
            outDir: "dist-electron",
            ...noSourcemap,
            rolldownOptions: {
              external: ["electron", "electron-updater"],
              output: { entryFileNames: "main.js" },
            },
          },
        },
      },
      preload: {
        input: path.join(import.meta.dirname, "src/preload/preload.ts"),
        vite: {
          build: {
            outDir: "dist-electron",
            ...noSourcemap,
            rolldownOptions: {
              output: { entryFileNames: "preload.js" },
            },
          },
        },
      },
    }),
  ],
  build: {
    outDir: "dist",
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    port: 5173,
  },
}));
