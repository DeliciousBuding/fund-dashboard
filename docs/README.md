最后更新：2026-08-29

<p align="center">
  <img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="Apache 2.0">
  <img src="https://img.shields.io/badge/runtime-Go%201.25-00ADD8?logo=go" alt="Go">
  <img src="https://img.shields.io/badge/backend-chi%20%2B%20SQLite-lightgrey" alt="chi+sqlite">
</p>

# Fund Dashboard · 基金看板

Personal fund & stock investment analytics — NAV/price tracking, XIRR, drawdown, portfolio penetration, market indices, and AI-assisted portfolio interrogation via MCP.

基金 + 股票投资数据可视化与分析平台 — 净值/股价追踪、XIRR、回撤、股权穿透、市场指数、MCP Agent 查询。

## Features

- Portfolio dashboard — aggregate NAV/price, P&L, XIRR
- XIRR engine — Newton multi-start + bisection fallback
- Drawdown / timeline / penetration / allocation workbench
- Market indices + US stock cache reads
- MCP JSON-RPC tools（operator 全量 / analyst 只读；写操作需 AgentOps `confirmation_id`+`token`，拒绝 bare `confirmed=true`）
- Admin diagnostics — freshness, verify, db-integrity, dashboard
- Agent harness / source brief / source events (facts-only)

## Architecture

见 [`ARCHITECTURE.md`](./ARCHITECTURE.md)。单容器 Go 后端（REST + MCP）+ SQLite。前端独立仓库维护（`packages/web` 于 2026-08-29 移出）。

| Layer | Tech |
|-------|------|
| Backend | Go 1.25+, chi, modernc/sqlite（pgx/v5 可选） |
| Contracts | zod（`packages/contracts`）——前端复用的 API 契约 SSOT |
| Image | 多架构（linux/amd64 + linux/arm64）Go 静态二进制 |

## Quick start

```bash
# Backend
go test ./... -count=1
go run ./cmd/fund-dashboard

# CI-style container smoke
./scripts/seed-ci-db.sh
docker build -t fund-dashboard:ci -f deploy/Dockerfile .
WEB_PORT=18080 docker compose -f deploy/docker-compose.ci.yml up -d
curl -fsS http://127.0.0.1:18080/api/health
```

构建与运行：见 [`../deploy/README.md`](../deploy/README.md)

## Repository layout

```
cmd/fund-dashboard/     Go entrypoint
internal/               app, httpapi, mcp, service, jobs, datasource, agenttools, ...
packages/contracts/     Zod contracts（前后端共享）
deploy/                 Dockerfile, CI compose, seed SQL
docs/                   architecture, testing, changelog
```

## Documentation map

| Doc | Role |
|-----|------|
| [`ARCHITECTURE.md`](./ARCHITECTURE.md) | 架构与分层 |
| [`TESTING.md`](./TESTING.md) | 测试体系 SSOT |
| [`CHANGELOG.md`](../CHANGELOG.md) | 产品 / 修复变更流水 |

## License

Apache-2.0
