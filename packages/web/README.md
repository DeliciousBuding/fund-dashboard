# Fund Dashboard 前端源码 ![version](https://img.shields.io/badge/version-v2.5-blue)

## 项目概述

基金交易数据可视化平台。左侧按基金类型分组的目录，右侧展示组合总览和单只基金详情。
通过 ECharts 展示净值走势（标注买卖点）、定投/手动买入占比、累计成本 vs 市值曲线。

## 技术栈

- Vite 5 + React 18 + TypeScript
- ECharts v6.1（图表）
- @cloudflare/kumo v2.5（UI 组件与样式系统）

## 文件结构

```
src/
├── main.tsx                        # 入口
├── App.tsx                         # 路由：overview 或 fund detail
├── index.css                       # 全局样式 + CSS 变量（浅色主题）
├── api/
│   └── index.ts                    # API 客户端 + TypeScript 类型定义
├── hooks/
│   ├── usePortfolio.ts             # 组合数据 hook
│   ├── useFundDetail.ts            # 基金详情 hook
│   └── useNasdaqOverview.ts        # 纳斯达克概览 hook
├── pages/
│   ├── Overview.tsx                # 组合总览页
│   └── FundDetail.tsx              # 单只基金详情页
└── components/
    ├── layout/
    │   └── Sidebar.tsx             # 左侧目录栏
    ├── MarketTicker.tsx            # 市场实时行情滚动条
    ├── PortfolioChart.tsx          # 组合净值走势图（标注买卖点）
    ├── NasdaqOverview.tsx          # 纳斯达克指数概览卡片
    ├── PortfolioPenetration.tsx    # 组合穿透分析（行业/地域分布）
    ├── PortfolioAllocation.tsx     # 组合资产配置饼图
    ├── InvestmentHarnessPanel.tsx  # 投资 harness 面板（定投/手动统计）
    ├── PnLDistributionChart.tsx    # 盈亏分布直方图
    ├── DcaPanel.tsx                # 定投（DCA）模拟与回测面板
    ├── FundDetailView.tsx          # 单只基金详情视图
    ├── TransactionTable.tsx        # 交易记录表格
    ├── TransactionForm.tsx         # 交易记录新增/编辑表单
    ├── StatCard.tsx                # 统计卡片通用组件
    └── ErrorBoundary.tsx           # 错误边界组件
```

## API 接口（后台 Bun + Hono :8765，通过 Vite proxy 转发）

| 端点 | 方法 | 说明 |
|------|------|------|
| `GET /api/portfolio` | GET | 组合统计：总交易数、买卖金额、盈亏、定投/手动分离 |
| `GET /api/funds` | GET | 所有基金列表：代码、名称、类型、持仓、盈亏 |
| `GET /api/funds/:code` | GET | 单只基金详情 + 全部交易记录 |
| `GET /api/funds/:code/nav` | GET | 净值历史（最近3年，日频） |
| `GET /api/funds/:code/pnl-distribution` | GET | 单只基金盈亏分布数据 |
| `GET /api/nasdaq/overview` | GET | 纳斯达克指数实时概览与历史走势 |
| `GET /api/benchmark/dca` | GET | DCA 定投回测模拟数据 |
| `POST /api/transactions` | POST | 新增交易记录 |
| `PUT /api/transactions/:seq` | PUT | 更新指定交易记录 |
| `DELETE /api/transactions/:seq` | DELETE | 删除指定交易记录 |

### 关键类型

```ts
interface FundInfo {
  code: string; name: string; type: string
  held_shares: number; current_value: number | null
  unrealized_pnl: number | null; pnl_pct: number | null; latest_nav: number | null
}

interface FundDetail {
  code: string; name: string
  held_shares: number; total_cost: number
  latest_nav: number | null; current_value: number | null
  unrealized_pnl: number | null; pnl_pct: number | null
  auto_buy_count: number; manual_buy_count: number
  auto_buy_amount: number; manual_buy_amount: number
  auto_tx: number; manual_tx: number
  buy_count: number; sell_count: number
  median_settlement: number
  transactions: Transaction[]
}

interface Transaction {
  seq: number; trade_time: string; confirm_date: string
  trade_type: string; direction: string  // direction: 'buy'|'sell'|'dividend'|'convert_in'|'convert_out'|'forced_redeem'
  amount: number; shares: number; fee: number
  nav: number | null; pnl: number | null
  trade_day_type: string; settlement_days: number | null
}

interface NavPoint {
  date: string; unit_nav: number; daily_change_pct: number
}

interface Portfolio {
  total_tx: number; unique_funds: number; held_funds: number
  total_buy: number; total_sell: number; total_fee: number
  unrealized_pnl: number
  auto_tx: number; manual_tx: number
  auto_amount: number; manual_amount: number
  first_trade: string; last_trade: string
  settlement_distribution: Record<string, number>
  trade_type_breakdown: Record<string, number>
}
```

## 功能需求

1. **组合总览页**：统计卡片（累计买入/卖出/盈亏/手续费/定投/手动）、持仓柱状图、定投 vs 手动饼图、Top 10 市值柱状图、持仓列表表格
2. **基金详情页**：统计卡片（持有份额/成本/净值/市值/盈亏/定投手动比）、净值走势图（标注买入点绿色、卖出点红色）、定投 vs 手动饼图、累计成本 vs 估算市值曲线、完整交易记录表
3. **左侧栏**：按类型分组（QDII/指数/混合/股票/债券/存单/黄金/货币/其他）、每只基金显示盈亏金额、已清仓基金半透明显示

## 设计要求

- 浅色主题，现代简洁
- 全中文
- 卡片式布局，微阴影
- 颜色：蓝 #4b8ef1 / 绿 #22b07d / 红 #e8556a / 琥珀 #f0a040
- 所有 ECharts 图表配色与 CSS 统一

## 开发命令

```bash
# 安装依赖
npm install

# 启动开发服务器
npm run dev

# 类型检查
npm run typecheck

# 运行测试
npm run test

# 监听模式运行测试
npm run test:watch

# 构建生产版本
npm run build

# 预览生产构建
npm run preview
```
