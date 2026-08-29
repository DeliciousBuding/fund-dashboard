// 壳层 UI 状态（本地持久化）：侧栏折叠、当前组合、持仓过滤、命令面板开关。
// 主题/密度/约定在 settings store；这里只管「工作台布局态」。
import { create } from "zustand";
import { persist } from "zustand/middleware";

interface UiState {
  sidebarCollapsed: boolean;
  portfolioId: number;
  heldOnly: boolean;
  paletteOpen: boolean;
  mobileNavOpen: boolean;
  toggleSidebar: () => void;
  setPortfolioId: (id: number) => void;
  setHeldOnly: (v: boolean) => void;
  setPaletteOpen: (v: boolean) => void;
  setMobileNavOpen: (v: boolean) => void;
}

export const useUi = create<UiState>()(
  persist(
    (set) => ({
      sidebarCollapsed: false,
      portfolioId: 1,
      heldOnly: true,
      paletteOpen: false,
      mobileNavOpen: false,
      toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
      setPortfolioId: (portfolioId) => set({ portfolioId }),
      setHeldOnly: (heldOnly) => set({ heldOnly }),
      setPaletteOpen: (paletteOpen) => set({ paletteOpen }),
      setMobileNavOpen: (mobileNavOpen) => set({ mobileNavOpen }),
    }),
    {
      name: "fund-ui",
      // 瞬时态不持久化：面板开关/移动抽屉刷新后复位
      partialize: (s) => ({
        sidebarCollapsed: s.sidebarCollapsed,
        portfolioId: s.portfolioId,
        heldOnly: s.heldOnly,
      }),
    },
  ),
);
