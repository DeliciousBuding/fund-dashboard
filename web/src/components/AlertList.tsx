// AlertList —— 告警行渲染唯一出处（信号页告警卡 / 工作台告警扫描共用）。
// 空态文案由调用方按语境提供；本组件只画非空列表。
import type { AlertItem } from "@fund-dashboard/contracts";
import { Badge } from "./ui/badge";

function severityTone(severity: string): "danger" | "warn" | "neutral" {
  if (severity === "high") return "danger";
  if (severity === "medium" || severity === "low") return "warn";
  return "neutral";
}

export function AlertList({ alerts }: { alerts: AlertItem[] }) {
  return (
    <ul className="space-y-2">
      {alerts.map((a) => (
        <li
          key={`${a.kind}-${a.code}-${a.severity}-${a.message}`}
          className="flex items-center gap-3 text-sm"
        >
          <Badge tone={severityTone(a.severity)}>{a.kind}</Badge>
          <span className="min-w-0 flex-1 truncate text-fg-2">
            {a.name || a.code} · {a.message}
          </span>
        </li>
      ))}
    </ul>
  );
}
