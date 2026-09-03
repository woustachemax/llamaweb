import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const backend = process.env.BACKEND_URL || "http://localhost:8080";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/app": { target: backend, changeOrigin: true },
      "/api": { target: backend, changeOrigin: true },
    },
  },
});
