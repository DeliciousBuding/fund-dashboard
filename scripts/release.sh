#!/usr/bin/env bash
#
# 发布脚本（两段式）：把 CHANGELOG 的 [Unreleased] 归入新版本，合并后打 tag。
#
#   ./scripts/release.sh <x.y.z>        # 1) 归版：开 chore/release-<v> 分支并提交，能提 PR 就提
#   ./scripts/release.sh --tag <x.y.z>  # 2) 归版 PR 合并进 main 后：打 tag 并推送
#
# 为什么是两段：本仓 main 开了分支保护（required_pull_request_reviews +
# enforce_admins=true），服务端拒绝直推。旧版脚本归版后直接 commit 到 main 再
# `git push origin main`，在受保护仓上必然失败；而且失败时 main 上已经留下一个推不
# 出去的本地提交，只能 reset --hard 回退——那正是全局 hook 的硬阻断项。所以这里从第
# 一步起就在分支上提交：任何路径都不需要在 main 上撤销任何东西。
#
# 环境变量：
#   RELEASE_CO_AUTHOR   可选。设了就在归版提交上加一条 Co-authored-by trailer
#                       （agent 代发版时留痕用；人工发版留空即可）。
#
# 推送 tag 后 GitHub Actions 会：
#   - release.yml（push tag 触发）跑 validate → test → image-smoke，再从 CHANGELOG.md
#     抽出 "## [x.y.z]" 段创建 GitHub Release（该 job 有 contents: write）
#   - 版本镜像复用 main push 的 SHA 镜像
#
# 归版提交只改 CHANGELOG.md，不进 runtime 镜像（deploy/Dockerfile 的 runtime 阶段只有
# `COPY --from=go-build /out/${BIN}`），所以二进制逐字节不变、无需为发版本身重部署。
# 但同一批里若还带了 .go 改动就要照常滚 pin：Go 把源码内容哈希进 per-package build
# ID，连注释变更都会改二进制 sha256。
set -euo pipefail

changelog="CHANGELOG.md"
today="$(date +%F)"

die() { echo "$*" >&2; exit 1; }

require_main() {
  local branch
  branch="$(git rev-parse --abbrev-ref HEAD)"
  [[ "$branch" == main ]] || die "请在 main 分支上操作（当前：$branch）"
  git diff --quiet || die "工作区有未提交改动，请先提交或暂存"
  git diff --cached --quiet || die "暂存区有未提交改动，请先提交或重置"
}

sync_main() {
  git fetch origin --quiet --tags
  local here there
  here="$(git rev-parse HEAD)"
  there="$(git rev-parse origin/main)"
  [[ "$here" == "$there" ]] || die "本地 main ${here:0:7} != origin/main ${there:0:7}；先 git pull --ff-only 再发版"
}

reject_existing_tag() {
  local tag="$1"
  if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
    die "本地已存在 tag $tag"
  fi
  if git ls-remote --exit-code --tags origin "refs/tags/$tag" >/dev/null 2>&1; then
    die "远端已存在 tag $tag（版本号不可复用，换一个）"
  fi
}

# 与 scripts/seed-ci-db.sh 同一约定：python3 优先，回落 python。
pick_python() {
  if command -v python3 >/dev/null 2>&1; then echo python3
  elif command -v python >/dev/null 2>&1; then echo python
  else die "需要 python3 或 python 来归并 $changelog"
  fi
}

commit_release() {
  local version="$1"
  if [[ -n "${RELEASE_CO_AUTHOR:-}" ]]; then
    git commit -m "chore(release): $version" -m "Co-authored-by: $RELEASE_CO_AUTHOR"
  else
    git commit -m "chore(release): $version"
  fi
}

