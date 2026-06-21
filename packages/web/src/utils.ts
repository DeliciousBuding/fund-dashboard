import type { FundInfo, SecurityInfo } from './api'

export const C = { blue: '#3172d9', up: '#d63649', down: '#199c63', amber: '#e07b2c' }; // 红涨绿跌(国内)

export const SHARED_CHART_GRID = { left: 55, right: 30, top: 32, bottom: 36 };

// --- Stock market configuration ---
export const STOCK_MARKETS: Record<string, { label: string; color: string }> = {
  sh: { label: '沪A', color: '#d63649' },   // Shanghai A-share
  sz: { label: '深A', color: '#199c63' },   // Shenzhen A-share
  hk: { label: '港股', color: '#e07b2c' },  // Hong Kong stock
  us: { label: '美股', color: '#3172d9' },  // US stock
};

/** Detect a stock code's market. Returns the market key (sh/sz/hk/us) or undefined. */
export function detectStockMarket(code: string): string | undefined {
  if (!code) return undefined;
  const c = code.trim();
  // 6xxxxx → Shanghai A-stock
  if (/^6\d{5}$/.test(c)) return 'sh';
  // 0xxxxx or 3xxxxx → Shenzhen A-stock
  if (/^[03]\d{5}$/.test(c)) return 'sz';
  // 5-digit → Hong Kong stock
  if (/^\d{5}$/.test(c)) return 'hk';
  // Alphabetic ticker (1-5 uppercase letters, e.g. NVDA, AAPL, META) → US stock
  if (/^[A-Za-z]{1,5}$/.test(c)) return 'us';
  return undefined;
}

export const CATS: Record<string, { name: string; funds: string[] }> = {
  nasdaq:  { name: '纳斯达克', funds: ['纳斯达克', '纳指'] },
  tech:    { name: '科技主题', funds: ['科创', '科技', '半导体', '芯片', '人工智能', '机器人', '计算机', '信息产业', '高端装备', '新能源车'] },
  dividend:{ name: '红利价值', funds: ['红利', '港股通红利', '主要消费', '金融'] },
  gold:    { name: '黄金商品', funds: ['黄金', '白银', '有色金属', '电力'] },
  bond:    { name: '债券存单', funds: ['债', '存单', '稳利'] },
  qdii:    { name: '海外其他', funds: [] },
  money:   { name: '货币基金', funds: ['货币', '日日盈'] },
  ashare:  { name: 'A股股票', funds: [] },
  hkstock: { name: '港股股票', funds: [] },
  other:   { name: '其他', funds: [] },
};

export const CAT_ORDER = ['nasdaq', 'dividend', 'tech', 'gold', 'bond', 'qdii', 'money', 'other'];

export function classify(f: FundInfo): string {
  // Stock detection: use security_type from backend, NOT code pattern guessing
  if (f.security_type === 'stock') {
    const mkt = f.market || detectStockMarket(f.code) || '';
    if (mkt === 'sh' || mkt === 'sz') return 'ashare';
    if (mkt === 'hk') return 'hkstock';
    if (mkt === 'us') return 'ashare'; // US stocks not in fund CATS — routed via stockGroups instead
    return 'ashare';
  }

  const n = f.name; const t = f.type;
  // Fund keyword classification
  for (const [cat, cfg] of Object.entries(CATS)) {
    if (cfg.funds.some(kw => n.includes(kw))) return cat;
  }
  if (t.toUpperCase().includes('QDII')) return 'qdii';
  if (t.includes('债')) return 'bond'; if (t.includes('货币')) return 'money';
  if (t.includes('指数')) return 'other'; if (t.includes('混合')) return 'other';
  if (t.includes('股票')) return 'tech';
  return 'other';
}

// ═══════ Stock-only category config (for SecurityInfo from /api/securities) ═══════

export const STOCK_CATS: Record<string, { name: string }> = {
  'stock-a':  { name: 'A股' },
  'stock-hk': { name: '港股通' },
  'stock-us': { name: '美股' },
};

export const STOCK_CAT_ORDER = ['stock-a', 'stock-hk', 'stock-us'];

/** Classify a SecurityInfo into a stock category based on its market field. */
export function classifySecurity(s: SecurityInfo): string {
  switch (s.market) {
    case 'sh':
    case 'sz': return 'stock-a';
    case 'hk': return 'stock-hk';
    case 'us': return 'stock-us';
    default:  return 'stock-a'; // fallback
  }
}

/**
 * Get a human-readable market label for a stock code.
 * Returns e.g. "沪A" for 6xxxxx, "深A" for 0xxxxx/3xxxxx, "港股" for 5-digit, "美股" for US tickers.
 */
export function getMarketLabel(code: string): string {
  const mkt = detectStockMarket(code);
  if (!mkt) return '';
  return STOCK_MARKETS[mkt]?.label ?? '';
}

/** Check if a security code represents a US stock (alphabetic ticker). */
export function isUSStock(code: string): boolean {
  return detectStockMarket(code) === 'us';
}

/** Get the currency symbol for a given market. */
export function getCurrencySymbol(market?: string): string {
  if (market === 'us') return '$';
  return '¥';
}

export function fmt(v: number): string {
  if (Math.abs(v) < 0.01) return '¥ 0.00';
  return v > 0 ? `+¥ ${v.toFixed(2)}` : `-¥ ${Math.abs(v).toFixed(2)}`;
}

export function fmtShort(v: number): string {
  const r = Math.round(v);
  if (r === 0) return '0';
  return r > 0 ? `+${r}` : `${r}`;
}

