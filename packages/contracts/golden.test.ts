// golden.test.ts — 契约双向门禁 node 侧。
//
// internal/httpapi/golden_test.go 把真实 Go 响应（滚动字段已哨兵化）dump 成
// testdata/golden/<域名>__<端点名>.json；本测试按显式映射表把每个金样本喂给
// 对应 zod schema —— Go 或 zod 任何一侧漂移都会在这里红。
//
// 金样本里 "__SCRUBBED__" / 0 是 Go 侧 golden_test.go 对时钟类字段的固定哨兵，
// 类型与真实 wire 一致（string / number），不影响形状校验。
//
// 重新生成金样本：GOLDEN_UPDATE=1 go test ./internal/httpapi -run TestGolden
import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import { z } from "zod";

import { FreshnessReportSchema } from "./schemas/admin.ts";
import { CheckAlertsResponseSchema } from "./schemas/alerts.ts";
import { CompareResultSchema, DrawdownResultSchema, XirrResultSchema } from "./schemas/analysis.ts";
import {
  AuthEventsResponseSchema,
  AuthSessionsResponseSchema,
  AuthStatusSchema,
} from "./schemas/auth.ts";
import { DcaComputeResultSchema, DcaPlansResponseSchema } from "./schemas/dca.ts";
import {
  FundDetailSchema,
  FundInfoSchema,
  NavPointSchema,
  TransactionsResponseSchema,
} from "./schemas/funds.ts";
import {
  InvestmentHarnessSnapshotSchema,
  InvestmentSourceBriefSchema,
  SourceEventsResponseSchema,
} from "./schemas/harness.ts";
import { ExchangeRateSchema, IndexLiveSchema, MarketIndexSchema } from "./schemas/market.ts";
import {
  PenetrationResultSchema,
  PortfolioAllocationSchema,
  PortfolioDefinitionSchema,
  PortfolioSchema,
  TimelinePointSchema,
} from "./schemas/portfolio.ts";
import { GenerateReportResultSchema } from "./schemas/reports.ts";
import { USStockInfoSchema } from "./schemas/stocks.ts";
import {
  SystemAgentResponseSchema,
  SystemAuditResponseSchema,
  SystemIntegrityReportSchema,
  SystemJobsResponseSchema,
  SystemStatusSchema,
} from "./schemas/system.ts";

// 显式映射表：金样本文件名 → 契约域 + zod schema（数组响应在此处包 z.array）。
const GOLDEN_SCHEMA_MAP: Record<string, { domain: string; schema: z.ZodTypeAny }> = {
  "admin__freshness.json": { domain: "admin", schema: FreshnessReportSchema },
  "alerts__check.json": { domain: "alerts", schema: CheckAlertsResponseSchema },
  "analysis__compare.json": { domain: "analysis", schema: CompareResultSchema },
  "analysis__fund_drawdown.json": { domain: "analysis", schema: DrawdownResultSchema },
  "analysis__fund_xirr.json": { domain: "analysis", schema: XirrResultSchema },
  "analysis__portfolio_xirr.json": { domain: "analysis", schema: XirrResultSchema },
  "auth__events.json": { domain: "auth", schema: AuthEventsResponseSchema },
  "auth__sessions.json": { domain: "auth", schema: AuthSessionsResponseSchema },
  "auth__status.json": { domain: "auth", schema: AuthStatusSchema },
  "dca__plans.json": { domain: "dca", schema: DcaPlansResponseSchema },
  "funds__dca_compute.json": { domain: "dca", schema: DcaComputeResultSchema },
  "funds__detail.json": { domain: "funds", schema: FundDetailSchema },
  "funds__list.json": { domain: "funds", schema: z.array(FundInfoSchema) },
  "funds__nav_history.json": { domain: "funds", schema: z.array(NavPointSchema) },
  "funds__transactions.json": { domain: "funds", schema: TransactionsResponseSchema },
  "harness__snapshot.json": { domain: "harness", schema: InvestmentHarnessSnapshotSchema },
  "harness__source_brief.json": { domain: "harness", schema: InvestmentSourceBriefSchema },
  "harness__source_events.json": { domain: "harness", schema: SourceEventsResponseSchema },
  "market__exchange_rate.json": { domain: "market", schema: ExchangeRateSchema },
  "market__index_live.json": { domain: "market", schema: IndexLiveSchema },
  "market__indices.json": { domain: "market", schema: z.array(MarketIndexSchema) },
  "portfolio__allocation.json": { domain: "portfolio", schema: PortfolioAllocationSchema },
  "portfolio__penetration.json": { domain: "portfolio", schema: PenetrationResultSchema },
  "portfolio__portfolios.json": { domain: "portfolio", schema: z.array(PortfolioDefinitionSchema) },
  "portfolio__summary.json": { domain: "portfolio", schema: PortfolioSchema },
  "portfolio__timeline.json": { domain: "portfolio", schema: z.array(TimelinePointSchema) },
  "reports__generate.json": { domain: "reports", schema: GenerateReportResultSchema },
  "stocks__aapl.json": { domain: "stocks", schema: USStockInfoSchema },
  "system__agent.json": { domain: "system", schema: SystemAgentResponseSchema },
  "system__audit.json": { domain: "system", schema: SystemAuditResponseSchema },
  "system__integrity.json": { domain: "system", schema: SystemIntegrityReportSchema },
  "system__jobs.json": { domain: "system", schema: SystemJobsResponseSchema },
  "system__status.json": { domain: "system", schema: SystemStatusSchema },
};

const goldenDir = join(dirname(fileURLToPath(import.meta.url)), "testdata", "golden");

function formatIssues(error: z.ZodError): string {
  return error.issues
    .map((issue) => {
      const path = issue.path.length > 0 ? issue.path.join(".") : "(root)";
      return `  - ${path}: ${issue.message}`;
    })
    .join("\n");
}

// 元测试：磁盘上的金样本与映射表必须一一对应 —— 新增金样本忘了映射、
// 或删了金样本没删映射，都要红。
test("golden files and schema mapping stay in sync", () => {
  assert.equal(existsSync(goldenDir), true, `missing golden dir: ${goldenDir}`);
  const onDisk = readdirSync(goldenDir)
    .filter((name) => name.endsWith(".json"))
    .sort();
  const mapped = Object.keys(GOLDEN_SCHEMA_MAP).sort();
  const unmapped = onDisk.filter((name) => !(name in GOLDEN_SCHEMA_MAP));
  const missing = mapped.filter((name) => !onDisk.includes(name));
  assert.deepEqual(unmapped, [], `golden files without a schema mapping: ${unmapped.join(", ")}`);
  assert.deepEqual(missing, [], `mapped golden files missing on disk: ${missing.join(", ")}`);
  assert.equal(onDisk.length, mapped.length);
});

for (const [file, { domain, schema }] of Object.entries(GOLDEN_SCHEMA_MAP)) {
  test(`golden/${domain}: ${file} matches the zod contract`, () => {
    const raw = readFileSync(join(goldenDir, file), "utf8");
    let data: unknown;
    assert.doesNotThrow(() => {
      data = JSON.parse(raw);
    }, `${file} is not valid JSON`);
    const result = schema.safeParse(data);
    if (!result.success) {
      assert.fail(
        `Go wire shape drifted from the zod contract for ${file}\n` +
          `schema: ${schema.description || domain}\n` +
          `zod issues:\n${formatIssues(result.error)}\n` +
          `If the Go change is intended, update the schema here (contracts SSOT) — ` +
          `never loosen it to make the golden pass.`,
      );
    }
  });
}
