/** MCP Report Tools — generate_report for AI agents */

import { z } from "zod";
import type { ToolRegistrar } from "../shared";
import { generateWeeklyReport, generateMonthlyReport } from "../../services/report";

export const registerReportTools: ToolRegistrar = (server) => {
  server.tool("generate_report", {
    description: `生成PDF投资报告。支持周报(weekly)和月报(monthly)。
报告包含：组合摘要、持仓明细、收益走势、穿透分析（底层股票暴露）、风险提示。
月报额外包含：交易记录、本月涨跌排行、年化收益XIRR、最大回撤。
返回HTML格式的报告内容，可通过format参数指定。`,
    inputSchema: z.object({
      type: z.enum(["weekly", "monthly"]).default("weekly").describe("报告类型：weekly=周报（7天），monthly=月报（30天）"),
      format: z.enum(["html", "pdf"]).default("html").describe("输出格式：html=HTML字符串，pdf=base64编码PDF（需要Chrome/Edge）"),
    }),
    handler: async (args) => {
      const fn = args.type === "monthly" ? generateMonthlyReport : generateWeeklyReport;
      const report = await fn();

      if (args.format === "pdf") {
        if (report.pdf) {
          // Return PDF as base64-encoded string (survives JSON serialization)
          const b64 = Buffer.from(report.pdf).toString("base64");
          return {
            content: [{
              type: "text",
              text: JSON.stringify({
                type: args.type,
                format: "pdf",
                generated_at: report.generatedAt,
                period: `${report.periodStart} — ${report.periodEnd}`,
                pdf_base64: b64,
                pdf_size_bytes: report.pdf.byteLength,
                note: "PDF binary is base64-encoded. Decode to get the raw PDF file.",
              }, null, 2),
            }],
          };
        }
        // PDF failed, return HTML instead
        return {
          content: [{
            type: "text",
            text: JSON.stringify({
              type: args.type,
              format: "html (pdf_fallback)",
              generated_at: report.generatedAt,
              period: `${report.periodStart} — ${report.periodEnd}`,
              pdf_error: report.pdfError,
              html: report.html,
              note: "PDF conversion failed — returning HTML instead. Length is truncated for JSON transport.",
            }, null, 2),
          }],
        };
      }

      // HTML format — return the populated HTML
      return {
        content: [{
          type: "text",
          text: JSON.stringify({
            type: args.type,
            format: "html",
            generated_at: report.generatedAt,
            period: `${report.periodStart} — ${report.periodEnd}`,
            pdf_available: report.pdf !== null,
            pdf_size_bytes: report.pdf?.byteLength ?? null,
            html: report.html,
            note: "Full HTML report. Save as .html and open in browser; print to PDF for a polished file.",
          }, null, 2),
        }],
      };
    },
  });
};
