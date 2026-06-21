# Production Dockerfile — Bun + Hono backend
# @fund-dashboard/contracts is a workspace member in dev. In the image we copy it
# directly into node_modules and drop the dep line — avoids workspace resolution
# (root workspaces lists web too, which isn't copied). contracts is pure TS; its
# only runtime dep (zod) is already in server deps.
FROM oven/bun:1.2.9-alpine

WORKDIR /app

COPY packages/server/package.json ./
# Drop the workspace contracts dep — we place contracts into node_modules directly
RUN sed -i '/@fund-dashboard\/contracts/d' package.json
# contracts as a local node_modules package
COPY packages/contracts/ ./node_modules/@fund-dashboard/contracts/

RUN bun install --production

COPY packages/server/ ./

RUN mkdir -p /app/data

COPY deploy/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8765

HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
  CMD wget -q -O - http://127.0.0.1:8765/api/health || exit 1

ENTRYPOINT ["/entrypoint.sh"]
