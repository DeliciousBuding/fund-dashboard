// 全局快捷键（03 §8）：⌘K/Ctrl+K 命令面板；g 系列导航；? 快捷键表（W7 落地）。
// 输入态（input/textarea/contentEditable）下自动失效。

import { useRouter } from "@tanstack/react-router";
import { useEffect } from "react";
import { useUi } from "../stores/ui";

const G_NAV: Record<string, string> = {
  o: "/",
  h: "/holdings",
  t: "/transactions",
  a: "/analysis",
  ",": "/settings",
};

function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable;
}

export function useGlobalHotkeys() {
  const router = useRouter();
  const setPaletteOpen = useUi((s) => s.setPaletteOpen);

  useEffect(() => {
    let gPending = false;
    let gTimer: ReturnType<typeof setTimeout> | null = null;

    const onKeyDown = (e: KeyboardEvent) => {
      // ⌘K / Ctrl+K：输入态也响应（从任何地方唤出面板的肌肉记忆）
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setPaletteOpen(true);
        return;
      }
      if (isTypingTarget(e.target) || e.metaKey || e.ctrlKey || e.altKey) return;

      if (e.key === "g") {
        gPending = true;
        if (gTimer) clearTimeout(gTimer);
        gTimer = setTimeout(() => {
          gPending = false;
        }, 800);
        return;
      }
      if (gPending) {
        gPending = false;
        if (gTimer) clearTimeout(gTimer);
        const to = G_NAV[e.key];
        if (to) void router.navigate({ to });
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
      if (gTimer) clearTimeout(gTimer);
    };
  }, [router, setPaletteOpen]);
}
