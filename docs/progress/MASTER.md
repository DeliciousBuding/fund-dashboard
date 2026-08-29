# MASTER — 前端剥离 + 后端收口（2026-08-29）

> 状态：**已完成并提交**（分支 `chore/strip-web-ui`）。本文件是本次改造的跨会话恢复锚点。

## 任务定义

用户目标（captain 调度 / /neat-freak 收口）：

1. **前端彻底剥离**：删 `packages/web`（React/Vite/Playwright/Vitest），仓库变为纯 Go 后端 + zod contracts；交由 Kimi-K3 重构全新前端（oneshot），避免旧代码误导「屎山雕花」。
2. **后端整理**：保留契约（`packages/contracts`）与后端全量；清理过重工程化/死资产；文档分层收口。
3. **质量门禁**：gofmt / vet / build / 全量测试全绿。

## 决策记录

| 决策 | 依据 |
|------|------|
| `packages/web` 整体删除（非改） | 前端要全新重构；旧代码留指针仍会误导 |
| `packages/contracts` 保留 | zod 契约为前后端共享 SSOT，K3 前端复用 |
| `docs/DESIGN.md` 删除 | 纯 UI 设计系统，随前端移出 |
| `docs/go-backend-rewrite/` 删除 | 重写期死资产，零引用 |
| mirror 从「内嵌 SPA」改「API-only」 | compose `FUND_STATIC_DIR=/app/web` 指向不存在目录会启动报错 |
| scheduler 每日 20:00 全刷 + DCA 仅工作日 | 修 QDII T+2 净值滞后（上轮体检发现）；DCA 是财务决策不放周末 |
| Go 内 SPA 注释/`StaticDir` 保留 | 功能性语义 + 可选 dev 特性，删除需动 3 文件，非本轮必要 |

## 验收证据（2026-08-29）

- `gofmt -l .` 干净
- `go vet ./...` 干净
- `go build` OK
- `go test ./...` 全量 19 包全绿（含 `TestTickDailyWindowRefreshesPriceAndSignalsDCAWeekdaysOnly`）
- 残留 grep：仅「已删除/已移出」历史说明
- 文档尺寸：AGENTS 83 行 / 全部 <100 行

## 后续（非本次范围）

- **前端重构**：Kimi-K3 oneshot，独立仓库（或 packages/web 重建但全新代码）
- **迁移 jp1**：CF proxy + compose 一栈（用户方向，未执行）
- **部署**：hk4 仍跑旧镜像（内嵌 SPA），新镜像 API-only 的部署需在迁移时一并处理
