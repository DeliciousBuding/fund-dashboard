# Fund Dashboard

个人投资组合分析平台：基金 / 股票持仓管理、净值追踪、XIRR 年化、最大回撤、穿透分析、DCA 定投回测、蒙特卡洛模拟，以及面向 AI agent 的 MCP server。

纯 **Go** 后端 + **React** 前端，单容器部署，SQLite 持久化。

## 功能

- **持仓管理**：基金 + 股票（A 股 / 港股 / 美股）统一管理，交易 CRUD，自动快照重算
- **量化分析**：XIRR 年化收益率、最大回撤、组合净值走势、盈亏分布、蒙特卡洛模拟、相关性热力图、基金对比雷达、DCA 定投回测
- **穿透分析**：基金持仓穿透到底层股票，按行业聚合
- **MCP server**：AI agent 工具（portfolio / transactions / admin / market / analysis / report），Streamable HTTP 传输，Bearer key 认证
- **双主题**：亮 / 暗主题，红涨绿跌

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go（`cmd/fund-dashboard` + `internal/*`），`net/http` + `go-chi` |
| 前端 | React + Vite + echarts（`packages/web`） |
| 契约 | zod schemas 前后端共享（`packages/contracts`） |
| 存储 | SQLite（默认），可选 PostgreSQL 双驱动 |
| 认证 | Bearer key（operator / analyst 双 scope）+ edge proxy 注入的写路径 key |

## 快速开始

```bash
# 测试
go test ./... -count=1
npm ci && npm test

# 构建
go build -o bin/fund-dashboard ./cmd/fund-dashboard
npm run build --workspace packages/web

# 容器
docker build -f deploy/Dockerfile -t fund-dashboard:local .
```

## 文档

| 文档 | 内容 |
|------|------|
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | 架构 |
| [`docs/DESIGN.md`](docs/DESIGN.md) | UI 设计系统 |
| [`docs/TESTING.md`](docs/TESTING.md) | 测试体系 |
| [`docs/CHANGELOG.md`](docs/CHANGELOG.md) | 产品变更 |
| [`deploy/README.md`](deploy/README.md) | 构建与运行 |
| [`AGENTS.md`](AGENTS.md) | Agent 共享约束 |

License: Apache-2.0
