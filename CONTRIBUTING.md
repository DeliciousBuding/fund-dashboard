# 贡献指南

感谢关注本项目。本文档说明如何参与开发、提交代码与发布版本。

## 开发流程（GitHub Flow）

本项目采用单 `main` 分支 + 功能分支的轻量流程：

1. 从 `main` 拉出功能分支（`git checkout -b feature/xxx`）
2. 在功能分支上开发并提交
3. 推送分支（`git push -u origin feature/xxx`）
4. 打开 Pull Request 回 `main`
5. CI 全绿后合并，删除功能分支

约定：

- 分支命名：`feature/`（新功能）、`fix/`（缺陷）、`refactor/`（重构）、`docs/`（文档）、`chore/`（杂项）
- 一个分支只做一件事，便于 review 与回滚
- 合并到 `main` 后自动触发 CI：`test-go` / `build-go` / `test-web` / `build-web` / `smoke-e2e`，全绿后构建多架构镜像推送到 GHCR

## 提交规范

提交信息用「类型: 描述」前缀，与 `CHANGELOG.md` 条目保持一致：

| 前缀 | 用途 |
|------|------|
| `feat` | 新功能 |
| `fix` | 缺陷修复 |
| `refactor` | 重构（无行为变更） |
| `perf` | 性能优化 |
| `docs` | 文档 |
| `test` | 测试 |
| `ci` | CI/CD / 构建 |
| `chore` | 杂项 |

示例：

```
feat: add DCA backtest Monte Carlo simulation

fix: clamp portfolio id to avoid absurd values
```

若涉及用户可见变更，请在 `CHANGELOG.md` 的 `[Unreleased]` 段追加一条扁平条目：

```markdown
- **新功能** 描述
- **修复** 描述
```

## 本地开发

```bash
# 后端
go test ./... -count=1
go vet ./...
```

提交前请确保：

- `gofmt -l .` 无输出（CI 有格式门禁）
- `go test ./...` 全绿
- 新逻辑附带测试

## 发布流程

版本遵循 [语义化版本](https://semver.org/lang/zh-CN/)（`主.次.补丁`）。发布步骤：

1. 确认所有改动已合入 `main`，本地切到 `main`、与 `origin/main` 同步且工作区干净
2. 归版（开 PR，不碰 `main`）：

   ```bash
   ./scripts/release.sh 2.0.0
   ```

   脚本会：把 `CHANGELOG.md` 的 `[Unreleased]` 归入 `## [2.0.0] - YYYY-MM-DD`、补一个新的空 `[Unreleased]` 段，然后在 `chore/release-2.0.0` 分支上提交并推送，装了 `gh` 就顺手开 PR。

   `main` 开着分支保护（required PR reviews + `enforce_admins`），**直推会被服务端拒**，所以归版必须走 PR；脚本从第一步起就在分支上提交，失败也不需要在 `main` 上 `reset --hard` 回退任何东西。

3. PR 合并后打 tag：

   ```bash
   ./scripts/release.sh --tag 2.0.0
   ```

   脚本会先校验：本地 `main` 与 `origin/main` 同步、工作区干净、`v2.0.0` 本地与远端都未被占用、`main` HEAD 的 `CHANGELOG.md` 确有 `## [2.0.0]` 段（否则 `release.yml` 抽不到发布说明），然后对 HEAD 打 tag 并推送。

4. 推送后 GitHub Actions：
   - `ci.yml`（main push / workflow_dispatch）构建 `amd64` + `arm64` 镜像并合成 multi-arch manifest 推送到 GHCR
   - `release.yml`（tag push）运行测试 + 镜像 smoke，并从 `CHANGELOG.md` 提取对应版本段作为 release notes，创建 GitHub Release

设 `RELEASE_CO_AUTHOR="Name <mail>"` 可让归版提交带一条 `Co-authored-by` trailer（agent 代发版时留痕；人工发版留空）。

> 归版提交只改 `CHANGELOG.md`，而它不进 runtime 镜像（`deploy/Dockerfile` 的 runtime 阶段只有 `COPY --from=go-build /out/${BIN}`），所以二进制逐字节不变、**无需为发版本身重部署**。但同一批里若还带了源码改动（`.go` 注释也算）就要照常滚 pin：Go 把源码内容哈希进 per-package build ID，注释变更同样会改二进制 sha256。

发布后可在 [Releases](https://github.com/DeliciousBuding/fund-dashboard/releases) 查看更新内容。

## Issue

提交 Issue 时请选择对应模板（Bug 报告 / 功能请求），描述复现步骤或使用场景。
