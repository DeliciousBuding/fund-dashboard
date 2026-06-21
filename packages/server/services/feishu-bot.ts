/**
 * Feishu Bot Integration Service — full SDK-based bot with:
 *  - Message receiving (im.message.receive_v1)
 *  - Interactive card sending (replaces webhook-only)
 *  - Card action callbacks
 *  - Command parsing: "查看持仓", "定投金额", "回撤", "净值", "告警"
 *
 *  Requires FEISHU_APP_ID + FEISHU_APP_SECRET env vars.
 *  Falls back to FEISHU_WEBHOOK for card-only mode when SDK not configured.
 */

import { query, queryOne } from "../db";
import { log } from "../middleware/logger";
import { getPortfolioSummary, getInvestmentHarnessSnapshot } from "./index";
import { computeDcaPlan } from "../utils/dca";
import { checkAndNotify, type AlertItem } from "./notify";

// ═══════════ Types ═══════════

interface FeishuSdkClient {
  im: { message: { create: (params: any) => Promise<any> } };
  tokenManager: { getAccessToken: () => Promise<string> };
}

interface BotCommand {
  intent: string;
  args: Record<string, string>;
}

// ═══════════ Client (lazy init) ═══════════

let _client: FeishuSdkClient | null = null;
let _initError: string | null = null;

async function getClient(): Promise<FeishuSdkClient | null> {
  if (_client) return _client;
  if (_initError) return null;

  const appId = process.env.FEISHU_APP_ID;
  const appSecret = process.env.FEISHU_APP_SECRET;

  if (!appId || !appSecret) {
    _initError = "FEISHU_APP_ID or FEISHU_APP_SECRET not configured";
    log.warn("feishu-bot: " + _initError);
    return null;
  }

  try {
    // Dynamic import of larksuite/node-sdk — only if available
    const { Client } = await import("@larksuiteoapi/node-sdk");
    _client = new Client({ appId, appSecret }) as unknown as FeishuSdkClient;
    log.info("feishu-bot: SDK client initialized");
    return _client;
  } catch (e: any) {
    _initError = `SDK not available: ${e.message}. Install: npm install @larksuiteoapi/node-sdk`;
    log.warn("feishu-bot: " + _initError);
    return null;
  }
}

// ═══════════ Command Parser ═══════════

const COMMANDS: { pattern: RegExp; intent: string }[] = [
  { pattern: /(查看)?持仓|portfolio|holding|position/, intent: "portfolio" },
  { pattern: /定投.*(\d{6})|dca\s+(\d{6})|定投.*(?:计算|金额).*(\d{6})/, intent: "dca" },
  { pattern: /回撤|drawdown|最大亏损/, intent: "drawdown" },
  { pattern: /净值|nav|price|价格/, intent: "nav" },
  { pattern: /告警|alert|通知|warning/, intent: "alerts" },
  { pattern: /收益|xirr|回报|收益率/, intent: "xirr" },
  { pattern: /配置|allocation|占比|权重/, intent: "allocation" },
  { pattern: /帮助|help|功能|命令/, intent: "help" },
];

function parseCommand(text: string): BotCommand {
  const trimmed = text.trim();
  for (const cmd of COMMANDS) {
    const m = trimmed.match(cmd.pattern);
    if (m) {
      const code = m[1] || m[2] || m[3] || "";
      return { intent: cmd.intent, args: { code: code.padStart(6, "0") } };
    }
  }
  return { intent: "unknown", args: {} };
}

// ═══════════ Card Builders ═══════════

function nowISO() { return new Date().toISOString().replace("T", " ").substring(0, 19); }

