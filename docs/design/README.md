# docs/design — 重设计定档（2026-08-29）

> 本目录是 fund-dashboard 下一代形态的设计 SSOT：**单租户 + 登录密码鉴权的 Web UI + MCP 数据面**，
> 部署形态为**公网直接暴露**（TLS 由边缘终止），安全基线按互联网面标准（见 06）。
> 所有文档基于 2026-08-29 六路代码测绘（httpapi / service / data / mcp / ops / history）的事实写成。
> 原则：决策已定档、依据写明、不讨论已被否决的选项之外的可能性。

## 文档索引

| 文档 | 内容 |
|------|------|
| [01-product.md](01-product.md) | 产品方向：定位、五大支柱、信息架构、页面清单、旧前端功能对等表 |
| [02-tech-stack.md](02-tech-stack.md) | 技术选型：前端/后端/构建/部署全表 + 理由 + 否决项 + 仓库结构 |
| [03-design-system.md](03-design-system.md) | 设计语言「静水流深」：tokens、字体、颜色语义、图表主题、动效、状态设计、a11y |
| [04-auth-security.md](04-auth-security.md) | 登录鉴权与安全：session 设计、表结构、CSRF、限流、CSP、MCP 共存矩阵 |
| [05-roadmap.md](05-roadmap.md) | 实施波次 W0–W7、验收标准、仓库整理清单、技术债登记 |
| [06-security-hardening.md](06-security-hardening.md) | 公网暴露加固（W1.6）：威胁模型、递增锁定、auth 审计、全 API 限流、可信代理、工作台系统 API |

## 决策速览

| # | 决策 | 一句话理由 |
|---|------|-----------|
| D1 | **web/ 回归 monorepo**（pnpm workspace，非独立仓库） | 单租户单交付物：一个二进制/一个镜像，契约同仓零协调成本；独立仓库从未落地，无沉没成本 |
| D2 | **Vite 7 + React 19 + TypeScript strict** | 旧前端验证过的组合，生态与 agent 熟悉度最高 |
| D3 | **Tailwind CSS v4 + shadcn/ui（深度定制）** | 设计 token 完全自控，Radix 可访问性兜底；否决 AntD/MUI（企业感重）与 kumo（旧栈，定制天花板低） |
| D4 | **ECharts 6 单图表库**（echarts/core 按需注册） | 一套主题通吃 折线/柱状/sunburst/treemap/heatmap/radar/scatter；旧前端的图表生命周期层（DPR/重初始化）直接继承 |
| D5 | **TanStack Router + Query + Table** | 类型安全 search params = 免费深链（tab/区间/组合全进 URL）；Query 缓存策略旧栈已验证 |
| D6 | **登录 = argon2id 密码 + 服务端 session cookie**；MCP/Admin 保持 Bearer 双轨 | 浏览器不再持任何 key；agent 面零改动；EdgeKey 降级为可选兼容层 |
| D7 | **go:embed 内嵌 SPA** → 单二进制；`FUND_STATIC_DIR` 保留为 dev 覆盖 | 部署形态不变（单容器），生产不需要任何静态目录配置 |
| D8 | 设计语言 **「静水流深 Quiet Capital」**：暗色优先、金色点缀、涨红跌绿（可切西式）、全等宽数字 | 金融产品的正确情绪：高密度但不吵，精确、克制、有质感 |
| D9 | **MCP 保持手写 JSON-RPC**（44 工具生产在跑），修 spec 兼容；官方 Go SDK 迁移进 backlog | 不为正确性之外的动机重写已验证面 |
| D10 | 范围纪律：**不做**多用户/注册/SaaS 化/券商下单/PG 新增投入 | 单租户边界写死在产品层，防 scope creep |
| D11 | **公网直接暴露**为正式部署形态：安全按互联网面标准（W1.6），TLS 由边缘终止 | 用户明确要求；加固包与系统审计随之上马 |

## 关键测绘事实（设计依据摘要）

- 后端约 50 条 REST 路由、44 个 MCP 工具已生产验证；**2026-08-29 测绘时登录/session/cookie/password 设施完全不存在**（全新建设面；W1 起已实现，现状见 `docs/ARCHITECTURE.md` §4）。
- **2026-08-29 测绘时无 recover 中间件、无应用层限流** —— W1 随登录一起补。
- SPA fallback 静态服务（`internal/httpapi/static.go`）已实现且可复用（8 MiB 上限、路径穿越防护、`/api`+`/mcp` JSON 404）。
- 旧前端（已删）的纯函数服务层（montecarlo / statistics / irr）带完整 vitest 测试，**直接移植**而非重写。
- 蒙特卡洛与相关性热力图**只在旧前端存在**，后端从未实现 —— 新前端先客户端化，后端/MCP 化入 backlog。
- 安全细节（2026-08-29 测绘时）：`edge_auth.go` 的 Origin 白名单**曾硬编码生产域名在公开仓库** —— W1 起改为 env 配置（`FUND_ALLOWED_ORIGINS`）。
- `agent_audit_events` / `agent_confirmations` 只增不减 —— W1 顺带加 TTL 清扫（挂在现有 03:00 调度）。
