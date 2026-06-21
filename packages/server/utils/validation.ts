/** Input validation — Zod schemas + Hono validation middleware */

import { z } from "zod";
import type { Context, Next } from "hono";
import { badRequest } from "./errors";

// ── Schema: Import transactions ────────────────────────────────────────

export const importTransactionSchema = z.object({
  fund_code: z.string().min(1, "fund_code is required"),
  trade_time: z.string().min(1, "trade_time is required"),
  direction: z.enum(["buy", "sell", "dividend"], {
    error: 'direction must be one of: buy, sell, dividend',
  }),
  confirm_amount: z.number().positive("confirm_amount must be positive"),
  fee: z.number().nonnegative("fee must be non-negative"),
  // Optional fields
  order_id: z.string().optional(),
  confirm_date: z.string().optional(),
  trade_type: z.string().optional(),
  fund_name: z.string().optional(),
  confirm_share: z.number().optional(),
  inferred_nav: z.number().optional(),
  signed_cash_flow: z.number().optional(),
  signed_share_change: z.number().optional(),
  security_code: z.string().optional(),
});

export type ImportTransaction = z.infer<typeof importTransactionSchema>;

// ── Schema: Import transactions body wrapper ───────────────────────────

export const importTransactionsBodySchema = z.object({
  transactions: z.array(importTransactionSchema).min(1, "transactions array is required"),
});

export type ImportTransactionsBody = z.infer<typeof importTransactionsBodySchema>;

// ── Schema: Create security ────────────────────────────────────────────

export const createSecuritySchema = z.object({
  fund_code: z.string().min(1, "fund_code is required"),
  fund_name: z.string().min(1, "fund_name is required"),
  security_type: z.enum(["fund", "stock", "index"], {
    error: 'security_type must be one of: fund, stock, index',
  }),
  market: z.string().optional(),
  currency: z.string().optional(),
  fund_type: z.string().optional(),
});

export type CreateSecurity = z.infer<typeof createSecuritySchema>;

// ── Schema: Update transaction ─────────────────────────────────────────

export const updateTransactionSchema = z.object({
  trade_time: z.string().optional(),
  confirm_date: z.string().optional(),
  trade_type: z.string().optional(),
  direction: z.enum(["buy", "sell", "dividend"]).optional(),
  confirm_amount: z.number().positive().optional(),
  confirm_share: z.number().optional(),
  fee: z.number().nonnegative().optional(),
  fund_code: z.string().optional(),
});

export type UpdateTransaction = z.infer<typeof updateTransactionSchema>;

// ── Schema: Adjust position ────────────────────────────────────────────

export const adjustPositionSchema = z.object({
  fund_code: z.string().min(1, "fund_code is required"),
  shares: z.number("shares is required"),
});

export type AdjustPosition = z.infer<typeof adjustPositionSchema>;

// ── Validation middleware ──────────────────────────────────────────────

import type { ZodSchema } from "zod";

export function validate(schema: ZodSchema) {
  return async (c: Context, next: Next) => {
    const body = await c.req.json().catch(() => ({}));
    const result = schema.safeParse(body);
    if (!result.success) {
      throw badRequest(result.error.issues.map(i => i.message).join("; "));
    }
    c.set("validated", result.data);
    await next();
  };
}
