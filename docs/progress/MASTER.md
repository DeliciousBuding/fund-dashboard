# Progress — 当前状态

最后更新：2026-08-30 23:53

- 当前形态：Go 后端 + `web/` React SPA + `packages/contracts`，前端由 `go:embed` 内嵌为单二进制/单镜像。
- W0–W7 已实施：session 鉴权、SQLite/PG 双驱动、REST/MCP、投资组合页面、分析/回测/穿透、系统工作台与审计均在主线。
- 当前质量门禁：Go format/vet/test-race/build、Web Biome/Vitest/TypeScript/build、容器/API smoke。
- 设计决策与剩余 backlog：[`docs/design/`](../design/README.md)；用户可见变更：[`CHANGELOG.md`](../../CHANGELOG.md)。
- 本文件只保留当前摘要，不记录分支/WIP/生产 pin；实现事实以源码和 CI 为准，live 部署事实由私有运维 SSOT 管理。
