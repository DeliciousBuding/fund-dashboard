# Fund Dashboard — Architecture

> 基金 + 股票投资数据可视化与分析平台  
> 最后更新：2026-08-30  
> 版本：v4 — monorepo（`web/` 内嵌 SPA）+ session 鉴权 + 公网暴露加固
> 设计与波次定档见 [`docs/design/`](design/README.md)。

## 1. 总览

单容器部署：Go 静态二进制内嵌 React SPA（go:embed），REST + MCP + 静态面同端口。SQLite 默认，PostgreSQL 可选。公网暴露形态：TLS 由边缘终止，应用层按互联网面标准加固（06 文档）。

```
Browser ── session cookie（argon2id 登录）──┐
      │  HTTPS                              │
      ▼                                     │
edge proxy ── TLS 终止 + HSTS + 限流（边缘）  │
      │                                     │
      ▼                                     │
:8765  fund-dashboard（单二进制）             │
      ├── SPA 静态面（go:embed，公开）        │
      ├── REST /api/*（SessionAuth 收门）◄────┘
      ├── /api/system/* 工作台面（session + CSRF）
      └── MCP /mcp（Bearer key 双 scope，per-key 限流）
      │
      ▼
SQLite（FUND_DB_PATH）或 PostgreSQL（FUND_DB_DRIVER=pg）
```

| 层 | 技术 |
|----|------|
| 后端 | Go 1.26+ · `go-chi` · `cmd/fund-dashboard` + `internal/*` |
| 契约 | zod schemas 前后端共享 · `packages/contracts` |
| 存储 | SQLite（默认）；可选 PostgreSQL 双驱动（`FUND_DB_DRIVER=pg`） |
| MCP | JSON-RPC over HTTP（单端点 `POST /mcp`）；operator / analyst 双 scope |
| 前端 | `web/`：React 19 + Vite 7 + Tailwind v4 + Radix + ECharts + TanStack（go:embed 内嵌）——见 `docs/design/02/03` |

> 历史：`packages/web` 旧前端 2026-08-29 移出，现由 `web/` + go:embed 取代（dist 缺失时回退占位页，`FUND_STATIC_DIR` 为 dev 覆盖口）。

## 2. 后端分层

```
cmd/fund-dashboard       入口：装配 config → repository → services → httpapi/mcp → jobs
internal/
├── app                  应用装配、生命周期
├── config               配置解析与校验（fail-closed）
├── httpapi              REST 薄封装（路由、SessionAuth/Bearer 鉴权、限流、JSON 写回）
├── auth                 argon2id 密码、session 签发/滑动/吊销、登录递增锁定、auth_events 审计
├── mcp                  MCP 工具薄封装（委托 services）
├── service              业务逻辑（portfolio / admin / …）
├── jobs                 定时任务（NAV 刷新、持仓抓取、快照重算）
├── datasource           外部数据源适配（基金净值 / 行情 / 持仓）
├── agenttools           agent 工具注册表 + 角色授权
├── agentops             写操作确认（confirmation_id + token）
└── repository           SQLite / PG 双驱动数据访问
```

分层原则：路由不写业务逻辑，业务集中在 `internal/service`；REST 与 MCP 都委托 service，不重复实现。

## 3. 数据流

```
外部数据源（天天基金 / 东方财富 / Yahoo Finance 等）
      │ HTTP（按 datasource 适配）
      ▼
internal/datasource ── 抓取 + 归一化
      ▼
internal/service  ── 计算（XIRR / 回撤 / 穿透 / 快照）
      ▼
internal/repository ── SQLite 持久化
      ▼
internal/httpapi / internal/mcp ── 对外暴露
```

定时任务（`internal/jobs`）驱动：NAV 价格刷新 → 快照重算；持仓抓取 → 穿透分析。

## 4. 鉴权边界

| 面 | 鉴权 |
|----|------|
| SPA 静态面（`/`、assets） | 公开（代码即开源）；数据全收 `/api` 门 |
| `/api/auth/status|setup|login` | 匿名 + 登录限流（递增锁定至 24h） |
| 其余 `/api/*` 读 | session cookie（滑动续约 30d / 封顶 90d）+ 全 API per-IP 限流 |
| 浏览器写路径 | session + `X-Fund-Request` CSRF 头 + Origin 白名单；EdgeKey 为可选兼容层 |
| `/api/system/*` 工作台 | 读走 SessionAuth，写走 session + CSRF（二次确认在 UI 层） |
| `/api/admin/*` | `Authorization: Bearer MCP_API_KEY`（空 key fail-closed） |
| `/mcp` | `MCP_API_KEY`（operator）或 `PUBLIC_MCP_KEY`（analyst），per-key 限流 |
| MCP 写工具 | `confirmation_id` + `confirmation_token`（拒绝 bare `confirmed=true`） |
| `/api/health` | 匿名 |

审计：`auth_events`（登录/锁定/改密/会话，180d 清扫）与 `agent_audit_events`（工具调用，90d 清扫）双流，工作台 `/system/audit` 合并时间线。公网加固细节见 `docs/design/06`。

## 5. 数据模型

核心表：`fund_details`（基金）、`transactions`（交易）、`nav_history`（净值历史）、`portfolio_snapshot`（组合快照）、`fund_holdings`（持仓穿透）、`source_events`（数据源事件）、`dca_plans`（定投计划）。

Schema 现状：权威基线为 `internal/repository/db/schema_pg.go`（`EnsurePGSchema`）与 `schema_sqlite.go`（`EnsureSQLiteSchema`，空库首装自举）；两驱动均在启动时幂等建表/建索引。列级演进走 `schema_meta` probing，不改既有表。
