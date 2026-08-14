#!/usr/bin/env bash
#
# 发布脚本：把 CHANGELOG 的 [Unreleased] 归入新版本，打 tag 并推送。
# 用法：./scripts/release.sh <x.y.z>   例如 ./scripts/release.sh 2.0.0
#
# tag 推送后，GitHub Actions 会：
#   - ci.yml    构建 amd64/arm64 镜像并合成 multi-arch manifest 推送到 GHCR
#   - release.yml 从 CHANGELOG.md 提取版本段作为 release notes 创建 GitHub Release
set -euo pipefail

version="${1:-}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "用法: $0 <x.y.z>（语义化版本，例如 2.0.0）" >&2
  exit 1
fi

tag="v$version"
today="$(date +%F)"
changelog="CHANGELOG.md"

# 必须在 main 分支，且工作区干净
branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$branch" != "main" ]]; then
  echo "请在 main 分支上发版（当前：$branch）" >&2
  exit 1
fi
if ! git diff --quiet; then
  echo "工作区有未提交改动，请先提交或暂存" >&2
  exit 1
fi

# 更新 CHANGELOG：[Unreleased] -> [x.y.z] - 日期，并在其上方保留新的空 [Unreleased]
python3 - "$version" "$today" "$changelog" <<'PY'
import sys

version, today, path = sys.argv[1], sys.argv[2], sys.argv[3]
text = open(path, encoding="utf-8").read()
marker = "## [Unreleased]"
if marker not in text:
    sys.exit("CHANGELOG.md 缺少 [Unreleased] 段")

replacement = f"{marker}\n\n## [{version}] - {today}\n"
text = text.replace(marker, replacement, 1)
open(path, "w", encoding="utf-8").write(text)
print(f"CHANGELOG: [Unreleased] -> [{version}] - {today}")
PY

git add "$changelog"
git commit -m "chore(release): $version"
git tag "$tag"
git push origin main
git push origin "$tag"

echo
echo "已推送 tag $tag。GitHub Actions 将自动创建 Release 并构建镜像。"
