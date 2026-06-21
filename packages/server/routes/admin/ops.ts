/** /api/admin OPS — 运维操作端点: 诊断 · 批量导入 · 调仓 · 重算 · 校验 · 完整性 · 备份 */

import { Hono } from "hono";
import { query, queryOne, getRwDb } from "../../db";
import { log } from "../../middleware/logger";
import { runIntegrityCheck, attemptAutoRepair, restoreFromBackup } from "../../services/db-integrity";
import { recalculateAllSnapshots } from "../../services";
import { join, resolve, sep } from "node:path";
import { existsSync } from "node:fs";
import { validate, importTransactionsBodySchema, adjustPositionSchema } from "../../utils/validation";
import type { ImportTransactionsBody, AdjustPosition } from "../../utils/validation";
import { badRequest } from "../../utils/errors";
import { recalcFundSnapshot } from "./crud";

const router = new Hono();

// ═══════════ DIAGNOSTICS ═══════════

router.get("/status", c => {
  const t0 = Date.now();
  const lastTx = queryOne<{ t: string }>("SELECT MAX(trade_time) as t FROM transactions");
  const lastNav = queryOne<{ d: string }>("SELECT MAX(date) as d FROM nav_history");
  const anomalies = query<{ seq: number; fund_code: string; direction: string; trade_time: string; anomaly: string }>("SELECT seq, fund_code, direction, trade_time, anomaly FROM transactions WHERE anomaly IS NOT NULL");
  const held = queryOne<{ n: number }>("SELECT COUNT(*) as n FROM portfolio_snapshot WHERE held_shares > 0.001");
  const navStats = queryOne<{ total: number; funds: number; first: string; last: string }>(`
    SELECT COUNT(*) as total, COUNT(DISTINCT fund_code) as funds, MIN(date) as first, MAX(date) as last FROM nav_history
  `);
  const secStats = queryOne<{ total: number; funds: number; stocks: number }>(`
    SELECT
      COUNT(*) as total,
      SUM(CASE WHEN security_type = 'fund' OR security_type IS NULL THEN 1 ELSE 0 END) as funds,
      SUM(CASE WHEN security_type = 'stock' THEN 1 ELSE 0 END) as stocks
    FROM fund_details
  `);
  return c.json({
    ok: true, uptime_sec: +(process.uptime().toFixed(1)), response_ms: Date.now() - t0,
    transactions: { count: queryOne<any>("SELECT COUNT(*) as n FROM transactions")?.n, last: lastTx?.t },
    nav: { count: navStats?.total, funds: navStats?.funds, first: navStats?.first, last: navStats?.last },
    portfolio: { held_funds: held?.n },
    securities: { total: secStats?.total || 0, funds: secStats?.funds || 0, stocks: secStats?.stocks || 0 },
    anomalies: { count: anomalies.length, items: anomalies.slice(0, 20) },
  });
});

router.get("/status/:code", c => {
  const code = c.req.param("code").padStart(6, "0");
  return c.json({
    code,
    ...queryOne<{ name: string; type: string; security_type: string; market: string }>("SELECT fund_name as name, fund_type as type, security_type, market FROM fund_details WHERE fund_code = ?", code),
    transactions: queryOne<{ n: number; first: string; last: string }>("SELECT COUNT(*) as n, MIN(trade_time) as first, MAX(trade_time) as last FROM transactions WHERE fund_code = ?", code),
    nav: queryOne<{ n: number; first: string; last: string }>("SELECT COUNT(*) as n, MIN(date) as first, MAX(date) as last FROM nav_history WHERE fund_code = ?", code),
    position: queryOne<{ held_shares: number; current_value: number; unrealized_pnl: number }>("SELECT held_shares, current_value, unrealized_pnl FROM portfolio_snapshot WHERE fund_code = ?", code) || { held_shares: 0 },
    trading: queryOne<{ purchase_status: string; redemption_status: string }>("SELECT purchase_status, redemption_status FROM fund_status WHERE fund_code = ?", code) || {},
  });
});

// ═══════════ DATA OPERATIONS ═══════════

router.post("/import-transactions", validate(importTransactionsBodySchema), async c => {
  const { transactions } = c.get("validated") as ImportTransactionsBody;
  const db = getRwDb();
  const insert = db.prepare(`
    INSERT OR IGNORE INTO transactions
    (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name,
     confirm_amount, confirm_share, fee, inferred_nav, signed_cash_flow, signed_share_change)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
  `);
  let imported = 0;
  const affectedFunds = new Set<string>();
  const doImport = db.transaction((txs: ImportTransactionsBody["transactions"]) => {
    for (const tx of txs) {
      // Accept both fund_code and security_code
      const code = (tx.fund_code || tx.security_code || "").padStart(6, "0");
      const r = insert.run(tx.order_id, tx.trade_time, tx.confirm_date, tx.trade_type, tx.direction,
        code, tx.fund_name, tx.confirm_amount, tx.confirm_share, tx.fee,
        tx.inferred_nav, tx.signed_cash_flow, tx.signed_share_change);
      if (r.changes) { imported++; affectedFunds.add(code); }
    }
  });
  try {
    doImport(transactions);
    for (const fc of affectedFunds) recalcFundSnapshot(fc);
    log.info(`imported ${imported}/${transactions.length} tx, recalc ${affectedFunds.size} funds`);
    return c.json({ ok: true, imported, total: transactions.length, affected_funds: affectedFunds.size });
  } catch (e: any) {
    log.error("import failed", { error: e.message });
    return c.json({ error: e.message }, 500);
  }
});

