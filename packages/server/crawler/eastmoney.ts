/** eastmoney direct API client — no AKShare dependency */

import { cachedFetch, TTL } from "../utils/api-cache";

const UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36";

// ═══════════════════════════════════════════
// Fund APIs
// ═══════════════════════════════════════════

/** Real-time NAV estimate (no auth needed) */
export async function fetchRealtimeNav(code: string): Promise<{ nav: number; date: string; change_pct: number } | null> {
  return cachedFetch(`em:rtnav:${code}`, TTL.STOCK_QUOTE, async () => {
    const url = `https://fundgz.1234567.com.cn/js/${code}.js`;
    const res = await fetch(url, { headers: { "User-Agent": UA } });
    const text = await res.text();
    const match = text.match(/jsonpgz\((\{.*\})\)/);
    if (!match) return null;
    const d = JSON.parse(match[1]);
    return { nav: +d.dwjz, date: d.jzrq, change_pct: +d.gszzl };
  });
}

/** Master fund list — all ~20k funds */
export async function fetchFundMasterList(): Promise<{ code: string; name: string; type: string }[]> {
  const url = "https://fund.eastmoney.com/js/fundcode_search.js";
  const res = await fetch(url, { headers: { "User-Agent": UA } });
  const text = await res.text();
  const match = text.match(/var r = (\[.*?\]);/s);
  if (!match) return [];
  const raw: any[][] = JSON.parse(match[1]);
  return raw.map(([code, , name, type]) => ({ code: String(code).padStart(6, "0"), name, type }));
}

/** NAV history via pingzhongdata JS (no auth, most reliable) */
export async function fetchNavHistory(code: string): Promise<{ date: string; unit_nav: number; change_pct: number }[]> {
  return cachedFetch(`em:navhist:${code}`, TTL.NAV_HISTORY, async () => {
    const url = `https://fund.eastmoney.com/pingzhongdata/${code}.js`;
    const res = await fetch(url, { headers: { "User-Agent": UA } });
    const text = await res.text();

    // Standard funds: Data_netWorthTrend
    let match = text.match(/var Data_netWorthTrend = (\[.*?\]);/s);
    if (match) {
      const raw: any[] = JSON.parse(match[1]);
      return raw.map((r: any) => ({
        date: new Date(r.x).toISOString().substring(0, 10),
        unit_nav: +r.y,
        change_pct: r.equityReturn ? +r.equityReturn : 0,
      })).filter((d: any) => {
        if (d.unit_nav < 0.01 || d.unit_nav > 100) {
          console.warn(`[eastmoney] fetchNavHistory(${code}): outlier NAV=${d.unit_nav} on ${d.date}, skipped`);
          return false;
        }
        return true;
      });
    }

    // Money market funds (货币基金): Data_millionCopiesIncome
    // Unit NAV is always ~1.0; store 万份收益 as daily_change_pct for reference
    match = text.match(/var Data_millionCopiesIncome = (\[.*?\]);/s);
    if (match) {
      const raw: any[] = JSON.parse(match[1]);
      return raw.map((r: any) => ({
        date: new Date(r[0]).toISOString().substring(0, 10),
        unit_nav: 1.0,
        change_pct: r[1] ? +r[1] : 0,
      }));
    }

    return [];
  });
}

/** Paginated NAV history via API (needs Referer, more complete) */
export async function fetchNavHistoryPaginated(
  code: string, pageIndex = 1, pageSize = 5000,
): Promise<{ date: string; unit_nav: number; accumulated_nav: number; change_pct: string }[]> {
  const url = `https://api.fund.eastmoney.com/f10/lsjz?fundCode=${code}&pageIndex=${pageIndex}&pageSize=${pageSize}&_=${Date.now()}`;
  const res = await fetch(url, {
    headers: { "User-Agent": UA, Referer: `https://fundf10.eastmoney.com/jjjz_${code}.html` },
  });
  const json = await res.json();
  if (json.ErrCode !== 0 || !json.Data?.LSJZList) return [];

  return json.Data.LSJZList.map((r: any) => ({
    date: r.FSRQ,
    unit_nav: +r.DWJZ,
    accumulated_nav: +r.LJJZ,
    change_pct: r.JZZZL,
  }));
}

