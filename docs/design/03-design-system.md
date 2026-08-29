# 03 — 设计系统「静水流深 Quiet Capital」

> 定档日期：2026-08-29 · 状态：已定档
> 参考坐标：Linear 的克制 × Stripe 的精致 × Bloomberg 的密度。
> 金融产品的正确情绪：**数字是主角，界面退后**。高密度但不吵，精确、克制、有质感。

## 1. 设计原则

1. **数字优先**：一切金额/百分比用等宽数字（tabular-nums），千分位，币种符号前置；涨跌必带方向色与符号。
2. **一层信息一层灰**：背景、表面、边框只做明度阶梯，不做色相杂耍；颜色只留给语义（涨跌/警告/品牌点缀）。
3. **暗色是默认**：盯盘场景暗色护眼；亮色是同等待遇的一等公民，不是反转滤镜。
4. **状态诚实**：数据陈旧有徽章、加载有骨架、为空有引导、出错有重试——永远不让用户猜。
5. **快是设计的一部分**：首屏 < 1.5s（本地），交互反馈 < 100ms，动效只加速感知不增加等待。

## 2. 色彩（OKLCH，CSS 变量，Tailwind v4 `@theme` 直挂）

### 中性阶梯（暗色默认）
| token | 值 | 用途 |
|-------|----|------|
| `--bg` | `oklch(0.16 0.012 255)` | 页面底（延续旧 PWA #0a0e14 气质） |
| `--surface-1` | `oklch(0.19 0.014 255)` | 卡片 |
| `--surface-2` | `oklch(0.22 0.015 255)` | 浮层/输入框 |
| `--surface-3` | `oklch(0.26 0.016 255)` | hover/按下 |
| `--border` | `oklch(0.30 0.015 255 / 0.6)` | 分隔线（暗色用边框而非投影分层） |
| `--fg` | `oklch(0.93 0.01 250)` | 主文字 |
| `--fg-2` | `oklch(0.70 0.015 250)` | 次级 |
| `--fg-3` | `oklch(0.52 0.015 250)` | 弱化/占位 |

亮色模式：同阶梯反转（`--bg oklch(0.98 …)` → `--fg oklch(0.20 …)`），边框透明度上调。

### 语义色
| token | 暗色值 | 语义 |
|-------|--------|------|
| `--accent` | `oklch(0.78 0.11 85)` | **品牌金**：主按钮、激活态、焦点环、图表主线 |
| `--up` | `oklch(0.66 0.19 25)` | 涨（中式=红） |
| `--down` | `oklch(0.70 0.15 160)` | 跌（中式=绿） |
| `--warn` | `oklch(0.75 0.14 75)` | 数据陈旧/阈值告警 |
| `--danger` | `oklch(0.63 0.20 25)` | 破坏性操作（删除） |
| `--info` | `oklch(0.72 0.11 220)` | 信息提示 |

- **涨跌色约定可切换**：中式（涨红跌绿，默认）⇄ 西式（涨绿跌红），设置页一键换，`--up/--down` 变量互换，全站即时生效（图表主题同步换）。
- 语义色严禁用作装饰；品牌金严禁表达涨跌。
- 对比度硬指标：正文 ≥ 4.5:1，大数字 ≥ 3:1，图表元素间可区分（非仅颜色编码，辅以形状/虚线）。

### 图表色板（10 类，暗色调优）
以 `--accent` 金为系列 1，其余按色相环 36° 等步长取 OKLCH(0.72, 0.10–0.14, h)：
`gold / cyan / violet / rose / emerald / orange / blue / lime / pink / teal`。
亮色模式明度 -0.12。色板入库为 `charts/palette.ts`，全站（图表、市场 badge、主题色）唯一出处。

## 3. 字体

| 项 | 定档 |
|----|------|
| 拉丁/数字 | **Inter Variable**（woff2 自托管，`web/public/fonts`，`font-display: swap`） |
| 中文 | 系统栈：`"PingFang SC","HarmonyOS Sans SC","Microsoft YaHei","Noto Sans CJK SC",sans-serif`（不自托管中文字体，体积不划算） |
| 数字特性 | 一切数据：`font-variant-numeric: tabular-nums`；Inter `cv11`（单故事 a 关闭）保持几何感 |
| 字阶 | 12 / 13 / 14（正文）/ 16 / 20 / 28 / 36（hero 数字）/ 48（仅登录与空态） |
| 字重 | 400 正文 · 500 强调 · 600 标题；禁 700+ |
| 行高 | 正文 1.55；数字行 1.2；标题 1.25 |

Hero 数字（总资产/XIRR）用 36px/500 + tabular-nums，货币符号 60% 缩小前置，涨跌 chip 右对齐——**数字列永远右对齐，文本列永远左对齐**。

