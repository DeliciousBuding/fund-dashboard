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

echo
echo "✓ coverage artifacts in $OUT (observe-only; no threshold gate)"
