import type { ReactNode } from "react";
import { cn } from "../../lib/utils";

// EmptyState — 四态纪律里的 empty：一句话说清为什么没有 + 可选主行动。
export function EmptyState(props: {
  title: string;
  description?: string;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-border px-6 py-12 text-center",
        props.className,
      )}
    >
      <p className="text-sm font-medium text-fg-2">{props.title}</p>
      {props.description ? <p className="max-w-sm text-xs text-fg-3">{props.description}</p> : null}
      {props.action ? <div className="mt-3">{props.action}</div> : null}
    </div>
  );
}
