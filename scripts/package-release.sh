#!/usr/bin/env bash
# Build reproducible release archives for GitHub Releases (pure Go).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${1:-${GITHUB_REF_NAME:-}}"
if [ -z "$VERSION" ]; then
  VERSION="$(git describe --tags --always 2>/dev/null || git rev-parse --short HEAD)"
fi

SHA="$(git rev-parse HEAD)"
OUT_DIR="dist/release"
STAGE_DIR="$OUT_DIR/stage"
BIN_DIR="$STAGE_DIR/bin"

rm -rf "$OUT_DIR"
mkdir -p "$BIN_DIR" "$STAGE_DIR/web" "$STAGE_DIR/deploy" "$STAGE_DIR/meta"

echo "Building Go binary..."
mkdir -p bin
CGO_ENABLED=0 go build -o "bin/fund-dashboard" -ldflags="-s -w" ./cmd/fund-dashboard/
cp bin/fund-dashboard "$BIN_DIR/fund-dashboard"

if [ ! -f packages/web/dist/index.html ]; then
  echo "Building web..."
  npm ci
  npm run build --workspace packages/web
fi
cp -R packages/web/dist "$STAGE_DIR/web/dist"

cp deploy/docker-compose.yml "$STAGE_DIR/deploy/docker-compose.yml"
cp deploy/docker-compose.ci.yml "$STAGE_DIR/deploy/docker-compose.ci.yml"
cp deploy/Dockerfile "$STAGE_DIR/deploy/Dockerfile"
cp deploy/.env.example "$STAGE_DIR/deploy/.env.example"
cp deploy/deploy.sh "$STAGE_DIR/deploy/deploy.sh"
cp deploy/rollback.sh "$STAGE_DIR/deploy/rollback.sh"
cp deploy/nginx-fund.conf "$STAGE_DIR/deploy/nginx-fund.conf"
cp deploy/README.md "$STAGE_DIR/deploy/README.md"
if [ -f deploy/hermes/mcp.json ]; then
  cp deploy/hermes/mcp.json "$STAGE_DIR/deploy/hermes-mcp.json"
fi
if [ -f deploy/ci-seed.sql ]; then
  cp deploy/ci-seed.sql "$STAGE_DIR/deploy/ci-seed.sql"
fi
if [ -f scripts/seed-ci-db.sh ]; then
  cp scripts/seed-ci-db.sh "$STAGE_DIR/deploy/seed-ci-db.sh"
fi

cat > "$STAGE_DIR/meta/release.json" <<EOF
{
  "version": "$VERSION",
  "git_sha": "$SHA",
  "runtime": "go",
  "binary": "bin/fund-dashboard",
  "web": "web/dist"
}
EOF

(
  cd "$STAGE_DIR"
  tar -czf "../fund-dashboard-bin-${VERSION}.tgz" bin
  tar -czf "../fund-dashboard-web-${VERSION}.tgz" web
  tar -czf "../fund-dashboard-deploy-${VERSION}.tgz" deploy meta
)

(
  cd "$OUT_DIR"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./*.tgz > SHA256SUMS
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 ./*.tgz > SHA256SUMS
  fi
)

echo "Release packages written to $OUT_DIR"
ls -la "$OUT_DIR"