/** Fund basic info from pingzhongdata JS */
export async function fetchFundInfo(code: string): Promise<{ name: string; type: string; inception: string } | null> {
  const url = `https://fund.eastmoney.com/pingzhongdata/${code}.js`;
  const res = await fetch(url, { headers: { "User-Agent": UA } });
  const text = await res.text();

  const nameMatch = text.match(/var fS_name = "(.*?)";/);
  const codeMatch = text.match(/var fS_code = "(.*?)";/);
  if (!nameMatch || !codeMatch) return null;

  // Inception date
  const dateMatch = text.match(/var fS_buyMinDate = "(.*?)";/);
  return { name: nameMatch[1], type: codeMatch[1], inception: dateMatch?.[1] || "" };
}

// ═══════════════════════════════════════════
// Stock APIs (eastmoney push2 / push2his)
// ═══════════════════════════════════════════

/**
 * Market code mapping for eastmoney push2 secid format.
 * secid = {market}.{code}  e.g. "1.600519", "0.000001", "116.00700"
 *
 * | Market | Prefix | Typical code prefixes        |
 * |--------|--------|------------------------------|
 * | SH     | 1      | 60xxxx, 68xxxx               |
 * | SZ     | 0      | 00xxxx, 30xxxx, 20xxxx       |
 * | HK     | 116    | 5-digit numeric, e.g. 00700  |
 * | BJ     | 0      | 8xxxxx, 4xxxxx               |
 */
export type StockMarket = "SH" | "SZ" | "HK" | "BJ";

export function marketToSecidPrefix(market: StockMarket): string {
  switch (market) {
    case "SH": return "1";
    case "SZ": return "0";
    case "HK": return "116";
    case "BJ": return "0";
    default: return "1";
  }
}

/** Auto-detect market from stock code prefix */
export function detectMarket(code: string): StockMarket {
  const c = code.replace(/\D/g, "");
  if (c.length === 5 && /^\d{5}$/.test(c)) {
    // 5-digit codes are typically HK stocks (e.g. 00700)
    return "HK";
  }
  if (c.startsWith("60") || c.startsWith("68")) return "SH";
  if (c.startsWith("00") || c.startsWith("30") || c.startsWith("20")) return "SZ";
  if (c.startsWith("8") || c.startsWith("4")) return "BJ";
  return "SH";
}

export function buildSecid(code: string, market: StockMarket): string {
  return `${marketToSecidPrefix(market)}.${code.replace(/\D/g, "")}`;
}

/**
 * Stock realtime quote via push2.eastmoney.com.
 *
 * Endpoint: https://push2.eastmoney.com/api/qt/stock/get
 * Parameters:
 *   secid  = {market}.{code}  e.g. "1.600519", "0.000001", "116.00700"
 *   fields = comma-separated field codes
 *   ut     = configurable via EASTMONEY_UT_TOKEN env var (default: fa5fd1943c7b386f172d6893dbfba10b)
 *   fltt   = 2
 *
 * Key field codes:
 *   f43  = 最新价 (current price), stored as integer * 100
 *   f44  = 最高价 (day high)
 *   f45  = 最低价 (day low)
 *   f46  = 开盘价 (open)
 *   f57  = 股票代码
 *   f58  = 股票名称
 *   f169 = 涨跌额 (change amount)
 *   f170 = 涨跌幅 (change %)
 *   f47  = 成交量 (volume, lots)
 *   f48  = 成交额 (amount)
 *   f168 = 换手率 (turnover rate %)
 *   f115 = 市盈率 (PE)
 *   f20  = 总市值 (total market value)
 *   f21  = 流通市值 (circulating market value)
 *
 * Response: { data: { f43: 15680, f58: "贵州茅台", ... } }
 */
