/**
 * DB Integrity & Recovery Service
 *
 * Thin re-export layer. Implementation split across:
 *   services/integrity/check.ts   — integrity checks
 *   services/integrity/repair.ts  — auto-repair + force reindex
 *   services/integrity/backup.ts  — backup, restore, cleanup, listing
 *   services/integrity/monitor.ts — periodic integrity monitoring
 */

export { runIntegrityCheck, quickIntegrityCheck } from "./integrity/check";
export type { IntegrityReport } from "./integrity/check";
export { attemptAutoRepair, forceReindex } from "./integrity/repair";
export { restoreFromBackup, backupDatabase, cleanupOldBackups, listBackups } from "./integrity/backup";
export type { RestoreResult } from "./integrity/backup";
export { startIntegrityMonitor, stopIntegrityMonitor } from "./integrity/monitor";
