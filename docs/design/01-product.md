# 01 — 产品方向

> 定档日期：2026-08-29 · 状态：已定档
> 一句话：**一道密码门后的私人「持仓 OS」——人走 Web UI，AI agent 走 MCP，同一份数据。**

## 1. 定位

单租户个人投资组合中枢。不是 SaaS、不是多用户平台、不是行情软件。
两个用户角色：

| 角色 | 入口 | 鉴权 |
|------|------|------|
| 我（唯一人类用户） | Web UI（浏览器/PWA） | 登录密码 + session cookie |
| AI agent（Claude / Codex 等） | MCP `POST /mcp` | Bearer key（operator/analyst 双 scope，现状保留） |

产品哲学：**数据是我的，界面是安静的，agent 是一等公民**。
后端已有的 agentops 确认流（写操作必须 confirmation_id + token）是这个产品最独特的资产——UI 与 MCP 共享同一套 service 层，AI 能做的事人不能做得更少，反之亦然。

## 2. 五大支柱

| 支柱 | 回答的问题 | 已有后端能力（测绘确认） |
|------|-----------|------------------------|
| **看** Overview | 我现在有多少钱？变了多少？ | summary / timeline / allocation / XIRR / 指数行情 / 汇率 / 新鲜度 |
| **记** Ledger | 钱从哪来到哪去？ | 交易 CRUD、5000 行幂等导入、XLSX 导出、持仓份额调整（adjust_position） |
| **析** Analysis | 我的决策质量如何？ | 回撤 / 对比（≤8 只，XIRR·波动率·Sharpe·Calmar）/ 四策略回测 / 持仓穿透 |
| **投** Automation | 接下来怎么投？ | DCA 计划 upsert/停用/list（无硬删除）、15 档智能加倍、工作日自动执行（幂等 order_id）、check_alerts |
| **问** Agent | AI 能替我做什么？ | 44 个 MCP 工具 + 上下文包 + 写确认流（operator/analyst 双 scope） |

## 3. 信息架构（路由定档）

```
/login            登录（未初始化时重定向 /setup）
/setup            首次启动设密码（仅未初始化可访问）
/                 总览 Overview
/holdings         持仓列表（全部头寸，可搜可排）
/holdings/$code   单标的详情
/transactions     交易台账（含导入/导出）
/analysis         分析套件（嵌套子路由）
  ├─ /analysis/compare      基金对比
  ├─ /analysis/backtest     策略回测
  ├─ /analysis/advanced     相关性热力图 + 蒙特卡洛
  └─ /analysis/penetration  持仓穿透
/dca              定投计划
/market           市场（指数看板 + 纳斯达克透视，继承旧 /nasdaq）
/insights         信号与事件（source events + 告警 + 数据新鲜度）
/reports          报告（生成 + 查看 + 导出）
/settings         设置（账户安全 / 偏好 / 数据 / 系统 / 关于）
/_design          设计系统目录（仅 dev build 注册）
```

深链纪律（继承旧前端优点）：tab、时间区间、组合、选中标的**全部进 URL search params**（TanStack Router 类型安全校验），任何视图可分享、可后退。

## 4. 页面规格（与后端 API 映射）

### `/` 总览
- 顶条：指数 ticker（SSE `/api/market/stream`，指数退避重连）+ 汇率 badge + **数据新鲜度条**（warn/critical 分级，一键刷新五个查询）
- Hero KPI：当前市值（大卡）· 未实现盈亏+% · 组合 XIRR · 持有标的数；次级 4up：总买/总卖/定投/手动
- 组合净值曲线（value/cost/PnL 三线，区间切换 1M/3M/6M/1Y/ALL）
- 配置 sunburst（类型/市场/主题三视角切换）+ 涨跌贡献榜（topGainer/topLoser chip）
- 盈亏分布直方图（继承旧 pnl_dist：半开区间分桶语义与测试随迁）
- API：`/api/portfolio/`、`/portfolio/timeline`、`/portfolio/xirr`、`/portfolio/allocation`、`/api/market/*`

### `/holdings` + `/holdings/$code`
- 列表：持仓表格（名称/代码/市值/成本/盈亏/盈亏%/占比/陈旧度），列可排序，移动端折叠成卡片
- 详情四 tab（继承旧设计并升级）：
  - **走势**：NAV 线 + 平均成本 markLine + 买卖点标记（买红卖绿，随涨跌色约定反转）
  - **定投**：智能金额模拟器（nav_deviation/change_pct 双模式 + 15 档信号解释）+ 回测小图
  - **概览**：XIRR / 最大回撤 / 成本 vs 市值曲线 / 申赎状态
  - **交易**：该标的流水（搜索、定投↔手动 toggle、编辑、删除确认）
- API：`/api/funds/{code}`、`/nav`、`/xirr`、`/drawdown`、`/dca`；股票走 `/api/stocks/{code}`

### `/transactions`
- 全量台账（TanStack Table：虚拟滚动、多列排序、类型/方向/标的过滤、全文搜索）
- 行内编辑 + 新增表单（买/卖/分红，金额/份额/手续费）+ 删除二次确认
- 持仓份额调整（adjust_position）与标的入册/编辑/删除——**W4 已交付 SessionAuth REST 端点**
- 导入（JSON 批量，幂等）+ 导出（CSV 客户端生成：表头/方向标签随 i18n、带 BOM 头兼容 Excel——继承旧 transactionsToCsv 语义；XLSX 走 `/api/export/transactions-xlsx`）

