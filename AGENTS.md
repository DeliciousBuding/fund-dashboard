# AGENTS.md — 综合投资系统

> 所有 AI agent（Claude Code, Codex, Cursor）遵守的共享约束

## S.U.P.E.R 设计原则

| 原则 | 含义 | 检查标准 |
|------|------|---------|
| **S**ingle Responsibility | 每个模块只做一件事 | 文件 < 300 行，函数 < 50 行，单一数据源 |
| **U**nified Interface | 跨层统一接口 | DataSource 接口统一所有外部数据，REST/MCP 薄封装 |
| **P**redictable Behavior | 无意外、无静默失败 | 所有错误返回 JSON，所有写操作 WAL checkpoint |
| **E**xtensible Design | 易于添加新市场/数据源 | 注册新 DataSource 只需实现接口，不改路由 |
| **R**eliable Operation | 容错、持久化、可监控 | health check 真实验证 DB，WAL 持久化到宿主机目录 |

## 项目架构

```
packages/
├── server/
│   ├── datasources/     # 外部数据源适配器 (eastmoney, yahoo, pdf-parser)
│   ├── services/        # 业务逻辑层 (portfolio, pricing, dca, xirr)
│   ├── routes/          # REST API 路由 (薄封装，委托 services)
│   ├── mcp/             # MCP 工具 (薄封装，委托 services)
│   ├── crawler/         # 底层爬虫函数（被 datasources 调用）
│   └── middleware/       # 日志、CORS、认证中间件
├── web/
│   └── src/
│       ├── components/  # React 组件 (layout/charts/cards/tables/views)
│       ├── api/         # API 类型和 fetch 函数
│       ├── hooks/       # 自定义 hooks (useDarkMode, useIsMobile)
│       └── utils.ts     # 工具函数 (classify, fmt, chartColors)
└── deploy/              # Docker + nginx 配置
```

## 数据模型

- **securities** (fund_details): 统一资产表 — fund/stock/etf/index
- **transactions**: 交易流水
- **price_history** (nav_history): 统一价格历史
- **portfolio_snapshot**: 持仓快照（物化视图）
- **fund_holdings**: QDII 穿透数据
- **indices**: 指数实时缓存

## 禁止事项

- ❌ 路由文件里写业务逻辑 → 必须在 services/
- ❌ 硬编码数据源 URL → 必须在 datasources/ 对应的适配器里
- ❌ REST 和 MCP 逻辑重复 → MCP 调用 services，REST 也调用 services
- ❌ 静默吞错误 → 所有 catch 必须 log.warn/log.error
- ❌ DB 挂载单个文件 → 必须挂载整个 data/ 目录（WAL 持久化）
- ❌ 前端用原生 HTML 代替 Kumo 组件 → Input/Select/Button/Table/Dialog

## 编码规范

- 所有 SQL 用参数化查询（`?` 占位符），禁止字符串拼接
- 日期统一 YYYY-MM-DD 格式
- 金额统一存为 REAL，展示用 `toFixed(2)`
- 颜色：红涨绿跌（`C.up='#d63649'`, `C.down='#199c63'`）
- MCP 工具描述必须用中文，Zod schema 每个字段加 `.describe()`
- 所有 API 错误返回 `{error: string}` JSON
