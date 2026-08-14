/**
 * trade_type DB write values — Chinese literals matching backend conventions
 * (seed SQL, MCP import, TransactionTable isAuto/isManual, summary auto_tx filters).
 * Not UI copy — never compare against i18n display strings (#184).
 */
export const TRADE_TYPE_USER_BUY = '用户买入'
export const TRADE_TYPE_DCA_BUY = '定投买入'
export const TRADE_TYPE_USER_SELL = '用户卖出'
export const TRADE_TYPE_DCA_SELL = '定投卖出'

/** Toggle buy trade_type between DCA and manual (DB codes only). */
export function toggleBuyTradeType(current: string): string {
  if (current === TRADE_TYPE_DCA_BUY) return TRADE_TYPE_USER_BUY
  if (current === TRADE_TYPE_USER_BUY) return TRADE_TYPE_DCA_BUY
  return current
}

export function isAutoTradeType(tradeType: string | null | undefined): boolean {
  return (tradeType || '').includes('定投')
}

export function isManualTradeType(tradeType: string | null | undefined): boolean {
  return (tradeType || '').includes('用户')
}
