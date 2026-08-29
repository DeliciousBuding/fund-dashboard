import { motion } from "motion/react";
import { cn } from "../../lib/utils";

// Segmented — 分段选择器（时间区间/视角切换），motion layoutId 滑动指示器。
export interface SegmentedOption<T extends string> {
  value: T;
  label: string;
}

export function Segmented<T extends string>(props: {
  value: T;
  options: SegmentedOption<T>[];
  onChange: (value: T) => void;
  size?: "sm" | "md";
  className?: string;
  id?: string;
}) {
  const layoutId = `segmented-${props.id ?? "default"}`;
  return (
    <div
      role="tablist"
      className={cn(
        "inline-flex items-center gap-0.5 rounded-lg border border-border bg-surface-1 p-0.5",
        props.className,
      )}
    >
      {props.options.map((opt) => {
        const active = opt.value === props.value;
        return (
          <button
            key={opt.value}
            type="button"
            role="tab"
            aria-selected={active}
            onClick={() => props.onChange(opt.value)}
            className={cn(
              "relative cursor-pointer rounded-md font-medium transition-colors",
              props.size === "sm" ? "px-2 py-1 text-xs" : "px-3 py-1.5 text-sm",
              active ? "text-fg" : "text-fg-3 hover:text-fg-2",
            )}
          >
            {active ? (
              <motion.span
                layoutId={layoutId}
                className="absolute inset-0 rounded-md bg-surface-3"
                transition={{ type: "spring", stiffness: 500, damping: 40 }}
              />
            ) : null}
            <span className="relative">{opt.label}</span>
          </button>
        );
      })}
    </div>
  );
}
