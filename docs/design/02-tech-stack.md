# 02 — 技术栈与仓库结构

> 定档日期：2026-08-29 · 状态：已定档
> 每个决策给理由与被否选项；不再回头讨论，除非出现新的硬约束。

## 1. 总表

| 层 | 选型 | 版本基线 |
|----|------|---------|
| 前端构建 | Vite | 7.x |
| 视图 | React | 19.x（Compiler 关闭，稳定后再评估） |
| 语言 | TypeScript strict | 5.9+（旧前端已验证 TS 7 兼容，W0 再定是否跟进 tsgo） |
| 样式 | Tailwind CSS | v4（CSS-first `@theme`，零 JS 配置） |
| 组件基座 | shadcn/ui（Radix 原语，深度定制） | 按需引入，源码进仓 |
| 图表 | ECharts | 6.x（`echarts/core` 按需注册） |
| 路由 | TanStack Router | latest（类型安全 search params） |
| 服务端状态 | TanStack Query | v5 |
| 表格 | TanStack Table + Virtual | v8 |
| 表单 | react-hook-form + zod resolver | latest |
| 本地状态 | zustand | v5 |
| 动效 | motion | latest（尊重 prefers-reduced-motion） |
| 命令面板 | cmdk | latest |
| Toast | sonner | latest |
| 图标 | lucide-react | latest |
| i18n | i18next（zh 默认 / en） | latest |
| PWA | vite-plugin-pwa | latest |
| 单测 | vitest + @testing-library/react | latest |
| e2e | Playwright | latest |
| Lint/Format | Biome | 2.x（单工具，含 import 排序） |
| 包管理 | pnpm workspaces | 10.x |
| Node | LTS | 24.x |
| 后端 | Go（现状保留，零框架更换） | 1.26 |
| 后端新增依赖 | `golang.org/x/crypto`（argon2id，现为间接依赖→转正） | — |
| 存储 | SQLite（默认，WAL）/ PG 驱动保留不投入 | — |

## 2. 关键决策与理由

> 编号说明：本节 D2.x 是 [README 决策速览](README.md) D1–D10 的细则展开（D2.1=D1，D2.2=D3，D2.3=D4，D2.4/D2.5=D5，D2.6/D2.7 为落地细则）。对外引用一律用 README 的 D 编号。

### D2.1 前端回归 monorepo（`web/` workspace）
- **理由**：单租户单交付物。`go:embed` 内嵌后仍是一个二进制一个镜像；契约 `packages/contracts` 同仓直引，零发布协调；CI 一条链覆盖前后端。独立仓库方案（2026-08-29 上午的文档声明）从未落地——GitHub 上不存在该仓库，无沉没成本。
- **否**：独立仓库（跨仓 release 协调对单人项目是纯税）；恢复旧路径 `packages/web`（新代码放根级 `web/`——更短，且与已删除的旧前端在命名上切割）。

### D2.2 shadcn/ui + Tailwind v4，而非组件库
- **理由**：设计系统完全自控（token → CSS 变量 → Tailwind theme），Radix 兜底可访问性（focus trap、键盘、aria），源码在仓内可逐像素打磨。「高级现代好看」要求视觉自主权，成品组件库（AntD/MUI）是天花板不是地板。
- **否**：AntD/MUI（企业感、包体大、定制痛苦）；@cloudflare/kumo（旧栈实测，组件覆盖面不足且风格锁定）；纯手写（重复造 a11y 轮子）。

### D2.3 ECharts 6 单图表库
- **理由**：产品线宽（line/bar/sunburst/treemap/heatmap/radar/scatter/effectScatter 全覆盖），`echarts/core` 按需注册控制体积；旧前端的图表生命周期资产（DPR cap 3、ResizeObserver、主题切换 notMerge 不闪、分辨率变化重初始化）直接继承。自研 D3 层是「最 polished 但最贵」，单租户项目不付这个税。
- **否**：lightweight-charts（只强在 K 线，本基金场景无 K 线需求）；Recharts（定制能力弱）；visx/D3（成本）；双图表库并存（体积与心智双税）。

### D2.4 TanStack Router（而非 react-router）
- **理由**：金融仪表盘的状态大半在 URL（tab/区间/组合/选中标的）——Router 的类型安全 search params + zod 校验把旧前端手写的 `useQueryRange`/`usePortfolioDeepLink` 变成框架能力；与 Query 的 loader 预取配合顺滑。
- **否**：react-router v7（能活，但 search param 类型安全要手写）；Next.js（SSR/服务端组件对单租户 SPA 是纯负担，且破坏 go:embed 单二进制模型）。

### D2.5 服务端状态 TanStack Query + zod 契约
- 旧栈已验证（staleTime 5min、fx 10min、AbortController signal 透传、in-flight 去重）。响应体用 `packages/contracts` 的 zod schema 在边界解析——**契约是前后端唯一 SSOT**。

### D2.6 Biome 2 替代 ESLint+Prettier
- **理由**：单工具、Rust 速度、内置 import 排序；agent 生成代码风格一致性由机器保证。
- **风险**：生态边角规则少 → 接受（UI 项目规则需求浅）。

### D2.7 后端零框架变更 + 单一新依赖
- chi/SQLite/手写 MCP 全部保留（生产验证）。唯一新增直接依赖 `golang.org/x/crypto`（argon2id；已是间接依赖，转正在 go.mod 一行）。session/限流手写（<300 行），不引 gorilla/sessions 这类为过时而生的库。

## 3. 仓库结构（定稿）

