# Backend-TS — Bun + Hono API

> Fund Dashboard 后端 v2.5
> 最后更新：2026-06-19

## 启动

```bash
bun install
bun main.ts        # 生产
bun --watch main.ts # 开发 (热重载)
bun test            # 运行测试
```

端口：`8765` (设 `PORT` 环境变量覆盖)

## 架构

```
main.ts                  ← 入口，挂载路由 + MCP
db.ts                    ← SQLite 读写双连接池 (WAL)
middleware/logger.ts     ← 结构化日志 (reqId + 计时)
middleware/rate-limit.ts ← 速率限制
datasources/             ← 数据源抽象层 (东方财富 / Yahoo Finance)
routes/
  portfolio.ts           ← /api/portfolio (组合、穿透、配置、harness)
  funds.ts               ← /api/funds + /api/securities (证券列表/详情)
  stocks.ts              ← /api/stocks (美股详情)
  market.ts              ← /api/market (指数、汇率)
  admin/                 ← /api/admin (CRUD / 导入 / 运维)
    crud.ts, import.ts, ops.ts
services/
  portfolio.ts           ← 组合统计、穿透、配置、harness、source-events
  pricing.ts             ← 定价与估值
  xirr.ts                ← Newton+二分法 XIRR
  summary.ts             ← 汇总计算、快照重建
  system.ts              ← 系统状态诊断
  source-events.ts       ← 来源事件队列
  db-integrity.ts        ← 完整性检查与自动修复
mcp/
  server.ts              ← MCP 注册中心 (34 个工具, 8 个模块)
  tools/query.ts         ← 查询 (5)
  tools/portfolio.ts     ← 组合 (6)
  tools/analysis.ts      ← 分析 (1)
  tools/operations.ts    ← 操作 (3)
  tools/admin.ts         ← 管理 (6)
  tools/market.ts        ← 市场 (5)
  tools/securities.ts    ← 证券CRUD (4)
  tools/transactions.ts  ← 交易CRUD (4)
crawler/
  nav.ts, eastmoney.ts, yahoofinance.ts, holdings.ts, scheduler.ts
__tests__/api.test.ts, api-integration.test.ts
```

## REST API (35+ 端点)

### 组合与穿透

| 端点 | 说明 |
|------|------|
| `GET /api/portfolio` | 组合统计 (含基金+股票) |
| `GET /api/portfolio/xirr` | 组合 XIRR |
| `GET /api/portfolio/timeline` | 每日市值时间线 |
| `GET /api/portfolio/penetration` | 股权穿透分析 (底层股票暴露) |
| `GET /api/portfolio/allocation` | 资产配置视图 |
| `GET /api/portfolio/harness` | Agent Harness 事实快照 |
| `GET /api/portfolio/source-brief` | 消息源检索上下文 |
| `GET /api/portfolio/source-events` | 来源事件队列 |
| `POST /api/portfolio/source-events` | 创建来源事件 |
| `PATCH /api/portfolio/source-events/:id` | 标记事件已读/有用 |

### 证券 (基金 + 股票)

| 端点 | 说明 |
|------|------|
| `GET /api/funds` | 全部证券列表 |
| `GET /api/funds/:code` | 证券详情 + 交易记录 |
| `GET /api/funds/:code/nav` | 价格历史 |
| `GET /api/funds/:code/xirr` | 证券 XIRR |
| `GET /api/funds/:code/drawdown` | 最大回撤 |
| `GET /api/funds/:code/dca` | 定投金额计算器 |
| `GET /api/funds/summary` | 按证券汇总 |

### 美股与市场

| 端点 | 说明 |
|------|------|
| `GET /api/stocks/:code` | 美股实时行情 + K线 |
| `GET /api/market/indices` | 美股指数 (纳斯达克100/标普500/道琼斯) |
| `GET /api/market/index/:code` | 单一指数实时行情 |
| `GET /api/market/index/:code/history` | 指数历史数据 |
| `GET /api/market/exchange-rate` | USD/CNY 汇率 |

### 管理

