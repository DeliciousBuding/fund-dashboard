# 05 — 实施路线图

> 定档日期：2026-08-29 · 状态：已定档
> 波次按依赖排序，每波独立可验收、可交付。**版本号 bump / tag / release 必须管理员批准**（全局纪律）。

## 波次总览

| 波次 | 主题 | 交付物 | 验收 |
|------|------|--------|------|
| W0 | 仓库地基 | pnpm workspace、web/ 脚手架、CI web 链、三阶段 Dockerfile、死代码清理 | CI 全绿；`docker build` 产出含占位 SPA 的镜像 |
| W1 | 鉴权与安全 | `internal/auth`、session 全套端点、CSRF/限流/recover/CSP、会话清扫 | 见 §W1 验收矩阵 |
| W1.6 ✅ | 公网暴露加固 | 递增锁定、密码策略、auth_events 审计、全 API 限流、可信代理、HSTS、工作台系统 API | 见 06 文档 §4 验收矩阵 |
| W2 ✅ | 设计系统 | tokens.css、组件库、图表主题、`/_design` 目录页、布局壳 | `/_design` 全组件四态目视通过 |
| W3 ✅ | 核心读面 | 总览 + 持仓列表 + 标的详情（走势 tab） | 数据正确性 vs 旧 API 抽查一致 |
| W4 ✅ | 交易与定投 | 台账 CRUD、导入导出、DCA 计划管理 | 旧 e2e 基线改写通过 |
| W5 ✅ | 分析套件 | compare / backtest / advanced（MC+相关性）/ penetration / market | 纯函数移植测试全绿 |
| W6 ✅ | 洞察·设置·工作台 | insights / reports / settings 四 tab / 系统工作台 / 审计时间线 | 改密码/会话管理/任务触发全流程手工验收 |
| W7 ✅ | 打磨与 MCP | 动效/a11y/性能/PWA、MCP spec 修复、文档收口 | Lighthouse 关键指标 + 44 工具回归 |

## W0 — 仓库地基（✅ 已完成 2026-08-29）

- pnpm workspace（`web` + `packages/contracts`）；删 `package-lock.json`；根 scripts 改 pnpm。
- web 脚手架：Vite 7 + React 19 + TS strict + Tailwind v4 + Biome + vitest 骨架；vite.config 设 outDir `../internal/webui/dist`、dev proxy `/api`+`/mcp` → :8765。
- `internal/webui/embed.go` + `dist/index.html` 占位；Dockerfile 加 node 阶段。
- 清理（2026-08-29 已全部落地：死脚本 ×2、npm lock、全仓 9 处「独立仓库」表述、Streamable HTTP 表述、sqlitecompat 引用、Go 版本、内嵌句指针、docs/README 指针化、MASTER.md 主机代号擦除、`deploy/.env.example` 补建）。
- CI：`test-web` job（biome + tsc + vitest + build）；smoke-e2e 追加 `GET /` HTML 断言。
- 验收：`go build`（无 web 产物）与 `docker build`（含产物）双路径通过；CI 全绿。

## W1 — 鉴权与安全（✅ 已完成 2026-08-29；设计见 04 文档）

- `internal/auth`：argon2id、session 签发/校验/滑动/吊销、登录限流器；两张新表（SQLite Go 侧建表 + PG DDL 入 `schema_pg.go`）。
- 7 个 `/api/auth/*` 端点 + SessionAuth 中间件 + 读路径收门 + 写路径 session 优先/EdgeKey 兼容。
- recover 中间件（全局链首位）；Origin 白名单 env 化（`FUND_ALLOWED_ORIGINS`）；CSP 进 SecurityHeaders。
- 03:00 调度追加三清（过期 session / 过期 confirmation / 90 天外 audit）。
- **验收矩阵**（全部通过）：未登录访问 `/api/portfolio` → 401 ✅；未登录 `GET /` 得到 SPA 壳（实施精炼为「静态全公开 + 数据全收门」，登录页永不白屏）✅；错密码 5 次 → 429 锁定 ✅；session 无 `X-Fund-Request` 写 → 403 ✅；EdgeKey 兼容可用 ✅；MCP 44 工具双 key 回归不变 ✅；`go test -race` 全绿 ✅；种子库端到端（setup→登录→读→写→MCP）人工验证通过 ✅。

## W1.6 — 公网暴露加固（✅ 已完成 2026-08-30；设计见 06 文档）

部署形态升级为**公网直接暴露**后的安全加固包：递增锁定 + 密码策略收紧 + auth_events
登录审计 + 全 API per-IP 限流 + MCP per-key 限流 + 可信代理解析 + HSTS/COOP +
session 化 `/api/system/*` 工作台 API（状态/任务/审计/爬虫触发）。
验收矩阵见 `06-security-hardening.md` §4。后端实现委托 sonnet subagent，前端页面随 W6 落地。

## W2 — 设计系统（✅ 已完成 2026-08-29）

