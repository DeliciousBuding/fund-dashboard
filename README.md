# Fund Dashboard

[![CI](https://github.com/DeliciousBuding/fund-dashboard/actions/workflows/ci.yml/badge.svg)](https://github.com/DeliciousBuding/fund-dashboard/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/DeliciousBuding/fund-dashboard?sort=semver)](https://github.com/DeliciousBuding/fund-dashboard/releases)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)

个人投资组合工作台：单租户 Web UI（登录密码鉴权）+ REST API + 面向 AI agent 的 MCP server，公网暴露形态设计。基金 / 股票持仓管理、净值追踪、XIRR 年化、最大回撤、穿透分析、DCA 定投与回测、蒙特卡洛模拟、相关性分析、系统工作台（任务/审计/告警）。

纯 **Go** 后端 + **React 19 / Vite 7 / Tailwind v4** 前端（`web/`，go:embed 内嵌 → 单二进制交付），SQLite 默认持久化（可选 PostgreSQL 双驱动）。设计与实施波次见 [docs/design/](docs/design/README.md)。

## 功能

- **持仓管理**：基金 + 股票（A 股 / 港股 / 美股）统一管理，交易 CRUD，自动快照重算
- **量化分析**：XIRR 年化收益率、最大回撤、组合净值走势、盈亏分布、蒙特卡洛模拟、相关性热力图、基金对比雷达、DCA 定投回测
- **穿透分析**：基金持仓穿透到底层股票，按行业聚合
- **Web UI**：总览 / 持仓 / 交易台账 / 定投 / 分析套件（对比·回测·相关性·蒙特卡洛·穿透）/ 市场 / 信号 / 报告 / 系统工作台 / 设置，桌面 + 移动端自适应，⌘K 命令面板，PWA 离线兜底
- **MCP server**：AI agent 工具（portfolio / transactions / admin / market / analysis / report），JSON-RPC over HTTP（单端点 `POST /mcp`），Bearer key 双 scope 认证
- **REST API**：`/api/*` 全量读写端点，zod 契约共享（`packages/contracts`）
- **安全**：argon2id 密码 + 服务端 session（滑动续约）、登录递增锁定、全 API 限流、CSRF 三重防护、CSP/HSTS、认证与 Agent 双审计时间线（docs/design/06）

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go（`cmd/fund-dashboard` + `internal/*`），`net/http` + `go-chi` |
| 契约 | zod schemas（`packages/contracts`）——前后端共享的 API 契约 SSOT |
| 存储 | SQLite（默认），可选 PostgreSQL 双驱动 |
| 认证 | Web 登录 session（argon2id + cookie，docs/design/04）+ MCP/Admin Bearer key 双轨 |
| 前端 | `web/`：React 19 + Vite 7 + Tailwind v4 + Radix + ECharts + TanStack 全家桶（go:embed 内嵌，docs/design/02/03） |

## 快速开始

```bash
# 测试
go test ./... -count=1

# 构建
go build -o bin/fund-dashboard ./cmd/fund-dashboard

# 容器
docker build -f deploy/Dockerfile -t fund-dashboard:local .
```

## 文档

| 文档 | 内容 |
|------|------|
| [`docs/design/`](docs/design/README.md) | **重设计定档**（产品方向 / 技术栈 / 设计系统 / 鉴权安全 / 路线图） |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | 架构 |
| [`docs/TESTING.md`](docs/TESTING.md) | 测试体系 |
| [`CHANGELOG.md`](CHANGELOG.md) | 变更日志 |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | 贡献与发布流程 |
| [`deploy/README.md`](deploy/README.md) | 构建与运行 |
| [`AGENTS.md`](AGENTS.md) | Agent 共享约束 |

## 发布

本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/)。发布通过 Git tag 触发：

```bash
./scripts/release.sh 2.0.0   # 归并 CHANGELOG + 打 tag v2.0.0 + 推送
```

main push / workflow_dispatch 触发 `ci.yml` 构建多架构镜像推送到 GHCR；tag push 触发 `release.yml` 创建 [Release](https://github.com/DeliciousBuding/fund-dashboard/releases)，release notes 取自 `CHANGELOG.md`。镜像拉取：

```bash
docker pull ghcr.io/deliciousbuding/fund-dashboard:latest
```

## 参与贡献

欢迎提 Issue 和 PR。开发流程：从 `main` 拉出 `feature/*` 分支，改动后提 PR 回 `main`。详见 [CONTRIBUTING.md](CONTRIBUTING.md)。

License: Apache-2.0