### `/analysis/*`
- **compare**：`?codes=` 多选 ≤8 → 归一化净值线 + 指标表（XIRR/波动率/Sharpe/回撤/Calmar）+ 雷达图
- **backtest**：四策略（定投/网格/动量/再平衡）参数面板 → 净值曲线 + 单笔对照 + 指标卡
- **advanced**：相关性热力图 + 蒙特卡洛分位扇形——**移植旧前端纯函数**（`services/statistics.ts`、`services/montecarlo.ts`，自带测试），客户端计算，即时交互
- **penetration**：底层股票暴露 treemap（点击钻取）+ 行业聚合表 + 披露覆盖率提示

### `/dca`
- 计划列表（标的/金额/频率/下次执行/累计执行/状态）+ 创建/编辑/停用
- 执行历史台账（dca_plan_executions）+ 手动触发 preview（DryRun）
- W4 已交付 SessionAuth REST 端点：upsert/disable/run

### `/market`
- 指数看板（8 默认符号，CN/HK/US 市场开盘状态着色）
- 纳斯达克透视（旧 /nasdaq 继承）：NDX 历史线 + 全体交易散点叠加 + 统计卡（买/卖次数金额、held/cleared、navPnl）
- 任意指数详情+历史（`/api/market/index/{code}/history` 已支持任意 code）列入 backlog，W5 评估泛化

### `/insights`
- **组合级投资支架卡**（harness 快照：跨标的信号、数据质量评分、推荐动作——字段对齐 MCP `get_investment_harness_snapshot`）
- 来源事件流（已读/有用标记、按标的过滤）+ 告警扫描结果（日涨跌/回撤/陈旧/定投命中四档）
- 数据新鲜度面板（缺失 NAV、陈旧清单、可行动建议）
- W6 已交付 SessionAuth REST 端点：check_alerts

### `/reports`
- 生成组合报告（`generate_report`，JSON v1）→ 富渲染视图 + 下载 JSON——W6 已交付 SessionAuth REST 端点
- PDF 导出列入 backlog（后端无 PDF 能力，前端 print 样式先行）

### `/settings`
- 账户安全：修改密码、活动会话列表（设备/IP/最后活跃）、逐条/全部登出
- 偏好：主题（暗/亮/跟随系统）、**涨跌色约定（中式涨红跌绿 / 西式涨绿跌红）**、密度（舒适/紧凑）、语言（中/英）
- 数据：新鲜度、完整性校验（verify）、DB 体检、备份状态
- Agent 面：44 个 MCP 工具只读清单（scope/权限/确认要求）+ 最近确认与审计事件摘要——W6 已开放 `/api/system/agent` 只读面（SessionAuth）
- 系统：版本、运行状态、API 连通性自检

## 5. 旧前端功能对等表

| 旧路由（已删） | 新归宿 | 处置 |
|---------------|--------|------|
| `/` Overview（6 tab） | `/` + `/analysis/penetration` + `/analysis/advanced` + `/insights` | tab 拆为独立路由，层级更清晰 |
| Overview `harness` tab（投资支架面板） | `/insights` 组合级支架卡 | 平移（字段对齐 MCP `get_investment_harness_snapshot`） |
| Overview `pnl_dist` tab（盈亏分布直方图） | `/` 总览区块 | 平移（半开区间分桶测试随迁） |
| PortfolioSwitcher（多组合切换） | AppShell 组合选择器（顶栏/侧栏） | 平移，状态进 URL `?portfolio=` |
| CSV 导出器（本地化表头 + BOM） | `web/src/lib/` 移植 | 平移（语义与测试随迁） |
| `/compare` | `/analysis/compare` | 平移，加雷达图主题化 |
| `/nasdaq` | `/market` | 平移泛化（指数透视组件） |
| `/fund/:code`（4 tab） | `/holdings/$code` | 平移，路径语义修正 |
| `/admin`（dev-only） | `/settings` 数据/系统区 | 运维信息并入设置，登录后可见 |
| 纯函数服务层 ×9（带测试） | `web/src/services/` | **原样移植**（montecarlo/statistics/irr/format/marketTime 等） |
| e2e ×7 spec | `web/e2e/` | 作为验收基线改写 |
| PWA / 离线横幅 / chunk 重载 | 保留 | vite-plugin-pwa 同方案 |
| 移动端底部导航 / safe-area | 保留升级 | 底栏四项：总览/持仓/交易/我的 |
| i18n 中英 | 保留 | zh 默认，en 次级 |

## 6. 非目标（写死）

- 多用户、注册、邀请、权限矩阵（单租户永远成立）
- 券商对接、真实下单（`broker_trade_execution` 永久 disabled）
- 实时 Level-2 行情、秒级推送（现有 SSE 指数 ticker 足够）
- PG 新增投入（驱动保留，不部署不验证）
- 后端渲染/SSR（SPA + embed 就是终态）
