/**
 * DB Backup & Restore Service
 *
 * Backup creation, restoration, cleanup, and listing.
 */

import { Database } from "bun:sqlite";
import { getRwDb, initSchema } from "../../db";
import { log } from "../../middleware/logger";
import * as fs from "node:fs";
import * as path from "node:path";
import * as zlib from "node:zlib";

export interface RestoreResult {
  success: boolean;
  source: string;
  target: string;
  tables_restored: number;
  rows_restored: number;
  errors: string[];
}

function resolveDbPaths() {
  const dbPath = process.env.DB_PATH || path.join(import.meta.dir, "..", "..", "..", "data", "fund.db");
  const dataDir = path.dirname(dbPath);
  const backupDir = process.env.BACKUP_DIR || path.join(dataDir, "backups");
  return { dbPath, dataDir, backupDir };
}

/** Create a gzipped backup of the current database. */
export async function backupDatabase(outputPath?: string): Promise<{ success: boolean; path: string; size: number; error?: string }> {
  try {
    const { dbPath, backupDir } = resolveDbPaths();
    if (!fs.existsSync(backupDir)) fs.mkdirSync(backupDir, { recursive: true });
    const dest = outputPath || path.join(backupDir, `fund_${new Date().toISOString().replace(/[:.]/g, "-")}.db.gz`);
    fs.writeFileSync(dest, zlib.gzipSync(fs.readFileSync(dbPath)));
    const stat = fs.statSync(dest);
    log.info("database backup created", { path: dest, size: stat.size });
    return { success: true, path: dest, size: stat.size };
  } catch (e: any) {
    log.error("database backup failed", { error: e.message });
    return { success: false, path: outputPath || "", size: 0, error: e.message };
  }
}

/**
 * Restore database from a backup file.
 * 1. Verify backup integrity 2. Replace live DB 3. Verify restored DB.
 */
export async function restoreFromBackup(backupPath: string): Promise<RestoreResult> {
  const result: RestoreResult = {
    success: false, source: backupPath,
    target: process.env.DB_PATH || "(default)",
    tables_restored: 0, rows_restored: 0, errors: [],
  };
  try {
    const { dbPath, backupDir } = resolveDbPaths();

    if (backupPath.includes("\x00")) {
      result.errors.push(`Path traversal rejected: backupPath contains an embedded null byte`);
      log.error("restore path traversal attempt blocked", { backupPath, backupDir });
      return result;
    }

    const resolvedBackup = path.resolve(backupPath);
    const resolvedBackupDir = path.resolve(backupDir);
    if (!resolvedBackup.startsWith(resolvedBackupDir + path.sep) && resolvedBackup !== resolvedBackupDir) {
      result.errors.push(
        `Path traversal rejected: backupPath "${backupPath}" resolves to "${resolvedBackup}" which is outside backup directory "${resolvedBackupDir}"`
      );
      log.error("restore path traversal attempt blocked", { backupPath, resolved: resolvedBackup, backupDir: resolvedBackupDir });
      return result;
    }

    const isGz = backupPath.endsWith(".gz");
    const tmpBackup = path.join(dbPath, "..", ".restore_tmp.db");
    if (isGz) {
      fs.writeFileSync(tmpBackup, zlib.gunzipSync(fs.readFileSync(resolvedBackup)));
    } else {
      fs.copyFileSync(resolvedBackup, tmpBackup);
    }

    // Verify backup integrity
    let tableCount = 0;
    const backupDb = new Database(tmpBackup, { readonly: true });
    try {
      const integrity = (backupDb.query("PRAGMA integrity_check").all() as { integrity_check: string }[])[0];
      if (integrity?.integrity_check !== "ok") {
        result.errors.push(`Backup file is corrupt: ${integrity?.integrity_check}`);
        backupDb.close();
        fs.unlinkSync(tmpBackup);
        return result;
      }
      tableCount = (backupDb.query("SELECT COUNT(*) as c FROM sqlite_master WHERE type='table'").get() as { c: number })?.c || 0;
      result.tables_restored = tableCount;
      try {
        const tables = backupDb.query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").all() as { name: string }[];
        let totalRows = 0;
        for (const { name } of tables) {
          try { totalRows += (backupDb.query(`SELECT COUNT(*) as c FROM "${name}"`).get() as { c: number })?.c || 0; } catch { /* skip */ }
        }
        result.rows_restored = totalRows;
      } catch { /* non-critical */ }
    } finally {
      backupDb.close();
    }

    // Replace live DB with verified backup
    const safetyCopy = dbPath + ".pre_restore_" + new Date().toISOString().replace(/[:.]/g, "-");
    if (fs.existsSync(dbPath)) {
      fs.copyFileSync(dbPath, safetyCopy);
      log.info("pre-restore safety copy saved", { path: safetyCopy });
    }
    fs.copyFileSync(tmpBackup, dbPath);
    fs.unlinkSync(tmpBackup);

    // Verify restored DB
    const restoredDb = new Database(dbPath, { readonly: true });
    try {
      const postIntegrity = (restoredDb.query("PRAGMA integrity_check").all() as { integrity_check: string }[])[0];
      if (postIntegrity?.integrity_check !== "ok") {
        result.errors.push(`Post-restore integrity failed: ${postIntegrity?.integrity_check}`);
        if (fs.existsSync(safetyCopy)) {
          fs.copyFileSync(safetyCopy, dbPath);
          result.errors.push("Rolled back to pre-restore state");
        }
        return result;
      }
      initSchema(getRwDb());
      result.success = true;
      log.info("database restored successfully", {
        source: path.basename(backupPath), tables: tableCount, rows: result.rows_restored,
      });
    } finally {
      restoredDb.close();
    }
    try { fs.unlinkSync(safetyCopy); } catch { /* ok */ }
  } catch (e: any) {
    result.errors.push(`Restore failed: ${e.message}`);
    log.error("database restore failed", { error: e.message, backup: backupPath });
  }
  return result;
}

