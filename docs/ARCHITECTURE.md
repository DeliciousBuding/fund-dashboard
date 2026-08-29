# Fund Dashboard — Architecture

> 基金 + 股票投资数据可视化与分析平台  
> 最后更新：2026-08-29  
> 版本：v3 — pure Go backend + SQLite
> 演进方向已定档：新一代 Web UI 以 `web/` workspace 回归本仓（登录 session 鉴权 + go:embed 内嵌），见 [`docs/design/`](design/README.md)（W0 起实施）。

## 1. 总览

单容器部署：Go 静态二进制（REST + MCP），SQLite 持久化。React / Vite SPA 于 2026-08-29 移出本仓库（`packages/web` 已删除）；新一代 SPA 以 `web/` workspace 回归并 go:embed 内嵌（见 [`docs/design/`](design/README.md)，W0 起）。

```
Browser / AI Agent
      │  HTTPS
      ▼
edge proxy ── TLS + path-scoped EdgeKey + rate limits + CSP
      │
      ▼
:8765  fund-dashboard (Go binary, single container)
      ├── REST /api/*
      └── MCP /mcp (Bearer key)
      │
      ▼
SQLite (FUND_DB_PATH, e.g. /app/data/fund.db)
```

| 层 | 技术 |
|----|------|
| 后端 | Go 1.26+ · `go-chi` · `cmd/fund-dashboard` + `internal/*` |
| 契约 | zod schemas 前后端共享 · `packages/contracts` |
| 存储 | SQLite（默认）；可选 PostgreSQL 双驱动（`FUND_DB_DRIVER=pg`） |
| MCP | JSON-RPC over HTTP（单端点 `POST /mcp`）；operator / analyst 双 scope |
| 前端 | `web/`（W0 起；React SPA，go:embed 内嵌）——见 `docs/design/02` |

> 历史前端：`packages/web`（React + Vite + echarts）已从本仓库移除（2026-08-29）。当前生产镜像为 API-only；`FUND_STATIC_DIR` 仍可作为可选静态目录挂载点（注意：指向不存在的目录会**启动失败**，app 装配硬校验）。注：`docs/design/` D7 已定档恢复 go:embed 内嵌 SPA（W0 起实施），届时生产镜像重新默认内嵌前端。

## 2. 后端分层

```
cmd/fund-dashboard       入口：装配 config → repository → services → httpapi/mcp → jobs
internal/
├── app                  应用装配、生命周期
├── config               配置解析与校验（fail-closed）
├── httpapi              REST 薄封装（路由、鉴权、JSON 写回）
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
| `/api/admin/*` | `Authorization: Bearer MCP_API_KEY`（空 key fail-closed） |
| `/mcp` | `MCP_API_KEY`（operator）或 `PUBLIC_MCP_KEY`（analyst） |
| MCP 写工具 | `confirmation_id` + `confirmation_token`（拒绝 bare `confirmed=true`） |
| 前端写路径 | edge proxy 注入 `X-Fund-Edge-Key`（`FUND_EDGE_KEY`） |
| `/api/health` | 匿名 |

## 5. 数据模型

核心表：`fund_details`（基金）、`transactions`（交易）、`nav_history`（净值历史）、`portfolio_snapshot`（组合快照）、`fund_holdings`（持仓穿透）、`source_events`（数据源事件）、`dca_plans`（定投计划）。

Schema 现状：权威基线为 `internal/repository/db/schema_pg.go`（PG 侧启动幂等 `CREATE IF NOT EXISTS`）；SQLite 业务库由外部预置，Go 侧仅建 agent 两表，启动无 schema 校验（旧 `internal/repository/sqlitecompat` 包已删除）。