## 4. 栅格、间距、圆角、层级

- 4pt 基网；常用步进 4/8/12/16/24/32/48。
- 布局：左侧栏 240px（折叠 64px）+ 内容 max-w 1440 居中；卡片 padding 20（紧凑密度 12）。
- 圆角：控件 8，卡片 12，浮层 14；全站三档，不发明新值。
- 暗色层级 = 表面明度 + 1px `--border`；投影仅在浮层（`0 8px 30px rgb(0 0 0 / 0.35)`）。
- 密度切换：舒适（默认）/ 紧凑（表格行高 44→32，卡片 padding 20→12），设置页持久化，CSS `data-density` 属性驱动。

## 5. 动效（motion 库；全部可被 prefers-reduced-motion 关闭）

| 场景 | 参数 |
|------|------|
| 页面进入 | fade + y:4→0，160ms，`cubic-bezier(0.2,0.8,0.2,1)` |
| Tab 指示器 | layoutId 弹簧（stiffness 500 / damping 40） |
| 数字变化 | count-up 400ms easeOut；**涨跌闪烁**：值变更瞬间底色 600ms 淡出（涨 `--up/15%`，跌 `--down/15%`） |
| 骨架 | shimmer 1.2s 线性循环 |
| 弹层 | scale 0.98→1 + fade，120ms |
| 图表 | 初始 600ms 展开；**数据更新不重演动画**（notMerge 更新，继承旧 useEChart 纪律） |

铁律：动效时长 ≤ 400ms；不动画 layout 属性（只 transform/opacity）；SSE 推送的指数 ticker 更新**无动画**（防视觉噪音）。

## 6. 图表主题（ECharts 全站主题对象 `charts/theme.ts`）

- 背景透明；网格线 `--border`；轴标签 `--fg-3` 12px；分割线虚线。
- NAV/净值线：2px `--accent` 主线 + 面积渐变（accent 12%→0%）；成本线虚线 `--fg-3`；**买卖点**：effectScatter，买=▲ `--up`、卖=▼ `--down`（随约定切换）。
- sunburst/treemap：走图表色板，hover 提亮 8%，标签 12px 截断。
- 热力图（相关性）：发散色阶 `--down → surface → --up`，零点中性。
- tooltip：surface-2 底 + 边框 + 12px，数字 tabular，涨跌着色，**不投影**。
- 空数据：不渲染图表，渲染 EmptyState（图表区同尺寸骨架）。

## 7. 状态设计（每个面板必须四态俱全）

| 态 | 规格 |
|----|------|
| loading | 骨架与最终布局**同形同位**（KPI 卡=灰条，图表=灰框，表格=灰行），禁 spinner 居中 |
| empty | 一句话说清「为什么没有」+ 一个主行动按钮（如「导入交易」）；线性 SVG 插图，禁 emoji |
| error | 面板级 ErrorBoundary：图标 + 稳定短错误码（后端 `{"error":code}` 直显）+ 重试按钮；崩一面板不崩全页 |
| stale | 数据陈旧徽章：`数据截至 MM-DD · N 天前`（warn ≥4 天 / critical ≥7 天），点击触发刷新——复用 `/api/admin/freshness` 语义 |

离线：顶条横幅（旧 OfflineBanner 方案：恢复时 invalidateQueries + toast）。chunk 加载失败：自动重载（60s 节流，旧方案继承）。

## 8. 交互速查

- **⌘K / Ctrl+K** 命令面板：跳页面、跳标的（名称/代码模糊搜，走 `/api/securities`）、动作（新增交易/触发刷新/生成报告）。
- 快捷键：`g o` 总览 · `g h` 持仓 · `g t` 交易 · `g a` 分析 · `g ,` 设置；`?` 打开快捷键表。
- 全局焦点环：2px `--accent` outline-offset 2；键盘可达性 = 交付门槛（tablist 方向键/Home/End，继承旧实现）。
- 表格：列宽可拖、排序状态进 URL、行 hover surface-3；移动端降级为卡片流。

## 9. 移动端

- 断点 768px：侧栏→抽屉 + 底部固定导航（总览/持仓/交易/我的），`safe-area-inset` 适配，点击目标 ≥ 28px。
- 图表横屏不强制旋转；长图区域横向滚动。
- `viewport-fit=cover`，禁用输入框聚焦自动放大（input font-size ≥ 16px）。

## 10. 落地物

- `web/src/styles/tokens.css`：本文件全部 token 的 CSS 变量实现（暗/亮/密度三轴）。
- `web/src/components/charts/theme.ts` + `palette.ts`：图表主题唯一出处。
- `/_design` 路由（dev-only）：组件目录页——每个组件全状态（默认/hover/disabled/loading/error/empty）一页看全，是 UI 评审与回归的目视基线。
