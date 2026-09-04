# AGENTS.md — Fund Dashboard

最后更新：2026-08-30 23:53

> 所有 AI agent（Claude Code / Codex / Cursor）在本文档仓库遵守的共享约束。
> 开源公开仓库。**禁止提交任何主机名、公网 IP、真实密钥、个人邮箱或本地路径。**
> 产品：个人投资组合工作台（公网暴露形态）—— Web UI（session 登录）+ REST API + MCP server。基金 / 股票持仓、净值追踪、XIRR、穿透分析、回测、定投、系统工作台。
> 本仓库 = Go 后端 + `web/` 前端（go:embed 内嵌单二进制）+ zod 契约。设计定档 [`docs/design/`](docs/design/README.md)（W0–W7 已实施）。

## S.U.P.E.R 设计原则

| 原则 | 仓库内检查标准 |
|------|---------------|
| **S**ingle Purpose | 路由只做协议适配；业务进 `internal/service`；数据访问进 `internal/repository` |
| **U**nidirectional | datasource/repository → service → REST/MCP/UI，禁止反向依赖和跨层旁路 |
| **P**orts over Implementation | REST 与 MCP 共用 service；SQLite/PG 经统一 repository/dialect 契约 |
| **E**nvironment-Agnostic | 主机、域名、密钥和部署拓扑只从环境/私有运维面注入，公开仓不固化 live 值 |
| **R**eplaceable | 前端、数据源、数据库驱动、边缘代理均通过明确边界可替换，不改核心业务语义 |

## 项目骨架

- 入口：`cmd/fund-dashboard`
- 业务：`internal/`（分层主干见 `docs/ARCHITECTURE.md` §2；完整包以 `go list ./internal/...` 为权威）
- 契约：`packages/contracts`（zod，前后端共享的 API 契约 SSOT）
- 部署：`deploy/Dockerfile`（三阶段：web 构建 → Go 静态二进制含内嵌 SPA）· `deploy/docker-compose.ci.yml`（CI smoke）
- 文档：`docs/{ARCHITECTURE,TESTING}.md` · `CHANGELOG.md` · `CONTRIBUTING.md`
- 前端：`web/`（React 19 + Vite 7 + Tailwind v4 + TanStack + ECharts；路由懒加载 + echarts 分包；设计定档 `docs/design/`）

## 鉴权边界

| 面 | 鉴权 | 说明 |
|----|------|------|
| `/api/admin/*` | `Authorization: Bearer MCP_API_KEY` | 空 key fail-closed |
| `/mcp` | 静态 key：`MCP_API_KEY` → Operator；`PUBLIC_MCP_KEY` → Analyst。**或** OAuth 访问令牌（ES256 JWT，`aud` = `<issuer>/mcp`）：`fund.read` → Analyst，`fund.write` → Operator | 双空且 OAuth 关闭时 fail-closed；作用域→角色映射单一来源（`oauth.RoleForScopes`），设计见 `docs/design/07` |
| `/.well-known/oauth-*`、`/oauth/token|register|revoke|jwks|about` | 匿名（发现文档按定义公开；公有客户端 + PKCE S256，服务端不发也不收 client secret） | **必须先于 SPA fallback 注册**，否则返回 200+HTML 会让连接器静默判定「无认证」；per-IP 限流 `FUND_OAUTH_RPM`，discovery 不计入 |
| `/oauth/authorize`、`/oauth/consent` | 复用 `fund_session` cookie；无会话 → 302 `/login?next=…`（回跳目标规范化后二次校验 `/oauth/` 前缀） | 同意页带一次性 `consent_token`（10min、单次消费），零 JS / 零内联样式以符合既有 CSP |
| MCP 写工具 | `confirmation_id` + `confirmation_token` | **拒绝** bare `confirmed=true` |
| 浏览器读/写 | session cookie（argon2id 登录，滑动续约）；写加 `X-Fund-Request` 头 + Origin 白名单 | 设计见 `docs/design/04` |
| 前端写路径（兼容层，默认关闭） | edge proxy 注入 `X-Fund-Edge-Key`（`FUND_EDGE_KEY`）；需显式 `FUND_EDGE_AUTH_ENABLED=true` 才生效 | 浏览器 JS 不持 key（会话写路径优先） |
| 全 `/api/*` | per-IP 限流（`FUND_API_RPM`）；`/mcp` per-key（`FUND_MCP_RPM`）；`/oauth/*` per-IP（`FUND_OAUTH_RPM`） | 公网加固见 `docs/design/06` |
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

CI 硬门禁：`test-go` / `build-go` / `test-web` / `build-web` / `smoke-e2e`（容器 + API/MCP + Chromium 浏览器关键路径）。

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