/** Delete backup files older than maxAgeDays (default 30). */
export async function cleanupOldBackups(maxAgeDays: number = 30): Promise<{ deleted: number; errors: string[] }> {
  const errors: string[] = [];
  let deleted = 0;
  try {
    const { backupDir } = resolveDbPaths();
    if (!fs.existsSync(backupDir)) return { deleted: 0, errors: [] };
    const cutoff = Date.now() - maxAgeDays * 86_400_000;
    for (const entry of fs.readdirSync(backupDir, { withFileTypes: true })) {
      if (!entry.isFile() || !entry.name.startsWith("fund_") || !entry.name.endsWith(".db.gz")) continue;
      const fullPath = path.join(backupDir, entry.name);
      try {
        if (fs.statSync(fullPath).mtimeMs < cutoff) { fs.unlinkSync(fullPath); deleted++; }
      } catch (e: any) { errors.push(`${entry.name}: ${e.message}`); }
    }
    if (deleted > 0) log.info(`cleaned up ${deleted} old backups`);
  } catch (e: any) { errors.push(e.message); }
  return { deleted, errors };
}

/** List available backup files sorted by modification time (newest first). */
export async function listBackups(): Promise<{ path: string; size: number; modified: string }[]> {
  const results: { path: string; size: number; modified: string }[] = [];
  try {
    const { backupDir } = resolveDbPaths();
    if (!fs.existsSync(backupDir)) return results;
    for (const entry of fs.readdirSync(backupDir, { withFileTypes: true })) {
      if (!entry.isFile() || !entry.name.startsWith("fund_") || !entry.name.endsWith(".db.gz")) continue;
      const fullPath = path.join(backupDir, entry.name);
      try {
        const stat = fs.statSync(fullPath);
        results.push({ path: fullPath, size: stat.size, modified: stat.mtime.toISOString() });
      } catch { /* skip */ }
    }
  } catch { /* return empty */ }
  results.sort((a, b) => b.modified.localeCompare(a.modified));
  return results;
}
