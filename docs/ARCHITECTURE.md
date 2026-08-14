# Fund Dashboard — Architecture

> 基金 + 股票投资数据可视化与分析平台  
> 最后更新：2026-08-14  
> 版本：v3 — pure Go backend + React SPA + SQLite

## 1. 总览

单容器部署：Go 静态二进制（REST + MCP）内嵌 React SPA 静态资源，SQLite 持久化。

```
Browser / AI Agent
      │  HTTPS
      ▼
edge proxy ── TLS + path-scoped EdgeKey + rate limits + CSP
      │
      ▼
:8765  fund-dashboard (Go binary, single container)
      ├── SPA static (FUND_STATIC_DIR)
      ├── REST /api/*
      └── MCP /mcp (Bearer key)
      │
      ▼
SQLite (FUND_DB_PATH, e.g. /app/data/fund.db)
```

| 层 | 技术 |
|----|------|
| 后端 | Go 1.25+ · `go-chi` · `cmd/fund-dashboard` + `internal/*` |
| 前端 | React + Vite · `packages/web` |
| 契约 | zod schemas 前后端共享 · `packages/contracts` |
| 存储 | SQLite（默认）；可选 PostgreSQL 双驱动（`FUND_DB_DRIVER=pg`） |
| MCP | Streamable HTTP `/mcp`；operator / analyst 双 scope |

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
| SPA 写路径 | edge proxy 注入 `X-Fund-Edge-Key`（`FUND_EDGE_KEY`） |
| `/api/health` | 匿名 |

## 5. 数据模型

核心表：`fund_details`（基金）、`transactions`（交易）、`nav_history`（净值历史）、`portfolio_snapshot`（组合快照）、`fund_holdings`（持仓穿透）、`source_events`（数据源事件）、`dca_plans`（定投计划）。

兼容性校验见 `internal/repository/sqlitecompat`（启动时 PRAGMA / schema 校验）。
