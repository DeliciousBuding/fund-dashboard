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
      "/api": "http://localhost:8765",
      "/mcp": "http://localhost:8765",
    },
  },
  build: {
    outDir: "../internal/webui/dist",
    emptyOutDir: true,
    sourcemap: false,
    target: "es2022",
    rollupOptions: {
      output: {
        // 供应商分包（W7 预算 <400KB gzip）：echarts+zrender 独立 chunk；
        // react 系 / tanstack 系 / 其余 vendor 分开，便于长缓存，避免单一巨型首屏 chunk。
        manualChunks(id) {
          if (!id.includes("node_modules")) return undefined;
          if (id.includes("echarts") || id.includes("zrender")) return "echarts";
          if (/[\\/]node_modules[\\/](react|react-dom|scheduler)[\\/]/.test(id)) return "react";
          if (id.includes("@tanstack")) return "tanstack";
          return "vendor";
        },
      },
    },
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
});
