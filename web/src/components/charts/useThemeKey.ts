// useThemeKey — 响应式读取 <html data-theme>（resolved 主题的 SSOT，
// stores/settings.applySettingsToDom 是唯一落点，手动切换与系统
// prefers-color-scheme 变化都汇到这里）。
// Chart 渲染期不再裸读 document：useSyncExternalStore + MutationObserver
// 在属性变化时触发重渲染，配合 useEChart 的 notMerge setOption，
// 已挂载图表即时重绘为新配色，无需路由重挂载。
// data-convention（涨跌色互换，交换 --up/--down）同样进入快照：
// 约定切换也会改变图表取色。

import { useSyncExternalStore } from "react";

function subscribe(onChange: () => void): () => void {
  const observer = new MutationObserver(onChange);
  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["data-theme", "data-convention"],
  });
  return () => observer.disconnect();
}

function getSnapshot(): string {
  const ds = document.documentElement.dataset;
  return `${ds.theme ?? "dark"}:${ds.convention ?? "cn"}`;
}

function getServerSnapshot(): string {
  return "dark:cn";
}

export function useThemeKey(): string {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
