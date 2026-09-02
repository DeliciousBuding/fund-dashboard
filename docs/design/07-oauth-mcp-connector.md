# 07 — OAuth 2.1 授权服务器与远程 MCP 连接器

> 本文档定档 fund-dashboard 作为**远程 MCP 资源服务器**接入 ChatGPT 自定义连接器
> （以及 Claude / Cursor 等任意标准 MCP 客户端）的认证设计与安全边界。
> 事实基线：2026-09-03 实现与验证（`scripts/smoke-oauth.sh` 58/58 通过）。

## 1. 问题与决策

改造前 `/mcp` 只有静态 Bearer key（`MCP_API_KEY` → operator、`PUBLIC_MCP_KEY` → analyst）。
把静态 key 交给第三方客户端意味着：无法按客户端区分、无法撤销单个客户端、
key 一旦泄露就是全量权限、且不符合 MCP 授权规范。

**D12：自建 OAuth 2.1 授权服务器（授权码 + PKCE S256，公有客户端），复用现有 Web 会话作为资源所有者认证；静态 key 双轨保留不变。**

- 否决「接外部 IdP（Auth0/Okta）」：单租户单用户，引入外部依赖只为发一个令牌，
  运维面和故障面都不划算；且外部 IdP 无法表达 `fund.read`/`fund.write` → 角色的映射。
- 否决「不透明令牌 + 数据库查表」：每次 MCP 调用都要落库，且第三方无法自行验签。
- 否决「只改 key 不做 OAuth」：ChatGPT 自定义连接器走的就是 OAuth 发现流程，静态 key 接不上。

## 2. 端点契约

| 端点 | 方法 | 说明 |
|------|------|------|
| `/.well-known/oauth-protected-resource` | GET | RFC 9728 资源元数据 |
| `/.well-known/oauth-protected-resource/mcp` | GET | 同上，带资源路径后缀的形式 |
| `/.well-known/oauth-authorization-server` | GET | RFC 8414 授权服务器元数据 |
| `/.well-known/oauth-authorization-server/mcp` | GET | 同上，带路径后缀 |
| `/.well-known/openid-configuration[/mcp]` | GET | 同上内容，兼容先探 OIDC 名的客户端 |
| `/oauth/authorize` | GET | 授权端点；无会话 → 302 `/login?next=…` |
| `/oauth/consent` | POST | 同意页提交（仅在需要同意时出现） |
| `/oauth/token` | POST | 令牌端点：`authorization_code` / `refresh_token` |
| `/oauth/register` | POST | RFC 7591 动态客户端注册 |
| `/oauth/jwks` | GET | ES256 公钥集（只含公钥） |
| `/oauth/revoke` | POST | RFC 7009 撤销（refresh token） |
| `/oauth/about` | GET | 人/agent 可读的自述文档 |
| `/oauth/assets/consent.css` | GET | 同意页样式（CSP `style-src 'self'` 要求外链） |

### 2.1 为什么 discovery 路径要注册两套形式

RFC 8615 的路径感知 well-known 构造规定：资源为 `https://host/mcp` 时，
元数据在 `https://host/.well-known/oauth-protected-resource/mcp`。
但真实客户端对「是否追加资源路径」并不一致——只服务一种形式是连接器
「静默发现失败、退化成无认证」的最常见原因。因此两种形式（外加 OIDC 别名）
全部返回同一份文档。

### 2.2 SPA fallback 陷阱

`internal/httpapi/static.go` 的 NotFound 兜底会把**任意未匹配路径**返回
`index.html` + HTTP 200。改造前实测
`GET /.well-known/oauth-protected-resource` → `200 text/html` + SPA 外壳。
对客户端而言这与「服务端没有认证」不可区分，是本次改造必须先堵的洞：

- OAuth 路由在 SPA fallback **之前**注册；
- `isAPIRoute` 扩展到 `/oauth*` 与 `/.well-known*`，未挂载时返回 JSON 404 而非 HTML；
- `scripts/smoke-oauth.sh` 第 3 节把这条钉成回归用例。

## 3. 令牌与作用域

访问令牌是 **ES256 JWT**（RFC 9068 形态），关键声明：

| claim | 值 | 作用 |
|-------|-----|------|
| `iss` | `FUND_PUBLIC_BASE_URL` | 签发者；验签时严格比对 |
| `aud` | `<issuer>/mcp` | **受众绑定**：同一签发者下的其它资源服务器无法重放 |
| `sub` | `fund-owner` | 单租户固定主体 |
| `scope` | `fund.read` / `fund.write` | 映射到角色 |
| `client_id` | 注册或 CIMD 解析出的客户端 | 审计归因 |
| `exp` / `iat` / `jti` | — | 寿命与唯一性 |

作用域 → 角色映射只有一处（`oauth.RoleForScopes` + `httpapi.mapOAuthRole`），
授权服务器与资源服务器不可能各说各话：

