# fund-dashboard — multi-stage: web (pnpm/Vite) → Go static binary (embeds SPA) → Alpine runtime.
#
# 多产物：ARG BIN 选构建命令（fund-dashboard 默认；fund-migrate 供一次性数据迁移镜像）。
# Build app:  docker build -f deploy/Dockerfile -t ghcr.io/deliciousbuding/fund-dashboard:<sha> .
# Build migr: docker build -f deploy/Dockerfile --build-arg BIN=fund-migrate -t .../fund-migrate:<sha> .
# Env: 见 deploy/.env.example（公网形态：FUND_AUTH_SECURE_COOKIE=true + FUND_ALLOWED_ORIGINS 必配）。

# ── web build: Vite outputs straight into internal/webui/dist (go:embed source)
FROM node:24-alpine AS web-build
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
FROM golang:1.26-alpine AS go-build

ARG BIN=fund-dashboard
RUN apk add --no-cache git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Fresh SPA from the web stage (never trust a locally built dist in context).
COPY --from=web-build /src/internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -o /out/${BIN} -ldflags="-s -w" ./cmd/${BIN}/

FROM alpine:3.20 AS runtime

ARG BIN=fund-dashboard
ENV BIN=${BIN}
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
