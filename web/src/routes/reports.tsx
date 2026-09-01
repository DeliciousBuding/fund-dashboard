// 报告 /reports —— 生成组合报告（JSON v1）→ 富渲染 + 下载。PDF 属 backlog（01 §4）。

import { type GenerateReportResult, GenerateReportResultSchema } from "@fund-dashboard/contracts";
import { useMutation } from "@tanstack/react-query";
import { Download, FileText } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Input, Label } from "../components/ui/input";
import { ApiError, api } from "../lib/api";
import { downloadText } from "../lib/csv";
import { useUi } from "../stores/ui";

// 章节标题中文化（known keys；未知 key 原样显示）
const SECTION_LABEL: Record<string, string> = {
  summary: "组合汇总",
  allocation: "配置结构",
  harness: "投资支架",
  dca: "定投计划",
  source_brief: "信源简报",
  source_events: "信源事件",
  xirr: "收益指标",
};

export function ReportsPage() {
  const portfolioId = useUi((s) => s.portfolioId);
  const [title, setTitle] = useState("");
  const [report, setReport] = useState<GenerateReportResult | null>(null);

  const generate = useMutation({
    mutationFn: async () => {
      const data = await api<unknown>("/api/reports", {
        method: "POST",
        body: { title: title || undefined, portfolio_id: portfolioId },
      });
      return GenerateReportResultSchema.parse(data);
    },
    onSuccess: (r) => setReport(r),
    onError: (e) =>
      toast.error("生成失败", { description: e instanceof ApiError ? e.code : String(e) }),
  });

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <FileText className="size-4 text-accent" />
            生成组合报告
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-wrap items-end gap-3">
          <div className="w-72">
            <Label htmlFor="report-title">标题（可选）</Label>
            <Input
              id="report-title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="默认：Portfolio report <日期>"
            />
          </div>
          <Button onClick={() => generate.mutate()} disabled={generate.isPending}>
            {generate.isPending ? "生成中…" : "生成报告"}
          </Button>
        </CardContent>
      </Card>

      {report && (
        <Card>
          <CardHeader className="flex-row flex-wrap items-center justify-between gap-2">
            <div>
              <CardTitle>{report.title}</CardTitle>
              <p className="mt-0.5 text-xs text-fg-3">
                {report.report_id} · 截至 {report.as_of} ·{" "}
                {new Date(report.generated_at).toLocaleString("zh-CN", { hour12: false })}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <Badge tone="accent">{report.decision_boundary}</Badge>
              <Button
                variant="outline"
                size="sm"
                onClick={() =>
                  downloadText(
                    `${report.report_id}.json`,
                    JSON.stringify(report, null, 2),
                    "application/json",
                  )
                }
              >
                <Download className="size-3.5" />
                下载 JSON
              </Button>
            </div>
          </CardHeader>
          <CardContent className="space-y-3">
            {Object.entries(report.sections).map(([key, value]) => (
              <details key={key} className="rounded-lg border border-border">
                <summary className="cursor-pointer px-4 py-2.5 text-sm font-medium text-fg-2 hover:text-fg">
                  {SECTION_LABEL[key] ?? key}
                </summary>
                <pre className="max-h-96 overflow-auto border-t border-border bg-surface-2 p-4 text-xs text-fg-2">
                  {JSON.stringify(value, null, 2)}
                </pre>
              </details>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
