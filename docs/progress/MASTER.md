# MASTER — Progress

最后更新：2026-09-04（W1/W2/W3 全量落地：#40/#41 + session 排序修复 #43 + L9 CI #42 + L8 #44；战役收口）

## 战役：反模式整治与升级（2026-09-03）

> 本节是本轮「第一性原理 + 反模式整治」的权威进度面：任务清单、地界、验收命令、合并队列。
> 会话中断后从「执行状态」表继续，不重启分析。战役收尾后本节收敛回上方摘要。

### 基线（派发前主 agent 亲手实测，任何 lane 不得倒退）

| 套件 | 命令 | 基线 |
|------|------|------|
| Go | `go vet ./...` + `go test ./... -count=1` | 全绿（exit 0） |
| web | `pnpm -C web test` | 10 文件 / 85 tests passed |
| contracts | `node --test "packages/contracts/**/*.test.ts"` | 108 pass / 0 fail / 0 skipped |

### 派发决策（我替领导拍的板）

1. 范围 = P0 全量 + P1 全量 + P2 快赢（后端打磨、CI lint/vuln）；「request 侧端点注册表」「jsdom 组件测试基建」本次不做（独立项目级工作量）。
2. XIRR 的 NAV 缺失 fallback 0 改为显式告警+结果标记，不静默。
3. edge-key 兼容层：`FUND_EDGE_AUTH_ENABLED` 默认翻 false，CHANGELOG 标注行为变更。
4. 迁移：自研轻量 `schema_migrations` 版本表，不引第三方迁移库。
5. DCA 补偿回看窗口：常量 7 天，本次不加 env（`internal/config` 由 L1 独占）。
6. 并行策略：git worktree 隔离（`.worktrees/<lane>`），波次合并，合并队列 = 派发顺序。
7. 共享写点唯一归属：CHANGELOG.md / AGENTS.md / docs/design 由主 agent 收尾统一改，各 lane 禁触；go.mod/go.sum、pnpm-lock、web/package.json 各 lane 禁触（不引新依赖）。

### 执行状态

| 波次 | Lane | 分支 | 地界（独占文件域） | 状态 |
|------|------|------|--------------------|------|
| W1 | L1 安全加固 | `lane/p0-security` | `internal/httpapi/{mcp,mcp_auth,router,oauth_pages}.go`、`internal/oauth/jwt*`、`internal/config/` | merged bffa8a5 |
| W1 | L2 DCA 补偿 | `lane/p0-dca` | `internal/service/portfolio/dca_run*`、`internal/jobs/scheduler.go`、`internal/jobs/dca*`（新建） | merged e9afae6 |
| W1 | L3 金额精度 | `lane/p0-precision` | `internal/snapshot/`、`internal/service/portfolio/{xirr,xirr_calc,summary}.go`、`internal/service/admin/integrity.go` | merged 734d230 |
| W2 | L4 契约金样本 | `lane/p1-golden` | `packages/contracts/**`、`internal/httpapi/golden_test.go`（新建）、`testdata/golden/`（新建） | merged 53d8b37（contracts 108→142 pass） |
| W2 | L5 crawl 收敛 | `lane/p1-crawl` | `internal/mcp/tools_read.go`、`tools_helpers.go`、`internal/httpapi/admin_crawl.go`、`internal/jobs/price_refresh.go`、`internal/service/admin/crawl*.go`（新建） | merged 53d8b37（随 W2） |
| W2 | L6 迁移+测试DDL | `lane/p1-schema` | `internal/repository/db/**`、`internal/testutil/`、存量 `*_test.go` 的手抄 DDL 迁移 | merged 73ae1ed |
| W2 | L7 前端 | `lane/p1-frontend` | `web/src/**`（不改 package.json/lockfile） | merged 25b7248 |
| W3 | L8 后端打磨 | `lane/p2-polish` | `internal/mcp/server.go`、`internal/httpapi/{system,admin_dashboard,spa_transactions,admin_transactions,session_auth}.go`、`internal/agentops/`、`internal/jobs/{nav_schema,scheduler}.go`、`internal/contracts/validation.go`、`internal/config/config.go`(edge-key 默认)+ `deploy/{docker-compose.yml,.env.example}`/`docs` 同步 | merged 5137b2f（随 PR #44） |
| W3 | L9 CI | `lane/p2-ci` | `.github/workflows/ci.yml`、`deploy/Dockerfile` | merged 25e6626（随 PR #42） |

### 落地机制（2026-09-04 实测，纠正原计划）

- **`main` 是受保护分支**：直推被拒（`GH006: Protected branch update failed`，"Changes must be made through a pull request"，`5 of 5 required status checks are expected`）。
- 必需检查 = `test-go` / `build-go` / `test-web` / `build-web` / `smoke-e2e`；**任何 lane 不得重命名或删除这五个 job id**，重命名即静默破坏分支保护、卡死所有 PR。
- 因此「本地合并进 main 再 push」的原始收尾方式不成立：本地 `main` 只作为**集成与复跑验收面**，真正落地必须走 PR。
- 落地队列：W1 → PR #40（`fix/p0-security-dca-precision` @ 734d230）。W2 收口后另开 PR；W3 同。
- 上一会话遗留：W1 三个 lane 已合并进本地 main 但从未落地（本地 main 领先 origin/main 12 个提交），原因是撞上分支保护。本会话已通过 PR #40 修正。
- push 时 `leak-guard` 对 `internal/config/config_test.go`、`internal/httpapi/oauth.go`、`oauth_consent_origin_test.go` 报的是**软提醒**（测试常量与注释散文），非阻断项，已确认为误报。

