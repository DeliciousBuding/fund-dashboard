# 04 — 鉴权与安全设计

> 定档日期：2026-08-29 · 状态：已定档
> 前提事实（测绘确认）：当前仓库**无任何** login/session/cookie/password 设施；鉴权 = 三把共享密钥（`MCP_API_KEY`/`PUBLIC_MCP_KEY` 走 Bearer，`FUND_EDGE_KEY` 走自定义头 `X-Fund-Edge-Key`）；无 recover 中间件；无应用层限流；`edge_auth.go` 的 Origin 白名单硬编码在公开仓库。本设计全部从零新建，向后兼容。

## 1. 目标与非目标

**目标**：浏览器进入任何页面前必须过一道密码门；浏览器 JS 永不持有任何长期密钥；agent 面（MCP/Admin Bearer）零改动。
**非目标**：多用户、RBAC、OAuth/OIDC（edge 层 OIDC 可继续叠加，与本设计正交）、TOTP（backlog）。

## 2. 密码与存储

- 哈希：**argon2id**（`golang.org/x/crypto/argon2`），参数 m=64 MiB / t=3 / p=2，salt 16B，PHC 字符串编码存储。
- 环境变量覆盖：`FUND_AUTH_PASSWORD_HASH`（PHC 串）若设置则优先生效（IaC 场景）；未设置且 DB 无凭据 → 进入**首次启动 setup 流**。
- 新增表（SQLite 由 `internal/auth` 启动建表，PG 入 `schema_pg.go`；**时间列一律 unix epoch BIGINT**——双方言零解析差异、SQL 可直接比较）：

```sql
auth_credentials (                -- 单行（CHECK id=1）
  id            INTEGER PRIMARY KEY CHECK (id = 1),
  password_hash TEXT NOT NULL,    -- argon2id PHC
  created_at    BIGINT NOT NULL,  -- unix epoch 秒
  updated_at    BIGINT NOT NULL
);
auth_sessions (
  id           TEXT PRIMARY KEY,  -- sha256 hex(session_token)，原值不落库
  created_at   BIGINT NOT NULL,
  expires_at   BIGINT NOT NULL,   -- 滑动过期：活跃即顺延
  last_seen_at BIGINT NOT NULL,
  ip           TEXT,
  user_agent   TEXT
);
```

- 新增 env：`FUND_ALLOWED_ORIGINS`（逗号分隔；默认 `http://localhost:5173,http://127.0.0.1:5173`）——**替换 edge_auth.go 硬编码白名单**；`FUND_AUTH_SESSION_TTL`（默认 720h=30d，滑动窗）；`FUND_AUTH_SESSION_MAX_AGE`（默认 2160h=90d，绝对上限）；`FUND_AUTH_SECURE_COOKIE`（默认 true，`http://localhost` 自动降级）。
- **initialized 判定（2×2）**：

| `FUND_AUTH_PASSWORD_HASH` | DB 凭据 | initialized | 行为 |
|---|---|---|---|
| 已设 | 任意 | true | 登录走 env hash；改密码端点禁用（403 `auth_env_managed`）；DB 凭据忽略 |
| 未设 | 有 | true | 正常登录；改密码写 DB |
| 未设 | 无 | false | 仅 `/setup` 可用 |

- setup 原子性：「检查 initialized + 写入」同事务；`CHECK(id=1)` 唯一冲突 → 409 `already_initialized`（并发双击安全）。

## 3. 端点（挂 `/api/auth/*`，session 中间件豁免）

| 端点 | 语义 | 限制 |
|------|------|------|
| `GET /api/auth/status` | `{initialized, authenticated, session_expires_at}`，前端启动第一发 | 公开；限流宽松 |
| `POST /api/auth/setup` | 仅 `initialized=false` 时可调：设密码 → 自动登录 | **强限流**；初始化后恒 409 |
| `POST /api/auth/login` | 校验密码 → 发 cookie | **强限流**；失败统一 401 `invalid_credentials`（不区分有无凭据） |
| `POST /api/auth/logout` | 删当前会话行 + 清 cookie | 需会话 |
| `POST /api/auth/password` | 改密码（验旧）→ **吊销其他所有会话** | 需会话；限流 |
| `GET /api/auth/sessions` | 会话列表（id 前 8 位/UA/IP/最后活跃/是否当前） | 需会话 |
| `POST /api/auth/sessions/{id}/revoke` | 吊销指定会话 | 需会话 |

Cookie：`fund_session=<base64url(32B random)>`；`HttpOnly; Secure; SameSite=Lax; Path=/`；滑动窗口 30d（过半顺延，封顶公式 `expires_at = min(now+TTL, created_at+MAX_AGE)`），绝对上限 90d 到点强制重登。login/setup 成功时**先吊销请求 cookie 指向的旧会话行**（存在的话）再签发新 token（防会话固定）。

## 4. 鉴权矩阵（实施后）

