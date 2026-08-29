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
- 合并到 `main` 后自动触发 CI：`test-go` / `build-go` / `smoke-e2e`，全绿后构建多架构镜像推送到 GHCR

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

1. 确认所有改动已合入 `main`，本地切到 `main` 并拉取最新
2. 运行发布脚本：

   ```bash
   ./scripts/release.sh 2.0.0
   ```

   脚本会：把 `CHANGELOG.md` 的 `[Unreleased]` 归入 `## [2.0.0] - YYYY-MM-DD`、补一个新的空 `[Unreleased]` 段、提交、打 tag `v2.0.0`、推送。

3. tag 推送触发 GitHub Actions：
   - `ci.yml` 构建 `amd64` + `arm64` 镜像并合成 multi-arch manifest 推送到 GHCR
   - `release.yml` 从 `CHANGELOG.md` 提取对应版本段作为 release notes，创建 GitHub Release

发布后可在 [Releases](https://github.com/DeliciousBuding/fund-dashboard/releases) 查看更新内容。

## Issue

提交 Issue 时请选择对应模板（Bug 报告 / 功能请求），描述复现步骤或使用场景。