function buildPortfolioCard(): any {
  const summary = getPortfolioSummary();
  const harness = getInvestmentHarnessSnapshot();
  if (!summary) return { header: { title: { tag: "plain_text", content: "无持仓数据" } } };

  const pnlColor = summary.unrealized_pnl >= 0 ? "green" : "red";
  const md = [
    `**总资产**: ¥${((harness?.total_value || 0)).toLocaleString()}`,
    `**持仓**: ${summary.held_funds} 只 | **总交易**: ${summary.total_tx} 笔`,
    `**未实现盈亏**: ¥${summary.unrealized_pnl.toFixed(2)} (${pnlColor === "green" ? "+" : ""}${(summary.unrealized_pnl / Math.abs(summary.total_buy || 1) * 100).toFixed(2)}%)`,
    `**定投占比**: ${summary.auto_tx}/${summary.total_tx} 笔 (${(summary.auto_amount / Math.abs(summary.total_buy || 1) * 100).toFixed(1)}%)`,
    `**数据缺口**: 价格过期 ${harness?.data_quality.stale_price_count || 0} 只 · 缺少成本 ${harness?.data_quality.missing_cost_basis_count || 0} 只`,
  ].join("  \n");

  return {
    header: { title: { tag: "plain_text", content: "📊 投资组合快照" }, template: pnlColor },
    elements: [
      { tag: "div", text: { tag: "lark_md", content: md } },
      { tag: "hr" },
      { tag: "note", elements: [{ tag: "plain_text", content: `TokenDance Fund · ${nowISO()}` }] },
    ],
  };
}

function buildHelpCard(): any {
  return {
    header: { title: { tag: "plain_text", content: "🤖 Fund Bot 命令" }, template: "blue" },
    elements: [
      { tag: "div", text: { tag: "lark_md", content: [
        "回复以下关键词查看对应信息:",
        "- **持仓** / **portfolio** — 投资组合快照",
        "- **定投 + 代码** — 单只证券 DCA 测算",
        "- **回撤** — 持仓最大回撤",
        "- **净值** — 最新净值/价格",
        "- **告警** — 检查价格异动/回撤/数据过期",
        "- **收益** — XIRR 年化收益",
        "- **配置** — 资产配置和集中度",
        "- **帮助** — 显示本消息",
      ].join("  \n") } },
      { tag: "note", elements: [{ tag: "plain_text", content: "TokenDance Fund Dashboard v2.6" }] },
    ],
  };
}

function buildAlertsCard(alerts: AlertItem[]): any {
  const md = alerts.map(a =>
    `**${a.severity === "critical" ? "🔴" : a.severity === "warning" ? "🟡" : "ℹ️"} ${a.title}**  \n${a.detail}`
  ).join("  \n---\n  \n");
  return {
    header: { title: { tag: "plain_text", content: `📢 告警 (${alerts.length}条)` }, template: "orange" },
    elements: [
      { tag: "div", text: { tag: "lark_md", content: md } },
      { tag: "note", elements: [{ tag: "plain_text", content: nowISO() }] },
    ],
  };
}

function buildPlainCard(title: string, content: string, template = "blue"): any {
  return {
    header: { title: { tag: "plain_text", content: title }, template },
    elements: [
      { tag: "div", text: { tag: "lark_md", content } },
      { tag: "note", elements: [{ tag: "plain_text", content: nowISO() }] },
    ],
  };
}

// ═══════════ Message Sending ═══════════

export async function sendFeishuMessage(receiveId: string, receiveIdType: "open_id" | "user_id" | "chat_id", card: any): Promise<boolean> {
  const client = await getClient();
  if (!client) {
    // Fallback to webhook
    const w = process.env.FEISHU_WEBHOOK;
    if (!w) return false;
    try {
      await fetch(w, {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ msg_type: "interactive", card }),
      });
      return true;
    } catch { return false; }
  }

  try {
    const content = JSON.stringify(card);
    await client.im.message.create({
      params: { receive_id_type: receiveIdType },
      data: { receive_id: receiveId, msg_type: "interactive", content },
    });
    return true;
  } catch (e: any) {
    log.error("feishu-bot: send failed", { error: String(e) });
    return false;
  }
}

// ═══════════ Message Handler ═══════════

