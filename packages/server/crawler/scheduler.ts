/** Crawler scheduler — interval-based + manual trigger for funds, stocks, and holdings */

import { log } from "../middleware/logger";
import { getDb } from "../db";
import { refreshAllHeld } from "./nav";
import { refreshAllHoldings } from "./holdings";

let cronTask: ReturnType<typeof setInterval> | null = null;

/** Start daily price auto-refresh for all held securities (funds + stocks) on weekdays at 20:00 CST,
 *  holdings refresh every Saturday at 10:00 CST, and WAL checkpoint daily at 3:00 AM CST. */
export function startScheduler() {
  if (cronTask) return;

  // Run every hour, check if it's time to run specific tasks
  cronTask = setInterval(() => {
    const now = new Date();
    // CST = UTC+8; compute explicitly regardless of system timezone
    const utcHour = now.getUTCHours();
    const cstHour = (utcHour + 8) % 24;
    // Day-of-week wraps forward when UTC hour >= 16 (CST is next day)
    const cstDay = utcHour + 8 >= 24
      ? (now.getUTCDay() + 1) % 7
      : now.getUTCDay();

    // Daily price refresh: weekdays 20:00 CST
    if (cstHour === 20 && cstDay >= 1 && cstDay <= 5) {
      log.info("scheduled price refresh triggered (funds + stocks)");
      refreshAllHeld().catch(e => log.error("scheduled refresh failed", { error: e.message }));
    }

    // Weekly holdings refresh: Saturday 10:00 CST
    if (cstHour === 10 && cstDay === 6) {
      log.info("scheduled holdings refresh triggered (weekly)");
      refreshAllHoldings().catch(e => log.error("scheduled holdings refresh failed", { error: e.message }));
    }

    // Daily WAL checkpoint: 3:00 AM CST (flushes WAL to main DB, keeps WAL mode active)
    if (cstHour === 3) {
      try {
        const db = getDb();
        db.run("PRAGMA wal_checkpoint(TRUNCATE)");
        log.info("daily WAL checkpoint completed");
      } catch (e: any) {
        log.error("daily WAL checkpoint failed", { error: e.message });
      }
    }
  }, 3600_000); // check every hour

  log.info("crawler scheduler started (weekdays 20:00 CST price, Saturdays 10:00 CST holdings)");
}

export function stopScheduler() {
  if (cronTask) { clearInterval(cronTask); cronTask = null; }
}

/** Manual full refresh for all held securities — funds and stocks (triggered via admin API) */
export async function manualRefresh(): Promise<{ total: number; added: number }> {
  log.info("manual price refresh triggered (funds + stocks)");
  return refreshAllHeld();
}

/** Manual holdings refresh (triggered via admin API) */
export async function manualHoldingsRefresh(): Promise<{ total: number; updated: number; details: any[] }> {
  log.info("manual holdings refresh triggered");
  return refreshAllHoldings();
}
