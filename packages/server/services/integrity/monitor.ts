/**
 * DB Integrity Monitor Service
 *
 * Periodic integrity monitoring with auto-repair fallback.
 */

import { log } from "../../middleware/logger";
import { runIntegrityCheck } from "./check";
import { attemptAutoRepair } from "./repair";

let monitorInterval: ReturnType<typeof setInterval> | null = null;

/**
 * Start periodic integrity monitoring.
 * Runs integrity_check every INTERVAL_MINUTES (default 60).
 * If corruption is detected, triggers auto-repair.
 * If auto-repair fails, logs CRITICAL and notifies.
 */
export function startIntegrityMonitor(intervalMinutes: number = 60) {
  if (monitorInterval) return;

  monitorInterval = setInterval(() => {
    const report = runIntegrityCheck();

    if (report.overall === "corrupted") {
      log.error("DB CORRUPTION DETECTED — auto-repair triggered", { report: JSON.stringify(report.checks) });

      const repair = attemptAutoRepair();
      log.error("auto-repair result", { repair });

      if (repair.needs_restore) {
        log.error("CRITICAL: auto-repair failed, manual restore required", { repair });
        // In production, this would trigger Feishu notification via webhook
        if (process.env.FEISHU_WEBHOOK) {
          fetch(process.env.FEISHU_WEBHOOK, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              msg_type: "text",
              content: {
                text: `[CRITICAL] fund-dashboard DB corruption detected and auto-repair FAILED.\n` +
                  `Integrity: ${report.checks.integrity_check.detail}\n` +
                  `Repair: ${repair.failed.join(", ")}\n` +
                  `Action: Manual restore from backup required.`,
              },
            }),
          }).catch(() => {});
        }
      }
    } else if (report.overall === "degraded") {
      log.warn("DB integrity degraded", { report: JSON.stringify(report.checks) });
    }
  }, intervalMinutes * 60_000);

  log.info(`DB integrity monitor started (every ${intervalMinutes}min)`);
}

export function stopIntegrityMonitor() {
  if (monitorInterval) {
    clearInterval(monitorInterval);
    monitorInterval = null;
    log.info("DB integrity monitor stopped");
  }
}