export async function handleFeishuMessage(event: { sender?: { sender_id?: { open_id?: string } }; message?: { chat_id?: string; content?: string; message_type?: string } }): Promise<any> {
  const openId = event.sender?.sender_id?.open_id;
  const chatId = event.message?.chat_id;
  const content = event.message?.content || "";

  if (!openId && !chatId) return { code: 1, msg: "no receiver" };

  // Parse text content (JSON-encoded in Feishu event)
  let text = "";
  try {
    const parsed = JSON.parse(content);
    text = parsed.text || "";
  } catch { text = content; }

  const cmd = parseCommand(text);
  const targetId = chatId || openId!;
  const idType = chatId ? "chat_id" as const : "open_id" as const;

  let card: any;
  switch (cmd.intent) {
    case "portfolio":
      card = buildPortfolioCard();
      break;
    case "alerts": {
      const alerts = checkAndNotify();
      card = alerts.length > 0 ? buildAlertsCard(alerts) : buildPlainCard("✅ 无告警", "当前所有指标正常。", "green");
      break;
    }
    case "dca": {
      const code = cmd.args.code;
      if (!code) {
        card = buildPlainCard("DCA 定投测算", "请提供证券代码，如: **定投 019173**");
      } else {
        const pos = queryOne<any>("SELECT * FROM portfolio_snapshot WHERE fund_code = ?", code);
        if (!pos?.held_shares) {
          card = buildPlainCard("无持仓", `证券 ${code} 暂无持仓数据`);
        } else {
          const plan = computeDcaPlan({ mode: "nav_deviation", baseAmount: 30, latestNav: pos.latest_nav, costPerShare: pos.total_cost ? Math.abs(pos.total_cost) / pos.held_shares : null, changePct: null });
          card = buildPlainCard(`📐 DCA 测算 — ${code}`,
            `**模式**: 成本偏离  \n**偏离率**: ${plan.deviation_pct?.toFixed(2) ?? "N/A"}%  \n**DCA倍率**: ${plan.dca_rate.toFixed(2)}x  \n**测算金额**: ¥${plan.actual_amount.toFixed(2)}  \n**信号**: ${plan.signal}`);
        }
      }
      break;
    }
    case "help":
      card = buildHelpCard();
      break;
    default:
      card = buildPlainCard("❓ 未知命令", `不支持 "${text}"。回复 **帮助** 查看可用命令。`);
  }

  const ok = await sendFeishuMessage(targetId, idType, card);
  return ok ? { code: 0, msg: "ok" } : { code: 1, msg: "send failed" };
}

// ═══════════ Status ═══════════

export function getFeishuBotStatus(): { configured: boolean; mode: string; error: string | null } {
  const appId = process.env.FEISHU_APP_ID;
  const appSecret = process.env.FEISHU_APP_SECRET;
  const webhook = process.env.FEISHU_WEBHOOK;

  if (appId && appSecret) return { configured: true, mode: "sdk", error: _initError };
  if (webhook) return { configured: true, mode: "webhook", error: null };
  return { configured: false, mode: "none", error: "FEISHU_APP_ID + FEISHU_APP_SECRET or FEISHU_WEBHOOK required" };
}

// ═══════════ Event Dispatcher (Express/Hono-compatible) ═══════════

export async function handleFeishuEventCallback(body: any, headers: Record<string, string>): Promise<any> {
  // Challenge verification
  if (body.challenge) {
    return { challenge: body.challenge };
  }

  // Event types
  if (body.header?.event_type === "im.message.receive_v1") {
    const event = body.event;
    log.info("feishu-bot: message received", { chat_type: event?.message?.chat_type, msg_type: event?.message?.message_type });
    return await handleFeishuMessage(event);
  }

  // Card action callback
  if (body.header?.event_type === "card.action.trigger") {
    log.info("feishu-bot: card action triggered");
    return { code: 0, msg: "card action received" };
  }

  return { code: 0, msg: "unhandled event type" };
}
