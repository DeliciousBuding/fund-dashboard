#!/usr/bin/env bash
# Optional coverage (observe-only — does not enforce thresholds).
# Artifacts under .tmp/coverage/ (gitignored).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${COVER_OUT:-$ROOT/.tmp/coverage}"
mkdir -p "$OUT"

echo "==> go test -coverprofile"
go test ./... -count=1 -coverprofile="$OUT/go.cover" -covermode=atomic
go tool cover -func="$OUT/go.cover" | tee "$OUT/go-func.txt"
go tool cover -func="$OUT/go.cover" | tail -1

if [[ -d packages/web ]]; then
  echo "==> vitest coverage (if configured)"
  if npm test --silent -- --coverage 2>"$OUT/vitest-cover.err" | tee "$OUT/vitest-cover.txt"; then
    echo "vitest coverage finished"
  else
    echo "vitest --coverage skipped or failed (non-fatal for observe script):"
    tail -20 "$OUT/vitest-cover.err" || true
  fi
fi

echo
echo "✓ coverage artifacts in $OUT (observe-only; no threshold gate)"
