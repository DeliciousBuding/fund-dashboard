/** MCP Analysis Tools — DCA computation */

import { z } from "zod";
import { padCode, type ToolRegistrar } from "../shared";
import { queryOne, query } from "../../db";
import { computeDcaPlan } from "../../utils/dca";
import { runBacktest } from "../../services/backtest";
import { calcXirr } from "../../services/xirr";

export const registerAnalysisTools: ToolRegistrar = (server) => {
  server.tool("compute_dca_amount", {
    description: `定投金额计算器。支持成本偏离模式(nav_deviation)和涨跌幅模式(change_pct)。
nav_deviation：根据当前价格相对持仓成本的偏离率查表。
change_pct：根据最近涨跌幅调节，跌多多投、涨多控仓。
适用于任何有价格和持仓的证券（基金或股票）。`,
    inputSchema: z.object({
      fund_code: z.string().describe("证券代码（基金、A/HK 股票或美股 ticker）"),
      base_amount: z.number().default(30).describe("基础定投金额（元）"),
      mode: z.enum(["nav_deviation", "change_pct"]).default("nav_deviation").describe("定投模式：nav_deviation=成本偏离，change_pct=最近涨跌幅"),
    }),
    handler: async (args) => {
      const code = padCode(args.fund_code);
      const st = queryOne<any>("SELECT security_type, market FROM fund_details WHERE fund_code = ?", code);
      const pos = queryOne<any>("SELECT * FROM portfolio_snapshot WHERE fund_code = ?", code);
      if (!pos || !pos.held_shares || pos.held_shares < 0.001) {
        return { content: [{ type: "text", text: JSON.stringify({ error: "no_position", message: "该证券无持仓，无法计算偏离率。建议首次按基础金额买入。" }) }] };
      }
      const latestNav: number = pos.latest_nav;
      const costBasis: number | null = pos.total_cost ? Math.abs(pos.total_cost) / pos.held_shares : null;
      const latestChange = queryOne<any>("SELECT daily_change_pct FROM nav_history WHERE fund_code = ? ORDER BY date DESC LIMIT 1", code)?.daily_change_pct ?? null;
      if (!latestNav || (args.mode === "nav_deviation" && (!costBasis || costBasis <= 0))) {
        return { content: [{ type: "text", text: JSON.stringify({ error: "insufficient_data", nav: latestNav, cost_per_share: costBasis }) }] };
      }
      const plan = computeDcaPlan({ mode: args.mode, baseAmount: args.base_amount, latestNav, costPerShare: costBasis, changePct: latestChange });
      return { content: [{ type: "text", text: JSON.stringify({
        fund_code: code,
        security_type: st?.security_type || "fund",
        market: st?.market || "",
        ...plan,
        range: plan.signal,
      }, null, 2) }] };
    },
  });

  server.tool("run_backtest", {
    description: `策略回测引擎。对指定基金的历史净值数据运行策略模拟（网格grid/动量momentum/再平衡rebalance/定投dca）。
返回总投入、最终市值、总收益率、年化收益、最大回撤、夏普比率、交易记录、每日时间线和一次性vs定投对比。`,
    inputSchema: z.object({
      fund_code: z.string().describe("基金代码"),
      strategy: z.enum(["grid", "momentum", "rebalance", "dca"]).default("dca").describe("回测策略"),
      start_date: z.string().describe("起始日期 YYYY-MM-DD"),
      base_amount: z.number().default(1000).describe("基础投资金额"),
      grid_levels: z.number().optional().describe("网格策略：网格层级数（默认5）"),
      momentum_months: z.number().optional().describe("动量策略：回看月数（默认3）"),
      target_weight: z.number().optional().describe("再平衡策略：目标权益权重0-1（默认0.6）"),
      rebalance_interval: z.number().optional().describe("再平衡策略：再平衡间隔月数（默认3）"),
    }),
    handler: async (args) => {
      const code = padCode(args.fund_code);
      const navs = query<{ date: string; fund_code: string; unit_nav: number }>(
        "SELECT date, fund_code, unit_nav FROM nav_history WHERE fund_code = ? AND date >= ? ORDER BY date",
        [code, args.start_date],
      );
      if (!navs.length) {
        return { content: [{ type: "text", text: JSON.stringify({ error: "no_data", message: `基金 ${code} 在 ${args.start_date} 之后无净值数据` }) }] };
      }
      const result = runBacktest(navs, {
        fund_code: code,
        strategy: args.strategy as any,
        start_date: args.start_date,
        base_amount: args.base_amount,
        grid_levels: args.grid_levels,
        momentum_months: args.momentum_months,
        target_weight: args.target_weight,
        rebalance_interval: args.rebalance_interval,
      });
      return { content: [{ type: "text", text: JSON.stringify(result, null, 2) }] };
    },
  });

  server.tool("compare_funds", {
    description: `基金多维度对比工具。对多个基金（通过代码列表）进行横向对比，
返回每个基金的年化收益(XIRR)、年化波动率、Sharpe比率、最大回撤(MaxDD)、Calmar比率。
适用于比较不同基金的风险收益特征，辅助投资决策。`,
    inputSchema: z.object({
      codes: z.array(z.string()).describe("基金代码列表，如 ['164906','161128']"),
    }),
    handler: async (args) => {
      if (!args.codes || !args.codes.length) {
        return { content: [{ type: "text", text: JSON.stringify({ error: "codes required" }) }] };
      }
      const codes = [...new Set(args.codes.map((c: string) => padCode(c)))];
      const results: any[] = [];

      for (const code of codes) {
        const fd = queryOne<{ fund_name: string; market: string }>(
          "SELECT fund_name, market FROM fund_details WHERE fund_code = ?", code,
        );
        if (!fd) {
          results.push({ code, name: code, xirr: null, volatility: null, sharpe: null, max_drawdown: null, calmar: null });
          continue;
        }

        // XIRR
        let xirr: number | null = null;
        const cfs = query<{ confirm_amount: number; direction: string; trade_time: string }>(
          "SELECT confirm_amount, direction, trade_time FROM transactions WHERE fund_code = ? AND direction IN ('buy','sell','dividend') ORDER BY trade_time", code,
        );
        if (cfs.length >= 2) {
          const lastDate = new Date(cfs[cfs.length - 1].trade_time);
          const cfList = cfs.map((tx: any) => ({
            amount: tx.direction === "buy" ? -(+tx.confirm_amount) : +(+tx.confirm_amount),
            years: (lastDate.getTime() - new Date(tx.trade_time).getTime()) / 31536000000,
          }));
          const shares = queryOne<{ s: number }>("SELECT SUM(signed_share_change) as s FROM transactions WHERE fund_code = ?", code);
          const latestNav = queryOne<{ unit_nav: number }>("SELECT unit_nav FROM nav_history WHERE fund_code = ? ORDER BY date DESC LIMIT 1", code);
          if (shares && shares.s > 0.001 && latestNav) cfList.push({ amount: shares.s * latestNav.unit_nav, years: 0 });
          const x = calcXirr(cfList);
          xirr = x !== null ? +((x * 100).toFixed(2)) : null;
        }

        // Volatility
        const navs = query<{ date: string; unit_nav: number }>(
          "SELECT date, unit_nav FROM nav_history WHERE fund_code = ? ORDER BY date", code,
        );
        let volatility: number | null = null;
        if (navs.length >= 10) {
          const returns: number[] = [];
          for (let i = 1; i < navs.length; i++) {
            returns.push(Math.log(navs[i].unit_nav / navs[i - 1].unit_nav));
          }
          const mean = returns.reduce((s, v) => s + v, 0) / returns.length;
          const variance = returns.reduce((s, v) => s + (v - mean) ** 2, 0) / (returns.length - 1);
          volatility = +(Math.sqrt(variance) * Math.sqrt(252) * 100).toFixed(2);
        }

        // Max drawdown
        let maxDd: number | null = null;
        if (navs.length) {
          let peak = +navs[0].unit_nav, md = 0;
          for (const r of navs) {
            const nav = +r.unit_nav;
            if (nav > peak) peak = nav;
            const dd = (peak - nav) / peak;
            if (dd > md) md = dd;
          }
          maxDd = +((md * 100).toFixed(2));
        }

        const sharpe = (xirr != null && volatility != null && volatility > 0.001)
          ? +(xirr / volatility).toFixed(4) : null;
        const calmar = (xirr != null && maxDd != null && maxDd > 0.01)
          ? +(xirr / maxDd).toFixed(4) : null;

        results.push({
          code, name: fd.fund_name, market: fd.market || "",
          xirr, volatility, sharpe, max_drawdown: maxDd, calmar,
        });
      }

      return { content: [{ type: "text", text: JSON.stringify({ funds: results }, null, 2) }] };
    },
  });
};