### 验收命令（明卷；合并后主 agent 在 main 复跑）

```bash
# CI 的 test-go 先跑 gofmt 门禁，再 vet，再 -race 测试；本清单原先漏掉 gofmt，
# 直接导致 PR #40 首跑 test-go 失败（本地 vet/test 全绿仍被 format check 拦下）。
gofmt -l $(git ls-files '*.go')   # 必须为空
go vet ./... && go test ./... -count=1
pnpm -C web test
pnpm -C web build
node --test "packages/contracts/**/*.test.ts"
CGO_ENABLED=0 go build -o /dev/null ./cmd/fund-dashboard/
```

- `-race` 只在 CI（ubuntu）有效：本机 Windows 跑 `-race` 会 `ThreadSanitizer failed to allocate`，属环境限制，不是代码缺陷；不得据此声称本地已验证 race。
- `internal/webui/dist` 只跟踪 `.gitkeep`，`pnpm -C web build` 产物被 gitignore，不会脏化工作区。

### 当前实测基线（2026-09-04，本地 main = W1+L4+L6+L7，commit 25b7248）

| 套件 | 结果 | 相对派发前基线 |
|------|------|----------------|
| `gofmt -l $(git ls-files '*.go')` | 空 | 新增门禁项 |
| `go vet ./...` | exit 0 | 持平 |
| `go test ./... -count=1` | 25/25 packages ok，0 fail | 持平 |
| `CGO_ENABLED=0 go build ./cmd/fund-dashboard` | exit 0 | 持平 |
| `pnpm -C web test` | 11 文件 / 92 tests passed | 10/85 → 11/92（L7 +1 文件 +7 tests） |
| `pnpm -C web build` | exit 0 | 持平 |
| `node --test packages/contracts` | 142 pass / 0 fail / 0 skipped | 108 → 142（L4 +34 golden tests） |

CI 侧（PR #40，ubuntu，含 race）：`test-go` pass 2m27s、`build-go` pass。

### 全局约束（写入每份任务书）

- 公开仓库：禁止提交主机名/公网 IP/真实密钥/个人邮箱/本地绝对路径。
- 遵守 AGENTS.md S.U.P.E.R：路由只做协议适配；业务进 internal/service；REST 与 MCP 共用 service。
- API 错误统一 `{error: string}`；MCP 工具描述中文；SQL 全参数化；日期 YYYY-MM-DD。
- 基线不可退：测试数只增不减、skipped=0、禁删测试/禁松断言/禁 `|| true`。
- 不改公共 API 形状（除任务书明确要求的字段）。

### 本会话收口记录（2026-09-04 主 agent）

- **W2 遗留一个 flaky golden（已修，#43）**：`/api/auth/sessions` 两条会话的 `last_seen_at` 同秒打平，`ORDER BY last_seen_at DESC` 无 tiebreaker，且 `current` 由随机 token 决定，导致已提交 golden 样例在两次 CI 运行间翻转、`test-go` 在**每个 PR** 上随机变红。修复 = store 加 `id DESC` 总序（双方言主键可移植）+ service 把当前会话恒定锚首位；不同秒会话顺序不变。已加 store 级回归测试（红→绿证明确实抓得住）。
- **L9 lint-go 集成期暴露 stylecheck ST1019**（4 个测试文件 `internal/repository/db` 双别名 import，因局部变量 `db` 遮蔽包别名）。处理 = 去掉重复 import、保留 stylecheck，而不是从策划集里删掉它（删掉会让闸门再也抓不到下一处重复导入）。
- **CI `audit-web` 为 advisory**：因 npm audit 端点从 GitHub runner 稳定 `ERR_SOCKET_TIMEOUT`（第三方网络抖动），基本每次都会红，但不阻塞（continue-on-error）。记录为已知限制，不是代码漏洞。
- **lint 集合计数是漂移源**：`golangci-lint` 报告与组合相关（`predeclared` 单独 9 vs 与 `revive` 同开 0），且默认 `--max-same-issues=3` 会截断重复——per-linter 独立跑 + 双 limit 0 才是可靠 oracle。计数不入 CI 配置，改为指向 backlog。

---

- 当前形态：Go 后端 + `web/` React SPA + `packages/contracts`，前端由 `go:embed` 内嵌为单二进制/单镜像。
- W0–W7 已实施：session 鉴权、SQLite/PG 双驱动、REST/MCP、投资组合页面、分析/回测/穿透、系统工作台与审计均在主线。
- 当前质量门禁：Go format/vet/test-race/build、Web Biome/Vitest/TypeScript/build、容器/API smoke。
- 设计决策与剩余 backlog：[`docs/design/`](../design/README.md)；用户可见变更：[`CHANGELOG.md`](../../CHANGELOG.md)。
- 本文件只保留当前摘要，不记录分支/WIP/生产 pin；实现事实以源码和 CI 为准，live 部署事实由私有运维 SSOT 管理。
