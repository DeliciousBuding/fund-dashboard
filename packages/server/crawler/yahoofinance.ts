/** Yahoo Finance v8 chart API client — US stocks, indexes, and exchange rates */

const UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36";

// ═══════════════════════════════════════════
// Types
// ═══════════════════════════════════════════

export interface StockQuote {
  symbol: string;
  name: string;
  price: number;
  previousClose: number;
  change: number;
  change_pct: number;
  high: number;
  low: number;
  open: number;
  volume: number;
  currency: string;
  marketTime: string;
}

export interface NavHistoryRow {
  date: string;
  close: number;
  /** close → unit_nav mapping for DB compatibility */
  unit_nav: number;
  change_pct: number;
  open: number;
  high: number;
  low: number;
  volume: number;
}

// ═══════════════════════════════════════════
// Internal helpers
// ═══════════════════════════════════════════

interface YahooChartResult {
  meta: {
    currency: string;
    symbol: string;
    exchangeName: string;
    instrumentType: string;
    regularMarketPrice: number;
    regularMarketTime: number;
    regularMarketDayHigh: number;
    regularMarketDayLow: number;
    regularMarketVolume: number;
    chartPreviousClose: number;
    longName?: string;
    shortName?: string;
    dataGranularity: string;
    range: string;
  };
  timestamp: number[];
  indicators: {
    quote: Array<{
      open: number[];
      high: number[];
      low: number[];
      close: number[];
      volume: number[];
    }>;
    adjclose?: Array<{
      adjclose: number[];
    }>;
  };
}

interface YahooChartResponse {
  chart: {
    result: YahooChartResult[] | null;
    error: { code: string; description: string } | null;
  };
}

async function fetchChart(symbol: string, range = "1d", interval = "1d"): Promise<YahooChartResult> {
  const url = `https://query1.finance.yahoo.com/v8/finance/chart/${encodeURIComponent(symbol)}?range=${encodeURIComponent(range)}&interval=${encodeURIComponent(interval)}`;
  const res = await fetch(url, { headers: { "User-Agent": UA } });
  if (!res.ok) {
    throw new Error(`Yahoo chart API HTTP ${res.status} for ${symbol}`);
  }
  const json = (await res.json()) as YahooChartResponse;
  if (json.chart.error) {
    throw new Error(`Yahoo chart API error for ${symbol}: ${json.chart.error.description}`);
  }
  if (!json.chart.result || json.chart.result.length === 0) {
    throw new Error(`Yahoo chart API returned no result for ${symbol}`);
  }
  return json.chart.result[0];
}

function formatDate(ts: number): string {
  return new Date(ts * 1000).toISOString().substring(0, 10);
}

// ═══════════════════════════════════════════
// 1. US Stock Quote
// ═══════════════════════════════════════════

/**
 * Fetch current quote for a US stock.
 * Returns price, change (absolute), change_pct, volume, and OHLC.
 */
export async function fetchUSStockQuote(symbol: string): Promise<StockQuote | null> {
  try {
    const result = await fetchChart(symbol, "1d", "1d");
    const { meta } = result;
    const q = result.indicators.quote[0];

    const price = meta.regularMarketPrice;
    const previousClose = meta.chartPreviousClose;
    const change = price - previousClose;
    const change_pct = previousClose > 0 ? (change / previousClose) * 100 : 0;

    // Use last non-null open value from the quote arrays (intraday may have multiple candles)
    const open = q.open.filter(v => v !== null).pop() ?? 0;
    const high = meta.regularMarketDayHigh;
    const low = meta.regularMarketDayLow;

    return {
      symbol: meta.symbol,
      name: meta.longName || meta.shortName || meta.symbol,
      price,
      previousClose,
      change: Math.round(change * 100) / 100,
      change_pct: Math.round(change_pct * 100) / 100,
      high,
      low,
      open,
      volume: meta.regularMarketVolume,
      currency: meta.currency,
      marketTime: formatDate(meta.regularMarketTime),
    };
  } catch (e: any) {
    console.warn(`fetchUSStockQuote(${symbol}) failed: ${e.message}`);
    return null;
  }
}

