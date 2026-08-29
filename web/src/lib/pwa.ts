// PWA 注册 + chunk 加载自愈 + 离线横幅逻辑（03 §7）。
// - SW 仅生产注册；/api 与 /mcp 永不进缓存（数据以服务端为准）
// - vite:preloadError（部署后旧 chunk 404）：60s 节流的自动 reload
// - 离线：横幅提示；恢复在线：invalidateQueries + toast
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { queryClient } from "./queryClient";

let lastChunkReload = 0;

export function registerPWA() {
  if (!import.meta.env.PROD || !("serviceWorker" in navigator)) return;
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js").catch(() => {
      // SW 注册失败不阻塞应用（隐私模式/旧浏览器）
    });
  });

  // chunk 自愈：新版本部署后旧 hash chunk 消失 → 强制刷新拉新壳
  window.addEventListener("vite:preloadError", () => {
    const now = Date.now();
    if (now - lastChunkReload < 60_000) return;
    lastChunkReload = now;
    window.location.reload();
  });
}

export function useOnlineStatus(): boolean {
  const [online, setOnline] = useState(() => navigator.onLine);
  useEffect(() => {
    const onOnline = () => {
      setOnline(true);
      void queryClient.invalidateQueries();
      toast.success("网络已恢复，数据已刷新");
    };
    const onOffline = () => setOnline(false);
    window.addEventListener("online", onOnline);
    window.addEventListener("offline", onOffline);
    return () => {
      window.removeEventListener("online", onOnline);
      window.removeEventListener("offline", onOffline);
    };
  }, []);
  return online;
}
