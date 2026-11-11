import { defineConfig } from "astro/config";
import react from "@astrojs/react";
import tailwindcss from "@tailwindcss/vite";

// Static output; in production nginx serves the build and reverse-proxies /api
// to the backend. In dev, the Vite dev server does the same proxying.
// API_PROXY_TARGET is set by docker-compose.dev.yml; it defaults to a backend
// running locally via `go run .`.
const apiTarget = process.env.API_PROXY_TARGET ?? "http://localhost:8080";

export default defineConfig({
  integrations: [react()],
  vite: {
    plugins: [tailwindcss()],
    server: {
      proxy: {
        "/api": { target: apiTarget, changeOrigin: true },
      },
      // Docker Desktop doesn't forward file events from bind mounts, so
      // docker-compose.dev.yml sets WATCH_POLLING=1.
      watch: process.env.WATCH_POLLING ? { usePolling: true, interval: 300 } : undefined,
    },
  },
});