| 面 | 鉴权 | 说明 |
|----|------|------|
| `/api/auth/*` `/api/health` | 公开 | login/setup 强限流 |
| 其余 `/api/*` 读路径 | **SessionAuth**（cookie） | 现状 public 读全部收进门内——单租户语义 |
| 浏览器写路径（transactions 等） | **SessionAuth**；EdgeAuth 兼容 fallback（session 优先） | 退出条件：`FUND_EDGE_AUTH_ENABLED`（W1 默认兼容开启、新部署建议关闭），W7+ 评估拆除代码 |
| `/mcp` | Bearer 双 key（本波次不变） | agent 面零改动。**后续演进**：design 07 在此之上加 OAuth 2.1 访问令牌，成为并行的第二条鉴权轨 |
| `/api/admin/*` `/api/agent/*` | Bearer `MCP_API_KEY`（**不变**） | 运维/agent 面零改动；W6 起 `/api/agent/tools*` 只读面额外接受 SessionAuth（供设置页） |
| SPA（HTML + `/assets/*` + manifest/sw/favicon） | 公开 | 代码本身是开源的，静态资源无敏感数据；**数据面全部收在 `/api/*` 门后**——这是更简洁且不会白屏登录页的模型（实现时精炼） |

实现：`internal/auth`（哈希/session/limiter）+ `internal/httpapi/auth.go`（handlers）+ `SessionAuth` 中间件。路由装配顺序插在全球链之后、业务组之前。

## 5. CSRF 与浏览器安全

三道叠加（单租户够用且正确）：
1. Cookie `SameSite=Lax` —— 跨站 POST 天然不带 cookie；
2. 变更类请求必须带自定义头 `X-Fund-Request: fetch`（浏览器跨站表单发不出自定义头）；
3. `Origin`/`Sec-Fetch-Site` 校验：逻辑从 `edge_auth.go`（现 `internal/httpapi/origin_check.go`）抽为共享中间件，白名单改读 `FUND_ALLOWED_ORIGINS`（**顺手消除公开仓库里的硬编码生产域名**），`Sec-Fetch-Site: cross-site` → 403。

## 6. 限流与兜底（补测绘发现的缺口）

- **登录限流器**（`internal/auth/limiter.go`，in-memory，不引库）：per-IP 5 次失败 → 15 分钟锁；全局滑动窗 20 次/小时；超限快速 429 + `Retry-After`（**不让请求睡眠**——tarpit 会堆积 goroutine）；所有失败 `slog.Warn`（含 request_id、IP、UA，**不含密码**）。IP 来源：edge 反代后取 `X-Forwarded-For` 最右一跳（edge 覆盖写入场景）；无 XFF 时 per-IP 桶退化为全局桶——单租户下接受（最坏情况是自己被锁 15 分钟，不损数据安全）。
- **recover 中间件**：补 chi `middleware.Recoverer` 语义的手写版（复用现有 WriteJSON 错误形状），全局链第一位——handler panic 返回 500 `internal_error` 而不是断连。
- 应用层通用限流仍交 edge（现状注释立场不变），应用内只对 auth 面设防。

## 7. 会话清扫（挂现有调度器）

03:00 CST WAL job 同一窗口追加：
- `DELETE FROM auth_sessions WHERE expires_at < now`
- `DELETE FROM agent_confirmations WHERE expires_at < now - 7 days`（测绘发现：过期行从不清理）
- `DELETE FROM agent_audit_events WHERE created_at < now - 90 days`（测绘发现：只增不减）

## 8. 嵌入 SPA 与安全头

- `internal/webui/embed.go`：`//go:embed all:dist` + `http.FS` 包装 → 复用 `static.go` 的 SPA fallback（8 MiB 上限、路径穿越防护已实现）。`FUND_STATIC_DIR` 非空时优先用磁盘目录（dev）。
- 缓存：`/assets/*`（Vite 指纹文件）→ `Cache-Control: public, max-age=31536000, immutable`；`index.html` → `no-cache`；`static.go` 增加按路径分类的缓存头。
- **CSP 由应用设置**（不再假设 edge 会做）：
  `default-src 'self'; script-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'`
  `style-src` 初版只给 `'self'`：ECharts/Radix 的 `element.style` 赋值不受 style-src 拦截，Vite 产物 CSS 全外链；W0/W2 实测 tooltip 等确需内联样式时再放开 `'unsafe-inline'` 并在注释点名触发库（注意 `'unsafe-inline'` 与 `'self'` 并存按最宽处理，等于没收紧，非必要不加）。
- 现有 SecurityHeaders 中间件保留（上游已设则不覆盖），CSP 加入同层。

## 9. 威胁模型快查（单租户）

| 威胁 | 缓解 |
|------|------|
| 暴破密码 | argon2id 慢哈希 + per-IP/全局限流 + 退避 + 日志 |
| cookie 窃取（XSS） | HttpOnly + CSP 无内联脚本 + 依赖最小化 + React 默认转义 |
| CSRF | SameSite=Lax + 自定义头 + Origin 校验三重 |
| 会话固定 | 登录成功即换新 token；改密码吊销其他会话 |
| token 泄库 | 库只存 sha256(token)，原值不可还原 |
| 会话永久有效 | 30d 滑动 + 90d 绝对上限 + 每日清扫 |
| 浏览器持 key | 消灭 EdgeKey 进浏览器的路径（session 优先，EdgeKey 仅 edge 注入兼容） |
