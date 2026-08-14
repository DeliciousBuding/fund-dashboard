#!/usr/bin/env bash
# Local unit gate: go vet + go test + web vitest.
# Usage: ./scripts/test-unit.sh
# Exit non-zero on first failure.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "==> go vet ./..."
go vet ./...

echo "==> go test ./... -count=1"
go test ./... -count=1

echo "==> packages/web vitest"
if [[ -f package.json ]]; then
  if command -v npm >/dev/null 2>&1; then
    npm test --silent
  else
    echo "npm not found" >&2
    exit 1
  fi
else
  echo "package.json missing" >&2
  exit 1
fi

echo
echo "✓ unit gate green (go vet + go test + vitest)"
