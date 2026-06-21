/**
 * DB Integrity Repair Service
 *
 * Auto-repair: REINDEX → ANALYZE → VACUUM (if needed).
 */

import { getRwDb } from "../../db";

/**
 * Attempt automatic repair of common SQLite issues.
 * Order: REINDEX → ANALYZE → VACUUM (if needed).
 * Returns repair report.
 */
export function attemptAutoRepair(): { repaired: string[]; failed: string[]; needs_restore: boolean } {
  const repaired: string[] = [];
  const failed: string[] = [];
  let needsRestore = false;

  try {
    const db = getRwDb();

    // Step 1: REINDEX — fixes corrupted indexes
    try {
      db.run("REINDEX");
      repaired.push("REINDEX completed");
    } catch (e: any) {
      failed.push(`REINDEX: ${e.message}`);
    }

    // Step 2: ANALYZE — rebuilds query planner statistics
    try {
      db.run("ANALYZE");
      repaired.push("ANALYZE completed");
    } catch (e: any) {
      failed.push(`ANALYZE: ${e.message}`);
    }

    // Step 3: Check if VACUUM is needed
    const freelist = (db.query("PRAGMA freelist_count").get() as { freelist_count: number })?.freelist_count || 0;
    if (freelist > 500) {
      try {
        db.run("VACUUM");
        repaired.push(`VACUUM completed (reclaimed ${freelist} freelist pages)`);
      } catch (e: any) {
        failed.push(`VACUUM: ${e.message}`);
      }
    }

    // Step 4: Final integrity check after repair
    const postCheck = (db.query("PRAGMA integrity_check").all() as { integrity_check: string }[])[0];
    if (postCheck?.integrity_check !== "ok") {
      needsRestore = true;
      failed.push(`Post-repair integrity still failing: ${postCheck?.integrity_check}`);
    }
  } catch (e: any) {
    failed.push(`Auto-repair aborted: ${e.message}`);
    needsRestore = true;
  }

  return { repaired, failed, needs_restore: needsRestore || failed.length > 0 };
}

/**
 * Force a full REINDEX of the database.
 * Returns true on success, false on failure.
 */
export function forceReindex(): boolean {
  try {
    getRwDb().run("REINDEX");
    return true;
  } catch {
    return false;
  }
}
