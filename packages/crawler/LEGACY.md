# LEGACY — Python fund-crawler (Archived)

> **Status: Archived / Superseded**
> This package is no longer the active data pipeline for fund-dashboard.
> It has been replaced by the TypeScript crawler in `packages/server/crawler/`.

---

## Why Legacy

The Python crawler (`fund-crawler`) was the original data ingestion pipeline for fund-dashboard.
It used **AKShare** (a third-party Python package wrapping eastmoney data) to batch-fetch NAV history,
fund details, dividend records, and purchase/redemption status, then enriched transaction CSVs
and generated reports.

In **June 2026**, the pipeline was migrated to **TypeScript** running inside the Bun server
(`packages/server/crawler/`). The new crawler:

- Calls eastmoney JS endpoints directly (no AKShare dependency)
- Runs incremental NAV refreshes (INSERT OR IGNORE into SQLite)
- Is scheduled via the Bun server's built-in cron (`scheduler.ts`)
- Shares the same SQLite database the server reads from — no CSV intermediates

## What Remains Here

| Path | Description |
|------|-------------|
| `src/fund_crawler/` | Python modules: crawl_nav, crawl_details, crawl_dividends, crawl_status, enrich, report, visualize, trading_cal, utils |
| `scripts/run_all.py` | One-shot full pipeline script (7 steps: crawl → enrich → report → visualize) |
| `pyproject.toml` | Python project metadata (AKShare, pandas, openpyxl, rich, httpx) |
| `uv.lock` | Locked dependency versions |

## When You Might Still Need This

- **Historical reference**: Understanding the original data flow and field mappings
- **Offline batch reprocessing**: If you need to regenerate all CSVs from scratch outside the server
- **AKShare-specific extraction**: The AKShare API wrappers in `crawl_nav.py` / `crawl_details.py` may have fields not covered by the TS direct-API crawler

## Migration Notes

| Old (Python) | New (TypeScript) |
|---|---|
| `crawl_nav.py` → CSV `output/nav/{code}.csv` | `packages/server/crawler/eastmoney.ts` → SQLite `nav_history` table |
| `crawl_details.py` → CSV `output/fund_details.csv` | `packages/server/crawler/eastmoney.ts` `fetchFundInfo()` |
| `enrich.py` → CSV `transactions_enriched.csv` | Seeded from initial Python import; live enrichment in SQLite views |
| `report.py` → CSV `portfolio_snapshot.csv` | `portfolio_snapshot` table, rebuilt via `POST /api/admin/recalculate-snapshot` |
| `visualize.py` (matplotlib) | ECharts in React frontend |
| Manual `uv run python scripts/run_all.py` | `POST /api/admin/crawl-nav` or automatic scheduler (weekdays 20:00 CST) |
