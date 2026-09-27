// @ts-check
import js from "@eslint/js";
import react from "eslint-plugin-react";
import globals from "globals";
import tseslint from "typescript-eslint";

// Onion dependency rules (docs/frontend-architecture.md). Each layer lists
// the layers it must not import; src/test/architecture.test.ts checks the
// same graph independently of this plugin configuration.
const layer = (name) => [`**/${name}/**`, `../${name}/*`, `../../${name}/*`, `../../../${name}/*`, `../../../../${name}/*`];
const forbid = (...layers) => ({
  "no-restricted-imports": ["error", {
    patterns: layers.map((name) => ({
      group: layer(name),
      message: `This layer must not depend on ${name}/ (see docs/frontend-architecture.md).`,
    })),
  }],
});
const noUi = {
  "no-restricted-imports": ["error", {
    paths: [
      { name: "preact", message: "Domain and application stay framework-free." },
      { name: "preact/hooks", message: "Domain and application stay framework-free." },
    ],
    patterns: [
      { group: ["preact/*", "*.css"], message: "Domain and application stay framework-free." },
      ...["application", "infrastructure", "presentation", "app"].map((name) => ({ group: layer(name), message: `Must not depend on ${name}/.` })),
    ],
  }],
};
const browserIo = ["fetch", "EventSource", "XMLHttpRequest", "WebSocket", "localStorage", "sessionStorage"]
  .map((name) => ({ name, message: "I/O belongs in infrastructure/, reached through an application port." }));

export default tseslint.config(
  { ignores: ["node_modules", "dist", "../internal/web/dist", "test-results", "playwright-report"] },
  js.configs.recommended,
  ...tseslint.configs.strict,
  {
    files: ["**/*.{ts,tsx}"],
    languageOptions: { globals: { ...globals.browser } },
    plugins: { react },
    settings: { react: { version: "18.3", pragma: "h", fragment: "Fragment" } },
    rules: {
      "react/no-danger": "error",
      "react/jsx-no-target-blank": "error",
      "react/jsx-key": "error",
      "@typescript-eslint/no-non-null-assertion": "error",
      "@typescript-eslint/consistent-type-imports": "error",
    },
  },
  {
    // Screen copy comes from locale dictionaries (#124); only symbols and
    // the brand may appear as JSX text.
    files: ["src/presentation/**/*.tsx", "src/app/**/*.tsx"],
    rules: {
      "react/jsx-no-literals": ["error", {
        noStrings: false,
        ignoreProps: true,
        allowedStrings: ["Insight Lab", "make build-demo", "→", "←", "↓", "↑", "·", "≠", "✓", "⚠", "⚙", "—", "-", "\"", "%", "/", ":", "(", ")", "|", "×", "1", "2", "3", "4"],
      }],
    },
  },
  { files: ["src/domain/**"], rules: noUi },
  { files: ["src/application/**"], rules: noUi },
  { files: ["src/infrastructure/**"], rules: forbid("presentation", "app") },
  {
    files: ["src/presentation/**"],
    rules: { ...forbid("infrastructure", "app"), "no-restricted-globals": ["error", ...browserIo] },
  },
  {
    files: ["src/domain/**", "src/application/**"],
    rules: { "no-restricted-globals": ["error", ...browserIo, { name: "document", message: "No DOM here." }, { name: "window", message: "No DOM here." }] },
  },
  {
    files: ["e2e/**", "scripts/**", "*.config.{ts,js}"],
    languageOptions: { globals: { ...globals.node } },
  },
);
