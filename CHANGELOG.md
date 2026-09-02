# Changelog

本文件记录用户可见的变更。格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 精神，结合本仓库的中文条目约定。

## 格式约定

- `[Unreleased]` 使用**扁平**条目：`- [类型] 描述`
- 类型：`新功能` / `改进` / `修复` / `文档` / `测试` / `chore`
- **禁止**在 `[Unreleased]` 内新增 `###` 子标题（减少并发冲突）
- 发版时由 maintainer 汇总为正式分段：`## [x.y.z] - YYYY-MM-DD`

## 发版流程

1. 确认所有改动已合入 `main`
2. 运行 `./scripts/release.sh <x.y.z>`（自动把 `[Unreleased]` 归入 `[x.y.z]`、打 tag `v<x.y.z>`、推送）
3. tag 推送触发 GitHub Actions，自动创建 Release（release notes 取自本文件对应版本段）

完整约定见 [CONTRIBUTING.md](CONTRIBUTING.md#发布流程)。

## [Unreleased]

- **新功能** OAuth 2.1 授权服务器（`internal/oauth`）：远程 MCP 客户端（ChatGPT 自定义连接器 / Claude / Cursor）可用标准授权码 + PKCE(S256) 接入 `/mcp`，不再需要外发静态 key。含 RFC 9728 / RFC 8414 发现文档（两套 well-known 路径形式 + OIDC 别名）、RFC 7591 动态注册、OpenAI client-id 元数据文档（CIMD，主机白名单 + SSRF 防护）、RFC 7009 撤销、ES256 JWT（`aud` 绑定 MCP 资源 URL）与 JWKS。
- **新功能** 「跳转网站登录后即授权成功」：`/oauth/authorize` 无会话时 302 到 `/login?next=…`（保留 PKCE challenge 等全部原始参数），SPA 登录成功后整页回跳；只读作用域 + 已登录默认免同意页（`FUND_OAUTH_AUTO_APPROVE=true`）。写作用域强制显示服务端渲染同意页（零 JS、零内联样式，符合既有 CSP）。
- **新功能** 作用域 → 角色映射：`fund.read` → analyst（写/运维工具不可见）、`fund.write` → operator（默认不广告，`FUND_OAUTH_ALLOW_WRITE_SCOPE` 控制）。签名密钥首次启动自动生成并持久化，无需密钥仪式，重启后已签发令牌继续有效。
- **修复** SPA fallback 会吞掉 `/.well-known/*` 并返回 `200 text/html`（改造前实测），使 OAuth 发现静默失败并被客户端误判为「服务端无认证」。现在 OAuth 路由先于 fallback 注册，且 `/oauth*`、`/.well-known*` 未匹配时返回 JSON 404。
- **修复** `initialize` 此前硬编码 `protocolVersion: 2025-06-18`；改为协商——回显客户端请求的受支持版本，未知或缺省回本服务端最新版。
- **修复** `safeOAuthReturn` 与前端同源校验都在路径规范化**之后**重新校验 `/oauth/` 前缀，堵住 `/oauth/../api/admin` 这类会被浏览器规范化后逃逸前缀的开放重定向；并拒绝 redirect_uri 中的 userinfo 与通配符主机。
- **改进** `tools/call` 响应增量补 `structuredContent`（同一份值的 JSON 对象，供客户端绑定 schema）与 `isError`；`content` 数组为仓库原有形态，对既有静态 key 调用方完全向后兼容。
- **改进** `/mcp` 401 现在带 `WWW-Authenticate: Bearer resource_metadata=…`（MCP 客户端据此发现并发起 OAuth），描述经清洗防止头注入。静态 `MCP_API_KEY` / `PUBLIC_MCP_KEY` 认证路径与语义完全不变（既有 operator/analyst key 消费者零改动）。
- **测试** 新增 `internal/oauth`（PKCE、码单次使用、JWT 失败关闭、密钥持久化、CIMD 白名单、作用域协商、完整授权码流程、刷新轮换、元数据）与 `internal/httpapi`（六条 discovery 路径、SPA 兜底回归、authorize 四种决策、同意页流程、令牌端点、MCP 集成、跨受众令牌拒绝、头注入清洗、静态 key 回归）测试；前端 `oauthReturn` 10 例。
- **测试** 新增 `scripts/smoke-oauth.sh`（11 节 58 项断言的端到端连接器冒烟）并接入 CI `OAuth MCP connector smoke`；本地 Linux 实跑 58/58 通过，`go test ./... -race` 全绿。
- **文档** 新增 `docs/design/07-oauth-mcp-connector.md`（端点契约、令牌声明、流程时序、威胁模型、兼容硬约束、连接器接入填表指南）；决策表补 D12；`deploy/.env.example` 补 OAuth 段。

- **改进** 系统工作台三个写触发响应（净值抓取/持仓抓取/一致性校验）补齐线型契约 `.strict()` 并接入触发按钮 parse——最后一个未校验写响应面闭环（此前漂移无感知）。
- **测试** 新增系统写响应线型用例 8 例（system 契约累计 12 例，全仓 108/108）。
- **chore** 第九轮全域审计：auth（限流升级/PHC 边界/会话存储）、快照重算引擎、确认令牌 HMAC、注册表授权、调度器清扫确认无新缺陷（历轮已加固）。

- **改进** MCP 工具执行结果审计正式接入生产：执行事件以 `event_type=execution` 落审计时间线（含请求 ID/调用方归属、闭集状态与错误类别），审计失败不阻主路径；无需表迁移。
- **修复** 指数行情端点对空白代码不再返回 `data:null` 的病态形状，改为 400 `code is required`（fail-closed，对齐线型契约）。
- **改进** 交易写响应（导入/更新/删除）补齐线型契约并 `.strict()`，前端表单与删除确认接入校验（此前响应体未校验，漂移无感知）。
- **重构** 重复列判定收敛为 `dialect.IsDuplicateColumnError` 单一来源（nav_history 补列处接入），补 PG `42701/duplicate_column` 识别。
- **测试** 新增契约用例 9 例（累计 100/100）；新增索引空白代码、重复列判定、审计归属、执行审计落库、启动期驱动校验顺序等回归（Go 24 包全绿）。

- **修复** 天天基金净值日期错一天（原按 UTC 换算导致净值日期存成前一天），现固定按北京时间 UTC+8 归一；存量日期订正另行审批。
- **修复** 会话过期后改密/撤销会话等受保护端点不再跳转登录页的问题：401 跳转白名单由 `/api/auth/` 前缀匹配改为显式公开端点集（探测/登录/退出/初始化）。
- **修复** 空基金代码被归一化成 `000000` 后打上（现请求前拒绝）、零值/负时间戳被解析成 1970-01-01、Yahoo 指数零价格被当作有效快照。
- **修复** 市场页交易散点加载失败静默吞成“买 0 笔 ¥0.00”、侧栏持仓列表失败误导显示“暂无持仓”，均改为显式错误态。
- **修复** fund-migrate 工具 SQLite `following` 整数列迁移到 PG `BOOLEAN` 硬失败；PG 连接池补 `driver.Validator` 转发，死连接不再滞留池中。
- **改进** 行情契约按线型证据收紧：`IndexHistorySchema` 补齐 4 个恒下发键并 `.strict()`、新增 `IndexLiveSchema`、`MarketIndexSchema` 价格/涨跌幅改必填 number；前端随之移除 2 处死分支、5 处死导出，改密/撤销/事件标记写响应接入契约校验。
- **改进** 持仓爬取代码清单超 5000 的静默截断改为告警日志可观测；admin/jobs 魔法数（LIMIT 5000/20、告警阈值 5/10/4 等）收口具名常量、portfolio clamp 去重。
- **改进** admin 服务新增 `NewServiceWithDriverChecked` fail-closed 错误传播入口，旧入口对未知驱动不再静默回退 SQLite。
- **重构** MCP 补记工具执行结果审计：闭集状态/错误类别信封 + 既有 sanitizer 分类 + 旁路通道（审计失败不阻主路径），生产持久化接线默认关闭待评审。
- **测试** 新增线型契约用例 16 例（累计 91/91）、前端 401 跳转用例 10 例（累计 75/75）、Go 24 包全绿（含执行审计、截断可观测、缺表判定、布尔迁移等回归）。

- **修复** admin 爬取三端点（单码/批量/陈旧刷新/快照重算）挂请求级超时（单码 2 分钟、批量 45 分钟），到点返回 504 `timeout`，被截断的批不再谎报 `complete`/`partial`。
- **修复** 快照首写竞态：同一代码并发首次重算不再出现重复行/报错，改为有界 UPDATE→INSERT 重试（兼容新旧主键形态）。
- **修复** 会话列表超过 200 条不再静默截断：接口返回 `total`/`truncated`，设置页显示“仅显示最近 200 条会话（共 N 条）”提示。
- **修复** 小写 `FUND_*` 环境变量此前被忽略（键大小写敏感），现统一大写归一化。
- **修复** 批量快照重算代码清单超过 5000 时改为告警日志可观测，不再静默丢尾部。
- **重构** 缺表判定四处重复实现收敛为 `dialect.IsMissingTableError`（补齐 PG `undefined_table`）；PG `HasColumn` 限定 `public` schema；`DaysSinceExpr` 双方言统一 UTC 基准（原 PG 按容器时区解释、午夜前后与 SQLite 漂移最大 2 天）。
- **重构** 驱动解析新增 `NewChecked` fail-closed 入口：未知驱动启动即报错，不再静默回退 SQLite（生产组装路径已迁移）。
- **重构** 前端契约两处线型漂移收紧：`ExchangeRateSchema` 补 `source`、`USStockInfoSchema` 补 `decision_boundary`/`side_effects`/`external_fetch` 与条件 `error`/`message`（此前被默认 strip 静默丢弃），两簇改 `.strict()`。
- **测试** 新增 admin 超时端到端 7 例、快照并发首写/注入式重试契约、会话截断全链路、线型契约形状共 20+ 用例（契约测试累计 70 例）。

- **修复** 迁移工具数据安全：`fund-migrate` 默认拒绝非空目标（预检即退出，未动任何数据），显式 `--force` 时 DELETE 移入每表事务（中途失败即回滚，不再出现清空到一半）；列探测按目标驱动分流。
- **改进** `fund-hash-password` 密码输入改环境变量/stdin 优先，argv 降为兼容手段并告警（进程列表泄露面收口）。
- **修复** 服务校验错误类型化：`writeServiceResult` 不再把数据库故障误报 400，校验错误（`ValidationError`）→400、内部故障→500 `internal_error`。
- **修复** 会话数超过 200 时按前缀撤销会话误报 `not_found`（过滤下推 store），并拒绝含 LIKE 通配符的前缀输入。
- **修复** 系统审计缺表判定兼容 PG 方言（原只认 SQLite `no such table`）。
- **重构** 中国市场时区收敛为单一来源包 `internal/chinatime`（删除三处重复定义与全部 `FixedZone` 散点，现代日期行为等价）。
- **重构** 前端契约三处 `.passthrough()` 收紧为 `.strict()`（Transaction/FundDetail/SourceEvent），`PortfolioSchema` 三字段改严格必填；未知字段漂移不再被静默吞掉。


- **修复** 调度器优雅停机补全：关停等待启动补跑任务（最长 45 分钟）结束，不再在进程退出后残留跑在已关闭数据库上；空指数刷新器不再在 20:00 窗口 panic。
- **修复** PG 部署 NAV 回填 upsert 的 ON CONFLICT 目标列与主键列序不匹配（报 42P10）；缺表判定兼容 PG `relation does not exist`，清扫不再误报失败。
- **修复** 批量 NAV/持仓刷新在全员失败时不再谎报 `complete`；持仓回填日期按中国市场日历，不再受 runner 本地时区影响。
- **修复** 快照重算（Light）一致性：无新 NAV 时保留旧 NAV 的同时估值列按同一保留值计算，不再出现 `latest_nav` 有值而 `current_value=0` 的矛盾行。
- **修复** 会话 TTL 配置超过绝对上限时创建即钳制（原长期闲置会话可活过 MaxAge，cookie 上限同步一致）；登录锁定计数饱和防整数溢出；改密成功与登录成功同样清零锁定状态；登录体拒绝尾随多余 JSON。
- **修复** 空库下组合定义与资产配置三个数组序列化为 `null` 被前端契约拒绝的问题，统一回 `[]`。
- **修复** MCP `upsert_dca_plan` 显式 `active:0` 现在能真正停用计划（原把 0 当缺省开成 1），注册表 schema 同步补 active 属性。
- **修复** 共享工具注册表惰性索引在首波并发 Lookup 下的数据竞争与锁拷贝告警。
- **修复** 配置了 PG DSN 但 `FUND_DB_DRIVER` 为空时 boot 失败（原对 PG 库执行 SQLite schema 引导）；调度器改在路由装配完成后启动，装配失败不再残留后台任务；静态目录拒绝普通文件。
- **改进** MCP 运行时加固：工具 handler panic 降级为稳定 JSON-RPC 错误（进程不退出）；错误回显泄漏面收紧（SQL 状态码/文件路径/URL）；审计身份字段 128 上限贯穿确认流程；确认准备失败不再落孤儿行；数值参数溢出钳制。
- **改进** 配置可观测性：非法环境变量（布尔/时长/速率上限）启动即告警并给出回退值；会话 TTL 超上限启动告警；删除 `Redacted()` 先写明文的死代码，修正 XFF 误导注释。
- **重构** 前端手写接口全部收进 packages/contracts 并接 zod 校验（auth/组合时间线/定投/台账导出/行情 SSE 帧）；修正 NavPoint 契约漂移；行情页指数加载失败不再误显示空数据态。
- **测试** 第四轮回归补洞：调度器关停等待补跑、注册表 64 goroutine 并发、角色×权限矩阵、审计边界、PG 冲突目标契约、空库序列化、尾随 JSON、到期边界、环境变量解析等。


- **修复** 限流器 sweep 语义修反：原先删掉耗干桶让被限流者一次 sweep 后 burst 满血重建，现只删已补满的空闲桶；限流 map 摊销式清扫过期 key，不再无限增长。
- **修复** 登录锁定期间审计洪泛：原先每个 429 都写一行 `auth_events`，现锁触发瞬间记一次；改密路径补上 lockout 事件。
- **修复** MCP 工具错误回显不再透出原始内部错误（SQL/拨号细节），技术错误降级为稳定 `internal_error`，完整错误落服务端日志。
- **修复** 无净值历史的标的 `/api/funds/{code}/nav` 返回 `[]` 而非 `null`（前端 zod 解析不再崩）；`risk_flags`/`signal_tags`/`dca.plans` 三处 Go nil slice → JSON null 在 contracts 归一化为空数组；DCA 计算错误分支字段改可选并接 `fetchValidated`。
- **改进** 会话过期清扫改走 `auth.Service.SweepExpired`（`jobs.AuthSessionSweeper` 接口），消除调度器裸 SQL；scheduler Stop 等待后台 loop 退出（优雅停机）；Yahoo 行情/汇率改共享 HTTP client 池。
- **改进** 前端构建分包：主 chunk 750KB→47KB（echarts/react/tanstack/vendor 独立）；告警列表/新鲜度/净值历史收敛为共享查询与组件；分析页与总览补齐 loading/error 态；登出失败补 toast；标的芯片改真按钮（a11y）。
- **安全** CI 修复断链 SHA pin（`pnpm/action-setup`、`softprops/action-gh-release`）；workflow_dispatch 输入改 env 间接注入；buildx 缓存按架构隔离；契约测试进 CI 门禁；dependabot 补 npm。
- **修复** `deploy/.env.example` 移除 `FUND_BACKUP_PRODUCER_ENABLED=true` 地雷（配置解析直接拒绝启动）；compose 补 9 个代码实际读取但未透传的环境变量。
- **重构** 三份重复的 rune-clamp 收敛到 `internal/textutil`；工具注册表校验接入生产加载路径；删除不可达的 crawl-nav legacy 适配器与 `WithClock`/`SessionMaxAge`/`SessionFromContext`/setup 限流死桶等死代码；对外文案实际去除内部代号（`source_brief`）。
- **修复** 交易导入/更新写路径 fund_code 长度校验是死代码：超长代码先被归一化静默截断到 32 字符，可能合并不同证券；改为归一化前拒绝。
- **修复** PG 写路径占位符 rebind 重写为词法状态机：双引号标识符/注释/dollar-quote 内的 `?` 不再误改写；未闭合构造返回带 offset 的错误，不再静默透传无效 SQL。
- **改进** `stock_kline_cache` upsert 的 prepared statement 改每次调用 prepare+用后 Close，消除进程级孤儿句柄；审计时间时钟可注入测试；备份目录读取失败补调试痕迹；删除 `httpapi.DB` 死别名。
- **改进** 设置页接口契约化：packages/contracts 新增 auth sessions/events 与 system agent（完整 14 字段）契约，前端删除 3 个手写接口改 `fetchValidated`，Sessions 卡补 empty 态。
- **测试** 补齐测试空洞：admin 写路径/告警阈值/新鲜度/状态边界（覆盖 47%→71%）、internal/contracts 校验器分支（→100%）、eastmoney meta/持仓解析、EnsurePGSchema 结构 pin+控制流 fake driver、rebind 表驱动 30 例。
- **测试** 限流器表驱动+并发、调度器优雅停机、快照重算数学、fund-migrate 保留字标识符引用、契约包 node:test 8 例、CSV 纯函数 7 例。
- **chore** `chi` v5.3.2；`zod` 统一 `^4.5.4`；`.dockerignore` 排除本地产物；文档路由数对齐实测。

- **修复** 导出 XLSX 文件名支持中文：Content-Disposition 改 RFC 6266 双文件名（ASCII fallback + `filename*` UTF-8 百分号编码）；XLSX 生成/写盘错误不再吞掉。
- **改进** 查询参数畸形值不再静默回落默认：`limit`/`offset`/`price_change_pct`/`drawdown_pct`/`stale_days` 返回 400 `invalid_query_param`；指数代码解码失败返回 400 `invalid_code`。
- **chore** `.gitattributes` 补齐 css/html/svg/sql/patch/toml 与无扩展名文本文件显式 LF，防 Windows autocrlf 行尾漂移。

- **chore** neat-freak 二轮收口：Go 三份 recalcSnapshot 收敛为 `internal/snapshot`；前端删死导出、收敛 toneClass/DIRECTION_LABEL/AlertItem/FreshnessReport 契约、修告警 severity 映射（high/medium/low/info）；Dockerfile 镜像 pin digest；CI setup-go pin `1.26.6`。
- **文档** 归版过时事实：测绘时间限定（登录/recover/Origin 白名单）、ARCHITECTURE 历史叙事指针化、TESTING smoke-prod 死链改 prose、CHANGELOG 死文件引用标注。
- **修复** NAV 历史 `daily_change_pct` 为 null 时被 zod 拒绝导致净值图解析失败（NavPoint schema 改 nullable）。
- **改进** `/api/transactions`、`/api/alerts`、`/api/reports`、`/api/dca/run` 响应契约入 zod contracts，读取面接 `fetchValidated` 运行时校验。
- **修复** 登录 PHC 参数无上限可被异常凭据打成内存/CPU DoS；`decodePHC` 按 OWASP 设 m/t/p 上限。
- **改进** MCP 工具失败、美股行情回写失败统一落 `slog`，不再静默吞错。
- **chore** 删除死代码/死导出（LoadFile、nullIfZero、DefaultUSIndexSymbols、buttonVariants 导出等）。
- **chore** CI Actions SHA 锁定 + workflow 级最小权限 + concurrency + dispatch 发布仅限 main。
- **文档** SECURITY 补 Web 登录/session/HSTS 边界；MCP 工具描述与 agent brief 去除内部代号 Hermes/DSA。

## [2.0.0] - 2026-09-01

- **修复** 工作台「数据新鲜度」徽章崩溃：前端映射键（ok/warn/critical）与后端 `freshnessHealth` 实际枚举（fresh/stale/degraded）不匹配且缺 `neutral` 兜底，`/system` 状态卡渲染即抛错；现按后端枚举对齐并补兜底。
- **修复** toast 样式在严格 CSP 下失效：sonner 运行时注入 `<style>` 被 `style-src 'self'` 拦截；改为静态样式表（`sonner/dist/styles.css`，`patches/sonner@2.0.8.patch` 移除注入调用），Docker web 构建阶段同步复制 `patches/`。
- **改进** oklch→rgb 主题取色画布加 `willReadFrequently`，消除 Chromium canvas readback 警告。
- **测试** Playwright 上下文 `serviceWorkers: "block"`：SW 预缓存会绕过 `page.route` 慢 chunk 拦截，禁用后导航反馈断言确定性成立。
- **改进** 导航直接点击与移动端触控新增顶部路由进度反馈；路由内容继续只保留入场动画并显式验收最终 `opacity=1`。
- **测试** 新增 Playwright Chromium 门禁：真实登录跳转、桌面/移动导航、intent route preload、慢 chunk 直接点击反馈、全核心路由透明态与 console error gate；失败 trace/screenshot/video 作为 CI artifact。
- **改进** 路由切换预加载：`createRouter` 开启 `defaultPreload: "intent"`（20ms 意图延迟），悬停/聚焦导航即预取目标路由的懒加载 chunk；点击提交通常直接命中缓存，消除切换导航时页面延迟出现/停在原地的感知。保持 viewport/render 级全量预载关闭，避免图表页连带拉入大体积 echarts chunk 破坏首屏预算。
- **修复** 登录、首次初始化、退出登录统一在导航前主动读取最新 `/api/auth/status` 并原子更新 TanStack Query 缓存，避免 60 秒 fresh cache 让路由守卫把成功的会话切换重定向回旧页面。
- **测试** 新增 auth cache transition 回归测试，覆盖成功替换 fresh cache 与刷新失败时保留旧状态。
- **文档** 新增 `docs/design/` 重设计定档（01 产品方向 / 02 技术栈 / 03 设计系统 / 04 鉴权安全 / 05 路线图 W0–W7）：下一代单租户 Web UI 以 `web/` workspace 回归本仓（登录 session 鉴权 + go:embed 内嵌单二进制），MCP 保持 Bearer 双 key 不变。
- **新功能** `web/` 前端回归（W0+W1.5）：Vite 7 + React 19 + TS strict + Tailwind v4 + TanStack Router/Query + Biome + pnpm workspace；登录/首次初始化页 + 受保护总览壳（真实组合 KPI）；构建直出 `internal/webui/dist` 经 go:embed 内嵌（未构建时服务占位页，二进制恒可编译）。
- **新功能** 部署链：Dockerfile 三阶段（node 构建 web → go 构建嵌入 → alpine 运行）；补建 `deploy/.env.example`；compose 透传 `FUND_AUTH_*`/`FUND_ALLOWED_ORIGINS`/`FUND_EDGE_AUTH_ENABLED`；CI 新增 `test-web` 门禁（biome/vitest/tsc/build），smoke-e2e 改走 setup→cookie 会话流并断言 SPA 嵌入。
- **新功能** 单租户登录鉴权（W1）：argon2id 密码哈希 + 服务端 session cookie（30d 滑动 / 90d 封顶），`/api/auth/*` 端点（status/setup/login/logout/password/sessions）；全部 `/api/*` 读路径收进 session 门内；浏览器写路径 session 优先、EdgeKey 兼容 fallback（`FUND_EDGE_AUTH_ENABLED=false` 可关）；CSRF = SameSite=Lax + `X-Fund-Request` 自定义头 + Origin 白名单（`FUND_ALLOWED_ORIGINS` env 化，移除硬编码生产域名）；登录限流（per-IP 5 次锁 15 分钟 + 全局 20/h，429+Retry-After）。
- **改进** 安全兜底：全局 recover 中间件（panic → 500 JSON，不再断连）；应用层 CSP 头（script-src 'self' 等）；静态资源缓存分级（`/assets/*` immutable、index/sw/manifest no-cache）；未注册 `/api/*` 非 GET 路径返回 JSON 404（原 405）。
- **修复** strip 遗留红测：`agenttools`/`contracts` 测试引用已删除的 docs/go-backend-rewrite 基线文件 → 改吃内嵌注册表/纯 Go 校验。
- **运维** 每日 03:00 窗口追加过期数据清扫：过期 session、过期 >7d 的 agent_confirmations、>90d 的 agent_audit_events（三表此前只增不减）。
- **chore** 删除死脚本 `scripts/package-release.sh`（引用 4 个不存在的 deploy 文件，必然失败）与 `scripts/_count_mcp_tools.py`（依赖仓外脚本）；删除根 `package-lock.json`（CI 无 npm 调用，W0 切 pnpm）。
- **文档** `docs/README.md` 指针化（消除与根 README 重复）；`docs/progress/MASTER.md` 消化 strip 交接并擦除主机代号；ARCHITECTURE 修正 sqlitecompat/Streamable HTTP/内嵌句/Go 版本表述。

- **chore** 前端移出：删除 `packages/web`（React/Vite/Playwright/Vitest），仓库变为纯 Go 后端 + zod contracts；镜像改为 API-only（Dockerfile/CI/release 同步；`FUND_STATIC_DIR` 移除出 compose）。
- **修复** scheduler price 窗口改为**每日 20:00 全量刷新持仓 NAV**（原工作日 20:00 stale-only），QDII T+2 净值不再滞后；DCA materialization 保持仅工作日。
- **chore** 删除 LEGACY `packages/crawler`（Python AKShare 离线爬虫；生产已是 Go datasource/jobs）。
- **改进** `GetMarketIndices` stale refresh 走 singleflight + 12s 上游超时，避免 HTTP/MCP 惊群。
- **改进** SQLite 打开路径显式 `PRAGMA journal_mode=WAL` + `synchronous=NORMAL`（`db.Open` / `sqlitedb.Open`；只读跳过 mode change），与 runbook/scheduler checkpoint 对齐，避免 fresh/dump 退回 DELETE+FULL。
- **改进** `upsertNavHistory` 整段 series 包事务（与 holdings 一致），减少 autocommit 逐行 fsync。
- **修复** `/api/market/indices` 输出按 `code` 稳定排序（map 遍历顺序不确定）。
- **chore** 删除根目录死脚本 `portfolio_risk.py`（硬编码个人持仓，隐私风险）。
- **测试** `db.Open` WAL/synchronous 断言；sqlitedb open 校验 journal_mode。
- **修复** write 控制路径：`RowsAffected` 驱动错误不再当 0 行；`AdjustPosition` 校验读 `held_shares`、admin/jobs recalc 读 `portfolio_id` 不再静默忽略 SQL 错误。
- **运维** 部署闭环：`deploy/push-ghcr.sh`（float 生产镜像 + pin tags）+ `deploy/hotswap 脚本`（env 去重、`FUND_VERSION` false-pin guard、placeholder 拒绝）；compose/deploy.sh/README 对齐。
- **改进** admin/ops dashboard `system.build_version`（`FUND_VERSION`；鉴权面，非 public health）；smoke 拒绝 `dev|latest|test|local`。
- **改进** `RecalcAll` / admin `recalculate-snapshot` / MCP `recalculate_snapshot`：`status=complete|partial|error` + `failed_codes[]`（恒数组，与 crawl-nav 对齐）；smoke 覆盖 admin+MCP all-mode body。
- **chore** 删除未接线死包 `internal/repository/portfolio`（~500 行）；AdminDashboard 去掉 Bun 时代 `node_version`。
- **测试** admin partial/all-failed；`RecalcAllStatus`；`useEChart` DPR；`pnlBucketIndex` 边界。
- **修复** 盈亏分布分桶：半开区间 `[min,max)`，避免精确边界（-10%/+10%）漏计。
- **修复** `recalcSnapshot` 查询 `portfolio_id` 失败不再静默默认 1（`ErrNoRows` 仍默认）；`navHistoryColumns` 尊重 ctx 取消并 debug 记 PRAGMA 失败。
- **改进** high-DPI / UIUX：`useEChart` 按 `devicePixelRatio`（cap 3）初始化并在分辨率变化时 re-init；字号小端 +1px；`hitTarget.min` 28；viewport-fit=cover；≥2x 略强 glass blur。
- **改进** MarketTicker 刷新/展开命中区 ≥28；ExchangeRateBadge 走 theme token；ChartFallback 复用全局 shimmer。
- **chore** i18n 删 dead keys（backtest.* 整块、allocation/penetration loading、comparison.bar/return、portfolio.tooltip）；`common.units` en=`funds`；`dca.totalInvested`。
- **chore** 删除未使用 `SHARED_CHART_GRID` 导出。
- **修复** ensureNavSchema：每刷新仅 once 探测列，缺失才 ALTER（避免每证券双 ALTER）。
- **修复** usStockSnap/indexHistory 进程缓存：过期清扫 + 上限 200 oldest 淘汰。
- **修复** WriteJSON：先 Marshal，失败 500+log，避免静默截断 JSON。
- **修复** pg rebind：单引号字面量内 `?` 不替换；补 `rebind_test.go`。
- **#277** 修复：MCP `get_source_events` 尊重 `unread_only`；`get_fund_status`/`get_source_events` 接受 `fund_code` 别名。
- **#276** 修复：移动端主内容 `paddingBottom` 避开固定底栏。
- **#275/#274** 修复：注册 YahooStock `PriceSource`，持仓股票可刷新。
- **#273** 修复：`portfolio_snapshot` upsert 显式 `portfolio_id`（ORDER BY + LIMIT 1），避免复合主键下标量子查询。
- **#272** 修复：`RecalcAllSnapshots` 单码失败 soft-fail 继续，不再整批中止。
- **#271** 修复：EastmoneyFund 拒绝非 2xx HTTP 响应。
- **#270** 修复：`get_full_dashboard` 描述与实际 payload 对齐。
- **#269** 修复：MCP 写工具缺 confirmation 时引导 prepare 端点。
- **#268** 修复：scheduler `Stop` 取消 startup catch-up AfterFunc / job context。
- **#267** 修复：OverviewPage `i18n` 解构，消除 formatNavDate 崩溃。
- **chore** smoke-prod：`pybin` 提前定义；tools/list 计数改 argv 传路径（Windows native python 可读 MSYS 临时文件）。（smoke-prod 现属私有运维仓）
- **#266** 稳健：agent confirmation request_id/caller 长度错误稳定码；ARCHITECTURE crawl-nav 文档 stale_only。
- **#265** 文档：SECURITY-AUDIT EdgeKey 注入范围与 #260 path-scoped 配置对齐。
- **#264** 文档：历史 git 密钥 residual 清单 + 轮换流程（**未** live 轮换）。
- **#263** 文档：GHCR IMAGE_TAG cutover runbook（现为 GHCR 生产镜像）。
- **#262** 数据：portfolio_snapshot 主键 `(fund_code, portfolio_id)` + PG 安全迁移 + 复合 ON CONFLICT。
- **#261** 安全：SPA + 公网读 API 启用 TokenDance ID OIDC（`auth_request`；未登录 302 login）。
- **#260** 安全：nginx EdgeKey 仅注入 SPA 写路径（transactions/source-events/ops）。
- **#259** 文档：Hermes mcp.json crawl_nav/get_data_freshness 描述对齐 stale_only / recommended_codes。
- **#258** 测试：smoke MCP crawl_nav(stale_only=true) 端到端（healthy 时 mode=stale_only）。
- **#257** 测试：smoke harness crawl_nav Input.stale_only + admin crawl-nav?stale_only=1。
- **#256** 测试：smoke-prod 断言 tools/list 中 crawl_nav.inputSchema.properties.stale_only。（smoke-prod 现属私有运维仓）
- **#255** 测试：smoke-prod 校验 get_data_freshness 的 recommended_codes / recommended_maintenance_args.stale_only。（smoke-prod 现属私有运维仓）
- **改进** 爬虫调度：startup + 工作日 20:00 CST 改为 **stale_only**；NAV upsert 增量丢弃 ≤ 本地 MAX(date)。
- **改进** 产品：NAV 新鲜度条（severity/warn/critical）+ portfolio React Query staleTime 5m。
- **修复** portfolioId 作用域：XIRR 终端市值优先 snapshot；compare/MCP/FE detail/DCA/harness 软过滤 source_events。
- **chore** 工程洁癖：删除死代码 AppDataContext；XIRR/Compare 显式 portfolioID；api encodeURIComponent + fetchPortfolioTimeline。


- **#254** 改进：harness 推荐 crawl_nav 时 Input 改为 `{stale_only:true}`，不再引导全量 held 刷新。

- **#253** 改进：Hermes portfolio-review/feishu-daily-digest 文档化 stale_only 回路；admin crawl-nav 支持 `stale_only=1` 与 MCP 对齐。

- **#252** 改进：get_data_freshness 返回 recommended_codes / recommended_maintenance_args；crawl_nav 支持 stale_only 仅刷新过期/缺失持仓（Agent 自动取最新信息路径）。

- **#251** 稳健：dca_run weekday Message、index_history 空 quote Message、integrity freelist Detail 动态模板夹紧。

- **#250** 稳健：getTransaction 读字段、DCA get-by-id 回退、dca_compute market/type、AdjustPosition fundName、harness Reason、source_brief Query/Reason/EntityName 自由文本夹紧。

- **#249** 稳健：SearchFunds/definitions/allocation/us_stock quote+profile/index_history 自由文本夹紧；repository ListSecurities/scanHolding 防御夹紧。

- **#248** 稳健：公开 portfolio 列表/穿透/搜索/DCA/detail 自由文本夹紧；scheduler 长任务 WithTimeout。

- **#247** 稳健：jobs 限速 sleep 可取消；Yahoo 多指数部分成功；indices 刷新失败 Message 脱敏为 `upstream_unavailable`。

- **#246** 稳健：market_indices soft LIMIT + 文本夹紧；summary GROUP BY soft LIMIT/键夹紧；scheduler WAL 走 context。

- **#245** 稳健：ensureNavSchema 走 ExecContext；alerts/holdings_coverage/status 自由文本 `clampAdminText`。

- **#244** 稳健：system_status anomaly 与 freshness Code/Name/Type/LastNAV 自由文本夹紧（共享 `clampAdminText`，对齐 #243）。

- **#243** 稳健：ops dashboard anomaly 自由文本字段截断；exchange_rate HTTP Client 显式 5s Timeout。
- **#242** 稳健：exchange_rate JSON LimitReader 1MiB；static SPA 8MiB 上限 + seekable 流式 ServeContent。
- **#241** 稳健：Yahoo 响应 LimitReader 4MiB；NAV/指数/美股 history 点上限 5000；holdings 行上限 500；upsertNavHistory 走 context。
- **#240** 安全：market SSE `: warn` 脱敏；integrity FK/table 错误 detail 稳定码；MCP `textJSONResult` 1MiB 上限。
- **#239** 稳健：CrawlCode security_type 查询改 QueryRowContext；integrity 用户表清单 soft LIMIT 500。
- **#238** 稳健：jobs getHeldSecurities QueryContext+LIMIT；CrawlAllHeld/RecalcAllSnapshots/fundHoldingsMatch soft LIMIT。
- **#237** 稳健：timeline transactions LIMIT 20000；admin verify missing NAV/negative positions LIMIT 5000。
- **#236** 稳健：harness/alerts held、holdings_coverage funds、allocation buckets soft LIMIT 5000。
- **#235** 稳健：admin freshness missing/stale soft LIMIT 5000；penetration funds/holdings/sector_map soft LIMIT。
- **#234** 稳健：summary settlement 分布改 GROUP BY；definitions/DCA-run/alerts/repo ListHoldings soft LIMIT。
- **#233** 安全/稳健：legacy `crawlHandler` 客户端错误脱敏为 `internal_error`（slog 全量）；`ListDCAPlans` soft LIMIT 5000。

- **#232** 稳健：XLSX 导出行字符串字段截断；legacy repository ListSecurities LIMIT 5000。

- **#231** 稳健：XLSX export 文件名去控制字符/长度上限；ListSecurities LIMIT 5000；getSecurityItem 直查。

- **#230** 稳健：portfolio 服务层 `clampPortfolioID`；agent-context `base_currency` 规范化；admin alerts/coverage 上限。

- **#229** 安全/稳健：AgentOps request_id/caller≤128；audit JSON 64KiB 截断；DeleteSecurity/ListDCA/MarkSource 小钳制。

- **#228** 稳健：指数 history 缓存键先规范化 range/interval；未知 range 默认 1y；jobs CrawlCode code≤32。

- **#227** 稳健：DCA frequency≤32、active 仅 0/1；指数 symbol 规范化截断 32。

- **#226** 稳健：`NormalizeSecurityCode` 与美股 symbol 截断至 32；MCP crawl/recalc 保留过长 code 守卫。

- **#225** 稳健：source-events 查询过滤字段截断；admin crawl/recalc `code` 长度 >32 拒绝。

- **#224** 稳健：source-event related 字段、DCA fund_name/日期、import signed 覆盖量级上限。

- **#223** 稳健：AdjustPosition 限制 shares≤1e9、reason≤200、fund_code≤32、portfolio_id≤1000。

- **#222** 安全/稳健：import/update 交易字段长度与金额上限（order_id/fund_name/trade_type/time、amount/fee/share ≤1e9）。

- **#221** 修复：CheckAlerts `maxDrawdownPct` 改为最近 2000 点 NAV 窗口（原先 ASC LIMIT 120 取最早历史，漏报现代回撤）。

- **#220** 稳健：基金详情交易列表 LIMIT 5000；DCA 幂等预检按 `(order_id, fund_code)`。
- **#219** 稳健：compare/timeline NAV 与 XIRR cashflow 查询加 LIMIT，防止超长历史拖垮内存/CPU。
- **#218** 稳健：drawdown NAV 最近 5000 点；证券/DCA 字段长度与金额边界校验。
- **#217** 稳健：SearchFunds/Stocks 查询长度上限；backtest base 上限、start_date 规范化、NAV 点 LIMIT 5000。
- **#216** 安全/稳健：source event 字段长度限制；backtest 无数据中性消息；MCP import 预检 5000；DCA active 0/1。
- **#215** 安全/i18n：MCP portfolio_id/offset/amount 等参数钳制；LanguageSwitcher 按钮面文案 i18n。
- **#214** 安全/稳健：import 最多 5000 行；portfolio_id 上限；MCP limit 类参数 intArgMax；DCA/指数/美股 soft message 脱敏。
- **#213** 安全：agent tools authorize `role` 白名单；DCA `base` 上限与 `mode` 白名单。
- **#212** 修复：AdjustPosition 检查 balancing INSERT `RowsAffected`；Finalize 审计失败时仍返回已消费确认（避免重复 finalize）。
- **#211** 修复：DCA 成交 `INSERT` 与 `dca_plan_executions` 同一事务；失败回滚，避免账本与交易分叉。
- **#210** 安全/文档：MCP `invalid_params`/`encode_tool_result` 脱敏；MASTER Remaining Gaps 清理已关 P1、标注多组合 PK 设计债。
- **#209** 修复：MCP 写确认两阶段（先 Verify 后成功再 Finalize/MarkUsed）；工具失败不烧 token；份额表单必填文案。
- **#208** 安全/i18n：admin/MCP body 限制；MCP HTTP 错误脱敏；DCA explanation 中性模板；NAV limit intQueryMax。
- **#207** 安全：agent confirmation body 限制；MCP `tool_error` 不再回传原始 err。
- **#206** 安全：source-events body 限制；admin crawl/recalc 错误脱敏；MCP compare codes≤8。
- **#205** 修复：compare `codes` 上限 8；agent confirmation 错误脱敏；admin/export `invalid_json`；MASTER #199/#203 收口。
- **#204** 安全：admin/SPA 写路径 5xx 走 writeSafeError；portfolio 查询 limit 上限；SPA import/put MaxBytesReader。
- **#203** 修复：`transactions` 唯一约束为 `(order_id, fund_code)`（转换双腿共享 order_id）；import/DCA/adjust NOT EXISTS 对齐；生产 0 真重复。
- **#202** 安全/修复：公开 portfolio/funds/market 走 writeSafeError（客户端 stable code，服务端带 request_id 记全错）；TransactionForm 强制 shares；PG `order_id` UNIQUE 索引 best-effort。
- **#201** 修复：DCA signal 稳定码 + SPA i18n；buy/sell 强制 confirm_share>0；NAV limit 上限 2000；export 2MiB/5000 行；scheduler claim UPDATE fail-closed。
- **#198** 修复：AdjustPosition 写入合成平衡交易（`持仓调整`）后 recalc；份额以 transactions SUM 为 SSOT，价格刷新不再覆盖。
- **#199** 修复：DCA 成交 INSERT 用 `WHERE NOT EXISTS(order_id)`；`RowsAffected=0` 记 skipped_duplicate，避免并发双写。
- **#200** 改进：ConfirmDialog focus trap/Escape/return-focus；PortfolioAllocation max-position 用 resolveAllocationLabel。
- **#197** 修复：SPA mutation 错误短 HTTP 脱敏；交易空态 noTx；risk_flags critical；holdings 原子重写；DESIGN/DR AgentOps SSOT。
- **#196** 修复：pg `rebindConn` 实现 `SessionResetter`；scheduler WAL 仅 SQLite probe 后执行；SECURITY-AUDIT AgentOps 已启用 SSOT。
- **#195** 修复：SPA `useChartData`/`useNasdaqData` 走 sanitizeUserError；FundDetail noCode i18n；DCA 轴万/k；冒烟 PUBLIC 密钥路径+prepare 401；Hermes/MATRIX AgentOps 已启用 SSOT。
- **#194** 修复：AgentOps `MarkUsed` 原子 `used_at IS NULL`（PG TOCTOU）；import 用 NOT EXISTS 幂等；scheduler claim 未知错误 fail-closed；生产已部署。
- **#5** 新功能：AgentOps 生产启用（`FUND_AGENT_OPS_ENABLED` + confirmation secret）；smoke-prod 增加 prepare→无确认拒绝→单次消费→复用拒绝；PUBLIC prepare 401；operator tools/list 44 / PUBLIC 26 不变。（smoke-prod 现属私有运维仓）
- **#193** 改进：Overview `first_trade`/`last_trade` 走 `formatNavDate` locale；FundDetail 图例点尺寸 token 化。
- **#192** 改进：DcaPanel / FundDetail 残余 spacing（marginTop 微间距 + toast bottom）收口 space token。
- **#191** 修复：penetration 空行业键 EN-primary `other`；Overview/App NAV 日期 `formatNavDate` 按 locale。
- **#190** 改进：删除交易 in-app ConfirmDialog（替代 window.confirm）；移动底栏 hitTarget.mobile；导出菜单 i18n；DCA 模式切换自动重算。
- **#189** 改进：残余 marginBottom/marginTop 20/16/12 → space token（ChartShell/Overview/Detail/Compare 等）。
- **#188** 修复：classify 补 ETF/EN 纳斯达克关键词 + isNasdaqFundName；SPA `sanitizeUserError` 脱敏用户错误；Nasdaq 页复用共享匹配。
- **#187** 修复：中文 Harness 文案「智能分析」；隐藏永久 disabled 的 CSV 导入按钮；弱化 import 不可用措辞。
- **#186** 修复：UIUX residual batch — chartHeight.distribution、Nasdaq benchmarkDesc i18n、usdCny/MA20 i18n、删除图标 critical 色。
- **#185** 文档：residual trade_type toggle DB-code SSOT after #184（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#184** 修复：FundDetail 切换买入类型只用 DB 中文 trade_type 常量（`tradeTypes` 服务）；EN locale 不再 no-op/写坏 DB。
- **#183** 文档：residual allocation risk_flags/agent_brief EN SSOT after #182（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#182** 修复：allocation `risk_flags` / `agent_brief` EN-primary（SPA chips + MCP/harness 事实串）。
- **#181** 文档：residual allocation label i18n SSOT after #180（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#180** 修复：allocation type/market bucket API 标签 EN-primary；SPA `AllocationRows` 按 key 走 i18n；扩展 `allocation.typeLabels/marketLabels`。
- **#179** 文档：residual DeleteSecurity stock_* cascade SSOT after #178（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#178** 修复：`DeleteSecurity` 级联清理 `stock_realtime`/`stock_kline_cache`/`stock_profile`；`fund_details` RowsAffected 错误上抛。
- **#177** 文档：residual DCA post-write + tableColumns SSOT after #176（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#176** 修复：DCA 成交后 snapshot recalc / execution ledger 错误写入 item.Message；`tableColumns` PG information_schema 查询错误上抛。
- **#175** 文档：residual integrity fkRows + PRAGMA quote SSOT after #174（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#174** 修复：`getPGIntegrity` 补 `fkRows.Err()`；`tableColumns` PRAGMA 表名 quote 防御。
- **#173** 文档：residual CheckAlerts maxDrawdownPct SSOT after #172（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#172** 修复：`maxDrawdownPct` 对 nav_history Query/Scan 真错误上抛（不再静默 0 回撤）。
- **#171** 文档：residual CheckAlerts dca_plans Scan SSOT after #170（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#170** 修复：`CheckAlerts` 对 `dca_plans` Query/Scan 真错误上抛（缺表仍软兼容；不再静默丢掉 dca_day）。
- **#169** 文档：residual AdjustPosition/CheckAlerts Scan SSOT after #168（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#168** 修复：`AdjustPosition` / `CheckAlerts` / `recalcSnapshotLight` 对 fund_details 与 NAV 的 QueryRow.Scan 真错误不再吞掉（ErrNoRows 仍软可选）。
- **#167** 文档：residual DCA Scan fix SSOT after #166（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#166** 修复：`RunDCAAutoInvest` 对 order_id / dca_plan_executions / NAV 的 QueryRow.Scan 错误不再吞掉（缺表仍软兼容；真错误失败）。
- **#165** 文档：residual Yahoo index EN fallback SSOT after #164（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#164** 修复：Yahoo `defaultIndexNames` 空 shortName 回退改为英文 Yahoo 风格标签（SPA 仍走 `market.index.*` i18n）。
- **#163** 文档：residual chart height tokens SSOT after #162（MASTER/HANDOFF/WAVE4/DESIGN）；开放集仍仅 #5。
- **#162** 改进：theme `chartHeight` 阶梯 + CSS `--fd-chart-h-*`；ChartShell 默认与 Fund/Portfolio/Nasdaq/Allocation/Penetration/MC/DCA/detail 图表高度收口 token。
- **#161** 文档：residual type=button SSOT after #160（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#160** 修复：残余 Kumo `<Button>` 补 `type="button"`（26 处；#126 之后的 residual）。
- **#159** 文档：residual t() fallback strip SSOT after #158（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#158** 改进：剥离已存在 key 的 `t(key, '中文')` / App `defaultValue` fallback（FundChart/ChartShell/FundComparison/PnL/App）；MarketTicker 指数 API 名 fallback 保留。
- **#157** 文档：residual NasdaqOverview i18n SSOT after #156（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#156** 修复：NasdaqOverview 图表 series/tooltip 硬编码中文（合计/日/笔）→ `nasdaq.totalLine|seriesBuy|seriesSell|tooltipBuy|tooltipSell`。
- **#155** 文档：residual XLSX locale export SSOT after #154（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#154** 修复：`POST /api/export/transactions-xlsx` 按 Accept-Language 输出中/英表头与方向标签；SPA 携带 i18n.language。
- **#153** 文档：residual missing SPA i18n catalog SSOT after #152（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#152** 修复：补齐 SPA 缺失 i18n 目录键（`nasdaq.*` 全段 + `fund.cost/buy/sellLabel` + `common.loading/loadError/noData/units/fund` + `nav.skipToContent`/`mobile.nav`），消除 EN 回退中文。
- **#151** 文档：residual penetration subtitle i18n SSOT after #150（MASTER/HANDOFF/WAVE4）；开放集仍仅 #5。
- **#150** 修复：`penetration.subtitle` 恢复 `{{equityCount}}`/`{{uniqueStocks}}` 插值（#142 回归）；PortfolioPenetration + vitest 对齐。
- **#149** 文档：residual layout chrome + skeleton SSOT after #147/#148（MASTER/HANDOFF/WAVE4/DESIGN）；开放集仍仅 #5。
- **#148** 改进：ChartFallback/PageFallback skeleton 条宽高走 `skeleton.barH/barW` + `statMinH`/`chartH`；theme + CSS `--fd-skeleton-*` 镜像。
- **#147** 改进：theme `layout.mobileNavHeight` + CSS `--fd-layout-mobile-nav-height`；App 移动底栏 `height` 改 token。
- **#146** 文档：residual LanguageSwitcher + opacity SSOT after #144/#145（MASTER/HANDOFF/WAVE4/DESIGN）；keyword/data 字典中文匹配与图表单次 opacity 为 intentional residual。
- **#145** 改进：Sidebar 装饰 logo `alt=""` + `aria-hidden`；theme `opacity` 微 token + CSS `--fd-opacity-*`；迁移 MarketTicker/Harness/TransactionTable/FundChart/Nasdaq 等 chrome/series opacity 字面量。
- **#144** 改进：残余 LanguageSwitcher 硬编码标签 → i18n keys（`nav.switchToEn` / `nav.switchToZh`；目标语标签在 zh+en 目录同文）。
- **#143** 文档：residual classify + sector display i18n SSOT after #141/#142（MASTER/HANDOFF/WAVE4/DESIGN）；keyword/data 字典中文匹配为 intentional residual。
- **#142** 改进：残余 `SECTOR_NAMES` 展示标签 → i18n keys（`penetration.sectors.*`）；`classifySector` 中文关键词/regex 匹配保留。
- **#141** 改进：残余 classify 展示标签 → i18n keys（`CATS`/`STOCK_CATS`/`STOCK_MARKETS` 用 `nameKey`/`labelKey`；Sidebar 走 `t()`）；`CATS.funds` 等中文 keyword 字典保留作基金名匹配。
- **#140** 文档：residual TransactionForm + Allocation i18n SSOT after #138/#139（MASTER/HANDOFF/WAVE4/DESIGN）。
- **#139** 改进：残余 PortfolioAllocation 类型/市场标签 + sunburst tooltip、tradeMarkers 默认买卖标签 → i18n keys（`allocation.*` + `fundDetail.dir.*`）。
- **#138** 改进：残余 TransactionForm 硬编码中文 → i18n keys（`fundDetail.txForm.*` + 方向标签；DB `trade_type` 常量保留中文码）。
- **#137** 文档：residual TransactionTable + CSV export i18n SSOT after #135/#136（MASTER/HANDOFF/WAVE4/DESIGN）。
- **#136** 改进：CSV 导出表头/方向标签走 i18n（`fundDetail.csv.*` + `fundDetail.dir.*`，随当前 locale）。
- **#135** 改进：残余 TransactionTable 硬编码中文 → i18n keys（搜索/列头/空态/方向标签 + zh/en）。
- **#134** 文档：residual i18n Chinese + borderRadius SSOT after #132/#133（MASTER/HANDOFF/WAVE4/DESIGN）。
- **#133** 改进：theme `radius.none` / `radius.xs` + CSS `--fd-radius-none/xs`；迁移 App/MarketTicker/Sidebar/Penetration 残留 borderRadius 字面量。
- **#132** 改进：残余 MarketTicker/FundDetail 硬编码中文文案 → i18n keys（分组/指数名/交易统计 + zh/en）。
- **#131** 文档：residual motion + i18n title SSOT after #129/#130（MASTER/HANDOFF/WAVE4/DESIGN）。
- **#130** 改进：残余硬编码中文 `title` 改 i18n keys（FundDetailView/MarketTicker/TransactionTable + zh/en）。
- **#129** 改进：theme `duration` / `easing` + `cssTransition` 微 token；迁移 FundDetail/MarketTicker/Card 残留 transition 字面量。
- **#128** 文档：residual a11y button + lineHeight SSOT after #126/#127（MASTER/HANDOFF/WAVE4/DESIGN）。
- **#127** 改进：theme `lineHeight` / `letterSpacing` 微 token + CSS 镜像；迁移 LanguageSwitcher/OfflineBanner/StatCard/Sidebar 字面量。
- **#126** 修复：残余 button 补 `type="button"`（InvestmentHarnessPanel / MarketTicker）。
- **#125** 文档：residual fontWeight/zIndex SSOT after #123/#124（MASTER/HANDOFF/WAVE4/DESIGN）。
- **#124** 改进：Card 内容层叠改 `zIndex.local`；theme + `--fd-z-local` 镜像。
- **#123** 改进：残余 fontWeight 字面量迁移（DcaPanel/OfflineBanner/PortfolioSwitcher/LanguageSwitcher/Nasdaq/TransactionTable 等）。
- **#119** 改进：theme `zIndex` 海拔阶梯 + fixed 层迁移。
- **#120** 改进：theme `fontWeight` 阶梯 + 高频字重字面量迁移。
- **#121** 改进：AdminDashboard 分区 aria-label + 异常表 caption/aria-label。
- **#122** 文档：DESIGN.md token 准确性对齐 #103–#118（+ #119 zIndex；glassSurfaceStyle / ChartShell / space·radius·fontSize / skeleton / critical / reduced-motion spin；G6 仍产品门控）。（docs/DESIGN.md 已删除）
- **#118** 改进：theme `fontSize` 字号阶梯 + SPA 内联 fontSize 字面量迁移（raw ≈ 0）。
- **#117** 修复：MarketTicker 刷新旋转动画改 `fd-spin`，并尊重 `prefers-reduced-motion`。
- **#116** 改进：ChartFallback/PageFallback skeleton 加载壳（减 CLS；respect reduced-motion）。
- **#115** 改进：残余间距最终清扫（App/DCA/Nasdaq/misc + page headers）；有意义 magic spacing ≈ 0。
- **#114** 改进：FundDetail/Sidebar/Compare/Penetration 残留间距 token 化。
- **#113** 改进：InvestmentHarnessPanel / MarketTicker / PortfolioAllocation 残留间距 token 化。
- **#112** 改进：抽取共享 `ChartFallback`；页面 loading/`padding:60` 收口到 `space[]`。
- **#111** 改进：Admin 异常表 Kumo Table；OfflineBanner 用 critical + safe-area；echarts shadow 走 chartShadowColor。
- **#110** 改进：Overview 子页签 tablist/tab；DCA `aria-pressed`；交易表 `aria-sort`。
- **#109** 修复：FundDetail 累计图改 useEChart；导出菜单 a11y（menu/Escape）+ glass tokens。
- **#108** 改进：CSV 导入在后端未实现时禁用（不再骗用户选文件）；`ExchangeRateBadge` 四页去重。
- **#107** 修复：错误/校验色改用 `theme.critical`，不再误用涨/盈利 `theme.up`。
- **#106** 改进：ChartShell/Overview/Admin/App 高影响间距收口到 `space[]`/`radius`。
- **#105** 改进：MonteCarlo/Allocation/Penetration/Compare 雷达收口 ChartShell 三态。
- **#104** 修复：统一 `glassSurfaceStyle` 磨砂路径；修 `data-mode`/`data-theme` 暗色 CSS 变量失效；补 `--fd-glass-shadow`。
- **#103** 改进：Overview 残留实心卡片统一 frosted glass（Allocation/Penetration/MonteCarlo + StatCardError）。
- **#102** 修复：US stock `previous_close` 误用 long-range `chartPreviousClose`（AAPL change_pct ~58%）。优先 `meta.previousClose`，其次历史倒数第二收盘，最后才回退 chartPreviousClose。
- **#101** 修复：MarketTicker A股分组仍用 `sh000001` 匹配不到 Yahoo `000001.SS`。增加 indexCodeMatch 别名，短名覆盖 Yahoo/Eastmoney 双代码。
- **#100** 修复：MarketTicker CN/HK 指数从未填充（仅 US Yahoo）。扩展 DefaultIndexSymbols 到 8（新增 ^HSI、000001.SS、399001.SZ、399006.SZ），SPA 代码映射（sh000001→000001.SS 等），市场区域自动检测（US/CN/HK），Deploy/Dockerfile 修复 docker chmod 权限问题。

- **#65** Role-aware harness/agent-context discovery: public HTTP remains least-privilege (26 tools); MCP operator restores full harness write/maintenance surface; public `agent-context` no longer leaks write tool names.
- **#66** `scripts/smoke-prod.sh` asserts public harness/agent-context discovery filters and operator MCP harness full surface.（smoke-prod 现属私有运维仓）
- **#67** `docs/ARCHITECTURE.md` accuracy: fail-closed MCP auth, PUBLIC 26, role-aware harness, adjust_position MCP vs admin HTTP.
- **#68** residual ARCHITECTURE topology: replace legacy topology diagrams; banner ARCHITECTURE_V3 historical.
- **#69** router EdgeAuth comments + HANDOFF/SECURITY checklist residual accuracy after public discovery wave.
- **#70** Hermes MCP catalog: generate_report JSON-only, check_alerts no webhook, write tools marked #5 confirmation-gated.
- **#71** residual SSOT polish: SIBLING/README PUBLIC 26, smoke PUBLIC tools/list count==26, document known money-fund anomaly.
- **#72** `.env.example` pure-Go accuracy: dual-driver PG, EdgeKey, AgentOps off-by-default, no bare confirmed=true / Bun Feishu claims.
- **#73** ops dashboard crawler freshness uses fund T+1 window (`stalePriceDays`) instead of false 24h zero; SPA label updated.
- **#74** MCP `get_data_freshness` only sets `recommended_maintenance_requires_run` when held NAV is stale/missing (no forced crawl_nav when healthy).
- **#75** smoke asserts freshness recommendation matches stale/missing; SPA `types.ts` drops packages/server claim.
- **#76** `deploy/docker-compose.yml` bannered as legacy SQLite path.
- **#77** `deploy/deploy.sh` / `rollback.sh` / `build.sh` bannered as legacy GHCR/SQLite path.
- **#78** smoke marks `smoke-probe` source-events read (no unread pollution); RELEASE-GOVERNANCE production host updated.
- **#79** DISASTER-RECOVERY rewritten for Azure PG; `docs/progress/MASTER.md` bannered as historical Bun single-container plan.
- **#80** smoke reuses existing `smoke-probe` source-event (PATCH) to stop unbounded row growth; unread remains 0.
- **#81** AgentOps checklist paths → `/prepare` + `/consume`; MASTER Remaining Gaps drops stale unimplemented-tools row.
- **#82** PENETRATION-DESIGN marked implemented; deploy/.env.example GHCR path, no Feishu side-effect claim.
- **#83** Hermes skills (feishu-daily-digest / portfolio-review) default to read-only MCP paths while AgentOps #5 is off.
- **#84** smoke default skips live crawl-nav/holdings; keeps recalculate-snapshot + 401 auth checks; `SMOKE_FULL_CRAWL=1` optional.
- **#85** HANDOFF §16 next steps only open/gated items; drop residual MCP34 completed-mix language.
- **#86** SECURITY residual: document public SPA mutations via nginx EdgeKey injection as accepted personal-site boundary (no edge change).
- **#87** crawl_nav/price refresh `added` counts only real inserts/value changes (no-op upsert → 0).
- **#88** crawl_fund_holdings skips rewrite when report slice unchanged (`added=0` on no-op).
- **#89** WAVE4 Wave6/MASTER closed-note SSOT polish (MCP44, residuals through #88).
- **#90** portfolio_snapshot recalc zeros dust `|held_shares| < 0.001` (sold-out float residue).
- **#91** MCP tools/list real JSON Schema (no typescript-zod stubs); accept `code`/`fund_code` aliases (run_backtest etc.); closed-position PnL zeroed with dust clamp.
- **#92** market indices: Yahoo refresh-on-read when cache >6h stale; weekday 20:00 scheduler refresh; SPA MarketTicker gets fresh quotes.
- **#93** `get_us_stock` aligns SQL to production PG stock_* columns (no open/high/low required); MCP `get_fund_detail` fills `xirr_pct`; banner `docs/AUDIT.md` historical.
- **#94** USD/CNY exchange-rate: in-process 1h cache + User-Agent; serve last-good on Yahoo 429 instead of SPA 502.
- **#95** restore `GET /api/market/index/{code}` + `/history` (NDX→^NDX, Yahoo chart, 30m cache) for NasdaqOverview.
- **#96** `GET /api/market/stream` SSE (`event: indices`) with 60s refresh + heartbeat for MarketTicker.
- **#97** restore `POST /api/export/transactions-xlsx` (excelize) for FundDetail Excel export menu.
- **#98** `/api/stocks/{code}` flat SPA `USStockInfo` contract + Yahoo refresh-on-read when cache empty.
- **#99** US stock re-fetches Yahoo when cached quote lacks OHLC or history empty; best-effort kline upsert.


- [安全] 公网 harness `available_agent_tools` 只读发现面，隐藏 write/maintenance/confirmation 工具（#64）
- [修复] AgentOps 关闭时 MCP 假 confirmation 不再 nil 解引用 panic（typed-nil interface）（#63）
- [文档] HANDOFF/WAVE4 对齐 PUBLIC tools/list 26（#62）
- [安全] PUBLIC tools/list 隐藏 confirmation-gated 工具（check_alerts/generate_report）（#61）
- [安全] MCP tools/list 按角色过滤：PUBLIC/analyst 不广告 write/maintenance 工具（#60）
- [文档] HANDOFF/SECURITY 对齐 #56–#58 scheduler durable 与 source-events EdgeAuth（#59）
- [修复] scheduler durable claim 使用 per-job `fund_code`（适配 crawl_log PK=fund_code）（#58）
- [安全] portfolio source-events 写路径 EdgeAuth + PG `RETURNING id`（#57）
- [修复] scheduler 启动 catch-up 与窗口 claim 做 CST once/day 持久化（crawl_log），避免同日 redeploy 全量打东财（#56）
- [安全] `/api/agent/tools*` 鉴权：registry/authorize 需 operator Bearer（#55）
- [修复] scheduler 同窗口 once-per-day 门闩，避免 5 分钟 ticker 在整点小时内重复刷新/DCA（#54）
- [改进] scheduler 工作日 20:00 价格刷新后执行 DCA materialization；周六 10:00 holdings crawl（#53）
- [修复] delete cascade `ant_transactions_raw` 列名对齐 `fund_code_cell`（#51 follow-up）
- [修复] `delete_fund` 级联补齐 dca_plan_executions/summary/crawl_log/ant_*/source_events（#51）
- [改进] `run_dca_auto_invest` 写入 `dca_plan_executions` 账本并做双重幂等（#52）
- [文档] README + Hermes 拓扑对齐 tools/list 44 / Azure PG（#50）
- [修复] portfolio_snapshot 无 market 列：adjust_position/check_alerts/dca_run 对齐生产 PG schema（#49）
- [文档] ARCHITECTURE.md MCP 计数对齐 tools/list 44（#48）
- [文档] AGENTOPS/SECURITY + harness 对齐 tools/list 44 全量（#47）
- [新功能] MCP `generate_report`（#46；tools 43→44 全量 registry 对齐；JSON facts-only，无 PDF；confirmation-gated）
- [新功能] MCP `run_dca_auto_invest`（#45；tools 42→43；dry_run 默认 true；幂等 order_id；confirmation-gated）
- [新功能] MCP `adjust_position` / `check_alerts`（#43/#44；tools 40→42；confirmation-gated，alerts 仅 facts-only 无 webhook）
- [新功能] MCP `add_fund` / `add_security` / `update_fund` / `delete_fund`（#42；tools 36→40；confirmation-gated，无 AgentOps 时 fail-closed）
- [新功能] MCP `upsert_dca_plan` / `disable_dca_plan`（#41；tools 34→36；无 AgentOps 时 fail-closed）
- [文档] deploy/HANDOFF 运维手册：admin crawl-nav/holdings/recalculate-snapshot（#40）
- [文档] SECURITY-AUDIT 运维清单覆盖 admin crawl/recalc（#39）
- [文档] ARCHITECTURE.md 标注历史文档 + SIBLING multi-arch 已落地（#38）
- [文档] ARCHITECTURE.md 管理端点对齐 Go crawl-nav/holdings/recalc（#37）
- [改进] Admin `POST /api/admin/crawl-nav?code=` 单证券刷新 + HANDOFF Go1.25 + smoke（#36）
- [新功能] Admin `POST /api/admin/recalculate-snapshot` + smoke 覆盖 crawl-holdings/recalc（#35）
- [新功能] Admin `POST /api/admin/crawl-holdings` 与 MCP crawl_fund_holdings 对齐（#34）
- [文档] HANDOFF/SECURITY/MATRIX 对齐 tools/list 34 与剩余 10 工具（#33）
- [新功能] MCP `crawl_fund_holdings` 接入 tools/list+call（#32；tools 33→34；Eastmoney jjcc）
- [新功能] MCP `recalculate_snapshot` 接入 tools/list+call（#31；tools 32→33）
- [新功能] MCP `crawl_nav` 接入 tools/list+call（#30；tools 31→32）
- [新功能] MCP `mark_source_event` 接入 tools/list+call（#29；tools 30→31）
- [文档] MCP registry 44 vs tools/list 矩阵（`docs/progress/MCP-TOOL-MATRIX.md`，#28）。（该矩阵已迁私有运维仓）
- [改进] 统一 skip-link：`index.html` 使用 `fd-skip-link` + token CSS（#27）
- [安全] 工作树脱敏：nginx EdgeKey include 本机 snippet、Hermes/HANDOFF 去明文密钥（#26；未轮换）
- [改进] 边缘 `limit_req` 挂载 `/mcp` `/api/admin` `/api/agent` `/api/`；README Go1.25 与公网读边界修正（#25）
- [改进] SPA 边缘 CSP + Permissions-Policy（#24）
- [修复] nginx `location /` 重申 HSTS/XFO/nosniff/Referrer-Policy（#23；避免 add_header 继承坑）
- [chore] CI/Release setup-go 对齐 go.mod `1.25.x`（#22）；radar 回落色走 `lightTheme.blue`
- [chore] GHCR `build-and-push` multi-arch（linux/amd64 + linux/arm64）+ GHA cache（#21）
- [测试] CI 可选 `e2e-full` job（workflow_dispatch `full_e2e=true`）跑全量 Playwright（#20）
- [改进] smoke-prod 自动加载 Hermes operator key；PUBLIC 写拒绝可验。（smoke-prod 现属私有运维仓）
- [改进] onAccent/markerBorder tokens 替换 OfflineBanner/Sidebar/Nasdaq/Penetration 残留 `#fff`（#19）
- [改进] sector 扩展色迁入 theme `sector*` tokens（#18）
- [改进] PortfolioSwitcher glass tokens + a11y listbox（配合 `?portfolio=` deep-link）
- [新功能] 组合 `?portfolio=` deep-link（`usePortfolioDeepLink`）+ `scripts/smoke-prod.sh` 生产冒烟。（该脚本现属私有运维仓）
- [新功能] 基金对比 `?codes=` deep-link 自动对比；critical CSS tokens
- [改进] 行业色板 SECTOR_COLORS 绑定 theme accents + SECTOR_FALLBACK
- [改进] 基金详情 `?detailTab=` deep-link；Ticker/Sidebar/OfflineBanner 去硬编码红绿
- [改进] Overview `?tab=` deep-link；Harness/Nasdaq/FundDetail 剩余 LayerCard → glass Card
- [改进] 交易 CRUD：Form/Table glass 化、删除 toast、aria-live、tabular nums
- [改进] 结构化 AccessLog（request_id/status/duration，跳过 assets，不记密钥）
- [改进] Admin/DCA/classify 去硬编码色；DcaPanel 改 glass Card
- [改进] StatCard/ChartShell 磨砂玻璃表面 + theme glass tokens；StatCard 去硬编码红绿
- [新功能] 图表 range deep-link：`useQueryRange`（FundChart/NasdaqOverview）
- [文档] 新增 `docs/DESIGN.md`（Vercel Web Interface Guidelines + Geist-inspired tokens）。（该文档已删除）
- [文档] Wave4 SDD：Issues #4–#7、`WAVE4-SDD` / `SIBLING-AUDIT` / `AGENTOPS-ENABLEMENT`
- [改进] UI：`space`/`radius` token、skip-link、`:focus-visible`、`prefers-reduced-motion`
- [改进] `usChangeColor` 走 theme token，去掉硬编码 hex
- [修复] PG 兼容：`INSERT OR REPLACE/IGNORE`、`sqlite_master`、XIRR 时间、timeline DATE cast
- [文档] HANDOFF/MASTER 对齐生产拓扑
