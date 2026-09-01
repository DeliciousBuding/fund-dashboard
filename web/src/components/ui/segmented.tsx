import { motion } from "motion/react";
import { cn } from "../../lib/utils";

function nextTabIndex(current: number, key: string, length: number): number {
  switch (key) {
    case "ArrowRight":
      return (current + 1) % length;
    case "ArrowLeft":
      return (current - 1 + length) % length;
    case "Home":
      return 0;
    case "End":
      return length - 1;
    default:
      return -1;
  }
}

// Segmented — 分段选择器（时间区间/视角切换），motion layoutId 滑动指示器。
interface SegmentedOption<T extends string> {
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
      aria-orientation="horizontal"
      onKeyDown={(e) => {
        const idx = props.options.findIndex((o) => o.value === props.value);
        const next = nextTabIndex(idx, e.key, props.options.length);
        if (next < 0 || next === idx) return;
        e.preventDefault();
        props.onChange(props.options[next].value);
      }}
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
            tabIndex={active ? 0 : -1}
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
