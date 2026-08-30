import js from "@eslint/js";
import tseslint from "typescript-eslint";
import reactHooks from "eslint-plugin-react-hooks";
import security from "eslint-plugin-security";
import globals from "globals";

/**
 * Lint gate for the Control IDE vendor plane. `eslint-plugin-security` is here for the
 * main/preload trust boundary (child_process, unsafe regex, path handling) — see
 * docs/architecture/control-ide-security-audit.md.
 */
export default tseslint.config(
  {
    ignores: ["dist/**", "dist-electron/**", "release/**", "coverage/**", "node_modules/**"],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  security.configs.recommended,
  {
    files: ["**/*.{ts,tsx}"],
    languageOptions: {
      ecmaVersion: 2023,
      sourceType: "module",
      globals: { ...globals.browser, ...globals.node },
    },
    rules: {
      "@typescript-eslint/no-explicit-any": "error",
      "@typescript-eslint/no-unused-vars": ["error", { argsIgnorePattern: "^_" }],
      "no-restricted-properties": [
        "error",
        {
          object: "window",
          property: "open",
          message:
            "Use openExternalUrl() from src/renderer/external.ts — renderer-created windows inherit the preload bridge (CIDE-01).",
        },
      ],
      "no-restricted-syntax": [
        "error",
        {
          selector: "JSXAttribute[name.name='dangerouslySetInnerHTML']",
          message: "Untrusted agent/API content must render as React text nodes, never raw HTML.",
        },
      ],
      // Paths reaching fs in main are validated by src/main/paths.ts; the generic
      // non-literal-filename heuristic flags every call there and adds no signal.
      "security/detect-non-literal-fs-filename": "off",
      "security/detect-object-injection": "off",
    },
  },
  {
    // Node tooling: the Electron boundary harness and any future build scripts.
    files: ["scripts/**/*.{js,mjs}", "*.config.{js,mjs}"],
    languageOptions: {
      ecmaVersion: 2023,
      sourceType: "module",
      globals: { ...globals.node, WebSocket: "readonly", fetch: "readonly" },
    },
    rules: {
      "security/detect-child-process": "off",
    },
  },
  {
    files: ["src/renderer/**/*.{ts,tsx}"],
    plugins: { "react-hooks": reactHooks },
    rules: {
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",
    },
  },
  {
    files: ["**/*.test.{ts,tsx}", "**/*.integration.test.{ts,tsx}", "vitest.setup.ts", "src/test-stubs/**"],
    rules: {
      "@typescript-eslint/no-explicit-any": "off",
      "security/detect-child-process": "off",
    },
  },
);
