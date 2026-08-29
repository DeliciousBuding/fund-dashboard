# Fund Dashboard

[![CI](https://github.com/DeliciousBuding/fund-dashboard/actions/workflows/ci.yml/badge.svg)](https://github.com/DeliciousBuding/fund-dashboard/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/DeliciousBuding/fund-dashboard?sort=semver)](https://github.com/DeliciousBuding/fund-dashboard/releases)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)

个人投资组合分析平台：基金 / 股票持仓管理、净值追踪、XIRR 年化、最大回撤、穿透分析、DCA 定投回测、蒙特卡洛模拟，以及面向 AI agent 的 MCP server。

纯 **Go** 后端，SQLite 持久化（可选 PostgreSQL）。前端独立仓库维护（`packages/web` 已于 2026-08-29 移出，见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)）。

## 功能

- **持仓管理**：基金 + 股票（A 股 / 港股 / 美股）统一管理，交易 CRUD，自动快照重算
- **量化分析**：XIRR 年化收益率、最大回撤、组合净值走势、盈亏分布、蒙特卡洛模拟、相关性热力图、基金对比雷达、DCA 定投回测
- **穿透分析**：基金持仓穿透到底层股票，按行业聚合
- **MCP server**：AI agent 工具（portfolio / transactions / admin / market / analysis / report），Streamable HTTP 传输，Bearer key 认证
- **REST API**：`/api/*` 全量读写端点，zod 契约共享（`packages/contracts`）

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go（`cmd/fund-dashboard` + `internal/*`），`net/http` + `go-chi` |
| 契约 | zod schemas（`packages/contracts`）——前后端共享的 API 契约 SSOT |
| 存储 | SQLite（默认），可选 PostgreSQL 双驱动 |
| 认证 | Bearer key（operator / analyst 双 scope）+ edge proxy 注入的写路径 key |

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

tag 推送后 GitHub Actions 自动构建多架构镜像（GHCR）并创建 [Release](https://github.com/DeliciousBuding/fund-dashboard/releases)，release notes 取自 `CHANGELOG.md`。镜像拉取：

```bash
docker pull ghcr.io/deliciousbuding/fund-dashboard:latest
```

## 参与贡献

欢迎提 Issue 和 PR。开发流程：从 `main` 拉出 `feature/*` 分支，改动后提 PR 回 `main`。详见 [CONTRIBUTING.md](CONTRIBUTING.md)。

License: Apache-2.0
