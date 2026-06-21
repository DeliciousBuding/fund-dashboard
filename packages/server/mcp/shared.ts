/** MCP Shared — helpers, schemas, and market schedule functions */

import { z } from "zod";
import type { McpServer } from "mcp-lite";

/** Normalize security code: pad numeric codes to 6 digits, uppercase alpha codes */
export function padCode(code: string): string {
  const normalized = code.trim();
  return /^\d+$/.test(normalized) ? normalized.padStart(6, "0") : normalized.toUpperCase();
}

/** Shared Zod schema fragments */
export const CodeArg = z.object({ code: z.string().describe("6位代码") });
export const FundCodeArg = z.object({ fund_code: z.string().describe("6位代码（基金或股票）") });

// ═══════════ Market Schedule Helpers ═══════════

export function getChinaMarketSchedule(now: Date): { status: string; next_open: string | null; next_close: string | null } {
  const day = now.getDay();
  const h = now.getHours();
  const m = now.getMinutes();

  if (day === 0 || day === 6) {
    const mon = new Date(now);
    mon.setDate(now.getDate() + (day === 6 ? 2 : 1));
    mon.setHours(9, 30, 0, 0);
    return { status: "休市", next_open: mon.toISOString().replace("T", " ").substring(0, 16), next_close: null };
  }

  const today930 = new Date(now); today930.setHours(9, 30, 0, 0);
  const today1130 = new Date(now); today1130.setHours(11, 30, 0, 0);
  const today1300 = new Date(now); today1300.setHours(13, 0, 0, 0);
  const today1500 = new Date(now); today1500.setHours(15, 0, 0, 0);

  const t = now.getTime();

  if (t < today930.getTime()) {
    return { status: "未开盘", next_open: today930.toISOString().replace("T", " ").substring(0, 16), next_close: today1500.toISOString().replace("T", " ").substring(0, 16) };
  }
  if (t >= today930.getTime() && t < today1130.getTime()) {
    return { status: "交易中（上午盘）", next_open: null, next_close: today1130.toISOString().replace("T", " ").substring(0, 16) };
  }
  if (t >= today1130.getTime() && t < today1300.getTime()) {
    return { status: "午间休市", next_open: today1300.toISOString().replace("T", " ").substring(0, 16), next_close: today1500.toISOString().replace("T", " ").substring(0, 16) };
  }
  if (t >= today1300.getTime() && t < today1500.getTime()) {
    return { status: "交易中（下午盘）", next_open: null, next_close: today1500.toISOString().replace("T", " ").substring(0, 16) };
  }
  const nextDay = new Date(now);
  nextDay.setDate(now.getDate() + 1);
  nextDay.setHours(9, 30, 0, 0);
  if (day === 5) nextDay.setDate(now.getDate() + 3);
  return { status: "已收盘", next_open: nextDay.toISOString().replace("T", " ").substring(0, 16), next_close: null };
}

export function getUSMarketSchedule(now: Date): { status: string; next_open: string | null; next_close: string | null } {
  const utcHour = now.getUTCHours();
  const utcMin = now.getUTCMinutes();
  const day = now.getUTCDay();
  const utcTime = utcHour * 60 + utcMin;
  const openMin = 13 * 60 + 30;
  const closeMin = 20 * 60;

  if (day === 0 || day === 6) {
    const mon = new Date(now);
    mon.setUTCDate(now.getUTCDate() + (day === 6 ? 2 : 1));
    mon.setUTCHours(13, 30, 0, 0);
    return { status: "休市", next_open: mon.toISOString().replace("T", " ").substring(0, 16) + " UTC", next_close: null };
  }
  if (utcTime < openMin) {
    const openDate = new Date(now); openDate.setUTCHours(13, 30, 0, 0);
    const closeDate = new Date(now); closeDate.setUTCHours(20, 0, 0, 0);
    return { status: "未开盘", next_open: openDate.toISOString().replace("T", " ").substring(0, 16) + " UTC", next_close: closeDate.toISOString().replace("T", " ").substring(0, 16) + " UTC" };
  }
  if (utcTime >= openMin && utcTime < closeMin) {
    const closeDate = new Date(now); closeDate.setUTCHours(20, 0, 0, 0);
    return { status: "交易中", next_open: null, next_close: closeDate.toISOString().replace("T", " ").substring(0, 16) + " UTC" };
  }
  const nextDay = new Date(now);
  nextDay.setUTCDate(now.getUTCDate() + 1);
  nextDay.setUTCHours(13, 30, 0, 0);
  if (day === 5) nextDay.setUTCDate(now.getUTCDate() + 3);
  return { status: "已收盘", next_open: nextDay.toISOString().replace("T", " ").substring(0, 16) + " UTC", next_close: null };
}

export function getHKMarketSchedule(now: Date): { status: string; next_open: string | null; next_close: string | null } {
  const day = now.getDay();
  const h = now.getHours();
  const m = now.getMinutes();

  if (day === 0 || day === 6) {
    const mon = new Date(now);
    mon.setDate(now.getDate() + (day === 6 ? 2 : 1));
    mon.setHours(9, 30, 0, 0);
    return { status: "休市", next_open: mon.toISOString().replace("T", " ").substring(0, 16), next_close: null };
  }

  const tHHMM = h * 60 + m;
  if (tHHMM < 9 * 60 + 30) {
    const openDate = new Date(now); openDate.setHours(9, 30, 0, 0);
    return { status: "未开盘", next_open: openDate.toISOString().replace("T", " ").substring(0, 16), next_close: null };
  }
  if (tHHMM >= 9 * 60 + 30 && tHHMM < 12 * 60) {
    return { status: "交易中（上午盘）", next_open: null, next_close: null };
  }
  if (tHHMM >= 12 * 60 && tHHMM < 13 * 60) {
    return { status: "午间休市", next_open: null, next_close: null };
  }
  if (tHHMM >= 13 * 60 && tHHMM < 16 * 60) {
    return { status: "交易中（下午盘）", next_open: null, next_close: null };
  }
  const nextDay = new Date(now);
  nextDay.setDate(now.getDate() + 1);
  nextDay.setHours(9, 30, 0, 0);
  if (day === 5) nextDay.setDate(now.getDate() + 3);
  return { status: "已收盘", next_open: nextDay.toISOString().replace("T", " ").substring(0, 16), next_close: null };
}

/** Tool registration type — each module exports a function of this shape */
export type ToolRegistrar = (server: McpServer) => void;
