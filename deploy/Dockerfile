# fund-dashboard — multi-stage: web (pnpm/Vite) → Go static binary (embeds SPA) → Alpine runtime.
#
# Build:  docker build -f deploy/Dockerfile -t ghcr.io/deliciousbuding/fund-dashboard:<sha> .
# Run:    docker compose -f deploy/docker-compose.yml up -d
# Env:    MCP_API_KEY (required), FUND_AUTH_PASSWORD_HASH / FUND_ALLOWED_ORIGINS /
#         FUND_EDGE_AUTH_ENABLED / FUND_STATIC_DIR — see deploy/.env.example

# ── web build: Vite outputs straight into internal/webui/dist (go:embed source)
FROM node:24-alpine AS web-build
WORKDIR /src
RUN corepack enable
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY web/package.json web/
COPY packages/contracts/package.json packages/contracts/
RUN pnpm install --frozen-lockfile
COPY web/ web/
COPY packages/contracts/ packages/contracts/
RUN pnpm -C web build

# ── go build: static binary, SPA embedded via //go:embed all:dist
FROM golang:1.26-alpine AS go-build

RUN apk add --no-cache git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Fresh SPA from the web stage (never trust a locally built dist in context).
COPY --from=web-build /src/internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -o /fund-dashboard -ldflags="-s -w" ./cmd/fund-dashboard/

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    addgroup -S fund && adduser -S -G fund -H -s /sbin/nologin fund

WORKDIR /app

COPY --from=go-build /fund-dashboard /app/fund-dashboard

# data dir is volume-mounted; ensure ownership so non-root can write SQLite/WAL.
RUN mkdir -p /app/data /app/backups && chown -R fund:fund /app

USER fund

EXPOSE 8765

HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
  CMD curl -fsS http://127.0.0.1:8765/api/health || exit 1

ENTRYPOINT ["/app/fund-dashboard"]
