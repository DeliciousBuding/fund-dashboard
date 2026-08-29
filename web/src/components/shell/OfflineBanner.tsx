// 离线横幅（03 §7）：断网时顶条提示；恢复逻辑在 lib/pwa.ts useOnlineStatus。
import { WifiOff } from "lucide-react";
import { useOnlineStatus } from "../../lib/pwa";

export function OfflineBanner() {
  const online = useOnlineStatus();
  if (online) return null;
  return (
    <div
      role="alert"
      className="flex items-center justify-center gap-2 border-b border-warn/30 bg-warn/10 px-4 py-1.5 text-xs text-warn"
    >
      <WifiOff className="size-3.5" />
      当前离线——展示的是缓存数据，操作暂不可用
    </div>
  );
}
