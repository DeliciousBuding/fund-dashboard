// 用户偏好（本地持久化）：主题 / 涨跌色约定 / 密度。
// 主题写到 <html data-theme>，密度写 data-density，约定写 data-convention。
import { create } from "zustand";
import { persist } from "zustand/middleware";

export type ThemeMode = "dark" | "light" | "system";
// convention: 中式涨红跌绿（默认）⇄ 西式涨绿跌红
export type PnlConvention = "cn" | "western";
export type Density = "comfortable" | "compact";

interface SettingsState {
  theme: ThemeMode;
  convention: PnlConvention;
  density: Density;
  setTheme: (theme: ThemeMode) => void;
  setConvention: (convention: PnlConvention) => void;
  setDensity: (density: Density) => void;
}

export const useSettings = create<SettingsState>()(
  persist(
    (set) => ({
      theme: "dark",
      convention: "cn",
      density: "comfortable",
      setTheme: (theme) => set({ theme }),
      setConvention: (convention) => set({ convention }),
      setDensity: (density) => set({ density }),
    }),
    { name: "fund-settings" },
  ),
);

function resolvedTheme(mode: ThemeMode): "dark" | "light" {
  if (mode !== "system") return mode;
  if (typeof window === "undefined") return "dark";
  return window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
}

// applySettingsToDom 把偏好写到 <html> 数据属性（CSS 变量轴）。
// western 约定通过交换 --up/--down 实现：涨跌色互换全站即时生效。
function applySettingsToDom(state: {
  theme: ThemeMode;
  convention: PnlConvention;
  density: Density;
}) {
  const root = document.documentElement;
  root.dataset.theme = resolvedTheme(state.theme);
  root.dataset.density = state.density;
  root.dataset.convention = state.convention;
}

// subscribeSettings 启动时挂一次：persist rehydrate + 后续变更都落到 DOM。
export function subscribeSettings() {
  applySettingsToDom(useSettings.getState());
  useSettings.subscribe(applySettingsToDom);
  window
    .matchMedia("(prefers-color-scheme: light)")
    .addEventListener("change", () => applySettingsToDom(useSettings.getState()));
}
