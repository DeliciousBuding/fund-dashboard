/** MCP Admin Tools — system status, fund status, data verification, freshness */

import { z } from "zod";
import { padCode, getChinaMarketSchedule, getHKMarketSchedule, getUSMarketSchedule, type ToolRegistrar } from "../shared";
import { query, queryOne } from "../../db";
import { getSystemStatus, getSourceEvents, createSourceEvent, markSourceEventRead, checkAndNotify, getNotifyConfig,
  getPortfolioSummary, getPortfolioTimeline, getPortfolioAllocation, getInvestmentHarnessSnapshot } from "../../services/index";

export const registerAdminTools: ToolRegistrar = (server) => {
  server.tool("get_system_status", {
    description: "获取系统完整诊断信息：交易数、净值覆盖面、异常记录、运行状态、全球市场交易时段（A股/港股/美股开收盘时间）",
    inputSchema: z.object({}),
    handler: async () => {
      const status = getSystemStatus();
      const now = new Date();
      return { content: [{ type: "text", text: JSON.stringify({
        ok: true, uptime_sec: +(process.uptime().toFixed(1)),
        ...status,
        market_schedule: {
          china_a_share: getChinaMarketSchedule(now),
          hong_kong: getHKMarketSchedule(now),
          us: getUSMarketSchedule(now),
        },
        server_time: now.toISOString().replace("T", " ").substring(0, 19),
      }, null, 2) }] };
    },
  });

  server.tool("get_fund_status", {
    description: "获取单只证券的管理状态：交易列表、价格覆盖、当前持仓、申赎状态",
    inputSchema: z.object({ code: z.string().describe("6位代码") }),
    handler: async (args) => {
      const code = padCode(args.code);
      const info = queryOne<any>("SELECT fund_name as name, fund_type as type, COALESCE(security_type,'fund') as security_type, COALESCE(market,'') as market FROM fund_details WHERE fund_code = ?", code);
      const tx = queryOne<any>("SELECT COUNT(*) as n, MIN(trade_time) as first, MAX(trade_time) as last FROM transactions WHERE fund_code = ?", code);
      const nav = queryOne<any>("SELECT COUNT(*) as n, MIN(date) as first, MAX(date) as last FROM nav_history WHERE fund_code = ?", code);
      const pos = queryOne<any>("SELECT * FROM portfolio_snapshot WHERE fund_code = ?", code) || { held_shares: 0 };
      const status = queryOne<any>("SELECT * FROM fund_status WHERE fund_code = ?", code) || {};
      return { content: [{ type: "text", text: JSON.stringify({
        code, name: info?.name, type: info?.type,
        security_type: info?.security_type, market: info?.market,
        transactions: { count: tx?.n, first: tx?.first?.substring(0, 10), last: tx?.last?.substring(0, 10) },
        nav: { count: nav?.n, first: nav?.first?.substring(0, 10), last: nav?.last?.substring(0, 10) },
        position: { shares: pos.held_shares || 0, cost: pos.total_cost ?? 0, value: pos.current_value ?? 0, pnl: pos.unrealized_pnl ?? 0, pnl_pct: pos.pnl_pct ?? 0 },
        trading: { purchase_status: status.purchase_status || "unknown", redemption_status: status.redemption_status || "unknown" },
      }, null, 2) }] };
    },
  });

  server.tool("verify_data", {
    description: "数据完整性校验：检查缺失净值、负持仓、空结算日等问题",
    inputSchema: z.object({}),
    handler: async () => {
      const issues: string[] = [];
      const fundsWithoutNav = query<any>("SELECT fd.fund_code, fd.fund_name FROM fund_details fd WHERE fd.fund_code NOT IN (SELECT DISTINCT fund_code FROM nav_history)");
      const negPos = query<any>("SELECT fund_code, held_shares FROM portfolio_snapshot WHERE held_shares < -0.001");
      const nullSd = queryOne<any>("SELECT COUNT(*) as n FROM transactions WHERE settlement_days IS NULL");
      if (fundsWithoutNav.length) issues.push(`${fundsWithoutNav.length} securities missing price data: ${fundsWithoutNav.map((f: any) => f.fund_code).join(", ")}`);
      if (negPos.length) issues.push(`${negPos.length} negative positions: ${negPos.map((p: any) => `${p.fund_code}=${p.held_shares}`).join(", ")}`);
      if (nullSd?.n) issues.push(`${nullSd.n} transactions missing settlement_days`);
      return { content: [{ type: "text", text: JSON.stringify({
        healthy: issues.length === 0,
        issues: issues.length ? issues : ["all clear"],
        details: { securities_without_nav: fundsWithoutNav.map((f: any) => f.fund_code), negative_positions: negPos.map((p: any) => ({ code: p.fund_code, shares: p.held_shares })), missing_settlement_count: nullSd?.n || 0 },
      }, null, 2) }] };
    },
  });

  server.tool("get_data_freshness", {
    description: "检查数据新鲜度：最后交易日期、最新净值日期、过期证券清单（>2天未更新）和可行动建议。数据过期时返回 actionable 建议。",
    inputSchema: z.object({}),
    handler: async () => {
      const lastTx = queryOne<any>("SELECT MAX(trade_time) as t FROM transactions");
      const lastNav = queryOne<any>("SELECT MAX(date) as d FROM nav_history");
      const anomalies = query<any>("SELECT seq, fund_code, direction, trade_time, anomaly FROM transactions WHERE anomaly IS NOT NULL");
      const fundsWithoutNav = query<any>("SELECT fd.fund_code, fd.fund_name FROM fund_details fd WHERE fd.fund_code NOT IN (SELECT DISTINCT fund_code FROM nav_history)");
      const staleFunds = query<any>("SELECT fund_code, MAX(date) as last_nav FROM nav_history GROUP BY fund_code HAVING MAX(date) < date('now', '-3 days')");
      const staleWithDays = query<any>("SELECT nh.fund_code, fd.fund_name, MAX(nh.date) as last_nav, CAST(julianday('now') - julianday(MAX(nh.date)) AS INTEGER) as stale_days FROM nav_history nh JOIN fund_details fd ON nh.fund_code = fd.fund_code GROUP BY nh.fund_code HAVING MAX(nh.date) < date('now', '-2 days') ORDER BY stale_days DESC LIMIT 10");
      const health = staleWithDays.length > 3 ? "stale" : staleWithDays.length > 0 ? "degraded" : "fresh";
      return { content: [{ type: "text", text: JSON.stringify({
        last_transaction: lastTx?.t, last_nav_date: lastNav?.d?.substring(0, 10),
        anomaly_count: anomalies.length,
        health,
        missing_nav_securities: fundsWithoutNav.map((f: any) => ({ code: f.fund_code, name: f.fund_name })),
        stale_nav_securities: staleFunds.map((f: any) => ({ code: f.fund_code, last_nav: f.last_nav })),
        stale_detail: staleWithDays.map((f: any) => ({ code: f.fund_code, name: f.fund_name, last_nav: f.last_nav, stale_days: f.stale_days })),
        actionable: health === "stale"
          ? `⚠️ ${staleWithDays.length} 只证券价格过期 >2天。建议: crawl_nav(all:true) 刷新全部持仓。`
          : health === "degraded"
            ? `⚡ ${staleWithDays.length} 只证券价格稍有过期。建议: crawl_nav(all:true) 或单只刷新。`
            : "✅ 数据新鲜度正常",
      }, null, 2) }] };
    },
  });

  // ═══════════ Source Events ═══════════

  server.tool("get_source_events", {
    description: "获取来源事件队列——Hermes/Agent 可消费的已抓取新闻、公告、搜索结果。支持按证券代码、来源、已读状态过滤。只提供事实引用，不做投资判断。",
    inputSchema: z.object({
      code: z.string().optional().describe("证券代码过滤，不提供则返回全部"),
      source: z.string().optional().describe("数据来源过滤，如 websearch、eastmoney、yahoo"),
      show_read: z.boolean().default(false).describe("是否包含已读事件，默认只返回未读"),
      limit: z.number().default(30).describe("返回条数上限，1-100"),
    }),
    handler: async (args) => {
      const events = getSourceEvents({
        related_security_code: args.code,
        source: args.source,
        show_read: args.show_read,
        limit: args.limit,
      });
      return { content: [{ type: "text", text: JSON.stringify({
        count: events.length,
        decision_boundary: "facts_only",
        events: events.map(e => ({
          id: e.id,
          title: e.title,
          url: e.url,
          source: e.source,
          snippet: e.snippet,
          query: e.query,
          related_security_code: e.related_security_code,
          related_security_name: e.related_security_name,
          is_read: !!e.is_read,
          is_useful: !!e.is_useful,
          fetched_at: e.fetched_at,
        })),
      }, null, 2) }] };
    },
  });

  server.tool("mark_source_event", {
    description: "标记来源事件已读/有用——Hermes/Agent 消费事件后反馈状态，用于后续过滤和质量排序。",
    inputSchema: z.object({
      id: z.number().describe("事件 ID"),
      is_read: z.boolean().optional().describe("标记为已读"),
      is_useful: z.boolean().optional().describe("标记为有用"),
    }),
    handler: async (args) => {
      const ok = markSourceEventRead(args.id, {
        is_read: args.is_read,
        is_useful: args.is_useful,
      });
      if (!ok) return { content: [{ type: "text", text: JSON.stringify({ error: "event not found or no fields to update", id: args.id }) }] };
      return { content: [{ type: "text", text: JSON.stringify({ ok: true, id: args.id }) }] };
    },
  });

  server.tool("check_alerts", {
    description: "手动触发告警检查——扫描价格异动/回撤/数据过期/定投日，通过飞书 Webhook 发送告警卡片，返回触发的告警列表。需配置 FEISHU_WEBHOOK 环境变量。",
    inputSchema: z.object({
      price_change_pct: z.number().default(5).describe("价格异动阈值（%，默认5）"),
      drawdown_pct: z.number().default(10).describe("回撤阈值（%，默认10）"),
      stale_days: z.number().default(3).describe("数据过期天数阈值（默认3）"),
    }),
    handler: async (args) => {
      const alerts = checkAndNotify({
        priceChangeThresholdPct: args.price_change_pct,
        drawdownThresholdPct: args.drawdown_pct,
        staleDaysThreshold: args.stale_days,
      });
      const webhookConfigured = !!process.env.FEISHU_WEBHOOK;
      return { content: [{ type: "text", text: JSON.stringify({
        ok: true,
        total: alerts.length,
        webhook_configured: webhookConfigured,
        by_type: {
          price_change: alerts.filter(a => a.type === "price_change").length,
          drawdown: alerts.filter(a => a.type === "drawdown").length,
          data_stale: alerts.filter(a => a.type === "data_stale").length,
          dca_day: alerts.filter(a => a.type === "dca_day").length,
        },
        alerts: alerts.map(a => ({ type: a.type, title: a.title, category: a.category, detail: a.detail, severity: a.severity, time: a.time })),
      }, null, 2) }] };
    },
  });

  // ═══════════ Full Dashboard (batch) ═══════════

  server.tool("get_full_dashboard", {
    description: "一次性获取 Agent 所需的全部仪表盘数据：summary + harness_snapshot + allocation + source_events(limit 5) + data_freshness + market_indices + portfolio_timeline。减少 Agent 多次 MCP 调用。",
    inputSchema: z.object({ portfolio_id: z.number().default(1).describe("组合ID") }),
    handler: async (args) => {
      const pid = args.portfolio_id ?? 1;

      // ── summary ──
      const s = getPortfolioSummary(pid);
      const held = query<any>(
        "SELECT ps.*, COALESCE(fd.security_type,'fund') as security_type, COALESCE(fd.market,'') as market FROM portfolio_snapshot ps LEFT JOIN fund_details fd ON ps.fund_code = fd.fund_code WHERE ps.held_shares > 0.001 AND ps.portfolio_id = ? ORDER BY ps.current_value DESC NULLS LAST",
        [pid],
      );
      const summary = s ? {
        total_transactions: s.total_tx, unique_funds: s.unique_funds, held_funds: held.length,
        total_buy: s.total_buy, total_sell: s.total_sell,
        total_fee: s.total_fee, unrealized_pnl: s.unrealized_pnl,
        auto_invest: { tx: s.auto_tx, amount: s.auto_amount },
        manual_invest: { tx: s.manual_tx, amount: s.manual_amount },
        date_range: { first: s.first_trade, last: s.last_trade },
        settlement_distribution: s.settlement_distribution,
        holdings: held.map((h: any) => ({
          code: h.fund_code, name: h.fund_name, shares: +h.held_shares,
          security_type: h.security_type, market: h.market,
          cost: h.total_cost, value: h.current_value, pnl: h.unrealized_pnl, pnl_pct: h.pnl_pct,
          nav: h.latest_nav,
        })),
      } : {
        total_transactions: 0, unique_funds: 0, held_funds: 0,
        total_buy: 0, total_sell: 0, total_fee: 0, unrealized_pnl: 0,
        auto_invest: { tx: 0, amount: 0 }, manual_invest: { tx: 0, amount: 0 },
        date_range: { first: "", last: "" }, settlement_distribution: {},
        holdings: [],
      };

      // ── harness ──
      const harness = getInvestmentHarnessSnapshot(pid);

      // ── allocation ──
      const allocation = getPortfolioAllocation(pid);

      // ── source_events (limit 5) ──
      const events = getSourceEvents({ limit: 5, show_read: true });

      // ── data_freshness ──
      const lastTx = queryOne<any>("SELECT MAX(trade_time) as t FROM transactions");
      const lastNav = queryOne<any>("SELECT MAX(date) as d FROM nav_history");
      const anomalies = query<any>("SELECT seq, fund_code, direction, trade_time, anomaly FROM transactions WHERE anomaly IS NOT NULL");
      const staleWithDays = query<any>("SELECT nh.fund_code, fd.fund_name, MAX(nh.date) as last_nav, CAST(julianday('now') - julianday(MAX(nh.date)) AS INTEGER) as stale_days FROM nav_history nh JOIN fund_details fd ON nh.fund_code = fd.fund_code GROUP BY nh.fund_code HAVING MAX(nh.date) < date('now', '-2 days') ORDER BY stale_days DESC LIMIT 10");
      const health = staleWithDays.length > 3 ? "stale" : staleWithDays.length > 0 ? "degraded" : "fresh";
      const freshness = {
        last_transaction: lastTx?.t, last_nav_date: lastNav?.d?.substring(0, 10),
        anomaly_count: anomalies.length,
        health,
        stale_detail: staleWithDays.map((f: any) => ({ code: f.fund_code, name: f.fund_name, last_nav: f.last_nav, stale_days: f.stale_days })),
        actionable: health === "stale"
          ? `⚠️ ${staleWithDays.length} 只证券价格过期 >2天。建议: crawl_nav(all:true) 刷新全部持仓。`
          : health === "degraded"
            ? `⚡ ${staleWithDays.length} 只证券价格稍有过期。建议: crawl_nav(all:true) 或单只刷新。`
            : "✅ 数据新鲜度正常",
      };

      // ── market_indices (local cache) ──
      const indexRows = query<any>("SELECT code, name, market, price, change_pct, change_amt, updated_at FROM indices ORDER BY code");
      const indices: Record<string, any> = {};
      for (const row of indexRows) {
        indices[row.code] = {
          name: row.name, market: row.market, price: row.price,
          change_pct: row.change_pct, change_amt: row.change_amt, updated_at: row.updated_at,
        };
      }

      // ── portfolio_timeline ──
      const tl = getPortfolioTimeline(pid);
      const timeline = {
        count: tl.length,
        first: tl[0]?.date,
        last: tl[tl.length - 1]?.date,
        data: tl.map(({ date, total_value, total_cost, pnl }) => ({ date, total_value, total_cost, pnl })),
      };

      return { content: [{ type: "text", text: JSON.stringify({
        summary,
        harness,
        allocation,
        source_events: {
          count: events.length,
          decision_boundary: "facts_only",
          events: events.map(e => ({
            id: e.id, title: e.title, url: e.url, source: e.source,
            snippet: e.snippet, query: e.query,
            related_security_code: e.related_security_code,
            related_security_name: e.related_security_name,
            is_read: !!e.is_read, is_useful: !!e.is_useful,
            fetched_at: e.fetched_at,
          })),
        },
        freshness,
        indices,
        timeline,
        generated_at: new Date().toISOString(),
      }, null, 2) }] };
    },
  });
};