export async function fetchStockRealtime(
  code: string, market?: StockMarket,
): Promise<{
  code: string; market: string; name: string; price: number;
  open: number; high: number; low: number;
  change_pct: number; change_amt: number;
  volume: number; amount: number; turnover: number;
  pe: number; total_mv: number; circ_mv: number;
} | null> {
  const mkt = market || detectMarket(code);
  const secid = buildSecid(code, mkt);
  const cleanCode = code.replace(/\D/g, "");

  const fields = [
    "f43", "f44", "f45", "f46", "f57", "f58",
    "f169", "f170", "f47", "f48", "f168",
    "f115", "f20", "f21",
  ].join(",");

  const ut = process.env.EASTMONEY_UT_TOKEN || "fa5fd1943c7b386f172d6893dbfba10b";
  const url = `https://push2.eastmoney.com/api/qt/stock/get?secid=${secid}&fields=${fields}&ut=${ut}&fltt=2&_=${Date.now()}`;

  const res = await fetch(url, { headers: { "User-Agent": UA } });
  const json = await res.json();
  if (!json?.data) return null;

  const d = json.data;
  return {
    code: cleanCode,
    market: mkt,
    name: d.f58 || "",
    price: +((d.f43 || 0) / 100),
    open: +((d.f46 || 0) / 100),
    high: +((d.f44 || 0) / 100),
    low: +((d.f45 || 0) / 100),
    change_pct: +(d.f170 || 0),
    change_amt: +((d.f169 || 0) / 100),
    volume: +(d.f47 || 0),
    amount: +(d.f48 || 0),
    turnover: +(d.f168 || 0),
    pe: +(d.f115 || 0),
    total_mv: +(d.f20 || 0),
    circ_mv: +(d.f21 || 0),
  };
}

/**
 * Stock K-line (daily) via push2his.eastmoney.com.
 *
 * Endpoint: https://push2his.eastmoney.com/api/qt/stock/kline/get
 * Parameters:
 *   secid   = {market}.{code}
 *   klt     = 101 (daily), 102 (weekly), 103 (monthly)
 *   fqt     = 0 (not adjusted), 1 (forward-adjusted / 前复权), 2 (backward-adjusted)
 *   beg     = start date YYYYMMDD
 *   end     = end date YYYYMMDD
 *   lmt     = max records (0 = all)
 *   fields1 = "f1,f2,f3,f4,f5" → code, market, name, decimal, dktotal
 *   fields2 = "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61"
 *             f51=日期, f52=开盘, f53=收盘, f54=最高, f55=最低,
 *             f56=成交量, f57=成交额, f58=振幅%, f59=涨跌幅%, f60=涨跌额, f61=换手率%
 *
 * Response:
 *   { data: { code, market, name, klines: ["2023-01-09,80.50,82.10,...", ...] } }
 *   Each kline string is comma-separated in fields2 order.
 */
export interface KlineRecord {
  date: string;
  open: number;
  close: number;
  high: number;
  low: number;
  volume: number;
  amount: number;
  amplitude: number;
  change_pct: number;
  change_amt: number;
  turnover_rate: number;
}

export async function fetchStockKline(
  code: string,
  market?: StockMarket,
  opts?: { beg?: string; end?: string; lmt?: number; fqt?: 0 | 1 | 2 },
): Promise<{ code: string; market: string; name: string; klines: KlineRecord[] } | null> {
  const mkt = market || detectMarket(code);
  const secid = buildSecid(code, mkt);
  const cleanCode = code.replace(/\D/g, "");

  const beg = opts?.beg || "19900101";
  const end = opts?.end || "20500101";
  const lmt = opts?.lmt ?? 10000;
  const fqt = opts?.fqt ?? 1;

  const fields1 = "f1,f2,f3,f4,f5";
  const fields2 = "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61";

  const url = `https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=${secid}&fields1=${fields1}&fields2=${fields2}&klt=101&fqt=${fqt}&beg=${beg}&end=${end}&lmt=${lmt}&ut=fa5fd1943c7b386f172d6893dbfba10b&_=${Date.now()}`;

  const res = await fetch(url, { headers: { "User-Agent": UA, Referer: "https://quote.eastmoney.com/" } });
  const json = await res.json();
  if (!json?.data) return null;

  const d = json.data;
  const klines: KlineRecord[] = (d.klines || []).map((line: string) => {
    const parts = line.split(",");
    return {
      date: parts[0] || "",
      open: +(parts[1] || 0),
      close: +(parts[2] || 0),
      high: +(parts[3] || 0),
      low: +(parts[4] || 0),
      volume: +(parts[5] || 0),
      amount: +(parts[6] || 0),
      amplitude: +(parts[7] || 0),
      change_pct: +(parts[8] || 0),
      change_amt: +(parts[9] || 0),
      turnover_rate: +(parts[10] || 0),
    };
  });

  return {
    code: cleanCode,
    market: mkt,
    name: d.name || "",
    klines,
  };
}