| scope | agenttools 角色 | 可见工具面 |
|-------|----------------|-----------|
| `fund.read` | `analyst` | read + external_context（写/运维工具**不可见**） |
| `fund.write` | `operator` | 追加 write + maintenance；每个写工具仍需二次确认 |

`fund.write` **默认不对外广告**（`FUND_OAUTH_ALLOW_WRITE_SCOPE=false`）。
即使打开了，写作用域也强制走同意页，不会被 auto-approve。

### 3.1 签名密钥

解析顺序：`FUND_OAUTH_SIGNING_KEY`（PKCS#8 PEM）→ 数据库持久化密钥 → 首次启动生成并持久化。
默认路径**无需任何密钥仪式**：首次启动生成 ES256 P-256 密钥写入 `oauth_signing_key`
单行表（`id INTEGER PRIMARY KEY CHECK (id = 1)`，`ON CONFLICT DO NOTHING` 保证并发启动收敛为一把），
重启/升级后已签发令牌继续有效。`kid` 取 RFC 7638 JWK 指纹，跨部署稳定。

## 4. 授权流程（「跳转网站登录后即授权成功」）

```
ChatGPT ──POST /mcp（无令牌）──────────────► fund
        ◄─401 + WWW-Authenticate: Bearer resource_metadata="…/oauth-protected-resource/mcp"
ChatGPT ──GET  /.well-known/…──────────────► fund   （发现授权端点）
ChatGPT ──POST /oauth/register 或 CIMD─────► fund   （取得/解析 client_id）
ChatGPT ──浏览器打开 /oauth/authorize?…&code_challenge=…
                                            fund：无 fund_session cookie
        ◄─302 /login?next=/oauth/authorize?…（原始参数含 PKCE challenge 全部保留）
用户    ──输密码登录─────────────────────► fund   （复用现有 Web 登录，不新建账号体系）
        ◄─SPA 整页跳转到 next
                                            fund：会话有效 + 只读作用域 + auto-approve
        ◄─302 https://chatgpt.com/…?code=…&state=…
ChatGPT ──POST /oauth/token（code + code_verifier，**不带 client_id**）
        ◄─{access_token, refresh_token, token_type:"Bearer", scope:"fund.read"}
ChatGPT ──POST /mcp  Authorization: Bearer <access_token>
```

要点：

- **登录页回跳**：改造前 SPA 登录后硬跳 `/`，OAuth 流程会在此断掉。
  现在 `web/src/routes/login.tsx` 读取 `?next=`，经 `lib/oauthReturn.ts` 校验后整页跳转
  （`/oauth/authorize` 是后端路由，SPA router 不认识，必须真实导航）。
- **回跳目标双侧校验**：Go `safeOAuthReturn` 与 TS `safeOAuthReturn` 独立实现同一套规则，
  且都在 `path.Clean` / `new URL()` **规范化之后**重新校验 `/oauth/` 前缀——
  只校验原始字符串会被 `/oauth/../api/admin` 绕过（浏览器导航时会规范化）。
- **auto-approve**：已登录 + 纯只读作用域 → 直接发码，不显示同意页
  （`FUND_OAUTH_AUTO_APPROVE=true`，默认）。这就是「登录后即授权成功」。
- **同意页仍然存在**，用于写作用域或关闭 auto-approve 时；服务端渲染，
  零 JS、零内联样式（CSP `style-src 'self'` / `script-src 'self'`），
  带一次性 `consent_token`（10 分钟、单次消费）作为 SameSite=Lax 之外的第三层 CSRF 防线。
- **令牌端点不要求 `client_id`**：OpenAI 连接器的 `/token` 请求不带该参数，
  客户端身份从授权码记录中取。带了就必须与授权时一致，否则拒绝（防换客户端洗码）。

## 5. 威胁模型与防线

| 威胁 | 防线 |
|------|------|
| 开放重定向（authorize / login `next`） | redirect_uri 精确匹配注册值；未注册 → **渲染错误页而非跳转**；`next` 规范化后二次校验 `/oauth/` 前缀 + 长度上限 2048 + 拒绝 CR/LF/TAB |
| 授权码重放 | 内存态、60s TTL、`Redeem` 先删后返，并发下只有一个赢家 |
| PKCE 降级 | 只接受 `S256`；`plain` 直接拒绝（OAuth 2.1） |
| 令牌跨资源重放 | `aud` 绑定 `<issuer>/mcp`，验签时严格比对 |
| JWT 算法混淆 / `alg:none` | 头部 `alg` 必须字面等于 `ES256`，`kid` 必须等于当前密钥指纹，两者任一不符在验签**之前**就拒绝 |
| 篡改声明 | 三段 JWS，签名覆盖 header+claims（测试用真签名换 write 声明验证被拒） |
| 刷新令牌重放 | 轮换：先撤销旧令牌再发新令牌；库中只存 sha256，明文不落库 |
| CIMD SSRF | 主机白名单（默认仅 `chatgpt.com`）、强制 https、禁跟随重定向、DNS 解析结果必须是公网单播（拒绝 loopback/private/link-local/CGNAT/组播/0.0.0.0/255.*）、响应体 64 KiB 上限、10 分钟缓存 |
| Host 头伪造签发者 | `FUND_PUBLIC_BASE_URL` 显式配置优先；production 必须解析出 https 源，否则**启动失败**；请求派生仅用于本地开发 |
| 密钥泄露 | JWKS 只发布公钥（测试断言不含 `d` 与 PEM 私钥）；`FUND_OAUTH_SIGNING_KEY` 命中 `isSecretKey` 自动脱敏 |
| WWW-Authenticate 头注入 | `sanitizeChallengeDescription` 剥除引号/反斜杠/CR/LF 并截断 160 字符 |
| 同意页 CSRF | 一次性 `consent_token` + SameSite=Lax + `form-action 'self'` |
| 暴力/扫描 | `/oauth/*` per-IP 限流（默认 60/min，burst 30）；discovery 文档在限流桶**之外**，探测不会被饿死 |