| 端点 | 说明 |
|------|------|
| `GET /api/admin/status` | 全量诊断 |
| `GET /api/admin/status/:code` | 单证券诊断 |
| `POST /api/admin/crawl-nav` | 触发价格爬取 |
| `POST /api/admin/crawl-holdings` | 触发持仓明细爬取 |
| `POST /api/admin/import-transactions` | 批量导入交易 |
| `POST /api/admin/import-csv` | CSV 批量导入 |
| `POST /api/admin/recalculate-snapshot` | 重算持仓快照 |
| `GET /api/admin/verify` | 数据一致性校验 |
| `GET /api/admin/db-integrity` | 数据库完整性检查 |
| `POST /api/admin/db-repair` | 自动修复 |
| `POST /api/admin/db-restore` | 从备份恢复 |
| `GET /api/admin/backup-status` | 备份状态 |
| `POST /api/admin/securities` | 创建证券 |
| `PUT /api/admin/securities/:code` | 更新证券 |
| `DELETE /api/admin/securities/:code` | 删除证券 |
| `PUT /api/admin/transactions/:seq` | 更新交易 |
| `DELETE /api/admin/transactions/:seq` | 删除交易 |

### 其他

| 端点 | 说明 |
|------|------|
| `GET /api/health` | 健康检查 |
| `GET /api/status` | → `/api/admin/status` |
| `GET /api/summary` | 汇总数据 |
| `POST /mcp` | MCP JSON-RPC (AI Agent 入口) |

## MCP 工具 (34 个, 8 模块)

### 查询 (5)

| 工具 | 用途 |
|------|------|
| `search_funds` | 搜索证券 (基金/股票，按名称/代码/类型) |
| `get_fund_detail` | 完整证券详情 (持仓/XIRR/交易/状态) |
| `get_nav_history` | 价格历史 |
| `get_fund_xirr` | 年化收益率 |
| `get_fund_drawdown` | 最大回撤 |

### 组合 (6)

| 工具 | 用途 |
|------|------|
| `get_portfolio_summary` | 组合全貌 (总资产/盈亏/持仓分布) |
| `get_portfolio_xirr` | 组合 XIRR + 当前市值 |
| `get_portfolio_timeline` | 每日总资产时间线 |
| `get_portfolio_allocation` | 资产配置 (按类型/市场/主题聚合) |
| `get_investment_harness_snapshot` | Agent 金融 Harness 事实快照 |
| `get_investment_source_brief` | 消息源检索上下文 |

### 分析 (1) / 操作 (3) / 管理 (6)

| 工具 | 用途 |
|------|------|
| `compute_dca_amount` | 定投金额计算器 (成本偏离/涨跌幅双模式) |
| `crawl_nav` | 触发价格爬取 (基金+股票) |
| `recalculate_snapshot` | 重建持仓快照 |
| `adjust_position` | 手动调整持仓份额 |
| `get_system_status` | 系统诊断 + 全球市场时段 |
| `get_fund_status` | 单证券管理状态 |
| `verify_data` | 数据完整性校验 |
| `get_data_freshness` | 数据新鲜度诊断 |
| `get_source_events` | 来源事件队列 (新闻/公告/搜索) |
| `mark_source_event` | 标记事件已读/有用 |

### 市场 (5) / 证券 (4) / 交易 (4)

| 工具 | 用途 |
|------|------|
| `get_market_indices` | 美股主要指数实时行情 |
| `get_portfolio_penetration` | 股权穿透 (底层股票权重与金额) |
| `get_us_stock` | 美股实时行情 + K线 |
| `search_stocks` | 搜索股票 (A股/港股/美股，本地+实时) |
| `crawl_fund_holdings` | 爬取基金季度持仓 |
| `add_fund` | 添加基金 |
| `add_security` | 添加证券 (基金/股票) |
| `update_fund` | 更新证券信息 |
| `delete_fund` | 删除证券 (级联) |
| `add_transaction` | 添加交易记录 |
| `update_transaction` | 修改交易记录 |
| `delete_transaction` | 删除交易记录 |
| `import_transactions` | 批量导入交易 |

## 特性

- **多证券类型**：基金 + A股/港股/美股，统一 `fund_details` 表，`security_type` + `market` 区分
- **市场指数**：纳斯达克100/标普500/道琼斯/纳斯达克综合，Yahoo Finance 实时 + 缓存
- **股权穿透**：通过基金季度报告计算底层股票实际权重，支持按行业聚合
- **资产配置**：按证券类型、市场、行业的配置视图 + 风险提示
- **Agent Harness**：为 AI Agent 提供的金融事实快照，只提供数据不做投资建议
- **Source Brief**：为 Hermes/WebSearch 生成消息源检索上下文
- **Source Events**：来源事件队列，Agent 可消费已抓取新闻/公告并标记已读/有用

## 测试

```bash
bun test
# 涵盖：health, portfolio, funds, stocks, market, admin, MCP, datasources
```
