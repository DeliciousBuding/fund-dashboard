# Go Backend Dockerfile — multi-stage, static binary on Alpine.
#
# packages/web was removed 2026-08-29; the next-gen web UI returns as web/
# and ships embedded via go:embed (see docs/design/, W0 adds a node build
# stage here). FUND_STATIC_DIR can still point at a mounted static dir for dev.
#
# Build:  docker build -f deploy/Dockerfile -t ghcr.io/deliciousbuding/fund-dashboard:<sha> .
# Run:    docker compose -f deploy/docker-compose.yml up -d
# Env:    FUND_HTTP_ADDR, FUND_DB_PATH, FUND_STATIC_DIR, MCP_API_KEY, PUBLIC_MCP_KEY, FUND_EDGE_KEY

FROM golang:1.26-alpine AS go-build

RUN apk add --no-cache git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
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