/**
 * Map stock K-line data to nav_history-compatible format.
 * close price → unit_nav, change_pct → daily_change_pct.
 * This lets stocks and funds share the same nav_history table for unified charting.
 */
export function stockKlineToNavHistory(
  klines: KlineRecord[],
): { date: string; unit_nav: number; change_pct: number }[] {
  return klines.map(k => ({
    date: k.date,
    unit_nav: k.close,
    change_pct: k.change_pct,
  }));
}

// ═══════════════════════════════════════════
// Fund Holdings (持仓)
// ═══════════════════════════════════════════

export interface FundHolding {
  stock_code: string;
  stock_name: string;
  weight_pct: number;
  shares: number;       // 万股
  market_value: number;  // 万元人民币
}

export interface FundHoldingsResult {
  fund_code: string;
  fund_name: string;
  report_date: string;  // e.g. "2026-03-31"
  holdings: FundHolding[];
}

interface FeederEtfMeta {
  etfCode: string;
  fundName: string;
  stockPositionPct: number;
}

function textFromHtml(html: string): string {
  return html
    .replace(/<script[\s\S]*?<\/script>/gi, "")
    .replace(/<style[\s\S]*?<\/style>/gi, "")
    .replace(/<[^>]+>/g, "")
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/&#37;/g, "%")
    .trim();
}

function parseNumericCell(value: string): number {
  const cleaned = value.replace(/,/g, "").replace(/%/g, "").replace(/[^\d.-]/g, "");
  return parseFloat(cleaned) || 0;
}

function roundPct(value: number): number {
  return Math.round((value + Number.EPSILON) * 100) / 100;
}

