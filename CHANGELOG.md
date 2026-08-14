# Changelog

本文件记录用户可见的变更。格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 精神，结合本仓库的中文条目约定。

## 格式约定

- `[Unreleased]` 使用**扁平**条目：`- [类型] 描述`
- 类型：`新功能` / `改进` / `修复` / `文档` / `测试` / `chore`
- **禁止**在 `[Unreleased]` 内新增 `###` 子标题（减少并发冲突）
- 发版时由 maintainer 汇总为正式分段：`## [x.y.z] - YYYY-MM-DD`

## 发版流程

1. 确认所有改动已合入 `main`
2. 运行 `./scripts/release.sh <x.y.z>`（自动把 `[Unreleased]` 归入 `[x.y.z]`、打 tag `v<x.y.z>`、推送）
3. tag 推送触发 GitHub Actions，自动创建 Release（release notes 取自本文件对应版本段）

完整约定见 [CONTRIBUTING.md](CONTRIBUTING.md#发布流程)。

## [Unreleased]

- **chore** 删除 LEGACY `packages/crawler`（Python AKShare 离线爬虫；生产已是 Go datasource/jobs）。
- **改进** `GetMarketIndices` stale refresh 走 singleflight + 12s 上游超时，避免 HTTP/MCP 惊群。
- **改进** SQLite 打开路径显式 `PRAGMA journal_mode=WAL` + `synchronous=NORMAL`（`db.Open` / `sqlitedb.Open`；只读跳过 mode change），与 runbook/scheduler checkpoint 对齐，避免 fresh/dump 退回 DELETE+FULL。
- **改进** `upsertNavHistory` 整段 series 包事务（与 holdings 一致），减少 autocommit 逐行 fsync。
- **修复** `/api/market/indices` 输出按 `code` 稳定排序（map 遍历顺序不确定）。
- **chore** 删除根目录死脚本 `portfolio_risk.py`（硬编码个人持仓，隐私风险）。
- **测试** `db.Open` WAL/synchronous 断言；sqlitedb open 校验 journal_mode。
- **修复** write 控制路径：`RowsAffected` 驱动错误不再当 0 行；`AdjustPosition` 校验读 `held_shares`、admin/jobs recalc 读 `portfolio_id` 不再静默忽略 SQL 错误。
- **运维** 部署闭环：`deploy/push-ghcr.sh`（float 生产镜像 + pin tags）+ `deploy/hotswap 脚本`（env 去重、`FUND_VERSION` false-pin guard、placeholder 拒绝）；compose/deploy.sh/README 对齐。
- **改进** admin/ops dashboard `system.build_version`（`FUND_VERSION`；鉴权面，非 public health）；smoke 拒绝 `dev|latest|test|local`。
- **改进** `RecalcAll` / admin `recalculate-snapshot` / MCP `recalculate_snapshot`：`status=complete|partial|error` + `failed_codes[]`（恒数组，与 crawl-nav 对齐）；smoke 覆盖 admin+MCP all-mode body。
- **chore** 删除未接线死包 `internal/repository/portfolio`（~500 行）；AdminDashboard 去掉 Bun 时代 `node_version`。
- **测试** admin partial/all-failed；`RecalcAllStatus`；`useEChart` DPR；`pnlBucketIndex` 边界。
- **修复** 盈亏分布分桶：半开区间 `[min,max)`，避免精确边界（-10%/+10%）漏计。
- **修复** `recalcSnapshot` 查询 `portfolio_id` 失败不再静默默认 1（`ErrNoRows` 仍默认）；`navHistoryColumns` 尊重 ctx 取消并 debug 记 PRAGMA 失败。
- **改进** high-DPI / UIUX：`useEChart` 按 `devicePixelRatio`（cap 3）初始化并在分辨率变化时 re-init；字号小端 +1px；`hitTarget.min` 28；viewport-fit=cover；≥2x 略强 glass blur。
- **改进** MarketTicker 刷新/展开命中区 ≥28；ExchangeRateBadge 走 theme token；ChartFallback 复用全局 shimmer。
- **chore** i18n 删 dead keys（backtest.* 整块、allocation/penetration loading、comparison.bar/return、portfolio.tooltip）；`common.units` en=`funds`；`dca.totalInvested`。
- **chore** 删除未使用 `SHARED_CHART_GRID` 导出。
- **修复** ensureNavSchema：每刷新仅 once 探测列，缺失才 ALTER（避免每证券双 ALTER）。
- **修复** usStockSnap/indexHistory 进程缓存：过期清扫 + 上限 200 oldest 淘汰。
- **修复** WriteJSON：先 Marshal，失败 500+log，避免静默截断 JSON。
- **修复** pg rebind：单引号字面量内 `?` 不替换；补 `rebind_test.go`。
- **#277** 修复：MCP `get_source_events` 尊重 `unread_only`；`get_fund_status`/`get_source_events` 接受 `fund_code` 别名。
- **#276** 修复：移动端主内容 `paddingBottom` 避开固定底栏。
- **#275/#274** 修复：注册 YahooStock `PriceSource`，持仓股票可刷新。
- **#273** 修复：`portfolio_snapshot` upsert 显式 `portfolio_id`（ORDER BY + LIMIT 1），避免复合主键下标量子查询。
- **#272** 修复：`RecalcAllSnapshots` 单码失败 soft-fail 继续，不再整批中止。
- **#271** 修复：EastmoneyFund 拒绝非 2xx HTTP 响应。
- **#270** 修复：`get_full_dashboard` 描述与实际 payload 对齐。
- **#269** 修复：MCP 写工具缺 confirmation 时引导 prepare 端点。
- **#268** 修复：scheduler `Stop` 取消 startup catch-up AfterFunc / job context。
- **#267** 修复：OverviewPage `i18n` 解构，消除 formatNavDate 崩溃。
- **chore** smoke-prod：`pybin` 提前定义；tools/list 计数改 argv 传路径（Windows native python 可读 MSYS 临时文件）。
- **#266** 稳健：agent confirmation request_id/caller 长度错误稳定码；ARCHITECTURE crawl-nav 文档 stale_only。
- **#265** 文档：SECURITY-AUDIT EdgeKey 注入范围与 #260 path-scoped 配置对齐。
- **#264** 文档：历史 git 密钥 residual 清单 + 轮换流程（**未** live 轮换）。
- **#263** 文档：GHCR IMAGE_TAG cutover runbook（现为 GHCR 生产镜像）。
- **#262** 数据：portfolio_snapshot 主键 `(fund_code, portfolio_id)` + PG 安全迁移 + 复合 ON CONFLICT。
- **#261** 安全：SPA + 公网读 API 启用 TokenDance ID OIDC（`auth_request`；未登录 302 login）。
- **#260** 安全：nginx EdgeKey 仅注入 SPA 写路径（transactions/source-events/ops）。
- **#259** 文档：Hermes mcp.json crawl_nav/get_data_freshness 描述对齐 stale_only / recommended_codes。
- **#258** 测试：smoke MCP crawl_nav(stale_only=true) 端到端（healthy 时 mode=stale_only）。
- **#257** 测试：smoke harness crawl_nav Input.stale_only + admin crawl-nav?stale_only=1。
- **#256** 测试：smoke-prod 断言 tools/list 中 crawl_nav.inputSchema.properties.stale_only。
- **#255** 测试：smoke-prod 校验 get_data_freshness 的 recommended_codes / recommended_maintenance_args.stale_only。
- **改进** 爬虫调度：startup + 工作日 20:00 CST 改为 **stale_only**；NAV upsert 增量丢弃 ≤ 本地 MAX(date)。
- **改进** 产品：NAV 新鲜度条（severity/warn/critical）+ portfolio React Query staleTime 5m。
- **修复** portfolioId 作用域：XIRR 终端市值优先 snapshot；compare/MCP/FE detail/DCA/harness 软过滤 source_events。
- **chore** 工程洁癖：删除死代码 AppDataContext；XIRR/Compare 显式 portfolioID；api encodeURIComponent + fetchPortfolioTimeline。

> **Status note (2026-07-18):** open non-gated P0/P1 **none**; **#5** AgentOps closed (prod enabled); residual closed through **#277**.  
> Older bullets below may still say “开放集仍仅 #5” in past residual waves — those phrases are **historical** (then-open set), not current.

- **#254** 改进：harness 推荐 crawl_nav 时 Input 改为 `{stale_only:true}`，不再引导全量 held 刷新。

- **#253** 改进：Hermes portfolio-review/feishu-daily-digest 文档化 stale_only 回路；admin crawl-nav 支持 `stale_only=1` 与 MCP 对齐。

- **#252** 改进：get_data_freshness 返回 recommended_codes / recommended_maintenance_args；crawl_nav 支持 stale_only 仅刷新过期/缺失持仓（Agent 自动取最新信息路径）。

- **#251** 稳健：dca_run weekday Message、index_history 空 quote Message、integrity freelist Detail 动态模板夹紧。

- **#250** 稳健：getTransaction 读字段、DCA get-by-id 回退、dca_compute market/type、AdjustPosition fundName、harness Reason、source_brief Query/Reason/EntityName 自由文本夹紧。

- **#249** 稳健：SearchFunds/definitions/allocation/us_stock quote+profile/index_history 自由文本夹紧；repository ListSecurities/scanHolding 防御夹紧。

- **#248** 稳健：公开 portfolio 列表/穿透/搜索/DCA/detail 自由文本夹紧；scheduler 长任务 WithTimeout。

- **#247** 稳健：jobs 限速 sleep 可取消；Yahoo 多指数部分成功；indices 刷新失败 Message 脱敏为 `upstream_unavailable`。

- **#246** 稳健：market_indices soft LIMIT + 文本夹紧；summary GROUP BY soft LIMIT/键夹紧；scheduler WAL 走 context。

- **#245** 稳健：ensureNavSchema 走 ExecContext；alerts/holdings_coverage/status 自由文本 `clampAdminText`。

- **#244** 稳健：system_status anomaly 与 freshness Code/Name/Type/LastNAV 自由文本夹紧（共享 `clampAdminText`，对齐 #243）。

- **#243** 稳健：ops dashboard anomaly 自由文本字段截断；exchange_rate HTTP Client 显式 5s Timeout。
- **#242** 稳健：exchange_rate JSON LimitReader 1MiB；static SPA 8MiB 上限 + seekable 流式 ServeContent。
- **#241** 稳健：Yahoo 响应 LimitReader 4MiB；NAV/指数/美股 history 点上限 5000；holdings 行上限 500；upsertNavHistory 走 context。
- **#240** 安全：market SSE `: warn` 脱敏；integrity FK/table 错误 detail 稳定码；MCP `textJSONResult` 1MiB 上限。
- **#239** 稳健：CrawlCode security_type 查询改 QueryRowContext；integrity 用户表清单 soft LIMIT 500。
- **#238** 稳健：jobs getHeldSecurities QueryContext+LIMIT；CrawlAllHeld/RecalcAllSnapshots/fundHoldingsMatch soft LIMIT。
- **#237** 稳健：timeline transactions LIMIT 20000；admin verify missing NAV/negative positions LIMIT 5000。
- **#236** 稳健：harness/alerts held、holdings_coverage funds、allocation buckets soft LIMIT 5000。
- **#235** 稳健：admin freshness missing/stale soft LIMIT 5000；penetration funds/holdings/sector_map soft LIMIT。
- **#234** 稳健：summary settlement 分布改 GROUP BY；definitions/DCA-run/alerts/repo ListHoldings soft LIMIT。
- **#233** 安全/稳健：legacy `crawlHandler` 客户端错误脱敏为 `internal_error`（slog 全量）；`ListDCAPlans` soft LIMIT 5000。

- **#232** 稳健：XLSX 导出行字符串字段截断；legacy repository ListSecurities LIMIT 5000。

- **#231** 稳健：XLSX export 文件名去控制字符/长度上限；ListSecurities LIMIT 5000；getSecurityItem 直查。

- **#230** 稳健：portfolio 服务层 `clampPortfolioID`；agent-context `base_currency` 规范化；admin alerts/coverage 上限。

- **#229** 安全/稳健：AgentOps request_id/caller≤128；audit JSON 64KiB 截断；DeleteSecurity/ListDCA/MarkSource 小钳制。

- **#228** 稳健：指数 history 缓存键先规范化 range/interval；未知 range 默认 1y；jobs CrawlCode code≤32。

- **#227** 稳健：DCA frequency≤32、active 仅 0/1；指数 symbol 规范化截断 32。

- **#226** 稳健：`NormalizeSecurityCode` 与美股 symbol 截断至 32；MCP crawl/recalc 保留过长 code 守卫。

- **#225** 稳健：source-events 查询过滤字段截断；admin crawl/recalc `code` 长度 >32 拒绝。

- **#224** 稳健：source-event related 字段、DCA fund_name/日期、import signed 覆盖量级上限。

- **#223** 稳健：AdjustPosition 限制 shares≤1e9、reason≤200、fund_code≤32、portfolio_id≤1000。

- **#222** 安全/稳健：import/update 交易字段长度与金额上限（order_id/fund_name/trade_type/time、amount/fee/share ≤1e9）。

- **#221** 修复：CheckAlerts `maxDrawdownPct` 改为最近 2000 点 NAV 窗口（原先 ASC LIMIT 120 取最早历史，漏报现代回撤）。

- **#220** 稳健：基金详情交易列表 LIMIT 5000；DCA 幂等预检按 `(order_id, fund_code)`。
- **#219** 稳健：compare/timeline NAV 与 XIRR cashflow 查询加 LIMIT，防止超长历史拖垮内存/CPU。
- **#218** 稳健：drawdown NAV 最近 5000 点；证券/DCA 字段长度与金额边界校验。
- **#217** 稳健：SearchFunds/Stocks 查询长度上限；backtest base 上限、start_date 规范化、NAV 点 LIMIT 5000。
- **#216** 安全/稳健：source event 字段长度限制；backtest 无数据中性消息；MCP import 预检 5000；DCA active 0/1。
- **#215** 安全/i18n：MCP portfolio_id/offset/amount 等参数钳制；LanguageSwitcher 按钮面文案 i18n。
- **#214** 安全/稳健：import 最多 5000 行；portfolio_id 上限；MCP limit 类参数 intArgMax；DCA/指数/美股 soft message 脱敏。
- **#213** 安全：agent tools authorize `role` 白名单；DCA `base` 上限与 `mode` 白名单。
- **#212** 修复：AdjustPosition 检查 balancing INSERT `RowsAffected`；Finalize 审计失败时仍返回已消费确认（避免重复 finalize）。
- **#211** 修复：DCA 成交 `INSERT` 与 `dca_plan_executions` 同一事务；失败回滚，避免账本与交易分叉。
- **#210** 安全/文档：MCP `invalid_params`/`encode_tool_result` 脱敏；MASTER Remaining Gaps 清理已关 P1、标注多组合 PK 设计债。
- **#209** 修复：MCP 写确认两阶段（先 Verify 后成功再 Finalize/MarkUsed）；工具失败不烧 token；份额表单必填文案。
- **#208** 安全/i18n：admin/MCP body 限制；MCP HTTP 错误脱敏；DCA explanation 中性模板；NAV limit intQueryMax。
- **#207** 安全：agent confirmation body 限制；MCP `tool_error` 不再回传原始 err。
- **#206** 安全：source-events body 限制；admin crawl/recalc 错误脱敏；MCP compare codes≤8。
- **#205** 修复：compare `codes` 上限 8；agent confirmation 错误脱敏；admin/export `invalid_json`；MASTER #199/#203 收口。
- **#204** 安全：admin/SPA 写路径 5xx 走 writeSafeError；portfolio 查询 limit 上限；SPA import/put MaxBytesReader。
- **#203** 修复：`transactions` 唯一约束为 `(order_id, fund_code)`（转换双腿共享 order_id）；import/DCA/adjust NOT EXISTS 对齐；生产 0 真重复。
- **#202** 安全/修复：公开 portfolio/funds/market 走 writeSafeError（客户端 stable code，服务端带 request_id 记全错）；TransactionForm 强制 shares；PG `order_id` UNIQUE 索引 best-effort。
- **#201** 修复：DCA signal 稳定码 + SPA i18n；buy/sell 强制 confirm_share>0；NAV limit 上限 2000；export 2MiB/5000 行；scheduler claim UPDATE fail-closed。
- **#198** 修复：AdjustPosition 写入合成平衡交易（`持仓调整`）后 recalc；份额以 transactions SUM 为 SSOT，价格刷新不再覆盖。
- **#199** 修复：DCA 成交 INSERT 用 `WHERE NOT EXISTS(order_id)`；`RowsAffected=0` 记 skipped_duplicate，避免并发双写。
- **#200** 改进：ConfirmDialog focus trap/Escape/return-focus；PortfolioAllocation max-position 用 resolveAllocationLabel。
- **#197** 修复：SPA mutation 错误短 HTTP 脱敏；交易空态 noTx；risk_flags critical；holdings 原子重写；DESIGN/DR AgentOps SSOT。
- **#196** 修复：pg `rebindConn` 实现 `SessionResetter`；scheduler WAL 仅 SQLite probe 后执行；SECURITY-AUDIT AgentOps 已启用 SSOT。
- **#195** 修复：SPA `useChartData`/`useNasdaqData` 走 sanitizeUserError；FundDetail noCode i18n；DCA 轴万/k；冒烟 PUBLIC 密钥路径+prepare 401；Hermes/MATRIX AgentOps 已启用 SSOT。
- **#194** 修复：AgentOps `MarkUsed` 原子 `used_at IS NULL`（PG TOCTOU）；import 用 NOT EXISTS 幂等；scheduler claim 未知错误 fail-closed；生产已部署。
- **#5** 新功能：AgentOps 生产启用（`FUND_AGENT_OPS_ENABLED` + confirmation secret）；smoke-prod 增加 prepare→无确认拒绝→单次消费→复用拒绝；PUBLIC prepare 401；operator tools/list 44 / PUBLIC 26 不变。
- **#193** 改进：Overview `first_trade`/`last_trade` 走 `formatNavDate` locale；FundDetail 图例点尺寸 token 化。
- **#192** 改进：DcaPanel / FundDetail 残余 spacing（marginTop 微间距 + toast bottom）收口 space token。
- **#191** 修复：penetration 空行业键 EN-primary `other`；Overview/App NAV 日期 `formatNavDate` 按 locale。
- **#190** 改进：删除交易 in-app ConfirmDialog（替代 window.confirm）；移动底栏 hitTarget.mobile；导出菜单 i18n；DCA 模式切换自动重算。
- **#189** 改进：残余 marginBottom/marginTop 20/16/12 → space token（ChartShell/Overview/Detail/Compare 等）。
- **#188** 修复：classify 补 ETF/EN 纳斯达克关键词 + isNasdaqFundName；SPA `sanitizeUserError` 脱敏用户错误；Nasdaq 页复用共享匹配。
- **#187** 修复：中文 Harness 文案「智能分析」；隐藏永久 disabled 的 CSV 导入按钮；弱化 import 不可用措辞。
- **#186** 修复：UIUX residual batch — chartHeight.distribution、Nasdaq benchmarkDesc i18n、usdCny/MA20 i18n、删除图标 critical 色。
- **#185** 文档：residual trade_type toggle DB-code SSOT after #184（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#184** 修复：FundDetail 切换买入类型只用 DB 中文 trade_type 常量（`tradeTypes` 服务）；EN locale 不再 no-op/写坏 DB。
- **#183** 文档：residual allocation risk_flags/agent_brief EN SSOT after #182（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#182** 修复：allocation `risk_flags` / `agent_brief` EN-primary（SPA chips + MCP/harness 事实串）。
- **#181** 文档：residual allocation label i18n SSOT after #180（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#180** 修复：allocation type/market bucket API 标签 EN-primary；SPA `AllocationRows` 按 key 走 i18n；扩展 `allocation.typeLabels/marketLabels`。
- **#179** 文档：residual DeleteSecurity stock_* cascade SSOT after #178（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#178** 修复：`DeleteSecurity` 级联清理 `stock_realtime`/`stock_kline_cache`/`stock_profile`；`fund_details` RowsAffected 错误上抛。
- **#177** 文档：residual DCA post-write + tableColumns SSOT after #176（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#176** 修复：DCA 成交后 snapshot recalc / execution ledger 错误写入 item.Message；`tableColumns` PG information_schema 查询错误上抛。
- **#175** 文档：residual integrity fkRows + PRAGMA quote SSOT after #174（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#174** 修复：`getPGIntegrity` 补 `fkRows.Err()`；`tableColumns` PRAGMA 表名 quote 防御。
- **#173** 文档：residual CheckAlerts maxDrawdownPct SSOT after #172（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#172** 修复：`maxDrawdownPct` 对 nav_history Query/Scan 真错误上抛（不再静默 0 回撤）。
- **#171** 文档：residual CheckAlerts dca_plans Scan SSOT after #170（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#170** 修复：`CheckAlerts` 对 `dca_plans` Query/Scan 真错误上抛（缺表仍软兼容；不再静默丢掉 dca_day）。
- **#169** 文档：residual AdjustPosition/CheckAlerts Scan SSOT after #168（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#168** 修复：`AdjustPosition` / `CheckAlerts` / `recalcSnapshotLight` 对 fund_details 与 NAV 的 QueryRow.Scan 真错误不再吞掉（ErrNoRows 仍软可选）。
- **#167** 文档：residual DCA Scan fix SSOT after #166（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#166** 修复：`RunDCAAutoInvest` 对 order_id / dca_plan_executions / NAV 的 QueryRow.Scan 错误不再吞掉（缺表仍软兼容；真错误失败）。
- **#165** 文档：residual Yahoo index EN fallback SSOT after #164（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#164** 修复：Yahoo `defaultIndexNames` 空 shortName 回退改为英文 Yahoo 风格标签（SPA 仍走 `market.index.*` i18n）。
- **#163** 文档：residual chart height tokens SSOT after #162（MASTER/HANDOFF/WAVE4/DESIGN）；开放集仍仅 #5。
- **#162** 改进：theme `chartHeight` 阶梯 + CSS `--fd-chart-h-*`；ChartShell 默认与 Fund/Portfolio/Nasdaq/Allocation/Penetration/MC/DCA/detail 图表高度收口 token。
- **#161** 文档：residual type=button SSOT after #160（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#160** 修复：残余 Kumo `<Button>` 补 `type="button"`（26 处；#126 之后的 residual）。
- **#159** 文档：residual t() fallback strip SSOT after #158（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#158** 改进：剥离已存在 key 的 `t(key, '中文')` / App `defaultValue` fallback（FundChart/ChartShell/FundComparison/PnL/App）；MarketTicker 指数 API 名 fallback 保留。
- **#157** 文档：residual NasdaqOverview i18n SSOT after #156（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#156** 修复：NasdaqOverview 图表 series/tooltip 硬编码中文（合计/日/笔）→ `nasdaq.totalLine|seriesBuy|seriesSell|tooltipBuy|tooltipSell`。
- **#155** 文档：residual XLSX locale export SSOT after #154（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#154** 修复：`POST /api/export/transactions-xlsx` 按 Accept-Language 输出中/英表头与方向标签；SPA 携带 i18n.language。
- **#153** 文档：residual missing SPA i18n catalog SSOT after #152（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#152** 修复：补齐 SPA 缺失 i18n 目录键（`nasdaq.*` 全段 + `fund.cost/buy/sellLabel` + `common.loading/loadError/noData/units/fund` + `nav.skipToContent`/`mobile.nav`），消除 EN 回退中文。
- **#151** 文档：residual penetration subtitle i18n SSOT after #150（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#150** 修复：`penetration.subtitle` 恢复 `{{equityCount}}`/`{{uniqueStocks}}` 插值（#142 回归）；PortfolioPenetration + vitest 对齐。
- **#149** 文档：residual layout chrome + skeleton SSOT after #147/#148（MASTER/HANDOFF/WAVE4/DESIGN）；开放集仍仅 #5。
- **#148** 改进：ChartFallback/PageFallback skeleton 条宽高走 `skeleton.barH/barW` + `statMinH`/`chartH`；theme + CSS `--fd-skeleton-*` 镜像。
- **#147** 改进：theme `layout.mobileNavHeight` + CSS `--fd-layout-mobile-nav-height`；App 移动底栏 `height` 改 token。
- **#146** 文档：residual LanguageSwitcher + opacity SSOT after #144/#145（MASTER/HANDOFF/WAVE4/DESIGN）；keyword/data 字典中文匹配与图表单次 opacity 为 intentional residual。
- **#145** 改进：Sidebar 装饰 logo `alt=""` + `aria-hidden`；theme `opacity` 微 token + CSS `--fd-opacity-*`；迁移 MarketTicker/Harness/TransactionTable/FundChart/Nasdaq 等 chrome/series opacity 字面量。
- **#144** 改进：残余 LanguageSwitcher 硬编码标签 → i18n keys（`nav.switchToEn` / `nav.switchToZh`；目标语标签在 zh+en 目录同文）。
- **#143** 文档：residual classify + sector display i18n SSOT after #141/#142（MASTER/HANDOFF/WAVE4/DESIGN）；keyword/data 字典中文匹配为 intentional residual。
- **#142** 改进：残余 `SECTOR_NAMES` 展示标签 → i18n keys（`penetration.sectors.*`）；`classifySector` 中文关键词/regex 匹配保留。
- **#141** 改进：残余 classify 展示标签 → i18n keys（`CATS`/`STOCK_CATS`/`STOCK_MARKETS` 用 `nameKey`/`labelKey`；Sidebar 走 `t()`）；`CATS.funds` 等中文 keyword 字典保留作基金名匹配。
- **#140** 文档：residual TransactionForm + Allocation i18n SSOT after #138/#139（MASTER/HANDOFF/WAVE4/DESIGN）。
- **#139** 改进：残余 PortfolioAllocation 类型/市场标签 + sunburst tooltip、tradeMarkers 默认买卖标签 → i18n keys（`allocation.*` + `fundDetail.dir.*`）。
- **#138** 改进：残余 TransactionForm 硬编码中文 → i18n keys（`fundDetail.txForm.*` + 方向标签；DB `trade_type` 常量保留中文码）。
- **#137** 文档：residual TransactionTable + CSV export i18n SSOT after #135/#136（MASTER/HANDOFF/WAVE4/DESIGN）。
- **#136** 改进：CSV 导出表头/方向标签走 i18n（`fundDetail.csv.*` + `fundDetail.dir.*`，随当前 locale）。
- **#135** 改进：残余 TransactionTable 硬编码中文 → i18n keys（搜索/列头/空态/方向标签 + zh/en）。
- **#134** 文档：residual i18n Chinese + borderRadius SSOT after #132/#133（MASTER/HANDOFF/WAVE4/DESIGN）。
- **#133** 改进：theme `radius.none` / `radius.xs` + CSS `--fd-radius-none/xs`；迁移 App/MarketTicker/Sidebar/Penetration 残留 borderRadius 字面量。
- **#132** 改进：残余 MarketTicker/FundDetail 硬编码中文文案 → i18n keys（分组/指数名/交易统计 + zh/en）。
- **#131** 文档：residual motion + i18n title SSOT after #129/#130（MASTER/HANDOFF/WAVE4/DESIGN）。
- **#130** 改进：残余硬编码中文 `title` 改 i18n keys（FundDetailView/MarketTicker/TransactionTable + zh/en）。
- **#129** 改进：theme `duration` / `easing` + `cssTransition` 微 token；迁移 FundDetail/MarketTicker/Card 残留 transition 字面量。
- **#128** 文档：residual a11y button + lineHeight SSOT after #126/#127（MASTER/HANDOFF/WAVE4/DESIGN）。
- **#127** 改进：theme `lineHeight` / `letterSpacing` 微 token + CSS 镜像；迁移 LanguageSwitcher/OfflineBanner/StatCard/Sidebar 字面量。
- **#126** 修复：残余 button 补 `type="button"`（InvestmentHarnessPanel / MarketTicker）。
- **#125** 文档：residual fontWeight/zIndex SSOT after #123/#124（MASTER/HANDOFF/WAVE4/DESIGN）。
- **#124** 改进：Card 内容层叠改 `zIndex.local`；theme + `--fd-z-local` 镜像。
- **#123** 改进：残余 fontWeight 字面量迁移（DcaPanel/OfflineBanner/PortfolioSwitcher/LanguageSwitcher/Nasdaq/TransactionTable 等）。
- **#119** 改进：theme `zIndex` 海拔阶梯 + fixed 层迁移。
- **#120** 改进：theme `fontWeight` 阶梯 + 高频字重字面量迁移。
- **#121** 改进：AdminDashboard 分区 aria-label + 异常表 caption/aria-label。
- **#122** 文档：DESIGN.md token 准确性对齐 #103–#118（+ #119 zIndex；glassSurfaceStyle / ChartShell / space·radius·fontSize / skeleton / critical / reduced-motion spin；G6 仍产品门控）。
- **#118** 改进：theme `fontSize` 字号阶梯 + SPA 内联 fontSize 字面量迁移（raw ≈ 0）。
- **#117** 修复：MarketTicker 刷新旋转动画改 `fd-spin`，并尊重 `prefers-reduced-motion`。
- **#116** 改进：ChartFallback/PageFallback skeleton 加载壳（减 CLS；respect reduced-motion）。
- **#115** 改进：残余间距最终清扫（App/DCA/Nasdaq/misc + page headers）；有意义 magic spacing ≈ 0。
- **#114** 改进：FundDetail/Sidebar/Compare/Penetration 残留间距 token 化。
- **#113** 改进：InvestmentHarnessPanel / MarketTicker / PortfolioAllocation 残留间距 token 化。
- **#112** 改进：抽取共享 `ChartFallback`；页面 loading/`padding:60` 收口到 `space[]`。
- **#111** 改进：Admin 异常表 Kumo Table；OfflineBanner 用 critical + safe-area；echarts shadow 走 chartShadowColor。
- **#110** 改进：Overview 子页签 tablist/tab；DCA `aria-pressed`；交易表 `aria-sort`。
- **#109** 修复：FundDetail 累计图改 useEChart；导出菜单 a11y（menu/Escape）+ glass tokens。
- **#108** 改进：CSV 导入在后端未实现时禁用（不再骗用户选文件）；`ExchangeRateBadge` 四页去重。
- **#107** 修复：错误/校验色改用 `theme.critical`，不再误用涨/盈利 `theme.up`。
- **#106** 改进：ChartShell/Overview/Admin/App 高影响间距收口到 `space[]`/`radius`。
- **#105** 改进：MonteCarlo/Allocation/Penetration/Compare 雷达收口 ChartShell 三态。
- **#104** 修复：统一 `glassSurfaceStyle` 磨砂路径；修 `data-mode`/`data-theme` 暗色 CSS 变量失效；补 `--fd-glass-shadow`。
- **#103** 改进：Overview 残留实心卡片统一 frosted glass（Allocation/Penetration/MonteCarlo + StatCardError）。
- **#102** 修复：US stock `previous_close` 误用 long-range `chartPreviousClose`（AAPL change_pct ~58%）。优先 `meta.previousClose`，其次历史倒数第二收盘，最后才回退 chartPreviousClose。
- **#101** 修复：MarketTicker A股分组仍用 `sh000001` 匹配不到 Yahoo `000001.SS`。增加 indexCodeMatch 别名，短名覆盖 Yahoo/Eastmoney 双代码。
- **#100** 修复：MarketTicker CN/HK 指数从未填充（仅 US Yahoo）。扩展 DefaultIndexSymbols 到 8（新增 ^HSI、000001.SS、399001.SZ、399006.SZ），SPA 代码映射（sh000001→000001.SS 等），市场区域自动检测（US/CN/HK），Deploy/Dockerfile 修复 docker chmod 权限问题。

### Security
- **#65** Role-aware harness/agent-context discovery: public HTTP remains least-privilege (26 tools); MCP operator restores full harness write/maintenance surface; public `agent-context` no longer leaks write tool names.
- **#66** `scripts/smoke-prod.sh` asserts public harness/agent-context discovery filters and operator MCP harness full surface.
- **#67** `docs/ARCHITECTURE.md` accuracy: fail-closed MCP auth, PUBLIC 26, role-aware harness, adjust_position MCP vs admin HTTP.
- **#68** residual ARCHITECTURE topology: replace legacy topology diagrams; banner ARCHITECTURE_V3 historical.
- **#69** router EdgeAuth comments + HANDOFF/SECURITY checklist residual accuracy after public discovery wave.
- **#70** Hermes MCP catalog: generate_report JSON-only, check_alerts no webhook, write tools marked #5 confirmation-gated.
- **#71** residual SSOT polish: SIBLING/README PUBLIC 26, smoke PUBLIC tools/list count==26, document known money-fund anomaly.
- **#72** `.env.example` pure-Go accuracy: dual-driver PG, EdgeKey, AgentOps off-by-default, no bare confirmed=true / Bun Feishu claims.
- **#73** ops dashboard crawler freshness uses fund T+1 window (`stalePriceDays`) instead of false 24h zero; SPA label updated.
- **#74** MCP `get_data_freshness` only sets `recommended_maintenance_requires_run` when held NAV is stale/missing (no forced crawl_nav when healthy).
- **#75** smoke asserts freshness recommendation matches stale/missing; SPA `types.ts` drops packages/server claim.
- **#76** `deploy/docker-compose.yml` bannered as legacy SQLite path.
- **#77** `deploy/deploy.sh` / `rollback.sh` / `build.sh` bannered as legacy GHCR/SQLite path.
- **#78** smoke marks `smoke-probe` source-events read (no unread pollution); RELEASE-GOVERNANCE production host updated.
- **#79** DISASTER-RECOVERY rewritten for Azure PG; `docs/progress/MASTER.md` bannered as historical Bun single-container plan.
- **#80** smoke reuses existing `smoke-probe` source-event (PATCH) to stop unbounded row growth; unread remains 0.
- **#81** AgentOps checklist paths → `/prepare` + `/consume`; MASTER Remaining Gaps drops stale unimplemented-tools row.
- **#82** PENETRATION-DESIGN marked implemented; deploy/.env.example GHCR path, no Feishu side-effect claim.
- **#83** Hermes skills (feishu-daily-digest / portfolio-review) default to read-only MCP paths while AgentOps #5 is off.
- **#84** smoke default skips live crawl-nav/holdings; keeps recalculate-snapshot + 401 auth checks; `SMOKE_FULL_CRAWL=1` optional.
- **#85** HANDOFF §16 next steps only open/gated items; drop residual MCP34 completed-mix language.
- **#86** SECURITY residual: document public SPA mutations via nginx EdgeKey injection as accepted personal-site boundary (no edge change).
- **#87** crawl_nav/price refresh `added` counts only real inserts/value changes (no-op upsert → 0).
- **#88** crawl_fund_holdings skips rewrite when report slice unchanged (`added=0` on no-op).
- **#89** WAVE4 Wave6/MASTER closed-note SSOT polish (MCP44, residuals through #88).
- **#90** portfolio_snapshot recalc zeros dust `|held_shares| < 0.001` (sold-out float residue).
- **#91** MCP tools/list real JSON Schema (no typescript-zod stubs); accept `code`/`fund_code` aliases (run_backtest etc.); closed-position PnL zeroed with dust clamp.
- **#92** market indices: Yahoo refresh-on-read when cache >6h stale; weekday 20:00 scheduler refresh; SPA MarketTicker gets fresh quotes.
- **#93** `get_us_stock` aligns SQL to production PG stock_* columns (no open/high/low required); MCP `get_fund_detail` fills `xirr_pct`; banner `docs/AUDIT.md` historical.
- **#94** USD/CNY exchange-rate: in-process 1h cache + User-Agent; serve last-good on Yahoo 429 instead of SPA 502.
- **#95** restore `GET /api/market/index/{code}` + `/history` (NDX→^NDX, Yahoo chart, 30m cache) for NasdaqOverview.
- **#96** `GET /api/market/stream` SSE (`event: indices`) with 60s refresh + heartbeat for MarketTicker.
- **#97** restore `POST /api/export/transactions-xlsx` (excelize) for FundDetail Excel export menu.
- **#98** `/api/stocks/{code}` flat SPA `USStockInfo` contract + Yahoo refresh-on-read when cache empty.
- **#99** US stock re-fetches Yahoo when cached quote lacks OHLC or history empty; best-effort kline upsert.


- [安全] 公网 harness `available_agent_tools` 只读发现面，隐藏 write/maintenance/confirmation 工具（#64）
- [修复] AgentOps 关闭时 MCP 假 confirmation 不再 nil 解引用 panic（typed-nil interface）（#63）
- [文档] HANDOFF/WAVE4 对齐 PUBLIC tools/list 26（#62）
- [安全] PUBLIC tools/list 隐藏 confirmation-gated 工具（check_alerts/generate_report）（#61）
- [安全] MCP tools/list 按角色过滤：PUBLIC/analyst 不广告 write/maintenance 工具（#60）
- [文档] HANDOFF/SECURITY 对齐 #56–#58 scheduler durable 与 source-events EdgeAuth（#59）
- [修复] scheduler durable claim 使用 per-job `fund_code`（适配 crawl_log PK=fund_code）（#58）
- [安全] portfolio source-events 写路径 EdgeAuth + PG `RETURNING id`（#57）
- [修复] scheduler 启动 catch-up 与窗口 claim 做 CST once/day 持久化（crawl_log），避免同日 redeploy 全量打东财（#56）
- [安全] `/api/agent/tools*` 鉴权：registry/authorize 需 operator Bearer（#55）
- [修复] scheduler 同窗口 once-per-day 门闩，避免 5 分钟 ticker 在整点小时内重复刷新/DCA（#54）
- [改进] scheduler 工作日 20:00 价格刷新后执行 DCA materialization；周六 10:00 holdings crawl（#53）
- [修复] delete cascade `ant_transactions_raw` 列名对齐 `fund_code_cell`（#51 follow-up）
- [修复] `delete_fund` 级联补齐 dca_plan_executions/summary/crawl_log/ant_*/source_events（#51）
- [改进] `run_dca_auto_invest` 写入 `dca_plan_executions` 账本并做双重幂等（#52）
- [文档] README + Hermes 拓扑对齐 tools/list 44 / Azure PG（#50）
- [修复] portfolio_snapshot 无 market 列：adjust_position/check_alerts/dca_run 对齐生产 PG schema（#49）
- [文档] ARCHITECTURE.md MCP 计数对齐 tools/list 44（#48）
- [文档] AGENTOPS/SECURITY + harness 对齐 tools/list 44 全量（#47）
- [新功能] MCP `generate_report`（#46；tools 43→44 全量 registry 对齐；JSON facts-only，无 PDF；confirmation-gated）
- [新功能] MCP `run_dca_auto_invest`（#45；tools 42→43；dry_run 默认 true；幂等 order_id；confirmation-gated）
- [新功能] MCP `adjust_position` / `check_alerts`（#43/#44；tools 40→42；confirmation-gated，alerts 仅 facts-only 无 webhook）
- [新功能] MCP `add_fund` / `add_security` / `update_fund` / `delete_fund`（#42；tools 36→40；confirmation-gated，无 AgentOps 时 fail-closed）
- [新功能] MCP `upsert_dca_plan` / `disable_dca_plan`（#41；tools 34→36；无 AgentOps 时 fail-closed）
- [文档] deploy/HANDOFF 运维手册：admin crawl-nav/holdings/recalculate-snapshot（#40）
- [文档] SECURITY-AUDIT 运维清单覆盖 admin crawl/recalc（#39）
- [文档] ARCHITECTURE.md 标注历史文档 + SIBLING multi-arch 已落地（#38）
- [文档] ARCHITECTURE.md 管理端点对齐 Go crawl-nav/holdings/recalc（#37）
- [改进] Admin `POST /api/admin/crawl-nav?code=` 单证券刷新 + HANDOFF Go1.25 + smoke（#36）
- [新功能] Admin `POST /api/admin/recalculate-snapshot` + smoke 覆盖 crawl-holdings/recalc（#35）
- [新功能] Admin `POST /api/admin/crawl-holdings` 与 MCP crawl_fund_holdings 对齐（#34）
- [文档] HANDOFF/SECURITY/MATRIX 对齐 tools/list 34 与剩余 10 工具（#33）
- [新功能] MCP `crawl_fund_holdings` 接入 tools/list+call（#32；tools 33→34；Eastmoney jjcc）
- [新功能] MCP `recalculate_snapshot` 接入 tools/list+call（#31；tools 32→33）
- [新功能] MCP `crawl_nav` 接入 tools/list+call（#30；tools 31→32）
- [新功能] MCP `mark_source_event` 接入 tools/list+call（#29；tools 30→31）
- [文档] MCP registry 44 vs tools/list 矩阵（`docs/progress/MCP-TOOL-MATRIX.md`，#28）
- [改进] 统一 skip-link：`index.html` 使用 `fd-skip-link` + token CSS（#27）
- [安全] 工作树脱敏：nginx EdgeKey include 本机 snippet、Hermes/HANDOFF 去明文密钥（#26；未轮换）
- [改进] 边缘 `limit_req` 挂载 `/mcp` `/api/admin` `/api/agent` `/api/`；README Go1.25 与公网读边界修正（#25）
- [改进] SPA 边缘 CSP + Permissions-Policy（#24）
- [修复] nginx `location /` 重申 HSTS/XFO/nosniff/Referrer-Policy（#23；避免 add_header 继承坑）
- [chore] CI/Release setup-go 对齐 go.mod `1.25.x`（#22）；radar 回落色走 `lightTheme.blue`
- [chore] GHCR `build-and-push` multi-arch（linux/amd64 + linux/arm64）+ GHA cache（#21）
- [测试] CI 可选 `e2e-full` job（workflow_dispatch `full_e2e=true`）跑全量 Playwright（#20）
- [改进] smoke-prod 自动加载 Hermes operator key；PUBLIC 写拒绝可验
- [改进] onAccent/markerBorder tokens 替换 OfflineBanner/Sidebar/Nasdaq/Penetration 残留 `#fff`（#19）
- [改进] sector 扩展色迁入 theme `sector*` tokens（#18）
- [改进] PortfolioSwitcher glass tokens + a11y listbox（配合 `?portfolio=` deep-link）
- [新功能] 组合 `?portfolio=` deep-link（`usePortfolioDeepLink`）+ `scripts/smoke-prod.sh` 生产冒烟
- [新功能] 基金对比 `?codes=` deep-link 自动对比；critical CSS tokens
- [改进] 行业色板 SECTOR_COLORS 绑定 theme accents + SECTOR_FALLBACK
- [改进] 基金详情 `?detailTab=` deep-link；Ticker/Sidebar/OfflineBanner 去硬编码红绿
- [改进] Overview `?tab=` deep-link；Harness/Nasdaq/FundDetail 剩余 LayerCard → glass Card
- [改进] 交易 CRUD：Form/Table glass 化、删除 toast、aria-live、tabular nums
- [改进] 结构化 AccessLog（request_id/status/duration，跳过 assets，不记密钥）
- [改进] Admin/DCA/classify 去硬编码色；DcaPanel 改 glass Card
- [改进] StatCard/ChartShell 磨砂玻璃表面 + theme glass tokens；StatCard 去硬编码红绿
- [新功能] 图表 range deep-link：`useQueryRange`（FundChart/NasdaqOverview）
- [文档] 新增 `docs/DESIGN.md`（Vercel Web Interface Guidelines + Geist-inspired tokens）
- [文档] Wave4 SDD：Issues #4–#7、`WAVE4-SDD` / `SIBLING-AUDIT` / `AGENTOPS-ENABLEMENT`
- [改进] UI：`space`/`radius` token、skip-link、`:focus-visible`、`prefers-reduced-motion`
- [改进] `usChangeColor` 走 theme token，去掉硬编码 hex
- [修复] PG 兼容：`INSERT OR REPLACE/IGNORE`、`sqlite_master`、XIRR 时间、timeline DATE cast
- [文档] HANDOFF/MASTER 对齐生产拓扑
