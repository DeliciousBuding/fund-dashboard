/**
 * DB Integrity Check Service
 *
 * Corruption detection: integrity_check, foreign_key_check, quick_check, freelist_count.
 * Table-level checksums for silent corruption detection.
 */

import { Database } from "bun:sqlite";
import { getDb, getRwDb } from "../../db";

// ═══════════════════════════════════════════════════════════════
// Types
// ═══════════════════════════════════════════════════════════════

export interface IntegrityReport {
  timestamp: string;
  overall: "ok" | "degraded" | "corrupted";
  checks: {
    integrity_check: { passed: boolean; detail: string };
    foreign_key_check: { passed: boolean; violations: number };
    quick_check: { passed: boolean; result: string };
    freelist_count: { passed: boolean; freelist: number; detail: string };
  };
  table_checksums: Record<string, string>;
  row_counts: Record<string, number>;
  recommendations: string[];
}

// ═══════════════════════════════════════════════════════════════
// 1. Full Integrity Check
// ═══════════════════════════════════════════════════════════════

/**
 * Run full integrity diagnostics on the database.
 * Returns a detailed IntegrityReport.
 */
export function runIntegrityCheck(db?: Database): IntegrityReport {
  const d = db || getRwDb();
  const report: IntegrityReport = {
    timestamp: new Date().toISOString(),
    overall: "ok",
    checks: {
      integrity_check: { passed: true, detail: "ok" },
      foreign_key_check: { passed: true, violations: 0 },
      quick_check: { passed: true, result: "ok" },
      freelist_count: { passed: true, freelist: 0, detail: "" },
    },
    table_checksums: {},
    row_counts: {},
    recommendations: [],
  };

  // --- integrity_check (full B-tree validation) ---
  try {
    const result = d.query("PRAGMA integrity_check").all() as { integrity_check: string }[];
    const detail = result.map(r => r.integrity_check).join("\n");
    report.checks.integrity_check.detail = detail;
    report.checks.integrity_check.passed = detail === "ok";
  } catch (e: any) {
    report.checks.integrity_check.passed = false;
    report.checks.integrity_check.detail = e.message;
  }

  // --- foreign_key_check ---
  try {
    const fk = d.query("PRAGMA foreign_key_check").all() as Record<string, unknown>[];
    report.checks.foreign_key_check.violations = fk.length;
    report.checks.foreign_key_check.passed = fk.length === 0;
  } catch (e: any) {
    report.checks.foreign_key_check.passed = false;
    report.checks.foreign_key_check.violations = -1;
  }

  // --- quick_check (faster, less thorough) ---
  try {
    const qc = d.query("PRAGMA quick_check").all() as { quick_check: string }[];
    const qcResult = qc.map(r => r.quick_check).join("\n");
    report.checks.quick_check.result = qcResult;
    report.checks.quick_check.passed = qcResult === "ok";
  } catch (e: any) {
    report.checks.quick_check.passed = false;
    report.checks.quick_check.result = e.message;
  }

  // --- freelist_count (high freelist = fragmentation + potential corruption) ---
  try {
    const fl = d.query("PRAGMA freelist_count").all() as { freelist_count: number }[];
    const freelist = fl[0]?.freelist_count || 0;
    report.checks.freelist_count.freelist = freelist;
    if (freelist > 1000) {
      report.checks.freelist_count.passed = false;
      report.checks.freelist_count.detail = `High freelist count (${freelist} pages). Consider VACUUM.`;
      report.recommendations.push("Run VACUUM to reclaim freelist pages and defragment");
    } else {
      report.checks.freelist_count.passed = true;
      report.checks.freelist_count.detail = `${freelist} pages (normal)`;
    }
  } catch (e: any) {
    report.checks.freelist_count.passed = false;
    report.checks.freelist_count.detail = e.message;
  }

  // --- Per-table checksums and row counts ---
  try {
    const tables = d.query(
      "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"
    ).all() as { name: string }[];

    for (const { name } of tables) {
      try {
        const count = d.query(`SELECT COUNT(*) as c FROM "${name}"`).get() as { c: number };
        report.row_counts[name] = count?.c || 0;

        // Checksum via total(unicode(...)) on first text column for quick change detection
        try {
          const cols = d.query(`PRAGMA table_info('${name}')`).all() as { name: string }[];
          const textCol = cols.find(c => c.name !== "seq" && c.name !== "order_id");
          if (textCol) {
            const cs = d.query(
              `SELECT hex(sha256(group_concat(CAST("${textCol.name}" AS TEXT), '\n'))) as h FROM (SELECT * FROM "${name}" ORDER BY rowid LIMIT 10000)`
            ).get() as { h: string } | undefined;
            report.table_checksums[name] = cs?.h || "empty";
          } else {
            report.table_checksums[name] = `rows=${report.row_counts[name]}`;
          }
        } catch {
          report.table_checksums[name] = `rows=${report.row_counts[name]}`;
        }
      } catch (e: any) {
        report.recommendations.push(`Table "${name}" is unreadable: ${e.message}`);
      }
    }
  } catch (e: any) {
    report.recommendations.push(`Failed to enumerate tables: ${e.message}`);
  }

  // --- Determine overall status ---
  const allPassed =
    report.checks.integrity_check.passed &&
    report.checks.foreign_key_check.passed &&
    report.checks.quick_check.passed &&
    report.checks.freelist_count.passed;

  if (!report.checks.integrity_check.passed || !report.checks.quick_check.passed) {
    report.overall = "corrupted";
    report.recommendations.push("CRITICAL: Database is corrupted. Restore from latest backup immediately.");
  } else if (!allPassed) {
    report.overall = "degraded";
  }

  return report;
}

// ═══════════════════════════════════════════════════════════════
// 2. Quick Integrity Check (lightweight)
// ═══════════════════════════════════════════════════════════════

/**
 * Lightweight integrity check using PRAGMA quick_check only.
 * Returns true if the database passes, false otherwise.
 */
export function quickIntegrityCheck(db?: Database): boolean {
  const d = db || getRwDb();
  try {
    const qc = d.query("PRAGMA quick_check").all() as { quick_check: string }[];
    return qc.map(r => r.quick_check).join("\n") === "ok";
  } catch {
    return false;
  }
}

