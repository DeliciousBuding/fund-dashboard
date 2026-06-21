/** MCP Market Tools — indices, penetration, US stock, stock search, fund holdings crawl */
import { z } from "zod";
import { padCode, getUSMarketSchedule, type ToolRegistrar } from "../shared";
import { query, queryOne, getRwDb } from "../../db";
import { log } from "../../middleware/logger";
import { fetchIndexQuote, fetchUSStockQuote, fetchUSStockHistory } from "../../crawler/yahoofinance";
import { fetchStockRealtime, fetchFundHoldings, type StockMarket } from "../../crawler/eastmoney";
export const registerMarketTools: ToolRegistrar = (server) => {
  server.tool("get_market_indices", {
    description: "获取美股主要指数实时行情（纳斯达克100 ^NDX、标普500 ^GSPC）。返回最新价格、涨跌幅、涨跌额。数据来源：Yahoo Finance。",
    inputSchema: z.object({}),
    handler: async () => {
      const results: Record<string, any> = {};
      const indices = [
        { symbol: "^NDX", name: "纳斯达克100" },
        { symbol: "^GSPC", name: "标普500" },
        { symbol: "^DJI", name: "道琼斯工业" },
        { symbol: "^IXIC", name: "纳斯达克综合" },
      ];
      for (const idx of indices) {
        try {
          const q = await fetchIndexQuote(idx.symbol);
          if (q) {
            results[idx.symbol] = {
              name: idx.name, price: q.price, change: q.change, change_pct: q.change_pct,
              high: q.high, low: q.low, market_time: q.marketTime, currency: q.currency,
            };
            getRwDb().run(
              "INSERT OR REPLACE INTO indices (code, name, market, price, change_pct, change_amt, updated_at) VALUES (?, ?, ?, ?, ?, ?, datetime('now'))",
              [idx.symbol, idx.name, "US", q.price, q.change_pct, q.change],
            );
          }
        } catch (e: any) {
          results[idx.symbol] = { error: e.message };
        }
      }
      const now = new Date();
      return { content: [{ type: "text", text: JSON.stringify({
        indices: results,
        market_status: getUSMarketSchedule(now),
        fetched_at: now.toISOString(),
      }, null, 2) }] };
    },
  });
  server.tool("get_portfolio_penetration", {
    description: '股权穿透分析——通过基金持仓数据，穿透计算底层股票的实际持有权重和金额，回答"我到底买了什么？"。基于 fund_holdings 表计算每只底层股票在所有基金中的合计权重和市值。',
    inputSchema: z.object({
      limit: z.number().default(30).describe("返回前N只股票的穿透结果"),
      sort_by: z.enum(["market_value", "weight_pct"]).default("market_value").describe("排序方式：market_value=持仓市值降序, weight_pct=权重降序"),
    }),
    handler: async (args) => {
      const held = query<any>(
        `SELECT ps.fund_code, ps.fund_name, ps.held_shares, ps.latest_nav, ps.current_value,
                COALESCE(fd.fund_type, '') as fund_type
         FROM portfolio_snapshot ps
         JOIN fund_details fd ON ps.fund_code = fd.fund_code
         WHERE ps.held_shares > 0.001 AND fd.security_type = 'fund'`,
      );

      const holdings = query<any>(
        `SELECT fh.fund_code, fh.stock_code, fh.stock_name, fh.weight_pct, fh.shares, fh.market_value, fh.report_date
         FROM fund_holdings fh
         WHERE fh.fund_code IN (SELECT fund_code FROM portfolio_snapshot WHERE held_shares > 0.001)
           AND fh.report_date = (SELECT MAX(fh2.report_date) FROM fund_holdings fh2 WHERE fh2.fund_code = fh.fund_code)`,
      );

      if (!holdings.length) {
        return { content: [{ type: "text", text: JSON.stringify({
          error: "no_holdings_data",
          message: "暂无基金持仓数据。请先运行 crawl_fund_holdings 爬取基金持仓明细。",
        }, null, 2) }] };
      }

      const fundValue: Record<string, number> = {};
      for (const f of held) {
        fundValue[f.fund_code] = f.current_value || (f.held_shares * f.latest_nav) || 0;
      }

      const stockAgg: Record<string, { stock_name: string; total_market_value: number; total_weight_pct: number; fund_count: number; funds: string[] }> = {};
      for (const h of holdings) {
        const fv = fundValue[h.fund_code];
        if (!fv || fv <= 0) continue;
        const key = h.stock_code;
        if (!stockAgg[key]) {
          stockAgg[key] = { stock_name: h.stock_name, total_market_value: 0, total_weight_pct: 0, fund_count: 0, funds: [] };
        }
        const effectiveValue = fv * (h.weight_pct / 100);
        stockAgg[key].total_market_value += effectiveValue;
        stockAgg[key].total_weight_pct += h.weight_pct;
        stockAgg[key].fund_count++;
        if (!stockAgg[key].funds.includes(h.fund_code)) {
          stockAgg[key].funds.push(h.fund_code);
        }
      }

      const totalPortfolioValue = Object.values(fundValue).reduce((a, b) => a + b, 0);
      const sorted = Object.entries(stockAgg)
        .sort(([, a], [, b]) => args.sort_by === "market_value" ? b.total_market_value - a.total_market_value : b.total_weight_pct - a.total_weight_pct)
        .slice(0, args.limit);

      const sectorRows = query<any>("SELECT stock_code, market, sector FROM sector_map");
      const sectorLookup: Record<string, string> = {};
      for (const s of sectorRows) sectorLookup[s.stock_code] = s.sector;

      return { content: [{ type: "text", text: JSON.stringify({
        generated_at: new Date().toISOString(),
        report_quarter: (() => {
          const lr = queryOne<any>("SELECT MAX(report_date) as rd FROM fund_holdings");
          if (lr?.rd) {
            const rd = String(lr.rd).substring(0, 10);
            const m = rd.substring(5, 7);
            const y = rd.substring(0, 4);
            return `${y}Q${Math.ceil(parseInt(m) / 3)}`;
          }
          return null;
        })(),
        total_portfolio_value_cny: +totalPortfolioValue.toFixed(2),
        funds_with_holdings: held.filter(f => fundValue[f.fund_code] > 0).length,
        stocks_found: Object.keys(stockAgg).length,
        top10_coverage_note: "仅披露各基金前十大持仓，实际穿透暴露可能更高。数据来源：天天基金季度报告。",
        penetration: sorted.map(([code, agg]) => {
          const sector = sectorLookup[code] || "其他";
          return {
            stock_code: code, stock_name: agg.stock_name, sector,
            estimated_market_value_cny: +agg.total_market_value.toFixed(0),
            penetration_pct: totalPortfolioValue > 0 ? +((agg.total_market_value / totalPortfolioValue) * 100).toFixed(2) : 0,
            cumulative_weight_pct: +agg.total_weight_pct.toFixed(2),
            fund_count: agg.fund_count, held_via_funds: agg.funds,
          };
        }),
        by_sector: (() => {
          const sectorAgg: Record<string, { total_exposure: number; stock_count: number }> = {};
          for (const [code, agg] of Object.entries(stockAgg)) {
            const sector = sectorLookup[code] || "其他";
            if (!sectorAgg[sector]) sectorAgg[sector] = { total_exposure: 0, stock_count: 0 };
            sectorAgg[sector].total_exposure += agg.total_market_value;
            sectorAgg[sector].stock_count += 1;
          }
          return Object.entries(sectorAgg)
            .map(([sector, a]) => ({
              sector,
              total_exposure_cny: +a.total_exposure.toFixed(0),
              penetration_pct: totalPortfolioValue > 0 ? +((a.total_exposure / totalPortfolioValue) * 100).toFixed(2) : 0,
              stock_count: a.stock_count,
            }))
            .sort((a, b) => b.total_exposure_cny - a.total_exposure_cny);
        })(),
        unavailable_funds: (() => {
          const uf: { fund_code: string; fund_name: string; reason: string }[] = [];
          for (const f of held) {
            const ft = (f.fund_type || "").toLowerCase();
            if (ft.includes("债券") || ft.includes("货币")) {
              uf.push({ fund_code: f.fund_code, fund_name: f.fund_name, reason: "bond_or_money_market" });
            } else if (!holdings.some((h: any) => h.fund_code === f.fund_code)) {
              uf.push({ fund_code: f.fund_code, fund_name: f.fund_name, reason: "no_holdings_data" });
            }
          }
          return uf;
        })(),
      }, null, 2) }] };
    },
  });

  server.tool("get_us_stock", {
    description: "获取美股实时行情和历史K线数据。输入美股代码（如 AAPL、MSFT、TSLA），返回最新报价和可选历史数据。数据来源：Yahoo Finance。",
    inputSchema: z.object({
      symbol: z.string().describe("美股代码，如 AAPL、MSFT、TSLA、GOOGL"),
      range: z.enum(["1d", "5d", "1mo", "3mo", "6mo", "1y", "2y", "5y", "ytd", "max"]).default("1y").describe("历史数据范围"),
      include_history: z.boolean().default(true).describe("是否返回历史K线数据，默认true"),
    }),
    handler: async (args) => {
      const symbol = args.symbol.toUpperCase().trim();
      const quote = await fetchUSStockQuote(symbol);
      let history: any[] = [];
      if (args.include_history) {
        const rows = await fetchUSStockHistory(symbol, args.range);
        history = rows.map(r => ({ date: r.date, close: r.close, change_pct: r.change_pct }));
      }
      if (!quote && !history.length) {
        return { content: [{ type: "text", text: JSON.stringify({ error: "no_data", symbol, message: `无法获取 ${symbol} 的数据，请检查代码是否正确` }) }] };
      }
      return { content: [{ type: "text", text: JSON.stringify({
        symbol,
        quote: quote ? {
          name: quote.name, price: quote.price, previous_close: quote.previousClose,
          change: quote.change, change_pct: quote.change_pct,
          open: quote.open, high: quote.high, low: quote.low,
          volume: quote.volume, currency: quote.currency, market_time: quote.marketTime,
        } : null,
        history: history.length ? {
          range: args.range, count: history.length,
          first_date: history[history.length - 1]?.date, last_date: history[0]?.date,
          data: history,
        } : null,
      }, null, 2) }] };
    },
  });

  server.tool("search_stocks", {
    description: "搜索美股/A股/港股股票，按名称或代码查找。优先查询本地数据库的 stock_profile 表，如无结果则调用东方财富实时搜索接口。支持中英文名称和代码前缀匹配。",
    inputSchema: z.object({
      query: z.string().describe("搜索关键词——股票名称（中/英文）、代码或代码前缀"),
      market: z.enum(["US", "SH", "SZ", "HK", "all"]).default("all").describe("限定市场：US=美股, SH=上海, SZ=深圳, HK=香港, all=全部市场"),
      limit: z.number().default(15).describe("返回条数上限"),
    }),
    handler: async (args) => {
      const q = `%${args.query}%`;
      const markets = args.market === "all" ? ["SH", "SZ", "HK", "US"] : [args.market];

      const localResults = query<any>(
        `SELECT code, name, market, sector, industry, market_cap, pe, description
         FROM stock_profile
         WHERE (name LIKE ? OR code LIKE ? OR code LIKE (? || '%'))
           AND market IN (${markets.map(() => "?").join(",")})
         LIMIT ?`,
        q, q, args.query, ...markets, args.limit * 2,
      );

      const localSecs = query<any>(
        `SELECT fund_code as code, fund_name as name, market, fund_type as type, security_type
         FROM fund_details
         WHERE (fund_name LIKE ? OR fund_code LIKE ? OR fund_code LIKE (? || '%'))
           AND security_type = 'stock'
           AND market IN (${markets.map(() => "?").join(",")})
         LIMIT ?`,
        q, q, args.query, ...markets, args.limit * 2,
      );

      const merged = new Map<string, any>();
      for (const r of [...localResults, ...localSecs]) {
        const key = `${r.code}_${r.market}`;
        if (!merged.has(key)) {
          merged.set(key, { code: r.code, name: r.name, market: r.market, sector: r.sector || null, industry: r.industry || null, market_cap: r.market_cap || null, pe: r.pe || null, source: "local" });
        }
      }

      if (merged.size < args.limit && markets.some(m => ["SH", "SZ", "HK"].includes(m))) {
        for (const mkt of markets.filter(m => ["SH", "SZ", "HK"].includes(m))) {
          try {
            const rt = await fetchStockRealtime(args.query, mkt as StockMarket);
            if (rt && rt.name) {
              const key = `${rt.code}_${rt.market}`;
              if (!merged.has(key)) {
                merged.set(key, { code: rt.code, name: rt.name, market: rt.market, price: rt.price, change_pct: rt.change_pct, pe: rt.pe, total_mv: rt.total_mv, source: "eastmoney_realtime" });
              }
            }
          } catch {}
        }
      }

      if (merged.size < args.limit && markets.includes("US")) {
        try {
          const yq = await fetchUSStockQuote(args.query.toUpperCase());
          if (yq) {
            const key = `${args.query.toUpperCase()}_US`;
            if (!merged.has(key)) {
              merged.set(key, { code: args.query.toUpperCase(), name: yq.name, market: "US", price: yq.price, change_pct: yq.change_pct, source: "yahoo" });
            }
          }
        } catch {}
      }

      const results = Array.from(merged.values()).slice(0, args.limit);
      return { content: [{ type: "text", text: JSON.stringify({ query: args.query, market_filter: args.market, count: results.length, results }, null, 2) }] };
    },
  });

  server.tool("crawl_fund_holdings", {
    description: "爬取基金持仓明细——从东方财富获取指定基金或全部权益类基金的季度持仓报告，存入 fund_holdings 表。数据用于股权穿透分析。",
    inputSchema: z.object({
      fund_code: z.string().optional().describe("6位基金代码，不提供则爬取全部权益类持仓基金的持仓"),
      all: z.boolean().default(false).describe("设为 true 爬取全部权益类持仓基金（QDII/股票/混合/指数/ETF）"),
    }),
    handler: async (args) => {
      if (args.fund_code) {
        const code = padCode(args.fund_code);
        const result = await fetchFundHoldings(code);
        if (!result || !result.holdings.length) {
          return { content: [{ type: "text", text: JSON.stringify({ ok: false, fund_code: code, message: "该基金暂无持仓数据或不是权益类基金" }) }] };
        }
        const db = getRwDb();
        const insert = db.prepare("INSERT OR REPLACE INTO fund_holdings (fund_code, stock_code, stock_name, weight_pct, shares, market_value, report_date) VALUES (?, ?, ?, ?, ?, ?, ?)");
        let count = 0;
        const doInsert = db.transaction((data: typeof result.holdings) => {
          for (const h of data) {
            const r = insert.run(code, h.stock_code, h.stock_name, h.weight_pct, h.shares, h.market_value, result!.report_date);
            if (r.changes) count++;
          }
        });
        doInsert(result.holdings);
        log.info(`mcp:crawl_fund_holdings ${code} — ${count} stocks`);
        return { content: [{ type: "text", text: JSON.stringify({ ok: true, fund_code: code, fund_name: result.fund_name, report_date: result.report_date, holdings_count: count }) }] };
      }
      const { refreshAllHoldings } = await import("../../crawler/holdings");
      refreshAllHoldings().then(r => log.info(`mcp:crawl_fund_holdings all done: ${r.updated}/${r.total}`));
      return { content: [{ type: "text", text: JSON.stringify({ ok: true, status: "started", message: "正在后台爬取全部权益类基金的持仓数据，请稍后查看结果" }) }] };
    },
  });
};
