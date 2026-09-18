import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// Inside compose the browser still uses VITE_API_BASE_URL (host localhost:8080).
// Proxy targets are for same-origin requests when BASE_URL is empty.
const apiProxyTarget = "http://localhost:8080";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    host: true,
    port: 5173,
    proxy: {
      "/api": { target: apiProxyTarget, changeOrigin: true },
      "/health": { target: apiProxyTarget, changeOrigin: true },
      "/ready": { target: apiProxyTarget, changeOrigin: true },
      "/metrics": { target: apiProxyTarget, changeOrigin: true },
    },
  },
});
