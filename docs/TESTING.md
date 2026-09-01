# TESTING — fund-dashboard test system
最后更新：2026-08-31

> 本文件回答：测什么、默认跑什么、何时加宽、关键路径对应哪条检查。
> 当前仓库同时包含 Go 后端、`web/` React 前端、共享 zod 契约与 go:embed 单镜像交付。

## 1. Pyramid（分层）

```text
        / production acceptance \   post-deploy / hotswap（真环境）
       / Chromium browser E2E    \  登录、导航、路由 chunk、渲染状态
      / container API/MCP smoke   \ seed DB + Docker + auth contracts
     / contract / component tests  \ Go httptest + Vitest
    /_____ unit + static checks _____\ Go / TypeScript / Biome
```

| Layer | Tool | Owns | Does **not** own |
|-------|------|------|------------------|
| **Unit** | `go test` / Vitest | service 逻辑、计算、前端纯函数、stub I/O | 真浏览器布局 |
| **Contract** | Go `httptest` / MCP tests | 状态码、JSON 字段、fail-closed 鉴权 | 视觉与 chunk 加载 |
| **Browser E2E** | Playwright Chromium | setup/login、受保护壳、桌面/移动导航、intent preload、直接点击加载反馈、opacity 回归、console error gate | 真生产凭据与外部数据源 |
| **Container smoke** | CI `smoke-e2e` | 镜像能起、API/MCP、内嵌 SPA 与浏览器关键路径 | 全站视觉像素比较 |
| **Production acceptance** | 私有运维 Playwright + `smoke-prod.sh`（私有运维仓，非本仓） | 线上镜像、入口、真实数据与回滚锚 | PR 默认门禁 |

## 2. 默认命令（本地）

```bash
# Go
./scripts/test-unit.sh
go test ./... -count=1

# Web unit / typecheck / build
pnpm install --frozen-lockfile
pnpm -C web test
pnpm exec biome check .
pnpm -C web build

# 浏览器 E2E：先启动 deploy/docker-compose.ci.yml，密码与环境变量一致
pnpm exec playwright install chromium
E2E_BASE_URL=http://127.0.0.1:8080 \
E2E_PASSWORD=ci-smoke-password-1 \
pnpm test:e2e

# 生产热换后：在私有运维仓运行 fund-dashboard/scripts/smoke-prod.sh（本仓无此脚本），命令见私有仓 README。
```

| 场景 | 跑什么 |
|------|--------|
| 改 Go service / jobs | `go test ./...` 或包级 `go test ./internal/...` |
| 改鉴权 / MCP 合同 | Go contract tests + container smoke |
| 改前端路由 / 壳 / 登录 | Vitest + web build + Playwright |
| 改 Dockerfile / CI 入口 | 完整 `smoke-e2e` |
| 改生产部署 | 私有运维 smoke + 四层 POSTCHECK |

## 3. CI contract

见 `.github/workflows/ci.yml`：

1. `test-go`：gofmt / vet / race tests。
2. `build-go`：静态 Go build。
3. `test-web`：Biome + Vitest。
4. `build-web`：TypeScript + Vite + go:embed 输出。
5. `smoke-e2e`：seed → image → compose → API/MCP auth → Playwright Chromium。
6. `build-and-push` 仅在 main push（或仅限 main 的 workflow_dispatch）且上述门禁成功后构建双架构镜像。

Playwright 失败时上传 `test-results/`（trace / screenshot / video / HTML report），保留 7 天。

## 4. Browser E2E contract

`playwright.config.ts` + `e2e/` 是浏览器测试 SSOT：

- global setup 读取 `/api/auth/status`，按状态执行 setup 或 login，并把 session cookie 写入隔离的 storage state；密码只从 `E2E_PASSWORD` 注入。
- 未认证测试清空 storage state，真实提交登录表单并断言跳转 `/` 与侧栏出现。
- intent preload 测试在 hover 后、click 前观察目标 route chunk，并断言 click 不重复下载。
- direct-click 测试不触发 hover，人工延迟 route chunk，断言顶部 progress 立即出现；桌面与 390px 移动视口都覆盖。
- 每次路由落稳后断言 `route-content` 最终 `opacity=1`，防止后台 rAF / 动画回归。
- console `error` 与 `pageerror` 默认视为失败；第三方浏览器扩展不参与 CI 浏览器环境。

## 5. Critical-path map

| Critical path | Automated check | Notes |
|---------------|-----------------|-------|
| `/api/health` facts-only | `internal/httpapi/health_test.go` · smoke-e2e · smoke-prod | 生产不泄露 version |
| setup/login/session redirect | auth endpoint tests · Playwright auth case | 覆盖 fresh auth cache 回归 |
| desktop/mobile route feedback | Playwright direct-click cases | chunk 延迟期间 progress 可见 |
| route preload | Playwright hover case | 目标 route chunk click 前到达且只请求一次 |
| route content opacity | Playwright route matrix | 每页最终 opacity=1 |
| Admin Bearer fail-closed | admin tests · smoke-e2e curl 401 · smoke-prod | 空 key fail-closed |
| MCP tools/list | `internal/mcp/*_test.go` · smoke-e2e · smoke-prod | 工具合同变更需同步测试 |
| AgentOps prepare + single-use | agent confirmation tests · smoke-prod | bare `confirmed=true` 拒绝 |
| SQLite open WAL | repository DB tests | fresh DB 不退 DELETE |
| NAV upsert transactional | jobs tests | 与 holdings 一致 |
| EdgeKey write paths | browser write auth tests · smoke-prod 条件 | 新部署可关闭兼容层 |

## 6. Authoring guidelines

### Go unit / contract
- 表驱动；外部 HTTP 必须 stub。
- DB 优先 `:memory:` / temp file；共享 helper 使用 `internal/testutil`。
- HTTP 使用 `httptest` + `NewRouter`；鉴权路径至少覆盖拒绝/成功。

### Web unit / browser
- 可纯函数化的计算留在 Vitest，不用浏览器放大测试成本。
- 浏览器 locator 优先 role / label；`data-testid` 只用于无稳定语义节点（如 route transition/progress）。
- 慢网络用 route interception 可控模拟，不依赖真实 CDN 或 sleep 猜测。
- 上下文禁用 Service Worker（`serviceWorkers: "block"`）：SW 预缓存会绕过 route interception，慢 chunk 场景必须走真实网络。
- 测试不得提交生产域名、账号、cookie、密码或 storage state；`test-results/` 已忽略。

## 7. Anti-patterns

| 不要 | 要 |
|------|-----|
| PR 打真实上游或生产域名 | seed DB + 本地容器 |
| 用 hover click 假装覆盖移动端直接点击 | `dispatchEvent("click")` + delayed chunk |
| 只断言 URL，不看旧页面透明/加载反馈 | progress + opacity 双断言 |
| 把 storage state / trace 提交仓库 | 输出到 `test-results/` |
| 在 unit 里打 Eastmoney/Yahoo | stub |
| 用 handoff 当测试政策 | 本文件 + CI + `e2e/` |