router.post("/adjust-position", validate(adjustPositionSchema), c => {
  const { fund_code, shares } = c.get("validated") as AdjustPosition;
  getRwDb().run("UPDATE portfolio_snapshot SET held_shares = ?, current_value = held_shares * latest_nav WHERE fund_code = ?",
    [shares, fund_code]);
  return c.json({ ok: true });
});

router.post("/recalculate-snapshot", c => {
  const result = recalculateAllSnapshots();
  log.info("snapshot recalculated", { funds: result.securities });
  return c.json({ ok: true, funds: result.securities });
});

router.get("/verify", c => {
  const issues: string[] = [];
  const fundsWithoutNav = query<{ fund_code: string; fund_name: string }>(`SELECT fd.fund_code, fd.fund_name FROM fund_details fd
    WHERE fd.fund_code NOT IN (SELECT DISTINCT fund_code FROM nav_history)`);
  const negPos = query<{ fund_code: string; held_shares: number }>("SELECT fund_code, held_shares FROM portfolio_snapshot WHERE held_shares < -0.001");
  const nullSd = queryOne<{ n: number }>("SELECT COUNT(*) as n FROM transactions WHERE settlement_days IS NULL");
  if (fundsWithoutNav.length) issues.push(`${fundsWithoutNav.length} funds missing NAV`);
  if (negPos.length) issues.push(`${negPos.length} negative positions`);
  if (nullSd?.n) issues.push(`${nullSd.n} tx missing settlement_days`);
  if (!issues.length) issues.push("all clear");
  return c.json({ ok: issues.length === 0 || (issues.length === 1 && issues[0] === "all clear"), issues });
});

// ═══════════ INTEGRITY & BACKUP ═══════════

router.get("/db-integrity", c => {
  const report = runIntegrityCheck();
  return c.json(report);
});

router.post("/db-repair", c => {
  const result = attemptAutoRepair();
  return c.json(result);
});

router.post("/db-restore", async c => {
  const body = await c.req.json().catch(() => ({}));
  const backupFile: string | undefined = body.backup_file;

  if (!backupFile) {
    // Find latest backup — path adjusted for routes/admin/ subdirectory
    const backupDir = join(process.env.DB_PATH ? join(process.env.DB_PATH, "..") : join(import.meta.dir, "..", "..", "..", "data"), "backups");
    const files = existsSync(backupDir)
      ? [...new Bun.Glob("fund_*.db.gz").scanSync({ cwd: backupDir, absolute: true })]
          .sort()
          .reverse()
      : [];

    if (files.length === 0) {
      return c.json({ error: "no backup files found", backup_dir: backupDir }, 404);
    }

    const result = await restoreFromBackup(files[0]);
    return c.json(result);
  }

  if (!existsSync(backupFile)) {
    return c.json({ error: `backup file not found: ${backupFile}` }, 404);
  }

  // Path traversal guard: only allow restore from the designated backup directory
  const backupDir = resolve(join(process.env.DB_PATH ? join(process.env.DB_PATH, "..") : join(import.meta.dir, "..", "..", "..", "data"), "backups"));
  const resolvedPath = resolve(backupFile);
  if (!resolvedPath.startsWith(backupDir + sep) && resolvedPath !== backupDir) throw badRequest('Invalid backup file path');

  const result = await restoreFromBackup(backupFile);
  return c.json(result);
});

router.get("/backup-status", c => {
  const backupDir = join(process.env.DB_PATH ? join(process.env.DB_PATH, "..") : join(import.meta.dir, "..", "..", "..", "data"), "backups");

  if (!existsSync(backupDir)) {
    return c.json({ status: "no_backup_dir", path: backupDir });
  }

  const files: string[] = [];
  const manifestFiles: string[] = [];
  const weeklyFiles: string[] = [];
  const monthlyFiles: string[] = [];

  try {
    for (const f of new Bun.Glob("fund_*.db.gz").scanSync({ cwd: backupDir, absolute: false })) {
      files.push(f);
    }
  } catch { /* ok */ }

  try {
    for (const f of new Bun.Glob("manifest_*.json").scanSync({ cwd: backupDir, absolute: false })) {
      manifestFiles.push(f);
    }
  } catch { /* ok */ }

  try {
    for (const f of new Bun.Glob("weekly/fund_*.db.gz").scanSync({ cwd: backupDir, absolute: false })) {
      weeklyFiles.push(f);
    }
  } catch { /* ok */ }

  try {
    for (const f of new Bun.Glob("monthly/fund_*.db.gz").scanSync({ cwd: backupDir, absolute: false })) {
      monthlyFiles.push(f);
    }
  } catch { /* ok */ }

  // Get latest backup age
  let latestAgeHours: number | null = null;
  let latestFile: string | null = null;
  if (files.length > 0) {
    const sorted = files.sort().reverse();
    latestFile = sorted[0];
    try {
      const stat = Bun.file(join(backupDir, sorted[0])).lastModified;
      if (stat) {
        latestAgeHours = (Date.now() - stat) / 3600_000;
      }
    } catch { /* ok */ }
  }

  return c.json({
    status: files.length > 0 ? "ok" : "no_backups",
    path: backupDir,
    latest: latestFile,
    latest_age_hours: latestAgeHours ? Math.round(latestAgeHours * 10) / 10 : null,
    count: { daily: files.length, weekly: weeklyFiles.length, monthly: monthlyFiles.length, manifests: manifestFiles.length },
  });
});

export default router;
