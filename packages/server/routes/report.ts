/** /api/report endpoints — PDF investment report generation
 *
 *  GET /api/report/weekly  — weekly PDF report (7-day window)
 *  GET /api/report/monthly — monthly PDF report (30-day window)
 *  GET /api/report/weekly/html  — weekly HTML preview
 *  GET /api/report/monthly/html — monthly HTML preview
 *
 *  Query params:
 *    format=html|pdf  (default: pdf)
 */

import { Hono } from "hono";
import { generateWeeklyReport, generateMonthlyReport } from "../services/report";
import { log } from "../middleware/logger";

const router = new Hono();

/** Helper: send a ReportResult as response based on format query param */
async function sendReport(c: any, report: Awaited<ReturnType<typeof generateWeeklyReport>>) {
  const format = c.req.query("format") || "pdf";

  if (format === "html") {
    return c.html(report.html);
  }

  // PDF binary
  if (report.pdf) {
    c.header("Content-Type", "application/pdf");
    c.header("Content-Disposition", `attachment; filename="fund-report-${report.periodEnd}.pdf"`);
    return c.body(report.pdf);
  }

  // PDF generation failed — fall back to HTML with note
  c.header("Content-Type", "text/html; charset=utf-8");
  const fallbackHtml = `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Report (HTML Fallback)</title></head><body>
    <p style="color:#dc2626;padding:12px;background:#fef2f2;border:1px solid #fecaca;border-radius:6px;">
      PDF conversion unavailable: ${report.pdfError || "Unknown error"}. Showing HTML fallback.
    </p>
    ${report.html}
  </body></html>`;
  return c.html(fallbackHtml);
}

// ── Weekly ─────────────────────────────────────────────────────────
router.get("/weekly", async (c) => {
  const t0 = Date.now();
  try {
    const report = await generateWeeklyReport();
    log.debug("weekly report generated", { duration: Date.now() - t0, pdfSize: report.pdf?.byteLength ?? "none" });
    return sendReport(c, report);
  } catch (e: any) {
    log.error("weekly report failed", { error: e.message });
    return c.json({ error: "report_generation_failed", message: e.message }, 500);
  }
});

// ── Monthly ────────────────────────────────────────────────────────
router.get("/monthly", async (c) => {
  const t0 = Date.now();
  try {
    const report = await generateMonthlyReport();
    log.debug("monthly report generated", { duration: Date.now() - t0, pdfSize: report.pdf?.byteLength ?? "none" });
    return sendReport(c, report);
  } catch (e: any) {
    log.error("monthly report failed", { error: e.message });
    return c.json({ error: "report_generation_failed", message: e.message }, 500);
  }
});

export default router;