// ═══════════════════════════════════════════
// 2. US Stock History
// ═══════════════════════════════════════════

/**
 * Fetch historical daily data for a US stock.
 * Mapping: close → unit_nav for nav_history compatibility.
 *
 * @param symbol - US stock ticker, e.g. "AAPL", "MSFT"
 * @param range - Yahoo range string: "1d","5d","1mo","3mo","6mo","1y","2y","5y","10y","ytd","max"
 */
export async function fetchUSStockHistory(
  symbol: string,
  range = "1y",
): Promise<NavHistoryRow[]> {
  try {
    const result = await fetchChart(symbol, range, "1d");
    const q = result.indicators.quote[0];
    const adjclose = result.indicators.adjclose?.[0]?.adjclose;

    // Use adjusted close for history when available (splits/dividends), fall back to close
    const closes = adjclose ?? q.close;

    const rows: NavHistoryRow[] = [];
    for (let i = 0; i < result.timestamp.length; i++) {
      const c = closes[i];
      if (c == null) continue;
      const date = formatDate(result.timestamp[i]);

      // Compute change_pct from previous close
      let change_pct = 0;
      if (i > 0 && closes[i - 1] != null) {
        const prev = closes[i - 1]!;
        if (prev > 0) {
          change_pct = Math.round(((c - prev) / prev) * 100 * 100) / 100;
        }
      }

      rows.push({
        date,
        close: Math.round(c * 100) / 100,
        unit_nav: Math.round(c * 100) / 100,
        change_pct,
        open: q.open[i] != null ? Math.round(q.open[i]! * 100) / 100 : 0,
        high: q.high[i] != null ? Math.round(q.high[i]! * 100) / 100 : 0,
        low: q.low[i] != null ? Math.round(q.low[i]! * 100) / 100 : 0,
        volume: q.volume[i] ?? 0,
      });
    }
    return rows;
  } catch (e: any) {
    console.warn(`fetchUSStockHistory(${symbol}, ${range}) failed: ${e.message}`);
    return [];
  }
}

// ═══════════════════════════════════════════
// 3. Index Quote
// ═══════════════════════════════════════════

/**
 * Fetch current quote for a Yahoo-listed index.
 * Supports: ^NDX (NASDAQ 100), ^GSPC (S&P 500), ^DJI (Dow Jones), ^IXIC (NASDAQ Composite), etc.
 */
export async function fetchIndexQuote(symbol: string): Promise<StockQuote | null> {
  // Indexes use the same chart API, just with caret-prefixed symbols
  return fetchUSStockQuote(symbol);
}

// ═══════════════════════════════════════════
// 4. Exchange Rate (USD/CNY)
// ═══════════════════════════════════════════

/**
 * Fetch USD/CNY exchange rate from Yahoo Finance.
 * Symbol: "USDCNY=X"
 * Returns the current mid-market rate.
 */
export async function fetchExchangeRate(): Promise<{
  rate: number;
  previousClose: number;
  change_pct: number;
  date: string;
} | null> {
  try {
    const result = await fetchChart("USDCNY=X", "1d", "1d");
    const { meta } = result;

    const rate = meta.regularMarketPrice;
    const previousClose = meta.chartPreviousClose;
    const change_pct = previousClose > 0 ? ((rate - previousClose) / previousClose) * 100 : 0;

    return {
      rate: Math.round(rate * 10000) / 10000,
      previousClose,
      change_pct: Math.round(change_pct * 100) / 100,
      date: formatDate(meta.regularMarketTime),
    };
  } catch (e: any) {
    console.warn(`fetchExchangeRate() failed: ${e.message}`);
    return null;
  }
}

// ═══════════════════════════════════════════
// 5. Index History (for nav_history format)
// ═══════════════════════════════════════════

/**
 * Fetch historical daily data for an index, formatted as nav_history rows.
 * Symbol examples: "^NDX", "^GSPC", "^DJI"
 */
export async function fetchIndexHistory(
  symbol: string,
  range = "1y",
): Promise<NavHistoryRow[]> {
  return fetchUSStockHistory(symbol, range);
}
