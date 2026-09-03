# 06 · 公网暴露安全加固设计（W1.6 加固包 + 工作台 API 面）

最后更新：2026-08-29

> 前提变化：部署形态从「自用/内网」升级为**公网直接暴露**。本文档是 W1 鉴权基线（04 文档）
> 之上的加固层，同时定义后台管理工作台与设置系统所需的 session 化系统 API。
> 全部保持 PG + SQLite 双方言、公开仓零泄露纪律。

## 1. 威胁模型（公网直连）

| 威胁 | 现状 | 加固目标 |
|------|------|---------|
| 凭证 stuffing / 密码暴破 | per-IP 5 次锁 15min + 全局 20/h | 递增锁定（指数到 24h）+ 更强密码策略 |
| 扫描器/爬虫打满资源 | 仅登录限流 | 全 API per-IP 限流 + 昂贵端点收紧 |
| Session 窃取（XSS/中间人） | HttpOnly+Secure+SameSite=Lax | + HSTS + COOP + 会话审计可见 |
| CSRF | SameSite + 自定义头 + Origin 白名单 | 维持（三重已足） |
| Clickjacking / MIME 嗅探 | frame-ancestors/nosniff 已有 | 维持 |
| MCP key 泄露/滥用 | 双 Bearer 分级 + 写确认 | + per-key 限流 + 掩码展示 + 轮换 runbook |
| 反代后 IP 伪造（限流绕过） | XFF 最右一跳 | 可信代理 CIDR 白名单（可选严格模式） |
| 运维盲（被打了不知道） | 无认证侧审计 | auth_events 登录/改密/锁定全量留痕 |

非目标：WAF/IDS（交给边缘）、TOTP 2FA（单租户收益低，留扩展位）、CAPTCHA（限流已覆盖）。

## 2. W1.6 后端加固包

### 2.1 传输安全头（`internal/httpapi/security_headers.go` 扩展）

- 新增 `Cross-Origin-Opener-Policy: same-origin`（默认所有路径）。
  **例外**：远端 MCP 客户端（ChatGPT/Claude/Cursor）在弹出窗口打开 `/oauth/authorize`、
  `/login` 等授权面。带 `same-origin` 的弹出文档会从跨源 opener 断开——客户端看到
  `popup.closed == true`（窗口明明还在屏上），于是判定授权被取消。这些面改发
  `Cross-Origin-Opener-Policy: unsafe-none`（授权页无跨源秘密数据，其他面仍保持 same-origin）。
- CSP 拆为共享常量 `baseContentSecurityPolicy`；**同意页**（GET /oauth/authorize）额外把
  `form-action` 向后追加已校验的客户端重定向源（如 `https://chatgpt.com`）。Chrome 对
  「表单提交重定向链」的每一跳都检查 `form-action`，基线 `form-action 'self'` 会静默拦掉
  同意 POST 后的 303 回跳到客户端回调——表现为点「授权」无反应、未跳转。追加的源只取
  scheme://host（不经路径/查询），无法扩大重定向面；其他所有响应维持严格基线。
- HSTS：`Strict-Transport-Security: max-age=31536000; includeSubDomains`，**仅当**
  `FUND_SECURE_COOKIES=true`（= 公网 TLS 部署信号）时下发；本地/HTTP 冒烟不受影响。
- 既有 nosniff/DENY/Referrer-Policy/Permissions-Policy/CORP 不动。

### 2.2 认证加固（`internal/auth/`）

- **递增锁定**：同一 IP 连续触发锁定，锁定时长 15m → 30m → 1h → 2h → … 指数翻倍，
  封顶 24h；登录成功清零计数。`Limiter` 增加 `lockStrikes map[string]int`。
- **密码策略**：`MinPasswordLen` 10 → 12；要求至少一个字母 + 一个数字（ASCII 判定）。
  只在 setup/change-password 时强制，存量哈希不受影响。
- **auth_events 审计表**（双方言，epoch BIGINT）：

  ```sql
  CREATE TABLE IF NOT EXISTS auth_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,  -- PG: GENERATED ALWAYS AS IDENTITY
    ts BIGINT NOT NULL,
    event TEXT NOT NULL,        -- setup|login_ok|login_fail|lockout|logout|password_change|session_revoke
    ip TEXT,
    user_agent TEXT,
    detail TEXT                 -- 短事实，永不写密码/token/哈希
  );
  CREATE INDEX IF NOT EXISTS idx_auth_events_ts ON auth_events(ts);
  ```

  - PG 进 `internal/repository/db/schema_pg.go`；SQLite 走 `auth.Store.EnsureSchema`（与 auth_sessions 同模式）。
  - 写入点：Setup/Login 成败/Limiter 触发锁定/Logout/ChangePassword/RevokeByIDPrefix。
  - 读端点：`GET /api/auth/events?limit=100`（SessionAuth，倒序，limit ≤ 500）。
  - 清扫：03:00 窗口 `DELETE WHERE ts < now-180d`（并入 scheduler sweepExpiredState）。

### 2.3 通用 API 限流（新 `internal/httpapi/ratelimit.go`）

