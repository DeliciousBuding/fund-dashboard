# Plan: 10-Audit Fix — Open Source Ready + Performance Sprint

## Context
10 parallel subagents audited the entire fund-dashboard project. Common themes emerged across audits. Target: fix all P0/P1 issues in one pass using parallel workflow.

## Fix Groups (parallel execution)

### Group A: Open Source Readiness (3 agents)
- **A1**: Apache 2.0 LICENSE + .env.example + CONTRIBUTING.md
- **A2**: README overhaul — badges, screenshots, quickstart, mature OSS style
- **A3**: Architecture doc (data flow diagram, schema, API reference)

### Group B: Performance (3 agents)  
- **B1**: Vite manualChunks fix (function form) → kumo -150KB, React separate
- **B2**: React.lazy code split (NasdaqOverview, FundDetailView, PortfolioChart) → initial JS -84%
- **B3**: PurgeCSS (Kumo CSS 113KB→40KB) + TitleComponent removal + preconnect hints

### Group C: Deploy & Infrastructure (2 agents)
- **C1**: Dockerfile path fix + DB volume rw + deploy/nginx.conf hardening
- **C2**: Root package.json (Bun workspaces) + script standardization

### Group D: Code Quality (2 agents)
- **D1**: Dark mode fixes (dataZoom slider, PortfolioChart areaStyle, chartColors up/down)
- **D2**: Python crawler → archive to `packages/crawler/README.md` (keep code, mark legacy) + dead code removal

## Verification
After all fixes: `bun test` pass, `npx vite build` pass, `docker build -f deploy/Dockerfile .` pass