function parseHoldingsApiText(code: string, text: string): FundHoldingsResult | null {
  if (!text) return null;

  const contentMatch = text.match(/var apidata=\{\s*content:"(.*?)",\s*arryear:/s);
  if (!contentMatch) return null;

  // Unescape JSON-string-escaped content (the content is a JSON string within JS)
  const rawContent = contentMatch[1]
    .replace(/\\"/g, '"')
    .replace(/\\\//g, "/")
    .replace(/\\n/g, "\n")
    .replace(/\\t/g, "\t")
    .replace(/\\\\/g, "\\");

  // Extract fund name from the title link
  const nameMatch = rawContent.match(/title='([^']+)'/);
  const fundName = nameMatch?.[1] || "";

  // Extract report date from the label text: 截止至：<font class='px12'>2026-03-31</font>
  const dateMatch = rawContent.match(/截止至：[^<]*<font[^>]*>(\d{4}-\d{2}-\d{2})<\/font>/);
  const reportDate = dateMatch?.[1] || "";

  const holdings: FundHolding[] = [];
  const rowPattern = /<tr\b[^>]*>([\s\S]*?)<\/tr>/gi;
  let rowMatch;
  while ((rowMatch = rowPattern.exec(rawContent)) !== null) {
    const cells = [...rowMatch[1].matchAll(/<td\b[^>]*>([\s\S]*?)<\/td>/gi)].map(m => m[1]);
    if (cells.length < 6) continue;

    const seq = textFromHtml(cells[0]);
    if (!/^\d+$/.test(seq)) continue;

    const stockCode = textFromHtml(cells[1]);
    const stockName = textFromHtml(cells[2]);
    const weightPct = parseNumericCell(cells[cells.length - 3]);
    const shares = parseNumericCell(cells[cells.length - 2]);
    const marketValue = parseNumericCell(cells[cells.length - 1]);

    if (!stockCode || !stockName || (weightPct <= 0 && shares <= 0 && marketValue <= 0)) continue;

    holdings.push({
      stock_code: stockCode,
      stock_name: stockName,
      weight_pct: weightPct,
      shares,
      market_value: marketValue,
    });
  }

  if (!holdings.length) {
    if (reportDate) {
      console.warn(
        `[eastmoney] fetchFundHoldings(${code}): report date found (${reportDate}) but holdings array is empty — possible HTML format change. ` +
        `Raw content first 500 chars: ${rawContent.substring(0, 500)}`,
      );
    }
    return null;
  }

  return {
    fund_code: code,
    fund_name: fundName,
    report_date: reportDate,
    holdings,
  };
}

async function fetchFundHoldingsDirect(code: string): Promise<FundHoldingsResult | null> {
  const url = `https://fundf10.eastmoney.com/FundArchivesDatas.aspx?type=jjcc&code=${code}&topline=50&year=&month=&rt=${Date.now()}`;
  const res = await fetch(url, {
    headers: { "User-Agent": UA, Referer: `https://fundf10.eastmoney.com/ccmx_${code}.html` },
  });
  return parseHoldingsApiText(code, await res.text());
}

function parseLatestStockPositionPct(text: string): number {
  const match = text.match(/Data_fundSharesPositions\s*=\s*(\[[\s\S]*?\]);/);
  if (!match) return 0;
  try {
    const points = JSON.parse(match[1]) as unknown;
    if (!Array.isArray(points) || !points.length) return 0;
    const latest = points[points.length - 1];
    if (!Array.isArray(latest) || latest.length < 2) return 0;
    const pct = Number(latest[1]);
    return Number.isFinite(pct) && pct > 0 ? pct : 0;
  } catch {
    return 0;
  }
}

async function fetchFeederEtfMeta(code: string): Promise<FeederEtfMeta | null> {
  const pageRes = await fetch(`https://fund.eastmoney.com/${code}.html`, {
    headers: { "User-Agent": UA, Referer: `https://fundf10.eastmoney.com/ccmx_${code}.html` },
  });
  const pageText = await pageRes.text();
  const etfMatch = pageText.match(/href=["']https?:\/\/fund\.eastmoney\.com\/(\d{6})\.html[^"']*["'][^>]*>\s*查看相关ETF/i);
  if (!etfMatch || etfMatch[1] === code) return null;

  const dataRes = await fetch(`https://fund.eastmoney.com/pingzhongdata/${code}.js?v=${Date.now()}`, {
    headers: { "User-Agent": UA, Referer: `https://fund.eastmoney.com/${code}.html` },
  });
  const dataText = await dataRes.text();
  const nameMatch = dataText.match(/var fS_name\s*=\s*"([^"]+)"/);
  const stockPositionPct = parseLatestStockPositionPct(dataText);
  if (stockPositionPct <= 0) return null;

  return {
    etfCode: etfMatch[1],
    fundName: nameMatch?.[1] || "",
    stockPositionPct,
  };
}

async function fetchFeederEtfProxyHoldings(code: string): Promise<FundHoldingsResult | null> {
  const meta = await fetchFeederEtfMeta(code);
  if (!meta) return null;

  const etfHoldings = await fetchFundHoldingsDirect(meta.etfCode);
  if (!etfHoldings?.holdings.length) return null;

  const scale = meta.stockPositionPct / 100;
  return {
    fund_code: code,
    fund_name: meta.fundName || etfHoldings.fund_name,
    report_date: etfHoldings.report_date,
    holdings: etfHoldings.holdings.map(h => ({
      stock_code: h.stock_code,
      stock_name: h.stock_name,
      weight_pct: roundPct(h.weight_pct * scale),
      shares: 0,
      market_value: 0,
    })).filter(h => h.weight_pct > 0),
  };
}

/**
 * Fetch fund holdings (持仓明细) from eastmoney FundArchivesDatas API.
 *
 * Endpoint: https://fundf10.eastmoney.com/FundArchivesDatas.aspx?type=jjcc&code=XXXXXX&topline=50&year=&month=
 *
 * Returns HTML table wrapped in `var apidata={ content:"...", ... };
 * The table columns: 序号, 股票代码, 股票名称, 最新价, 涨跌幅, 相关资讯, 占净值比例, 持股数(万股), 持仓市值(万元)
 *
 * For ETF feeder funds whose direct stock table is empty, Eastmoney exposes a
 * related ETF link on the fund page. In that case we proxy through the related
 * ETF holdings and scale each stock weight by the feeder fund stock-position
 * estimate from pingzhongdata.
 */
export async function fetchFundHoldings(code: string): Promise<FundHoldingsResult | null> {
  const direct = await fetchFundHoldingsDirect(code);
  if (direct) return direct;
  return fetchFeederEtfProxyHoldings(code);
}
