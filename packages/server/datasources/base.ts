/**
 * DataSource — abstract interface for all external price/quote providers.
 *
 * Every data source (eastmoney, yahoo, etc.) implements this interface.
 * The crawler routes requests to the correct source based on market.
 */

// ── Quote (single snapshot) ────────────────────────────────────────

export interface Quote {
  /** Security code in source-native format (e.g. "600519", "NVDA") */
  code: string;
  /** Market identifier: SH | SZ | HK | US */
  market: string;
  /** Human-readable name */
  name: string;
  /** Current/latest price */
  price: number;
  /** Price change percentage (e.g. 2.5 = +2.5%) */
  changePct: number;
  /** Price change amount in source currency */
  changeAmt: number;
  /** ISO currency code: CNY, USD, HKD */
  currency: string;
  /** ISO timestamp of the quote */
  updatedAt: string;
  /** Optional: day open/high/low */
  open?: number;
  high?: number;
  low?: number;
  /** Optional: volume, previous close */
  volume?: number;
  previousClose?: number;
}

// ── History point ──────────────────────────────────────────────────

export interface HistoryPoint {
  /** ISO date YYYY-MM-DD */
  date: string;
  /** Closing price */
  price: number;
  /** Daily change % (optional, can be computed) */
  changePct: number;
}

// ── DataSource interface ───────────────────────────────────────────

export interface DataSource {
  /** Unique name for this source, e.g. "eastmoney", "yahoo" */
  readonly name: string;

  /** Fetch a real-time quote for a security */
  fetchQuote(code: string, market?: string): Promise<Quote | null>;

  /** Fetch historical price data */
  fetchHistory(code: string, market?: string, days?: number): Promise<HistoryPoint[]>;

  /** Market identifier for source-specific routing */
  readonly markets: string[];
}

// ── Registry ───────────────────────────────────────────────────────

const registry: Map<string, DataSource> = new Map();

export function registerSource(source: DataSource): void {
  for (const mkt of source.markets) {
    registry.set(mkt, source);
  }
}

export function getSource(market: string): DataSource | undefined {
  // Direct match first
  const direct = registry.get(market);
  if (direct) return direct;
  // Fallback: try matching by prefix
  for (const [mkt, src] of registry) {
    if (market.startsWith(mkt)) return src;
  }
  return undefined;
}

/** Detect market from code pattern */
export function detectMarket(code: string): string {
  const c = code.replace(/\D/g, "");
  if (/^6\d{5}$/.test(c)) return "SH";
  if (/^[03]\d{5}$/.test(c)) return "SZ";
  if (/^\d{5}$/.test(c)) return "HK";
  if (/^[A-Z]{1,5}$/.test(code)) return "US";
  // 6-digit fund codes default to CN fund
  if (/^\d{6}$/.test(c)) return "CN";
  return "CN";
}
