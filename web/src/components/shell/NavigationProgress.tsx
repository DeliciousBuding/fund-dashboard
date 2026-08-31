// 路由级加载反馈：覆盖直接点击、移动端触控和慢 chunk，避免页面仍停在旧内容时无响应感。
import { useRouterState } from "@tanstack/react-router";
import { motion, useReducedMotion } from "motion/react";
import { cn } from "../../lib/utils";

export function NavigationProgress() {
  const isLoading = useRouterState({ select: (state) => state.isLoading });
  const reduceMotion = useReducedMotion();

  return (
    <div
      data-testid="route-progress"
      role="progressbar"
      aria-label="页面加载中"
      aria-hidden={!isLoading}
      className={cn(
        "pointer-events-none fixed inset-x-0 top-0 z-[100] h-0.5 overflow-hidden transition-opacity duration-100",
        isLoading ? "opacity-100" : "opacity-0",
      )}
    >
      {isLoading ? (
        <motion.div
          className="h-full w-1/3 bg-accent shadow-[0_0_10px_var(--accent)]"
          initial={{ x: reduceMotion ? "0%" : "-100%" }}
          animate={{ x: reduceMotion ? "200%" : "300%" }}
          transition={{
            duration: reduceMotion ? 0.6 : 0.9,
            ease: "easeInOut",
            repeat: Number.POSITIVE_INFINITY,
          }}
        />
      ) : null}
    </div>
  );
}