## 6. 与既有面的兼容性（硬约束）

- **`MCP_API_KEY` 不变、不轮换**：既有 operator 消费者继续走静态 key。
  认证顺序为「先静态 key（常量时间比较）→ 再 OAuth JWT」，两条路径互不干扰。
- **`tools/call` 响应形态不变**：仓库原有的 `textJSONResult` 已产出 MCP 要求的
  `content` 数组。本次只**增量**补 `structuredContent`（同一份值的 JSON 对象，
  供客户端绑定 schema）与 `isError`，对只读 `content` 的消费者完全向后兼容。
  > 实现过程中一度引入过「按认证类型双轨包装」的开关，经查证属重复包装，已删除——
  > 单一契约优于可用配置项切换的双契约。
- **`initialize` 协议版本协商**：此前硬编码 `2025-06-18`。现在回显客户端请求的版本
  （限 `2025-06-18` / `2025-03-26` / `2024-11-05`），未知或缺省则回本服务端最新版。
- **生产 schema**：PG 侧三张表进 `schema_pg.go`（幂等 `CREATE … IF NOT EXISTS`），
  SQLite 侧由 `oauth.Store.EnsureSchema` 建，沿用 `internal/auth` 的既有分工，
  `TestPGAndSQLiteSchemaParity` 的单向约束（SQLite ⊆ PG）不受影响。

## 7. 接入 ChatGPT 自定义连接器

前置：ChatGPT → Settings → Apps & Connectors → 打开 **Developer mode**。

| 表单字段 | 填什么 |
|----------|--------|
| Name / Description | 自定义 |
| Authentication | **OAuth** |
| MCP Server URL | `https://<你的域名>/mcp` |
| Client ID / Client Secret | **留空**（公有客户端 + PKCE；连接器会自行注册或走 CIMD） |
| Default scopes | 可留空（连接器会从 discovery 读取），或填 `fund.read` |

保存后 ChatGPT 会：探 `/.well-known/oauth-protected-resource/mcp` → 取授权端点 →
弹浏览器登录 → 回跳拿码 → 换令牌。之后在对话里 @ 该连接器即可调用只读工具。

## 8. 配置项

见 `deploy/.env.example` 的「OAuth 2.1 授权服务器」段。全部可选，
唯一在 production 下强制的是**能解析出一个 https 签发者**
（`FUND_PUBLIC_BASE_URL`，或一个 https 的 `FUND_ALLOWED_ORIGINS` 首项）。

## 9. 验证

- 单元/集成：`internal/oauth`（PKCE、码单次使用、JWT 失败关闭、密钥持久化、
  CIMD 白名单、作用域协商、完整授权码流程、刷新轮换、元数据）、
  `internal/httpapi`（六条 discovery 路径、SPA 兜底不吞 well-known、
  authorize 四种决策、同意页流程、令牌端点、MCP 集成、静态 key 回归、
  跨受众令牌拒绝、`safeOAuthReturn`、头注入清洗）。
- 端到端：`scripts/smoke-oauth.sh <base-url> <password>`，11 节 58 项断言，
  已接入 CI（`OAuth MCP connector smoke`）。本地 Linux 实跑 58/58 通过。
- race：`go test ./... -race` 全绿（`keySet`/`codeStore`/`consentStore`/CIMD 缓存四把锁）。

## 10. 未做与后续

- **Company Knowledge / deep research 兼容层**（只读 `search` + `fetch` 工具，
  返回 `{results:[{id,title,url}]}` 与 `{id,title,text,url}`）未实现——
  当前目标是自定义连接器工具调用，不是知识库检索。需要时再加，
  注意 `url` 必须非空字符串才会生成引用。
- 令牌撤销列表（access token 提前失效）未做：访问令牌 1h 短寿，
  撤销语义由 refresh token 撤销 + 会话失效覆盖。
- 多用户/多主体不在范围内（D10 单租户边界）。
