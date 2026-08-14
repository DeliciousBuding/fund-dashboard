# DESIGN.md — fund-dashboard UI Design System

> 最后更新：2026-07-20  
> 状态：Wave 5 design-system residual 已对齐（#103–#149）；**2026-07-20 high-DPI**：图表 `devicePixelRatio`、字号微提、viewport-fit=cover、≥2x glass blur  
> 实现源：`packages/web/src/styles/theme.ts` + `packages/web/src/index.css` + `packages/web/src/hooks/useEChart.ts` + `docs/charts-design-system.md` + Kumo v2.5  
> 参考：
> - [Vercel Web Interface Guidelines](https://vercel.com/design/guidelines)
> - [Geist Design System](https://vercel.com/geist/introduction)
> - [Vercel Design hub](https://vercel.com/design)

本文件是 **agent 与人类共享的 UI SSOT**。改视觉/交互先改本文件或 `theme.ts`，禁止在组件内散落字面量颜色/间距。

---

## 1. 产品定位与视觉原则

| 原则 | 含义 |
|------|------|
| Facts-first | 数字、状态、风险提示优先于装饰 |
| CN 市场语义 | **红涨绿跌**（硬约束）：`up=#d63649`，`down=#199c63` |
| Token-first | 颜色/阴影/边框/间距/字号走 `theme.ts` 或 `--fd-*` CSS 变量 |
| All states | 每个视图设计 loading / empty / sparse / dense / error |
| A11y default | 语义 HTML 优先；焦点可见；不只靠颜色传达状态 |
| Motion restraint | 仅在解释因果时动画；尊重 `prefers-reduced-motion` |

Geist 启发（非 1:1 复制）：

- 高对比表面层级（canvas → surface → hover）
- 半透明边框 + 分层阴影
- 数据用 tabular nums
- 交互态提高对比，而不是只变色

---

## 2. Token 表（与 `theme.ts` / `index.css` 对齐）

### 2.1 Surface / Text

| Token | Light | Dark | 用途 |
|-------|-------|------|------|
| `canvas` | `#f8fafc` | `#0b0f17` | 页面底 |
| `surface` | `#ffffff` | `#131922` | 卡片/面板 |
| `surfaceHover` | `#f1f5f9` | `#1c2433` | hover / skeleton base |
| `border` | `#e2e8f0` | `rgba(255,255,255,0.08)` | 主边框 |
| `borderSubtle` | `#f1f5f9` | `rgba(255,255,255,0.05)` | 细分隔 / skeleton wash |
| `text` | `#0f172a` | `#e5e7eb` | 主文案 |
| `textSubtle` | `#475569` | `#9ca3af` | 次文案 |
| `textMuted` | `#94a3b8` | `#64748b` | 轴标签/提示 |
| `onAccent` | `#ffffff` | `#0b0f17` | 实心 accent chip 上的字/图标 |
| `markerBorder` | `#ffffff` | `#ffffff` | scatter/marker 描边 |

### 2.2 Semantic accents

| Token | Light | Dark | 语义 |
|-------|-------|------|------|
| `up` | `#d63649` | `#f87171` | 涨 / 盈利（**仅**收益语义） |
| `down` | `#199c63` | `#4ade80` | 跌 / 亏损 |
| `critical` | `#d63649` | `#f87171` | 失败 / 校验 / OfflineBanner（**禁止**用 `up` 表示错误） |
| `blue` | `#3172d9` | `#4dabf7` | 主系列 / brand |
| `amber` | `#e07b2c` | `#fbbf24` | 警告 / MA |
| `violet` | `#8b5cf6` | `#a78bfa` | 系列 |
| `cyan` | `#06b6d4` | `#22d3ee` | 系列 |

CSS 镜像：`--fd-color-up` / `--fd-color-down` / `--fd-color-critical` / `--fd-color-blue`（`[data-theme|data-mode=dark]` 与 `prefers-color-scheme` 同步）。

**禁止**在组件中写 `#d63649` 等字面量。`usChangeColor` 已走 `getTheme`。错误/校验色必须用 `theme.critical`（#107），不得复用 `theme.up`。

### 2.3 Elevation / Glass

| Token | Light | Dark |
|-------|-------|------|
| `shadowCard` | soft dual-layer | soft dark |
| `shadowHover` | elevated | elevated dark |
| `glassSurface` | `rgba(255,255,255,0.72)` | `rgba(19,25,34,0.62)` |
| `glassBorder` | white translucent | white 10% |
| `glassBlur` | `blur(28px) saturate(185%)` regular | `blur(30px) saturate(165%)` |
| `glass materials` | `ultraThin` / `regular` / `thick` via `glassMaterial()` + Card `material` | same |
| ambient | `ambientCanvas` + orbs under AppLayout main | same |
| `glassHighlight` | top-edge sheen gradient | subtle top sheen |
| `glassShadow` | soft + inset highlight | soft + inset |

**单一磨砂路径（#103–#104，已闭合）：**

1. `Card` prop `glass` — 默认 frosted shell（StatCard / ChartShell / Overview 子卡）。
2. `glassSurfaceStyle(t, { borderRadius? })` — chips / toast / nav / 非 Card 表面；**禁止**再分叉第二套 glass CSS。
3. CSS vars：`--fd-glass-surface` / `--fd-glass-border` / `--fd-glass-blur` / `--fd-glass-shadow`（`data-theme` 与 `data-mode` 均生效）。
4. 材质档：`glassMaterial(t, ultraThin|regular|thick)`；`prefers-reduced-transparency` → 实心 surface。
5. Ambient：`ambientCanvasStyle(t)` 挂在 AppLayout main，让 blur 有景可取。
4. 图表强调阴影走 `chartShadowColor(t, alpha?)`，不硬编码 black（#111）。

### 2.4 Spacing / radius / type / hit targets / z-index

`theme.ts` 导出数值 scale；`index.css` 镜像为 `--fd-*`：

| Scale | Keys / values | CSS |
|-------|---------------|-----|
| `space` | `1=4 … 8=60`（4,8,12,16,24,32,48,60） | `--fd-space-1` … `--fd-space-8` |
| `radius` | `none=0` `xs=2` `sm=6` `md=10` `lg=14` | `--fd-radius-none/xs/sm/md/lg`（`none`/`xs` #133） |
| `fontSize` | `xs=11` `sm=12` `md=13` `base=14` `lg=15` `xl=16` `2xl=18` `3xl=20` `4xl=30`（high-DPI 微提；小端 +1） | `--fd-font-xs` … `--fd-font-4xl` |
| `fontWeight` | `regular=400` `medium=500` `semibold=600` `bold=700` | `--fd-fw-regular` … `--fd-fw-bold`（#120 / residual #123） |
| `lineHeight` | `none=1` `badge=16px` | `--fd-lh-none` / `--fd-lh-badge`（#127） |
| `letterSpacing` | `tight=-0.02em` `wide=0.3`（px） | `--fd-ls-tight` / `--fd-ls-wide`（#127） |
| `duration` | `fast=150` `normal=200` `slow=300`（ms） | `--fd-duration-fast/normal/slow`（#129） |
| `easing` | `standard=ease` `inOut=ease-in-out` | 与 `cssTransition()` 组合（#129） |
| `opacity` | `disabled=0.4` `muted=0.5` `soft=0.55` `seriesSoft=0.6` `seriesStrong=0.9` `solid=1` | `--fd-opacity-disabled/muted/soft/series-soft/series-strong/solid`（#145） |
| `layout` | `mobileNavHeight=56` | `--fd-layout-mobile-nav-height`（#147） |
| `skeleton` | `barH.sm/md/lg=10/14/22`；`barW.xs…xl=72/100/140/160/220`；`statMinH=80`；`chartH=420` | `--fd-skeleton-bar-h-*` / `--fd-skeleton-bar-w-*` / `--fd-skeleton-stat-min-h` / `--fd-skeleton-chart-h`（#148） |
| `hitTarget` | `min=28` `mobile=44` | `--fd-hit-min` / `--fd-hit-mobile` |
| `zIndex` | `base=0` `local=1` `dropdown=100` `sticky=200` `modal=1000` `toast=2000` `banner=9999` `skip=10000` | `--fd-z-base` … `--fd-z-skip`（#119；`local` #124） |

规则（#106 / #112–#115 / #118–#120 / #123–#124 / #126–#127 / #129 / #133 / #145 / #147 / #148 residual 已闭合）：

- 有意义的 margin/padding/gap 用 `space[]` 或 `--fd-space-*`；禁止组件内 magic spacing。
- 字号用 `fontSize.*` 或 `--fd-font-*`；SPA 内联 fontSize 字面量 ≈ 0。
- 字重用 `fontWeight.*` 或 `--fd-fw-*`；禁止组件内 raw `400/500/600/700`（#120 高频 + #123 residual）。
- 行高/字距用 `lineHeight.*` / `letterSpacing.*` 或 `--fd-lh-*` / `--fd-ls-*`；禁止组件内 raw lineHeight/letterSpacing（#127）。
- 过渡用 `duration.*` / `easing.*` 或 `cssTransition(props, opts)` / `--fd-duration-*`；禁止组件内 raw `.15s/.2s/.3s` 或 `transition: all`（#129）。
- 圆角用 `radius.*` 或 `--fd-radius-*`；Card 默认 `radius.lg`；方/微圆角用 `radius.none` / `radius.xs`（#133 residual App/MarketTicker/Sidebar/Penetration）。
- 共用 chrome/series 透明度用 `opacity.*` 或 `--fd-opacity-*`（disabled/muted/soft/seriesSoft/seriesStrong/solid；#145 residual MarketTicker/Harness/TransactionTable/FundChart/Nasdaq）；图表单次强调（如 PnL bar 0.85）可留本地。
- 固定壳尺寸用 `layout.*` 或 `--fd-layout-*`（#147 residual App 移动底栏 `mobileNavHeight`）；禁止组件内 raw chrome height。
- 共享 skeleton 条宽高用 `skeleton.barH.*` / `skeleton.barW.*` / `skeleton.statMinH` / `skeleton.chartH / chartHeight.default (#162)` 或 `--fd-skeleton-*`（#148 residual ChartFallback/PageFallback）；禁止 loading 壳内 raw bar width/height。
- 固定层叠用 `zIndex.*` / `--fd-z-*`（skip-link > banner > toast > modal > sticky > dropdown）。
- 父级定位内的局部层叠用 `zIndex.local`（如 Card 内容盖在 glass highlight 上，#124）；禁止裸 `zIndex: 1`。
- 非 submit 按钮显式 `type="button"`（#126 residual Harness/MarketTicker）。
- 装饰性 logo/图标：`alt=""` + `aria-hidden`（#145 residual Sidebar ndaq 图标）；有意义图片保留描述性 alt。

### 2.4a High-DPI / Retina（2026-07-20）

| 面 | 约定 |
|----|------|
| Viewport | `index.html`：`width=device-width, initial-scale=1.0, viewport-fit=cover`（配合已有 `env(safe-area-inset-*)`） |
| 图表 canvas | `useEChart` → `echarts.init(el, undefined, { devicePixelRatio: min(dpr, 3), renderer: 'canvas' })`；`matchMedia('(resolution: …dppx)')` 变化时 dispose+re-init |
| 字号 | 小端 +1px（xs…lg）；chartAxis/tooltip/legend/dataZoom 走 `fontSize.*` token，禁止字面量 10/11/12 |
| 命中 | `hitTarget.min=28`（桌面/触控笔记本）；mobile 仍 44 |
| Glass ≥2x | `@media (min-resolution: 2dppx)` 略加强 `--fd-glass-blur`；`prefers-reduced-transparency` 仍强制实心 |

不改 rem 根字号、不做全站 rem 重写；保持 px token + CSS vars 双写。

### 2.5 Kumo CSS 变量（布局/表单）

表单、表格、布局优先 `@cloudflare/kumo`：

- `var(--color-kumo-surface)`
- `var(--color-kumo-border)`
- `var(--text-color-kumo-subtle)` 等

图表与数据色走 `theme.ts`；布局 chrome 走 Kumo。Admin 异常表等走 Kumo Table（#111）。

---

## 3. 组件分层

```
pages/            路由页（薄）
components/       领域组件（Portfolio*, Fund*, Admin*, …）
components/ui/    通用壳（Card 等）
components/charts/ 声明式图表语义层（ChartShell, series factories）
components/*Fallback 共享 skeleton 加载壳（ChartFallback / PageFallback）
styles/theme.ts   token + echarts 共享 option
index.css         --fd-* mirrors + focus/skip-link + fd-spin
```

规则：

1. 新图表必须用 `ChartShell` + `useChartData` / `useEChart` + series factories（见 `docs/charts-design-system.md`）。
2. 懒加载图表/页面用 `ChartFallback` / `PageFallback`（#112 / #116），形状贴近最终内容以减 CLS；skeleton 条尺寸走 `skeleton.*`（#148）。
3. 表单/表格优先 Kumo，不原生裸 HTML。
4. 数字比较前 `Number()`；金额/涨跌百分比用 tabular nums（`.fd-tabular-nums`）。
5. 图标按钮必须 `aria-label`；装饰图标 `aria-hidden`；装饰 logo 图 `alt=""` + `aria-hidden`（#145）；非 submit 按钮写 `type="button"`（#126）。

---

## 4. 交互规范（Vercel Web Interface Guidelines 落地）

### 4.1 键盘与焦点

- 所有可操作控件可 Tab 到达
- 使用 `:focus-visible`（不要去掉轮廓）；全局 outline 走 accent/blue
- 对话框/抽屉：焦点陷阱 + 关闭后焦点归还
- 命中区域 ≥ `hitTarget.min`（24px）；移动 ≥ `hitTarget.mobile`（44px）
- skip-link：`.fd-skip-link`（键盘可见）

### 4.2 导航与 URL

- 路由用 `<Link>` / `<a>`，不用 div onClick 导航
- 筛选、组合切换、range（1m/3m/1y）应可 deep-link（query 或 path）
- 破坏性操作（删除交易）需确认或可撤销

### 4.3 加载与反馈

- skeleton 形状贴近最终内容，避免 CLS
  - 图表懒加载：`ChartFallback`（glass Card + shimmer；`data-testid="chart-loading"`）
  - 页面懒加载：`PageFallback`（4-up stat skeleton + `ChartFallback`）
- spinner 最短可见 ~300–500ms；过短不闪
- 刷新旋转统一 `.fd-spin`（#117）；**禁止**组件内自写 spin keyframes
- 异步结果用 `aria-live="polite"` 宣告（toast / 导入结果 / fallback shells）
- 按钮 loading：保留标签 + 禁用，防重复提交

### 4.4 表单

- 每个控件有 label；点 label 聚焦控件
- 不要提前禁用 submit；提交后禁用 + 展示校验
- 错误靠近字段；submit 后 focus 第一个错误；错误色 `theme.critical`
- 移动端 input 字体 ≥ 16px 或正确 viewport，防 iOS 缩放
- 未实现后端能力须诚实禁用（如 CSV 导入 #108），禁止假成功路径

### 4.5 动画

- 仅 `transform` / `opacity`
- **禁止** `transition: all`
- 时长/缓动走 `duration` / `easing` 或 `cssTransition()`（#129）；不写 raw `.15s ease`
- 尊重 `prefers-reduced-motion: reduce`：
  - 全局：`index.css` 将 animation/transition 压到近零
  - `.fd-spin`：`animation: none`
  - skeleton shimmer（`ChartFallback`）：关闭动画

### 4.6 性能

- 大表虚拟化或分页
- 图表按需 `useCoreCharts()` 注册
- 图片/字体设尺寸，防 CLS
- 关键路径 bundle 保持 code-split（现有 Vite chunks）

### 4.7 A11y 语义（#109–#110 residual 已闭合）

- Overview 子页签：`tablist` / `tab`
- DCA 周期切换：`aria-pressed`
- 交易表排序：`aria-sort`
- 导出菜单：menu + Escape 关闭 + 焦点管理

---

## 5. 图表语义（摘要）

完整规范见 `docs/charts-design-system.md`。硬约束复述：

| 规则 | 说明 |
|------|------|
| 红涨绿跌 | `theme.up` / `theme.down` only（收益语义） |
| 三态占位 | `chart-loading` / `chart-error` / `chart-empty` testids via `ChartShell` |
| ChartShell 覆盖 | Overview / MonteCarlo / Allocation / Penetration / Compare 等残余已收口（#105） |
| 不静默吞错 | 仅吞 AbortError |
| 共享轴/tooltip | `chartAxis` / `chartTooltip` / `chartLegend` / `chartDataZoom` |
| 阴影 | `chartShadowColor(t)` — 不硬编码 black |
| FundDetail 累计图 | `useEChart`（#109） |

Series 色盲友好顺序：`blue → up → down → amber → violet → cyan`。

---

## 6. 文案与内容

- 标题/按钮：简洁、动作导向；中文为主，技术标识保留英文
- 用户可见 `title` / tooltip：走 `t('…')` i18n keys（zh/en）；禁止组件内硬编码中文 title（#130 residual FundDetail/MarketTicker/TransactionTable）
- 用户可见文案（分组/指数名/交易统计/toast 等）：走 i18n keys（zh/en）；禁止组件内硬编码中文 UI 文案（#132 residual MarketTicker groups/indices + FundDetail transaction stats）
- 交易表 UI（搜索 placeholder/aria、列头、空态、方向/结算徽章展示文案）：走 `fundDetail.txTable.*` / `fundDetail.dir.*` / `tx.*` i18n（#135 residual TransactionTable）；DB `trade_type` 形状匹配（如 `includes('定投')`）可保留，不得用展示文案做数据判断
- 导出文件用户可见表头/方向标签：走 i18n（#136 residual CSV `transactionsToCsv` → `fundDetail.csv.*` + `fundDetail.dir.*`；随 active locale）
- 交易表单 UI（标题/方向/分类/金额份额手续费/校验/提交）：走 `fundDetail.txForm.*` + `fundDetail.dir.*` / `common.cancel`（#138 residual TransactionForm）；DB 写入 `trade_type` 仍用中文常量码（`用户买入`/`定投买入`/`用户卖出`/`定投卖出`），仅 option label 走 i18n
- 资产配置 sunburst 类型/市场标签与 tooltip、图表买卖 marker 默认名：走 `allocation.typeLabels.*` / `allocation.marketLabels.*` / `allocation.sunburstTooltip` + `fundDetail.dir.*`（#139 residual PortfolioAllocation + tradeMarkers）；禁止组件内硬编码中文图例/tooltip
- 分类/市场展示标签：`CATS`/`STOCK_CATS`/`STOCK_MARKETS` 存 `nameKey`/`labelKey`，Sidebar 等渲染走 `t()`（#141 residual classify display）；`SECTOR_NAMES` 存 `penetration.sectors.*` i18n key，Penetration 渲染走 `t()`（#142 residual sector display）
- 语言切换控件：目标语标签走 `nav.switchToEn` / `nav.switchToZh`（#144 residual LanguageSwitcher）；zh+en 目录同文，命名「切换到的语言」而非当前语言
- **intentional residual**：`CATS.funds` 中文关键词、`classifySector` 中文 regex/关键词等 **data dictionary** 用于匹配东财/DB 中文基金名与股票名——**不得**误当成 UI 文案 i18n 化；分类逻辑依赖中文语料
- 错误信息：说明发生了什么 + 下一步（刷新/检查权限）
- 空态：说明原因 + 可选 CTA（如「添加交易」）
- 品牌名/代码：`translate="no"`（基金代码、MCP tool 名）

---

## 7. 主题切换

- 默认跟随 `prefers-color-scheme`
- light/dark 等价打磨（不可 dark 只是反色）
- 主题属性：`data-theme` **与** `data-mode` 均驱动 CSS 变量（#104）
- 切换时图表 `setOption(..., { notMerge: true })` 清残影（见 `useEChart`）

---

## 8. 差距清单

| ID | 差距 | 严重度 | 状态 |
|----|------|--------|------|
| G1 | 缺少本 DESIGN.md 作为 agent SSOT | P1 | ✅ 落地 |
| G2 | 统一 spacing/radius/type/zIndex/fontWeight/lineHeight/motion/opacity/layout/skeleton scale | P2 | ✅ `theme.space/radius(none…lg)/fontSize/fontWeight/lineHeight/letterSpacing/duration/easing/opacity/layout/skeleton/zIndex` + `--fd-*`（#106/#112–#115/#118–#120/#123–#124/#127/#129/#133/#145/#147/#148） |
| G3 | `usChangeColor` 硬编码 hex | P2 | ✅ 走 `getTheme` |
| G4 | deep-link 状态不完整（range/tabs/portfolio） | P2 | ✅ `useQueryRange` + `usePortfolioDeepLink` + codes/tab/detailTab |
| G5 | focus-visible / skip-link / reduced-motion | P2 | ✅ skip-link + `:focus-visible` + `fd-spin`/shimmer reduced-motion（#117） |
| G6 | 读 API 公网无 OIDC | P2 | **产品门控** — 见 HANDOFF；**不**在本 wave 打开 |
| G7 | 双 glass CSS 分叉 / Overview 实心卡 | P2 | ✅ 单一 `glassSurfaceStyle` + Card glass（#103–#104） |
| G8 | ChartShell 三态残余（MonteCarlo/Allocation/…） | P2 | ✅ 收口 ChartShell（#105） |
| G9 | 错误色误用 `theme.up` | P2 | ✅ `theme.critical`（#107） |
| G10 | 加载态 CLS / 自写 spinner | P2 | ✅ `ChartFallback`/`PageFallback` + `.fd-spin`（#112/#116/#117） |

Wave 5 residual design 项（#103–#149；含 motion #129 / i18n title #130 / residual Chinese #132/#135/#138/#139/#141/#142/#144 / CSV export i18n #136 / borderRadius #133 / opacity + logo alt #145 / layout chrome #147 / skeleton size #148 / SSOT #131/#134/#137/#140/#143/#146/#149）视为 **已闭合**。仅 G6 仍为产品决策门控。中文 keyword/data dictionaries（基金名/行业名匹配）与图表单次 opacity 为 intentional residual，非 open UI 债。

---

## 9. 验收检查表（UI 变更 PR）

- [ ] 未新增颜色字面量（除 `theme.ts` / 有意的 CSS token 镜像）
- [ ] 间距/字号/字重/行高字距/时长缓动/透明度/圆角/层叠/布局壳/skeleton 走 `space` / `fontSize` / `fontWeight` / `lineHeight` / `letterSpacing` / `duration` / `easing` / `opacity` / `radius` / `zIndex` / `layout` / `skeleton` 或 `--fd-*`；非 submit 按钮 `type="button"`；禁止 raw borderRadius / 共用 chrome opacity / layout height / skeleton bar 字面量（图表单次强调可本地）
- [ ] 用户可见 `title`/tooltip/文案/导出表头/表单标签/图例 marker/分类·行业展示名/语言切换标签走 i18n（无硬编码中文 title、UI 文案、CSV 表头或 chart label）；keyword 数据字典中文匹配除外；装饰 logo `alt=""` + `aria-hidden`
- [ ] 错误/校验用 `critical`，不用 `up`
- [ ] glass 走 `Card glass` 或 `glassSurfaceStyle`，无第二套 glass
- [ ] light + dark 各截一张（或说明无法截图原因）
- [ ] loading/empty/error 可见（图表走 ChartShell testids）
- [ ] 键盘可达 + focus-visible；新动画尊重 reduced-motion；transition 走 token 不写 `all`
- [ ] 红涨绿跌正确
- [ ] `npm test`（web）通过

---

## 10. 与后端/生产边界

- UI 不持有 `MCP_API_KEY` / `FUND_EDGE_KEY`
- 写路径依赖 nginx EdgeKey；失败时诚实展示错误，不静默
- Admin 面板只展示 facts；不提供不可逆运维按钮除非二次确认
- AgentOps #5 **已启用**（operator prepare/consume）；SPA **不**持 MCP key；无 OIDC 公网读面变更（G6 产品门控）

---

*Sources: [Vercel Web Interface Guidelines](https://vercel.com/design/guidelines), [Geist](https://vercel.com/geist/introduction), [Vercel Design](https://vercel.com/design).*