```
fund-dashboard/
├── cmd/fund-dashboard/         # Go 入口（不变）
├── internal/
│   ├── app  config  httpapi  mcp  service  jobs
│   ├── datasource  agenttools  agentops  audit  confirmations
│   ├── repository/{db,sqlitedb,agentstate}  dialect  contracts  testutil
│   ├── auth/                   # 新增：密码哈希 + session 管理 + 限流
│   └── webui/                  # 新增：go:embed 接线（dist/ 由 web 构建产出）
│       └── dist/index.html     # 占位文件进 git（未构建前端时二进制可编译）
├── web/                        # 新增：前端 workspace（pnpm）
│   ├── src/
│   │   ├── routes/             # TanStack Router（文件式路由）
│   │   ├── components/{ui,charts,layout}
│   │   ├── features/{overview,holdings,transactions,analysis,dca,market,insights,reports,settings,auth}
│   │   ├── lib/                # api client（zod 边界）、queryClient、utils
│   │   ├── services/           # 纯函数（旧仓移植：montecarlo/statistics/irr/format/…）+ 测试
│   │   ├── stores/  i18n/  styles/
│   │   └── main.tsx  router.tsx
│   ├── e2e/                    # Playwright
│   ├── public/{fonts,icons}    # Inter 自托管 woff2
│   └── package.json  vite.config.ts  biome.json
├── packages/contracts/         # zod 契约 SSOT（不变，web 经 workspace 引用）
├── deploy/  scripts/  docs/
├── pnpm-workspace.yaml         # 新增：[web, packages/contracts]
└── package.json                # 根脚本改 pnpm；package-lock.json 删除
```

## 4. 构建与部署

### Dockerfile 三阶段
```
node:24-alpine  → pnpm install --frozen-lockfile && pnpm -C web build
                （outDir 直出 internal/webui/dist）
golang:1.26-alpine → CGO_ENABLED=0 go build（go:embed all:dist 吃真实产物）
alpine:3.20     → 不变（非 root fund 用户、tzdata、healthcheck）
```
- `internal/webui/dist/index.html` 占位文件入库：保证任何时候 `go build` 可编译；Docker 内被真实构建覆盖。
- 生产 compose **零变更**（无 FUND_STATIC_DIR；嵌入即默认）。`FUND_STATIC_DIR` 保留为 dev/调试覆盖。

### 开发流
- `pnpm -C web dev`：Vite dev server :5173，`/api` + `/mcp` proxy → `127.0.0.1:8765`。
- `go run ./cmd/fund-dashboard`：后端照旧。
- 类生产本地验证：`pnpm -C web build && go build ./cmd/fund-dashboard`（嵌入产物）。

### CI 变更
- 新增 `test-web` 门禁：biome check → vitest → tsc --noEmit → vite build。
- `smoke-e2e` 追加断言：`GET /` 返回 HTML 且含 app root 挂载点；`GET /assets/*` 带 immutable 缓存头。
- e2e（Playwright）独立 job，依赖 smoke 镜像，跑登录→总览→交易 CRUD 关键路径。
- 根 `package.json` scripts 改 pnpm 语义；删除 `package-lock.json`，提交 `pnpm-lock.yaml`。

## 5. 仓库整理清单（W0 执行）

| 项 | 处置 | 依据 |
|----|------|------|
| `scripts/package-release.sh` | ✅ 已删除（2026-08-29） | 引用 4 个不存在的 deploy 文件（另有 1 个有 `-f` 保护），set -e 下必然失败；release.yml 未调用 |
| `scripts/_count_mcp_tools.py` | ✅ 已删除（2026-08-29） | 依赖 smoke-prod.sh 产出的 fd-tools.json，该脚本不在本仓 |
| `package-lock.json` | ✅ 已删除（2026-08-29） | 切 pnpm（CI 无 npm 调用） |
| `deploy/.env.example` | **W0 补建** | `deploy/README.md` 引用但文件不存在 |
| 「前端独立仓库」表述 | ✅ 已修（2026-08-29，全仓 9 处：ARCHITECTURE×2、根 README、docs/README、TESTING×3、deploy/README、compose/Dockerfile 注释、progress/MASTER） | 定档 monorepo，独立仓库从未存在 |
| 「Streamable HTTP」表述 | ✅ 已修（2026-08-29，ARCHITECTURE + 根 README） | 实际是无 session 的手写 JSON-RPC POST（spec 兼容修见 05 路线图 W7） |
| `docs/ARCHITECTURE.md`「生产镜像不再内嵌前端资源」句 | ✅ 已加演进指针（2026-08-29） | 现状属实但与 D7（go:embed 内嵌）方向冲突，已标注 W0 起恢复内嵌 |
| `docs/ARCHITECTURE.md` sqlitecompat 引用 | ✅ 已修（2026-08-29） | 包已删（79d8b7b），现状=启动零 schema 校验 |
| Go 版本表述 | ✅ 已修（2026-08-29） | ARCHITECTURE 统一 1.26；docs/README 已指针化、旧徽章随之消失 |
| `docs/README.md` 与根 README 重复 | ✅ 已指针化（2026-08-29） | docs/README 只留文档路由表 |

## 6. 否选项汇总（不再讨论）

Next.js / SSR · AntD / MUI / kumo · lightweight-charts / Recharts / D3 自绘 · react-router · ESLint+Prettier 双件套 · gorilla/sessions 等会话库 · JWT 无状态会话（无法服务端吊销） · 独立前端仓库 · PG 新增投入。
