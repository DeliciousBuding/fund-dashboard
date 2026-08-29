# AGENTS.md — Fund Dashboard

最后更新：2026-08-29

> 所有 AI agent（Claude Code / Codex / Cursor）在本文档仓库遵守的共享约束。
> 开源公开仓库。**禁止提交任何主机名、公网 IP、真实密钥、个人邮箱或本地路径。**
> 产品：个人投资组合仪表盘 —— 基金 / 股票持仓、净值追踪、XIRR、穿透分析、回测、MCP server。
> 前端已于 2026-08-29 移出本仓库；新一代 Web UI 以 `web/` workspace 回归本仓（设计定档 [`docs/design/`](docs/design/README.md)，W0 起实施）。本仓库 = Go 后端 + zod 契约 + web/。

## S.U.P.E.R 设计原则

| 原则 | 含义 | 检查标准 |
|------|------|---------|
| **S**ingle Responsibility | 每个模块只做一件事 | 文件精简，函数短小，单一数据源 |
| **U**nified Interface | 跨层统一接口 | DataSource 接口统一所有外部数据，REST/MCP 薄封装 |
| **P**redictable Behavior | 无意外、无静默失败 | 所有错误返回 JSON；写路径可审计、可回放 |
| **E**xtensible Design | 易于添加新市场/数据源 | 注册新 DataSource 只需实现接口，不改路由 |
| **R**eliable Operation | 容错、持久化、可监控 | health 真实验证 DB；默认 SQLite，PG 驱动可选未接 |

## 项目骨架

- 入口：`cmd/fund-dashboard`
- 业务：`internal/{app,httpapi,mcp,service,jobs,datasource,agenttools,agentops,repository}`
- 契约：`packages/contracts`（zod，前后端共享的 API 契约 SSOT）
- 部署：`deploy/Dockerfile`（Go 静态二进制 API-only）· `deploy/docker-compose.ci.yml`（CI smoke）
- 文档：`docs/{ARCHITECTURE,TESTING}.md` · `CHANGELOG.md` · `CONTRIBUTING.md`
- 前端：`web/`（规划中→W0 起实施；Vite + React SPA，go:embed 内嵌单二进制；设计定档 `docs/design/`）

## 鉴权边界

| 面 | 鉴权 | 说明 |
|----|------|------|
| `/api/admin/*` | `Authorization: Bearer MCP_API_KEY` | 空 key fail-closed |
| `/mcp` | `MCP_API_KEY` → Operator；`PUBLIC_MCP_KEY` → Analyst | 双空 fail-closed |
| MCP 写工具 | `confirmation_id` + `confirmation_token` | **拒绝** bare `confirmed=true` |
| 前端写路径 | edge proxy 注入 `X-Fund-Edge-Key`（`FUND_EDGE_KEY`） | 浏览器 JS 不持 key |
| `/api/*`（浏览器读+写） | session cookie（密码登录，argon2id） | **规划 W1**，设计见 `docs/design/04-auth-security.md` |
| `/api/health` | 匿名 | 生产省略 version |

## 禁止事项

- ❌ 路由文件里写业务逻辑 → 必须在 `internal/service`
- ❌ 硬编码数据源 URL → 必须在 `internal/datasource`
- ❌ REST 和 MCP 逻辑重复 → 都调用 services
- ❌ 静默吞错误 → 所有 catch 必须 log
- ❌ 提交主机名 / 公网 IP / 真实密钥 / 个人邮箱 / 本地绝对路径
- ❌ MCP 写操作只靠客户端 `confirmed=true` → 必须 agentops token

## 编码规范

- 所有 SQL 用参数化查询（`?`；pg 驱动层重绑定 `$N`），禁止字符串拼接
- 日期统一 YYYY-MM-DD
- 金额 REAL，交易/快照精度一致
- MCP 工具描述中文；API 错误 `{error: string}`
- 类型优先 `packages/contracts/`（zod）；Go DTO 以 service 为准

## 验证

测试分层见 [`docs/TESTING.md`](docs/TESTING.md)（unit / contract / container smoke）。

```bash
./scripts/test-unit.sh          # go vet + go test ./...

# 等价拆分
go test ./... -count=1
go vet ./...
go build -o bin/fund-dashboard ./cmd/fund-dashboard

# 可选覆盖率（无失败阈值）
./scripts/test-cover.sh
```

CI 硬门禁：`test-go` / `build-go` / `smoke-e2e`（容器 + API 冒烟）。

## 文档地图

| 用途 | 文件 |
|------|------|
| **重设计定档（2026-08-29）** | `docs/design/README.md` |
| 架构 | `docs/ARCHITECTURE.md` |
| 测试体系 | `docs/TESTING.md` |
| 产品变更 | `CHANGELOG.md` |
| 贡献 / 发布 | `CONTRIBUTING.md` |
| 文档路由 | `docs/README.md` |
| 构建 / 运行 | `deploy/README.md` |
