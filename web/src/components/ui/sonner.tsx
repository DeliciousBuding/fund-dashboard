import { Toaster as SonnerToaster } from "sonner";

// Toaster — sonner 包装，贴设计 tokens（暗色表面 + 边框 + 语义色由 sonner 主题走 CSS 变量）。
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