- 中间件 `RateLimit`：per-IP 令牌桶，默认 600 req/min、burst 60（`FUND_API_RPM` 可调）。
  命中 429 JSON `{"error":"rate_limited"}` + `Retry-After`。挂在 `/api/` 组最外层（含登录之外的全部）。
- 昂贵写端点收紧（60/min）：`/api/transactions/import`、`/api/reports`、`/api/export/*`、
  `/api/dca/run`、`/api/portfolio/adjust-position`。
- MCP `/mcp`：per-key（Bearer 值哈希）120/min，`FUND_MCP_RPM` 可调；401/429 不计入。
- 内存实现（单实例单租户足够），重启清零可接受；**不睡眠、不排队**。

### 2.4 可信代理解析（`internal/httpapi/auth.go` clientIP 重构）

- 新 env `FUND_TRUSTED_PROXIES`（逗号分隔 CIDR/IP）。设置后：仅当直连对端（RemoteAddr）
  命中白名单才采信 XFF，且取 XFF 链中**首个不可信 IP**；未设置时维持现状（最右一跳，
  文档注明「假设单跳反代」）。CIDR 解析失败 fail-closed（视为不可信）。

### 2.5 MCP 面

- 确认 Bearer 比较为常量时间（`crypto/subtle`），不是则修。
- per-key 限流见 2.3。密钥轮换 = 改 env + 重启（runbook 进 README deploy 段）。
- 工作台 Agent 页展示：key 仅显示 `****` + 后 4 位（从 env 读，不落库不外传）。

### 2.6 工作台系统 API（全部 SessionAuth；写操作走 BrowserWriteAuth + CSRF）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/system/status` | version、db driver、go version、uptime、DB 文件大小（SQLite）、freshness health |
| GET | `/api/system/jobs` | 调度器任务清单：name、schedule、last_run、last_error、next_run（scheduler 加运行态跟踪） |
| GET | `/api/system/freshness` | 复用 admin GetFreshness（已有 `/api/freshness`，此处不重复，前端直接用那个） |
| GET | `/api/system/integrity` | admin VerifyIntegrity 结果 |
| GET | `/api/system/audit?limit=100` | auth_events + agent_audit_events 合并时间线（倒序） |
| POST | `/api/system/crawl-nav` | 触发 NAV 抓取（复用 admin crawl 适配器，operator 级） |
| POST | `/api/system/crawl-holdings` | 触发持仓抓取 |
| POST | `/api/system/verify` | 触发一致性校验 |
| GET | `/api/system/agent` | MCP 端点说明、工具统计（read/write/disabled 计数）、密钥掩码（后 4 位）、env 变量名清单 |

scheduler 运行态：`internal/jobs/scheduler.go` 增加 `JobStatus` 记录（LastRun/LastError/NextRun），
包级只读快照方法 `StatusSnapshot()`。

## 3. 工作台与设置系统（前端 IA，W6 落地）

侧栏新增「系统」分组（图标 lucide）：

- **工作台** `/system`：状态卡（版本/DB/调度器/新鲜度健康度）、任务中心（触发按钮 + 最近运行表）、
  告警卡（`/api/alerts`）、操作全部二次确认 Dialog。
- **审计** `/system/audit`：合并时间线（登录事件 + Agent 工具调用），事件类型 chip 过滤、
  时间倒序、detail 展开。
- **设置** `/settings` 四 tab：
  1. 安全：改密（旧+新×2 + 强度提示）、会话列表（设备/IP/最近活跃 + 单个撤销 + 「退出其他全部」）、
     登录事件最近 20 条。
  2. 偏好：主题（暗/亮/跟随系统）、密度、红绿约定 —— 已有 settings store，接 UI。
  3. 数据：导入交易（existing import 端点）、导出 XLSX/CSV、备份状态说明。
  4. Agent：MCP 端点地址、工具面统计、密钥掩码展示 + 轮换指引、confirmation 机制说明。

## 4. 验收矩阵（W1.6 后端，已实施并有对应测试：`internal/auth` + `internal/httpapi`）

- [x] 锁定递增：连续 3 轮触发锁定 → Retry-After 递增且 ≤24h 封顶；成功后清零
- [x] 密码策略：11 位纯数字被拒（400 + min 提示），12 位字母+数字通过
- [x] auth_events：login_ok/login_fail/lockout/logout/password_change 均落表，无密码/token 泄露
- [x] GET /api/auth/events 需 session；limit 上限 500
- [x] 通用限流：突发 60 后第 61 个请求 429 + Retry-After（用小 limit 配置测试）
- [x] FUND_TRUSTED_PROXIES：可信网段时取 XFF 客户端位，不可信直连忽略 XFF
- [x] HSTS 仅在 secure-cookies 开启时下发
- [x] /api/system/* 全部：无 session 401；有 session 200；POST 无 CSRF 头 403
- [x] crawl 触发经确认后有审计/结果回执
- [x] 双方言：新表 PG DDL 入 schema_pg.go，SQLite EnsureSchema；race 全绿
- [x] 公开仓卫生：无 IP/host/密钥入码（leak-guard 门禁）