- `tokens.css`（暗/亮/密度三轴）+ Tailwind `@theme` 映射；Inter 自托管；字阶与数字规范落地。
- shadcn/ui 按需引入并定制：button/input/dialog/sheet/drawer/tabs/table/tooltip/dropdown/skeleton/toast/badge/card。
- 图表主题 `charts/theme.ts`+`palette.ts`；`useEChart` 生命周期层（DPR cap/ResizeObserver/主题热切）重写。
- AppShell：侧栏（市场分组/搜索/heldOnly）+ 顶栏（ticker 槽位）+ 移动底栏；⌘K 骨架。
- `/_design` 目录页：全组件 × 全状态。
- 验收：`/_design` 目视过审；暗/亮切换无闪屏；axe 关键页零 critical。

## W3–W6 — 页面波次

按 01 文档 §4 页面规格执行；通用纪律：
- 每页四态（loading/empty/error/stale）齐备；数字规范（tabular/右对齐/涨跌色）；深链进 URL。
- 每波带 Playwright 场景；旧 e2e 七份 spec 作验收基线逐条改写。
- 纯函数移植：九件套连同原测试一起搬——`montecarlo`/`statistics`/`irr` → W5；`classify`/`sector` → W5（穿透/侧栏）；`tradeTypes`/`format`/`marketTime`/`userError` → W3/W4 随页面。

**后端 REST 补面**（SessionAuth + `X-Fund-Request`；以下能力现状均为 MCP-only，UI 用不了）：
- W4：`POST/PUT/DELETE /api/dca/plans`（upsert/disable）、`POST /api/dca/run`（dry_run 支持）、`POST /api/securities`（标的入册）、`POST /api/portfolio/adjust-position` —— ✅ 2026-08-29 已交付（`spa_extensions.go`，含台账 `GET /api/transactions` 探测适配 legacy 列）
- W6：`check_alerts`、`generate_report` 翻 REST —— ✅ 同批交付（`/api/alerts`、`/api/reports`、`/api/freshness`）；`/api/agent/tools*` 只读面接受 SessionAuth（供设置页 Agent 面）
- W1.6：`/api/system/*` 工作台面（06 文档 §2.6）
- W4 验收追加：CSV 导出表头/方向标签随 i18n、带 BOM（继承旧 transactionsToCsv 语义）
- W6 验收追加：insights 页含组合级 harness 支架卡（字段对齐 MCP `get_investment_harness_snapshot`）

## W7 — 打磨与 MCP 收口（✅ 已完成 2026-08-30）

- 性能：首屏预算 < 400KB gzip（不含图表懒加载）；图表按路由 lazy；虚拟滚动台账。
- PWA：manifest/SW/离线横幅/chunk 失败自愈（旧方案继承）。
- MCP spec 修复：`notifications/initialized` 按规范吞掉（202 无响应体）；文档「Streamable HTTP」表述与实际对齐；官方 Go SDK 迁移评估（backlog，不承诺）。
- 文档收口：ARCHITECTURE 升 v4（monorepo + session）；docs/design 标注「已实施」。

## 技术债登记（测绘发现，排入对应波次）

| 债 | 处置 | 波次 |
|----|------|------|
| 无 recover 中间件 | 补 | W1 |
| 无应用层限流（auth 面） | 登录限流器 | W1 |
| Origin 白名单硬编码生产域名于公开仓 | env 化 | W1 |
| audit / confirmations 表只增不减 | 每日清扫 | W1 |
| `qa_report` 死表零引用 | 保留 DDL 不清理（生产库在仓外），文档标注 | W0 |
| `status.go` tableExists 在 PG 缺表场景假错误（`internal/service/admin/status.go:198`） | 修为 err==ErrNoRows 双路模式 | W1 顺手 |
| SQLite 业务 schema 无版本化迁移、启动零校验 | 接受现状（生产 dump 外部管理）；启动加必需表探测 WARN 日志 | W1 |
| 蒙特卡洛/相关性仅前端实现 | 先客户端移植；后端/MCP 化评估 | W5 / backlog |
| MCP 非真 Streamable HTTP、notifications 报错 | spec 兼容修复 | W7 |
| 报告无 PDF | 前端打印样式先行；后端 PDF 评估 | W6 / backlog |
| 快照重算三处近重复实现 | 抽取共享实现（顺手做，不阻塞） | W4 |
| 全局环境变量散落 `os.Getenv` | 收编进 config.Parse（顺手做） | W1 |
| CHANGELOG 无正式版本段 / 仓库 0 tags | 首个版本发版时按 release.sh 约定切段（**需管理员批准**） | W7 文档收口 |

## 风险与回滚

- **W1 是最大的行为变更**（读路径收门）：部署窗口选在无 agent 任务时段；`FUND_AUTH_PASSWORD_HASH` 预置可跳过 setup 流直接可用；回滚 = 镜像回退（旧镜像无 session 代码，行为如旧）。
- go:embed 占位机制保证「没构建前端也能编译后端」，CI 双路径防回归。
- 每波独立 PR；细碎的 WIP commit 在合并前 squash（全局 Git 纪律）。
