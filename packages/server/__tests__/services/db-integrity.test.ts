/** DB Integrity Service Unit Tests */
import { describe, test, expect, mock, beforeAll } from "bun:test";
import { Database } from "bun:sqlite";
import { tmpdir } from "node:os";
import { mkdirSync, writeFileSync, rmSync, existsSync } from "node:fs";
import { join, resolve } from "node:path";

const memDb = new Database(":memory:");
memDb.run(`CREATE TABLE IF NOT EXISTS test_table (id INTEGER PRIMARY KEY, name TEXT)`);
memDb.run(`INSERT INTO test_table VALUES (1, 'hello')`);

// ── Mock db.ts ──────────────────────────────────────────────────────

mock.module("../../db", () => ({
  getDb: () => memDb,
  getRwDb: () => memDb,
  query: (sql: string, ...params: any[]) => memDb.query(sql).all(...params),
  queryOne: (sql: string, ...params: any[]) => memDb.query(sql).get(...params),
  initSchema: () => {},
}));

// ── Import services ─────────────────────────────────────────────────

import { runIntegrityCheck, attemptAutoRepair, restoreFromBackup } from "../../services/db-integrity";

// ═══════════════════════════════════════════════════════════════════════
// Tests
// ═══════════════════════════════════════════════════════════════════════

describe("runIntegrityCheck", () => {
  test("returns overall: 'ok' on fresh DB", () => {
    const report = runIntegrityCheck(memDb);
    expect(report).toHaveProperty("timestamp");
    expect(report).toHaveProperty("overall");
    expect(report).toHaveProperty("checks");
    expect(report).toHaveProperty("table_checksums");
    expect(report).toHaveProperty("row_counts");
    expect(report).toHaveProperty("recommendations");
    expect(report.overall).toBe("ok");
  });

  test("returns table_checksums with expected keys", () => {
    const report = runIntegrityCheck(memDb);
    expect(typeof report.table_checksums).toBe("object");
    expect(report.table_checksums).toHaveProperty("test_table");
  });

  test("checks.integrity_check is ok", () => {
    const report = runIntegrityCheck(memDb);
    expect(report.checks.integrity_check.passed).toBe(true);
  });

  test("row_counts contains test_table with count 1", () => {
    const report = runIntegrityCheck(memDb);
    expect(report.row_counts["test_table"]).toBe(1);
  });
});

describe("attemptAutoRepair", () => {
  test("returns shape with repaired/failed/needs_restore on mocked DB", () => {
    const result = attemptAutoRepair();
    expect(result).toHaveProperty("repaired");
    expect(result).toHaveProperty("failed");
    expect(result).toHaveProperty("needs_restore");
    expect(Array.isArray(result.repaired)).toBe(true);
    expect(Array.isArray(result.failed)).toBe(true);
    expect(typeof result.needs_restore).toBe("boolean");
  });
});

// ═══════════════════════════════════════════════════════════════════════
// restoreFromBackup — Path Traversal / Security Tests
// ═══════════════════════════════════════════════════════════════════════

describe("restoreFromBackup path validation", () => {
  let testBackupDir: string;

  beforeAll(() => {
    // Create a temp backup directory structure
    testBackupDir = join(tmpdir(), "fund-dashboard-test-backups-" + Date.now());
    mkdirSync(testBackupDir, { recursive: true });
    // Create a minimal valid SQLite DB as a fake backup
    const tmpDb = new Database(join(testBackupDir, "fund_test.db"));
    tmpDb.run("CREATE TABLE test (id INTEGER)");
    tmpDb.close();
    // Set BACKUP_DIR for the tests
    process.env.BACKUP_DIR = testBackupDir;
    // Unset DB_PATH to use default resolution (won't matter since we use BACKUP_DIR)
  });

  test("rejects path traversal via ../ from backup directory", async () => {
    const traversalPath = join(testBackupDir, "..", "..", "etc", "passwd");
    const result = await restoreFromBackup(traversalPath);
    expect(result.success).toBe(false);
    expect(result.errors.length).toBeGreaterThan(0);
    expect(result.errors[0]).toContain("Path traversal rejected");
  });

  test("rejects absolute path outside backup directory", async () => {
    // Use a path definitely outside the backup dir (system root or /etc)
    const outsidePath = process.platform === "win32"
      ? "C:\\Windows\\System32\\drivers\\etc\\hosts"
      : "/etc/passwd";
    const result = await restoreFromBackup(outsidePath);
    expect(result.success).toBe(false);
    expect(result.errors.length).toBeGreaterThan(0);
    expect(result.errors[0]).toContain("Path traversal rejected");
  });

  test("rejects path that resolves outside backup dir via symlink-like dots", async () => {
    // Path with redundant ../ that still resolves outside
    const sneakyPath = join(testBackupDir, "subdir", "..", "..", "..", "var", "log");
    const result = await restoreFromBackup(sneakyPath);
    expect(result.success).toBe(false);
    expect(result.errors.length).toBeGreaterThan(0);
    expect(result.errors[0]).toContain("Path traversal rejected");
  });

  test("allows valid path within backup directory (.gz)", async () => {
    // Create a gzipped test backup
    const validPath = join(testBackupDir, "fund_valid_test.db.gz");
    // Even though the file doesn't actually exist, the path validation
    // should pass and the error should be about gunzip/missing file, not path traversal
    const result = await restoreFromBackup(validPath);
    // The restore should fail (file doesn't exist or isn't valid gz) but
    // NOT because of path traversal
    const pathTraversalErrors = result.errors.filter(e => e.includes("Path traversal rejected"));
    expect(pathTraversalErrors.length).toBe(0);
  });

  test("allows valid path within backup directory (.db)", async () => {
    // Create a real .db file in the backup dir
    const validDb = join(testBackupDir, "fund_valid_restore.db");
    const tmpDb = new Database(validDb);
    tmpDb.run("CREATE TABLE test (id INTEGER)");
    tmpDb.run("INSERT INTO test VALUES (1)");
    tmpDb.close();

    const result = await restoreFromBackup(validDb);
    // Should not be a path traversal rejection
    const pathTraversalErrors = result.errors.filter(e => e.includes("Path traversal rejected"));
    expect(pathTraversalErrors.length).toBe(0);
    // Clean up
    try { rmSync(validDb); } catch { /* ok */ }
  });

  test("rejects backupPath with embedded null bytes (path manipulation)", async () => {
    // Null byte tricks: /backups/valid.db\0../../etc
    const nullBytePath = join(testBackupDir, "fund_test.db") + "\x00../../etc/passwd";
    const result = await restoreFromBackup(nullBytePath);
    expect(result.success).toBe(false);
    expect(result.errors.length).toBeGreaterThan(0);
    expect(result.errors[0]).toContain("Path traversal rejected");
  });

  // Cleanup
  try { rmSync(testBackupDir, { recursive: true, force: true }); } catch { /* ok */ }
  delete process.env.BACKUP_DIR;
});
