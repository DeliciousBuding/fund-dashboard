// 占位页：页面波次（W3–W6）落地前的干净空态——说清「为什么」+ 指向路线图。
import { Construction } from "lucide-react";

export function WipPage({ title, wave }: { title: string; wave: string }) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 rounded-xl border border-border bg-surface-1 px-6 py-16 text-center">
      <Construction className="size-8 text-fg-3" />
      <h2 className="text-lg font-medium text-fg">{title}</h2>
      <p className="max-w-sm text-sm text-fg-3">
        本页属于 {wave} 波次，正在建设中。数据与 API 已就绪，界面随后到达。
      </p>
    </div>
  );
}
