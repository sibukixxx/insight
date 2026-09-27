import { fileURLToPath } from "node:url";
import preact from "@preact/preset-vite";
import { defineConfig } from "vite";

// The build output is the directory Go embeds (internal/web/dist, see
// internal/web/embed.go). It is committed so `make build` needs no Node;
// `make web-check` fails when it no longer matches these sources.
const outDir = fileURLToPath(new URL("../internal/web/dist", import.meta.url));

export default defineConfig({
  plugins: [preact()],
  build: {
    outDir,
    emptyOutDir: false,
    assetsDir: "assets",
    sourcemap: false,
  },
  css: {
    modules: {
      // Readable, content-independent class names keep the committed
      // output stable and diffable.
      generateScopedName: "[name]__[local]",
    },
  },
  server: {
    // `pnpm dev` proxies the API to a running `insight-lab serve`.
    proxy: { "/api": "http://127.0.0.1:8787" },
  },
});
