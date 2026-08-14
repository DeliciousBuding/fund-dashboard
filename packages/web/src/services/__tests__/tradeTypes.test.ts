import { describe, it, expect } from 'vitest'
import {
  TRADE_TYPE_DCA_BUY,
  TRADE_TYPE_USER_BUY,
  toggleBuyTradeType,
  isAutoTradeType,
  isManualTradeType,
} from '../../services/tradeTypes'

describe('tradeTypes (#184)', () => {
  it('toggles DCA ↔ manual buy using Chinese DB codes only', () => {
    expect(toggleBuyTradeType(TRADE_TYPE_DCA_BUY)).toBe(TRADE_TYPE_USER_BUY)
    expect(toggleBuyTradeType(TRADE_TYPE_USER_BUY)).toBe(TRADE_TYPE_DCA_BUY)
  })

  it('does not map i18n English display strings to DB codes', () => {
    expect(toggleBuyTradeType('DCA Buy')).toBe('DCA Buy')
    expect(toggleBuyTradeType('Manual Buy')).toBe('Manual Buy')
  })

  it('detects auto/manual via Chinese substrings', () => {
    expect(isAutoTradeType(TRADE_TYPE_DCA_BUY)).toBe(true)
    expect(isManualTradeType(TRADE_TYPE_USER_BUY)).toBe(true)
    expect(isAutoTradeType(TRADE_TYPE_USER_BUY)).toBe(false)
  })
})
