import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// The build output lands in the Go module so `go:embed` picks it up
// (internal/webui/dist). In dev, /api + /mcp proxy to the Go backend.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      "/api": "http://127.0.0.1:8765",
      "/mcp": "http://127.0.0.1:8765",
    },
  },
  build: {
    outDir: "../internal/webui/dist",
    emptyOutDir: true,
    sourcemap: false,
    target: "es2022",
    rollupOptions: {
      output: {
        // echarts 单独成 chunk：图表页懒加载时再拉，首屏不含（W7 预算 <400KB gzip）。
        manualChunks: {
          echarts: ["echarts/core", "echarts/charts", "echarts/components", "echarts/renderers"],
        },
      },
    },
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
});
