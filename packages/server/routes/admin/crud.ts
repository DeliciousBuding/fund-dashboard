/** /api/admin CRUD — 交易 & 证券 CRUD 端点 */

import { Hono } from "hono";
import { queryOne, getRwDb } from "../../db";
import { log } from "../../middleware/logger";
import { validate, createSecuritySchema, updateTransactionSchema } from "../../utils/validation";
import type { CreateSecurity, UpdateTransaction } from "../../utils/validation";
import type { TransactionRow, FundDetailRow } from "../../utils/types";
import { badRequest } from "../../utils/errors";

const router = new Hono();

/** 重算单只证券的快照 (aggregate from transactions + nav_history) */
export function recalcFundSnapshot(fundCode: string) {
  const db = getRwDb();
  db.run("DELETE FROM portfolio_snapshot WHERE fund_code = ?", [fundCode]);
  const agg = queryOne<{ shares: number; cost: number }>(`
    SELECT SUM(signed_share_change) as shares, SUM(signed_cash_flow) as cost
    FROM transactions WHERE fund_code = ?
  `, fundCode);
  const nav = queryOne<{ unit_nav: number }>("SELECT unit_nav FROM nav_history WHERE fund_code = ? ORDER BY date DESC LIMIT 1", fundCode);
  const detail = queryOne<{ security_type: string | null }>("SELECT security_type FROM fund_details WHERE fund_code = ?", fundCode);
  if (agg && nav?.unit_nav && agg.shares > 0.001) {
    db.run(`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type)
      VALUES (?, (SELECT fund_name FROM fund_details WHERE fund_code = ?), ?, ?, ?,
        ? * ?, (? * ?) + ?, CASE WHEN ? != 0 THEN ((? * ?) + ?) / ABS(?) * 100 END, ?)`,
      [fundCode, fundCode, agg.shares, agg.cost, nav.unit_nav,
       agg.shares, nav.unit_nav, agg.shares, nav.unit_nav, agg.cost,
       agg.cost, agg.shares, nav.unit_nav, agg.cost, agg.cost, detail?.security_type || "fund"]);
  }
}

// ═══════════ TRANSACTION CRUD ═══════════

router.delete("/transactions/:seq", c => {
  const seq = parseInt(c.req.param("seq"));
  if (!seq) return c.json({ error: "seq required" }, 400);
  const tx = queryOne<TransactionRow>("SELECT * FROM transactions WHERE seq = ?", seq);
  if (!tx) return c.json({ error: "not found" }, 404);
  getRwDb().run("DELETE FROM transactions WHERE seq = ?", [seq]);
  recalcFundSnapshot(tx.fund_code);
  log.info(`deleted tx seq=${seq} fund=${tx.fund_code}`);
  return c.json({ ok: true, deleted: { seq, fund_code: tx.fund_code, direction: tx.direction, amount: +tx.confirm_amount } });
});

router.put("/transactions/:seq", validate(updateTransactionSchema), async c => {
  const seq = parseInt(c.req.param("seq"));
  if (!seq) throw badRequest("seq required");
  const tx = queryOne<TransactionRow>("SELECT * FROM transactions WHERE seq = ?", seq);
  if (!tx) return c.json({ error: "not found" }, 404);

  const body = c.get("validated") as UpdateTransaction;
  const allowed = ["trade_time", "confirm_date", "trade_type", "direction", "confirm_amount", "confirm_share", "fee", "fund_code"];
  const sets: string[] = [];
  const vals: any[] = [];
  for (const k of allowed) {
    if (body[k] !== undefined) { sets.push(`${k} = ?`); vals.push(body[k]); }
  }
  if (!sets.length) return c.json({ error: "no valid fields to update" }, 400);

  // Recompute derived fields
  const dir = body.direction || tx.direction;
  const amount = body.confirm_amount !== undefined ? body.confirm_amount : tx.confirm_amount;
  const shares = body.confirm_share !== undefined ? body.confirm_share : tx.confirm_share;
  const signedCash = dir === "buy" ? -amount : amount;
  const signedShare = dir === "dividend" ? 0 : dir === "buy" ? shares : -shares;
  sets.push("signed_cash_flow = ?", "signed_share_change = ?");
  vals.push(signedCash, signedShare);

  vals.push(seq);
  getRwDb().run(`UPDATE transactions SET ${sets.join(", ")} WHERE seq = ?`, vals);
  const fundCode = body.fund_code || tx.fund_code;
  recalcFundSnapshot(fundCode);
  log.info(`updated tx seq=${seq} fund=${fundCode}`);
  return c.json({ ok: true, updated: { seq, fields: sets.filter(s => !s.startsWith("signed_")).map(s => s.split(" ")[0]) } });
});

// ═══════════ SECURITIES CRUD (funds + stocks) ═══════════

/** POST /api/admin/securities — create a security (fund or stock) */
router.post("/securities", validate(createSecuritySchema), c => {
  const body = c.get("validated") as CreateSecurity;
  const code = body.fund_code.padStart(6, "0");
  const securityType = body.security_type;
  const market = body.market || "";
  getRwDb().run(
    "INSERT OR REPLACE INTO fund_details (fund_code, fund_name, fund_type, security_type, market) VALUES (?, ?, ?, ?, ?)",
    [code, body.fund_name, body.fund_type || "", securityType, market]
  );
  log.info(`security created: ${code} ${body.fund_name} type=${securityType} market=${market}`);
  return c.json({ ok: true, fund_code: code, fund_name: body.fund_name, security_type: securityType, market });
});

