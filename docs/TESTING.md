# TESTING — fund-dashboard test system
最后更新：2026-07-21 15:50

> 本文件回答：测什么、默认跑什么、何时加宽、关键路径对应哪条检查。

## 1. Pyramid（分层）

```text
        /  prod smoke   \     post-deploy / hotswap（真环境）
       / optional full e2e \  手工或 workflow_dispatch full_e2e
      /  container smoke-e2e \  CI 硬门禁（seed DB + Docker + Playwright smoke）
     /   contract / httptest    \  Go httpapi·MCP 形状与鉴权
    /____ unit (Go + Vitest) ____\  默认本地与 PR 最快反馈
```

| Layer | Tool | Owns | Does **not** own |
|-------|------|------|------------------|
| **Unit** | `go test` · Vitest | 纯逻辑、service、组件条件渲染、stub 外部 I/O | 真浏览器布局、真 OIDC cookie |
| **Contract** | Go `httptest` / MCP tests | 状态码、JSON 字段、fail-closed 鉴权 | 视觉像素 |
| **Container smoke** | CI `smoke-e2e` + `e2e/smoke.spec.ts` | 镜像能起、健康检查、核心 API、SPA 壳 | 全站点击矩阵 |
| **Full e2e** | Playwright `e2e/*` | 导航/详情等宽路径（可选） | 默认 PR 门禁 |
| **Prod smoke** | `scripts/smoke-prod.sh` | 线上鉴权、MCP 44/26、AgentOps、build_version pin | 全量 crawl（默认 skip） |

## 2. 默认命令（本地）

```bash
# 一键 unit 门禁（推荐日常）
./scripts/test-unit.sh
# 等价：
go vet ./...
go test ./... -count=1
npm test   # packages/web vitest

# 生产热换后
./scripts/smoke-prod.sh

# 可选：完整 Playwright（需服务已起）
cd packages/web && CI=1 BASE_URL=http://127.0.0.1:8080 npx playwright test e2e/

# 可选：覆盖率观察（无失败阈值）
./scripts/test-cover.sh
```

| 场景 | 跑什么 |
|------|--------|
| 改 Go service / jobs | `go test ./...` 或包级 `go test ./internal/...` |
| 改 UI 组件 | `npm test` |
| 改鉴权 / MCP 合同 | Go contract tests + 部署后 smoke-prod |
| 改 Dockerfile / 入口 | CI smoke-e2e 或本地 compose CI |
| 大 UI 波次 | 额外 full Playwright / 人工点检 |

## 3. CI 合同

见 `.github/workflows/ci.yml`：

1. `test-go` → `build-go` → `test-web` → `build-web`
2. **`smoke-e2e`**（seed → image → compose → curl auth → **仅** `e2e/smoke.spec.ts`）
3. `build-and-push` **依赖** smoke-e2e 绿
4. **Full e2e** 仅 `workflow_dispatch` + `full_e2e=true`，**不**挡 PR

**明确禁止**：把 full Playwright 或真上游 crawl 设为默认 PR 门禁（效率回退）。

## 4. Critical-path map

| Critical path | Automated check | Notes |
|---------------|-----------------|-------|
| `/api/health` facts-only | `internal/httpapi/health_test.go` · e2e smoke · smoke-prod | 无 version 泄露生产策略 |
| Admin Bearer fail-closed | `admin_*_test.go` · smoke-e2e curl 401 · smoke-prod | 空 key fail-closed |
| MCP tools/list op 44 / pub 26 | `internal/mcp/*_test.go` · smoke-prod | 计数变更必须改文档+测 |
| AgentOps prepare + single-use | `agent_confirmations_test.go` · smoke-prod | bare `confirmed=true` 拒绝 |
| recalculate-snapshot `failed_codes[]` | admin/jobs tests · smoke-prod all-mode | 恒数组 |
| Market indices cache / sort | `market_indices_test.go` · `market_test.go` | map 序稳定 |
| SQLite open WAL (writable) | `repository/db/open_sqlite_test.go` · sqlitedb open_test | fresh DB 不退 DELETE |
| NAV upsert transactional | jobs tests | 与 holdings 一致 |
| SPA shell loads | Vitest components · Playwright smoke | prod SPA 需 OIDC session 才深测 |
| `system.build_version` pin | smoke-prod admin dashboard | 拒绝 dev/latest |
| EdgeKey write paths | smoke-prod 条件 · edge_auth tests | 无 session 时 skip 写 |
| OIDC unauth → 302 | smoke-prod harness/agent-context | 深度 SPA 需 `SMOKE_TD_SESSION` |

## 5. 编写用例指南

### Go unit / contract
- 表驱动；外部 HTTP **必须** stub（`fetchIndexFn` 等）。
- DB：优先 `:memory:` / temp file；共享 helper 见 `internal/testutil`（Phase B）。
- HTTP：`httptest` + `NewRouter`；鉴权矩阵每条路径至少 401/200 一对。
- 并发行为（singleflight 等）用短 sleep + 计数，避免 flaky barrier。

### Vitest / Testing Library
- 测行为与可访问名，少绑具体 CSS class。
- Kumo 经 vitest alias mock（`vitest.config.ts`）。
- 图表：测数据变换与 fallback，不测 canvas 像素。

### Playwright
- **默认维护 `e2e/smoke.spec.ts`**。
- 优先 `getByRole` / `data-testid`；避免固定 `waitForTimeout` 作为唯一等待。
- full suite 允许 skip 当侧栏文案缺失；不要把 skip 当绿掩盖回归。

### smoke-prod
- 默认全量 operator（有 key 时）；可用分段 flag（Phase C）做快速公网检查。
- **禁止**把真实 key 打进日志或提交。

## 6. Anti-patterns

| 不要 | 要 |
|------|-----|
| PR 默认 full e2e | unit + CI smoke |
| e2e 断言 HTML 长度当业务正确 | 合同字段 / role 文本 |
| 在 unit 里打真 Eastmoney/Yahoo | stub |
| 复制整段 CREATE TABLE 到每个测试 | testutil / seed |
| 用 HANDOFF/历史 MASTER 当测试政策 | 本文件 + AGENTS 验证段 |

## 7. 边界

`go test` / `smoke-prod` 是本仓库唯一测试权威；Bun 时代测试已随旧代码移除。
