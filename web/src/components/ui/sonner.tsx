import { Toaster as SonnerToaster } from "sonner";
import "sonner/dist/styles.css";

// Toaster — sonner 包装，贴设计 tokens（暗色表面 + 边框 + 语义色由 sonner 主题走 CSS 变量）。
// 样式走静态 stylesheet（sonner/dist/styles.css，见 patches/sonner@2.0.8.patch 移除运行时 <style> 注入）——严格 CSP style-src 'self' 下运行时注入会被拦。
export function Toaster() {
  return (
    <SonnerToaster
      position="bottom-right"
      toastOptions={{
        style: {
          background: "var(--surface-2)",
          border: "1px solid var(--border)",
          color: "var(--fg)",
        },
      }}
    />
  );
}
