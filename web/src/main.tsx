import { QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "@fontsource-variable/inter";
import "./styles/tokens.css";
import { Toaster } from "./components/ui/sonner";
import { registerPWA } from "./lib/pwa";
import { queryClient } from "./lib/queryClient";
import { router } from "./router";
import { subscribeSettings } from "./stores/settings";

// 启动顺序：偏好落 DOM（防主题闪屏）→ 渲染 → PWA 注册（prod）。
subscribeSettings();
registerPWA();

const container = document.getElementById("root");
if (!container) throw new Error("missing #root");

createRoot(container).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>
  </StrictMode>,
);