# ── 第二段：打 tag ──────────────────────────────────────────────────────────
if [[ "${1:-}" == --tag ]]; then
  version="${2:-}"
  [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "用法: $0 --tag <x.y.z>"
  tag="v$version"
  require_main
  sync_main
  reject_existing_tag "$tag"
  head_changelog="$(git show "HEAD:$changelog")"

  # 守卫 1：tag 必须指向 CHANGELOG 里真有该版本段的提交，否则 release.yml 抽不到
  # 发布说明，只能落到「自动生成的发布」兜底文案。
  grep -q "^## \[$version\]" <<<"$head_changelog" \
    || die "HEAD 的 $changelog 里没有 \"## [$version]\" 段——归版 PR 还没合进 main？"

  # 守卫 2：归版必须已经做完。[Unreleased] 里若还有实质条目，这些改动会被算进一个
  # 根本没描述它们的 tag。本仓 2.0.0 就有这个隐患：它的 CHANGELOG 段停在 e0b554f
  # （PR #16 文档归版），而其上 [Unreleased] 又堆了 PR #25..#29——此时给 HEAD 打
  # v2.0.0 在语法上完全合法，语义上却把未发布的连接器工作贴上了旧版本号。
  pending="$(awk '
    /^## \[Unreleased\]/ { inu = 1; next }
    /^## \[/              { inu = 0 }
    inu && /^- /           { c++ }
    END { print c + 0 }' <<<"$head_changelog")"
  [[ "$pending" == 0 ]] \
    || die "[Unreleased] 里还有 $pending 条未归版条目；先跑 $0 <x.y.z> 归版、合并 PR，再 --tag"

  # 守卫 3：请求的版本必须是文件里最新的那个版本段。给旧版本补 tag 时，那个版本的树
  # 早已不是 HEAD，tag 会指向错误内容——要补请显式对引入该段的提交打 tag。
  latest="$(awk '/^## \[/ && $0 != "## [Unreleased]" {
                  sub(/^## \[/, ""); sub(/\].*$/, ""); print; exit
                }' <<<"$head_changelog")"
  [[ "$latest" == "$version" ]] \
    || die "$changelog 里最新的版本段是 [$latest]，不是 [$version]；不给旧版本打 tag（要补请显式对引入该段的提交打）"

  git tag "$tag"
  git push origin "$tag"
  echo
  echo "已推送 tag $tag -> $(git rev-parse --short HEAD)"
  echo "release.yml 将创建 GitHub Release（发布说明取自 $changelog 的 [$version] 段）。"
  echo "跟踪：gh run list --workflow=release.yml --limit 3"
  exit 0
fi

# ── 第一段：归版 + 开 PR ────────────────────────────────────────────────────
version="${1:-}"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] \
  || die "用法: $0 <x.y.z>（归版并开 PR） | $0 --tag <x.y.z>（合并后打 tag）"
tag="v$version"
require_main
sync_main
reject_existing_tag "$tag"

if grep -q "^## \[$version\]" "$changelog"; then
  die "$changelog 里已有 \"## [$version]\" 段：该版本号已被用过。
  本仓 2.0.0 就是「只有 CHANGELOG 段、从未打 tag」的先例，别再制造第二个。
  要给 2.0.0 补 tag 请显式对引入该段的那个提交打 tag，不要用本脚本重新归版。"
fi

branch="chore/release-$version"
if git show-ref --verify --quiet "refs/heads/$branch"; then
  die "本地已存在分支 $branch（先删掉或改名）"
fi
if git ls-remote --exit-code --heads origin "$branch" >/dev/null 2>&1; then
  die "远端已存在分支 $branch（上一次归版没收尾？）"
fi

pybin="$(pick_python)"
git checkout -b "$branch"

"$pybin" - "$version" "$today" "$changelog" <<'PYEOF'
import sys

version, today, path = sys.argv[1], sys.argv[2], sys.argv[3]
text = open(path, encoding="utf-8").read()
marker = "## [Unreleased]"
count = text.count(marker)
if count != 1:
    sys.exit(f"{path} 的 [Unreleased] 头数量为 {count}（必须恰好 1 个；重复头请先手工合并，否则发版归并会错位）")

# 归版：本次条目落到 [x.y.z] - 日期，其上方保留一个新的空 [Unreleased]。
replacement = f"{marker}\n\n## [{version}] - {today}\n"
text = text.replace(marker, replacement, 1)
open(path, "w", encoding="utf-8").write(text)
print(f"CHANGELOG: [Unreleased] -> [{version}] - {today}")
PYEOF

git add "$changelog"
commit_release "$version"
git push -u origin "$branch"

body="$(cat <<'BODYEOF'
把 `[Unreleased]` 归入 `## [@VERSION@ - @TODAY@]`，并在其上方保留新的空 `[Unreleased]`。

**只改 `CHANGELOG.md`**：它不进 runtime 镜像（`deploy/Dockerfile` 的 runtime 阶段只有
`COPY --from=go-build /out/$BIN /app/$BIN`），所以二进制逐字节不变，**不需要为发版本身重部署**。

合并后打 tag：

```bash
./scripts/release.sh --tag @VERSION@
```

tag 推送触发 `release.yml`：validate -> test -> image-smoke -> github-release，
发布说明自动取自 `CHANGELOG.md` 的 `[@VERSION@]` 段。
BODYEOF
)"
body="${body//@VERSION@/$version}"
body="${body//@TODAY@/$today}"

echo
if command -v gh >/dev/null 2>&1; then
  if gh pr create --base main --head "$branch" --title "chore(release): $version" --body "$body"; then
    echo "PR 已创建。合并后运行：./scripts/release.sh --tag $version"
  else
    echo "gh pr create 失败——请手工开 PR（base main, head $branch），合并后运行：./scripts/release.sh --tag $version" >&2
  fi
else
  echo "未找到 gh：请手工开 PR（base main, head $branch），合并后运行：./scripts/release.sh --tag $version"
fi

git checkout main
echo "已切回 main；main 未被改动，归版提交在 $branch 上。"