/** PUT /api/admin/securities/:code — update a security */
router.put("/securities/:code", async c => {
  const code = c.req.param("code").padStart(6, "0");
  const existing = queryOne<FundDetailRow>("SELECT * FROM fund_details WHERE fund_code = ?", code);
  if (!existing) return c.json({ error: "security not found" }, 404);
  const body = await c.req.json().catch(() => ({}));
  const sets: string[] = []; const vals: any[] = [];
  if (body.fund_name) { sets.push("fund_name = ?"); vals.push(body.fund_name); }
  if (body.fund_type) { sets.push("fund_type = ?"); vals.push(body.fund_type); }
  if (body.security_type !== undefined) { sets.push("security_type = ?"); vals.push(body.security_type); }
  if (body.market !== undefined) { sets.push("market = ?"); vals.push(body.market); }
  if (!sets.length) return c.json({ error: "no fields to update" }, 400);
  vals.push(code);
  getRwDb().run(`UPDATE fund_details SET ${sets.join(", ")} WHERE fund_code = ?`, vals);
  log.info(`security updated: ${code}`);
  return c.json({ ok: true, code });
});

/** DELETE /api/admin/securities/:code — delete a security (cascades) */
router.delete("/securities/:code", c => {
  const code = c.req.param("code").padStart(6, "0");
  const existing = queryOne<FundDetailRow>("SELECT * FROM fund_details WHERE fund_code = ?", code);
  if (!existing) return c.json({ error: "security not found" }, 404);
  const db = getRwDb();
  db.run("DELETE FROM fund_status WHERE fund_code = ?", [code]);
  db.run("DELETE FROM portfolio_snapshot WHERE fund_code = ?", [code]);
  db.run("DELETE FROM nav_history WHERE fund_code = ?", [code]);
  db.run("DELETE FROM transactions WHERE fund_code = ?", [code]);
  db.run("DELETE FROM fund_details WHERE fund_code = ?", [code]);
  log.info(`security deleted: ${code} ${existing.fund_name} type=${existing.security_type} (incl. all related data)`);
  return c.json({ ok: true, deleted: { code, name: existing.fund_name, security_type: existing.security_type } });
});

// ═══════════ BACKWARDS-COMPAT FUNDS CRUD (delegates to /securities) ═══════════

router.post("/funds", async c => {
  const body = await c.req.json().catch(() => null);
  if (!body?.fund_code || !body?.fund_name) return c.json({ error: "fund_code + fund_name required" }, 400);
  const code = body.fund_code.padStart(6, "0");
  const securityType = body.security_type || "fund";
  const market = body.market || "";
  getRwDb().run(
    "INSERT OR REPLACE INTO fund_details (fund_code, fund_name, fund_type, security_type, market) VALUES (?, ?, ?, ?, ?)",
    [code, body.fund_name, body.fund_type || "", securityType, market]
  );
  log.info(`fund created: ${code} ${body.fund_name} type=${securityType}`);
  return c.json({ ok: true, fund_code: code, fund_name: body.fund_name, security_type: securityType, market });
});

router.put("/funds/:code", async c => {
  const code = c.req.param("code").padStart(6, "0");
  const existing = queryOne<FundDetailRow>("SELECT * FROM fund_details WHERE fund_code = ?", code);
  if (!existing) return c.json({ error: "fund not found" }, 404);
  const body = await c.req.json().catch(() => ({}));
  const sets: string[] = []; const vals: any[] = [];
  if (body.fund_name) { sets.push("fund_name = ?"); vals.push(body.fund_name); }
  if (body.fund_type) { sets.push("fund_type = ?"); vals.push(body.fund_type); }
  if (body.security_type !== undefined) { sets.push("security_type = ?"); vals.push(body.security_type); }
  if (body.market !== undefined) { sets.push("market = ?"); vals.push(body.market); }
  if (!sets.length) return c.json({ error: "no fields to update" }, 400);
  vals.push(code);
  getRwDb().run(`UPDATE fund_details SET ${sets.join(", ")} WHERE fund_code = ?`, vals);
  log.info(`fund updated: ${code}`);
  return c.json({ ok: true, code });
});

router.delete("/funds/:code", c => {
  const code = c.req.param("code").padStart(6, "0");
  const existing = queryOne<FundDetailRow>("SELECT * FROM fund_details WHERE fund_code = ?", code);
  if (!existing) return c.json({ error: "fund not found" }, 404);
  const db = getRwDb();
  db.run("DELETE FROM fund_status WHERE fund_code = ?", [code]);
  db.run("DELETE FROM portfolio_snapshot WHERE fund_code = ?", [code]);
  db.run("DELETE FROM nav_history WHERE fund_code = ?", [code]);
  db.run("DELETE FROM transactions WHERE fund_code = ?", [code]);
  db.run("DELETE FROM fund_details WHERE fund_code = ?", [code]);
  log.info(`fund deleted: ${code} ${existing.fund_name} (incl. all related data)`);
  return c.json({ ok: true, deleted: { code, name: existing.fund_name } });
});

export default router;
