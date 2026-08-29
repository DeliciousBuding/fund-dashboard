#!/usr/bin/env bash
# Local unit gate: go vet + go test.
# Usage: ./scripts/test-unit.sh
# Exit non-zero on first failure.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "==> go vet ./..."
go vet ./...

echo "==> go test ./... -count=1"
go test ./... -count=1

echo
echo "✓ unit gate green (go vet + go test)"
