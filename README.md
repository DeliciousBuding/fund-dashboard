# Fund Dashboard

个人投资组合管理 —— 基金 / 股票持仓、净值追踪、XIRR 年化、穿透分析、回测、MCP server。

## 功能

- **持仓管理**：基金 + 股票（A 股 / 港股 / 美股）统一管理，交易 CRUD，自动快照重算
- **量化分析**：XIRR 年化收益率、最大回撤、组合净值走势、盈亏分布、蒙特卡洛模拟、相关性热力图、基金对比雷达、DCA 定投回测
- **穿透分析**：基金持仓穿透到底层股票（"我到底买了什么？"），按行业聚合
- **MCP Server**：34 个 AI Agent 工具（search / portfolio / transactions / admin / operations / securities / market / analysis / report），Streamable HTTP 传输，Bearer key 认证
- **双主题**：亮 / 暗主题等重打磨，红涨绿跌（CN 约定），echarts 6 tree-shaking 统一生命周期
- **数据源**：天天基金（净值）、Yahoo Finance（美股 / 指数）、东方财富（基金持仓）

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Bun + Hono + native SQLite (`packages/server`) |
| 前端 | React 18 + Vite 5 + echarts 6 + Cloudflare Kumo (`packages/web`) |
| 契约 | zod 4 schemas 前后端共享 (`packages/contracts`) |
| MCP | mcp-lite（zero-deps，Hono-compatible） |

## 快速开始

```bash
# 安装（npm workspaces — 根目录一次性装全部）
npm install

# 开发（web :5176 + server :8765）
npm run dev

# 构建
npm run build

# 测试
npm test
```

环境变量（`packages/server`）：

| 变量 | 作用 |
|---|---|
| `MCP_API_KEY` | admin scope —— REST `/api/admin/*` + 全部 34 个 MCP tools |
| `PUBLIC_MCP_KEY` | public scope —— 仅 MCP tools（用于公开暴露，与 admin 物理隔离） |
| `DB_PATH` | SQLite 路径（默认 `data/fund.db`） |
| `CORS_ORIGIN` | 允许的前端来源（逗号分隔，默认 localhost） |

## MCP Server

`/mcp` 端点暴露 34 个 AI Agent 工具，可供 Claude / Cursor / Cline 等客户端调用。Streamable HTTP 传输，Bearer key 认证（双 scope）。

```bash
# 列出所有 tools
curl -X POST http://localhost:8765/mcp \
  -H "Authorization: Bearer $MCP_API_KEY" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","method":"tools/list","id":1}'
```

公开暴露时建议前置 nginx 限流 + Cloudflare proxy（隐藏源站 / DDoS），仅 `/mcp` 路径公开，其余管理接口保持内网保护。

## 项目结构

```
packages/
├── server/      Bun + Hono 后端 + MCP + 爬虫
├── web/         React + Vite 前端
└── contracts/   zod schemas（前后端契约 SSOT）
```

## License

[Apache-2.0](LICENSE)
