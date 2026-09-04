# fund-dashboard — multi-stage: web (pnpm/Vite) → Go static binary (embeds SPA) → Alpine runtime.
#
# 多产物：ARG BIN 选构建命令（fund-dashboard 默认；fund-migrate 供一次性数据迁移镜像）。
# Build app:  docker build -f deploy/Dockerfile -t ghcr.io/deliciousbuding/fund-dashboard:<sha> .
# Build migr: docker build -f deploy/Dockerfile --build-arg BIN=fund-migrate -t .../fund-migrate:<sha> .
# Env: 见 deploy/.env.example（公网形态：FUND_AUTH_SECURE_COOKIE=true + FUND_ALLOWED_ORIGINS 必配）。

# ── web build: Vite outputs straight into internal/webui/dist (go:embed source)
FROM node:24.20.0-alpine@sha256:e67514e5d0f6c46656005e1b693b2ec9d52e80b641307de684d4a015ba7a4eaf AS web-build
WORKDIR /src
RUN corepack enable
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
# pnpm-workspace.yaml 的 patchedDependencies 需要补丁文件在场（sonner CSP 修复）。
COPY patches/ patches/
COPY web/package.json web/
COPY packages/contracts/package.json packages/contracts/
RUN pnpm install --frozen-lockfile
COPY web/ web/
COPY packages/contracts/ packages/contracts/
RUN pnpm -C web build

# ── go build: static binary, SPA embedded via //go:embed all:dist
FROM golang:1.26.7-alpine@sha256:28d89ee9cc0ff9fec75c82ca201e6bf7fdf9a679d4b7b24dfa04f2bb766bb468 AS go-build

ARG BIN=fund-dashboard
# git stays for the module proxy's `,direct` fallback (default
# GOPROXY=https://proxy.golang.org,direct). It is NOT needed for VCS stamping:
# .dockerignore excludes .git/ (verified absent from the build context) and the
# build below pins -buildvcs=false. It lives only in this discarded build stage,
# so it adds nothing to the shipped image.
RUN apk add --no-cache git

WORKDIR /src

# Cache-friendly ordering, least-frequently-changed first, and only the paths the
# Go toolchain actually reads. `go list ./...` resolves to packages under cmd/
# and internal/ exclusively, and every //go:embed target
# (internal/agenttools/default_registry.json, internal/webui/dist,
# internal/webui/placeholder) lives under internal/. Docs, web sources, e2e and
# deploy files therefore no longer invalidate the Go compile layer.
# Keep this list in sync if a root-level Go package is ever added: the build then
# fails loudly on the missing package instead of silently shipping less.
COPY go.mod go.sum ./
RUN go mod download

COPY internal/ internal/
COPY cmd/ cmd/

# Fresh SPA from the web stage (never trust a locally built dist in context:
# .dockerignore excludes internal/webui/dist, so this COPY is the only source of
# the embedded assets and must stay after the internal/ copy above).
COPY --from=web-build /src/internal/webui/dist ./internal/webui/dist

# -trimpath keeps builder paths out of the binary (reproducible across builders).
# -buildvcs=false removes a failure mode: the context has no .git, so stamping
# can never add information, and nothing in the app reads VCS build settings
# (no debug.ReadBuildInfo / vcs.revision anywhere in the tree).
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /out/${BIN} -ldflags="-s -w" ./cmd/${BIN}/

FROM alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce AS runtime

ARG BIN=fund-dashboard
ENV BIN=${BIN}
# ca-certificates: outbound HTTPS to upstream quote sources.
# tzdata: Asia/Shanghai is copied into /etc/localtime just below.
# curl: required by the HEALTHCHECK in this file AND by deploy/docker-compose.ci.yml,
#   which overrides the healthcheck with its own `curl -fsS .../api/health`.
#   Slimming the image by dropping curl would break the CI smoke gate, so do not
#   swap it for busybox wget without updating that compose file in the same change.
RUN apk add --no-cache ca-certificates tzdata curl && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    addgroup -S fund && adduser -S -G fund -H -s /sbin/nologin fund

WORKDIR /app

COPY --from=go-build /out/${BIN} /app/${BIN}

# data dir is volume-mounted; ensure ownership so non-root can write SQLite/WAL.
RUN mkdir -p /app/data /app/backups && chown -R fund:fund /app

USER fund

EXPOSE 8765

HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
  CMD curl -fsS http://127.0.0.1:8765/api/health || exit 1

# ARG 无法用于 JSON ENTRYPOINT；ENV BIN 由构建 ARG 注入（app 或 migrate 镜像共用本 Dockerfile）
ENTRYPOINT ["/bin/sh", "-c", "exec /app/$BIN \"$@\"", "--"]
