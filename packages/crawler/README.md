# Fund Data Crawler

Enrich fund transaction records with historical NAV, fund details, and dividend data from eastmoney (via AKShare).

## Quick Start

```bash
# Install dependencies
uv sync

# Run full pipeline (crawl + enrich + report)
uv run python scripts/run_all.py

# Or step by step
uv run python -m fund_crawler.crawl_nav --all
uv run python -m fund_crawler.crawl_details --all
uv run python -m fund_crawler.crawl_dividends --all
uv run python -m fund_crawler.enrich
uv run python -m fund_crawler.report
```

## Data Flow

```
transactions_clean.csv (433 rows, 61 funds)
    │
    ├─► crawl_nav ───────► output/nav/{code}.csv
    ├─► crawl_details ───► output/fund_details.csv
    ├─► crawl_dividends ─► output/fund_dividends.csv
    │
    └─► enrich ──────────► output/transactions_enriched.csv
         │
         └─► report ─────► output/portfolio_snapshot.csv
                           output/summary_report.json
```

## Output Files

| File | Description |
|------|-------------|
| `nav/{code}.csv` | Daily NAV history per fund |
| `fund_details.csv` | Fund type, company, name |
| `fund_dividends.csv` | Dividend/distribution records |
| `transactions_enriched.csv` | Original transactions + matched NAV, cost basis, P&L |
| `portfolio_snapshot.csv` | Current holdings summary per fund |
| `summary_report.json` | Top-level stats and breakdowns |
