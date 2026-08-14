import { create } from 'zustand'
import type { FundInfo, Portfolio, SecurityInfo, ExchangeRate } from '../api'

export interface AppState {
  // Data
  funds: FundInfo[]
  securities: SecurityInfo[]
  portfolio: Portfolio | null
  portfolioXirr: number | null
  exchangeRate: ExchangeRate | null
  loadError: string

  // UI state
  dark: boolean
  portfolioId: number
  sidebarOpen: boolean
  sidebarSearch: string
  heldOnly: boolean

  // Actions
  setFunds: (funds: FundInfo[]) => void
  setSecurities: (securities: SecurityInfo[]) => void
  setPortfolio: (p: Portfolio | null) => void
  setPortfolioXirr: (xirr: number | null) => void
  setExchangeRate: (rate: ExchangeRate | null) => void
  setLoadError: (err: string) => void
  setDark: (dark: boolean) => void
  setPortfolioId: (id: number) => void
  setSidebarOpen: (open: boolean) => void
  setSidebarSearch: (search: string) => void
  toggleDark: () => void
  toggleHeldOnly: () => void
  resetForRetry: () => void
}

export const useAppStore = create<AppState>((set) => ({
  // Data defaults
  funds: [],
  securities: [],
  portfolio: null,
  portfolioXirr: null,
  exchangeRate: null,
  loadError: '',

  // UI defaults
  dark: false,
  portfolioId: 1,
  sidebarOpen: true,
  sidebarSearch: '',
  heldOnly: true,

  // Actions
  setFunds: (funds) => set({ funds }),
  setSecurities: (securities) => set({ securities }),
  setPortfolio: (portfolio) => set({ portfolio }),
  setPortfolioXirr: (portfolioXirr) => set({ portfolioXirr }),
  setExchangeRate: (exchangeRate) => set({ exchangeRate }),
  setLoadError: (loadError) => set({ loadError }),
  setDark: (dark) => set({ dark }),
  setPortfolioId: (portfolioId) => set({ portfolioId }),
  setSidebarOpen: (sidebarOpen) => set({ sidebarOpen }),
  setSidebarSearch: (sidebarSearch) => set({ sidebarSearch }),
  toggleDark: () => set((s) => ({ dark: !s.dark })),
  toggleHeldOnly: () => set((s) => ({ heldOnly: !s.heldOnly })),
  resetForRetry: () => set({ loadError: '', funds: [], securities: [], portfolio: null }),
}))