// ═══════ US market helpers ═══════

/** Detect whether a date falls within US Eastern Daylight Time (DST).
 *  US DST starts on the 2nd Sunday of March and ends on the 1st Sunday of November.
 *  Uses UTC-based date arithmetic to avoid local timezone interference. */
export function isUSEasternDST(date: Date = new Date()): boolean {
  const year = date.getUTCFullYear();

  // 2nd Sunday of March
  const mar1 = new Date(Date.UTC(year, 2, 1)); // March = month 2
  const firstSundayMar = 1 + ((7 - mar1.getUTCDay()) % 7);
  const dstStart = new Date(Date.UTC(year, 2, firstSundayMar + 7));

  // 1st Sunday of November
  const nov1 = new Date(Date.UTC(year, 10, 1)); // November = month 10
  const firstSundayNov = 1 + ((7 - nov1.getUTCDay()) % 7);
  const dstEnd = new Date(Date.UTC(year, 10, firstSundayNov));

  return date >= dstStart && date < dstEnd;
}

/** US stock market trading hours check (naive, not holiday-aware).
 *  US markets are open 9:30am–4:00pm ET on weekdays.
 *  EDT (Mar–Nov): UTC-4 → 13:30–20:00 UTC.
 *  EST (Nov–Mar): UTC-5 → 14:30–21:00 UTC.
 */
export function isUSMarketOpen(): boolean {
  const now = new Date();
  const day = now.getUTCDay();
  // Weekend check
  if (day === 0 || day === 6) return false;

  const offset = isUSEasternDST(now) ? 4 : 5; // EDT = UTC-4, EST = UTC-5
  const etHours = now.getUTCHours() - offset;
  const etMinutes = now.getUTCMinutes();
  const totalMin = etHours * 60 + etMinutes;
  // Regular session: 9:30am–4:00pm ET
  return totalMin >= (9 * 60 + 30) && totalMin < (16 * 60);
}

/** Format a number as compact USD string (e.g. $1.2B, $350M) */
export function fmtUSDCompact(v: number): string {
  if (Math.abs(v) >= 1e12) return `$${(v / 1e12).toFixed(2)}T`;
  if (Math.abs(v) >= 1e9) return `$${(v / 1e9).toFixed(1)}B`;
  if (Math.abs(v) >= 1e6) return `$${(v / 1e6).toFixed(0)}M`;
  if (Math.abs(v) >= 1e3) return `$${(v / 1e3).toFixed(0)}K`;
  return `$${v.toFixed(2)}`;
}

/** Format a CNY amount with exchange rate conversion to USD */
export function fmtCNYtoUSD(cny: number, rate: number): string {
  return `$${(cny / rate).toFixed(2)}`;
}

/** Get US stock price change color (red/green) following CN convention */
export function usChangeColor(change: number): string {
  return change > 0 ? '#d63649' : change < 0 ? '#199c63' : 'var(--text-color-kumo-subtle)';
}

export function getDateRange(key: string, allDates: string[], txDates: string[]): [number, number] {
  if (key === 'tx' && txDates.length > 1) {
    // find nearest valid dates in NAV data (handles non-trading days)
    let i0 = allDates.findIndex(d => d >= txDates[0]);
    if (i0 < 0) i0 = 0;
    let i1 = allDates.length - 1;
    for (let j = allDates.length - 1; j >= 0; j--) {
      if (allDates[j] <= txDates[txDates.length - 1]) { i1 = j; break; }
    }
    i0 = Math.max(0, i0 - 10);
    i1 = Math.min(allDates.length - 1, i1 + 10);
    return [i0, i1];
  }
  const lastIdx = allDates.length - 1;
  const days: Record<string, number> = { '1m': 30, '3m': 90, '6m': 180, '1y': 365 };
  if (days[key] && allDates.length) {
    const last = allDates[lastIdx];
    const cutoff = new Date(last); cutoff.setDate(cutoff.getDate() - days[key]);
    const cutoffStr = cutoff.toISOString().substring(0, 10);
    let i0 = allDates.findIndex(d => d >= cutoffStr);
    return [i0 < 0 ? 0 : i0, lastIdx];
  }
  return [0, lastIdx];
}

/** Theme-aware chart colors */
export function chartColors(dark: boolean) {
  return {
    blue: dark ? '#4dabf7' : C.blue,
    up: dark ? '#f87171' : C.up,     // 深色背景用更亮的红
    down: dark ? '#4ade80' : C.down,  // 深色背景用更亮的绿
    amber: C.amber,
    text: dark ? '#e5e7eb' : '#374151',
    hairline: dark ? 'rgba(255,255,255,0.08)' : '#f3f4f6',
    gridBg: dark ? 'rgba(255,255,255,0.04)' : 'rgba(49,114,217,0.12)',
    gridBgEnd: dark ? 'rgba(255,255,255,0)' : 'rgba(49,114,217,0)',
    sliderBorder: dark ? 'rgba(255,255,255,0.12)' : '#e5e7eb',
    sliderBg: dark ? 'rgba(255,255,255,0.04)' : '#f9fafb',
    textSecondary: dark ? '#9ca3af' : '#6b7280',
  };
}

export function sharedAxis(dark: boolean) {
  return {
    axisLabel: { fontSize: 11, color: dark ? '#e5e7eb' : '#9ca3af' },
    splitLine: { lineStyle: { color: dark ? 'rgba(255,255,255,0.06)' : '#f3f4f6' } },
  };
}
