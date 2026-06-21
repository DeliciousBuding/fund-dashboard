#!/usr/bin/env python3
"""Run the full fund data crawl + enrich + report + visualize pipeline."""

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "src"))

from fund_crawler import (
    crawl_nav,
    crawl_details,
    crawl_dividends,
    crawl_status,
    enrich,
    report,
    visualize,
)


def run_all(
    skip_nav: bool = False,
    skip_details: bool = False,
    skip_dividends: bool = False,
    skip_status: bool = False,
):
    print("=" * 60)
    print("  Fund Data Crawler — Full Pipeline")
    print("=" * 60, "\n")

    if not skip_nav:
        print("Step 1/7: Crawling NAV history...")
        crawl_nav()
        print()

    if not skip_details:
        print("Step 2/7: Crawling fund details...")
        crawl_details()
        print()

    if not skip_dividends:
        print("Step 3/7: Crawling dividend records...")
        crawl_dividends()
        print()

    if not skip_status:
        print("Step 4/7: Crawling fund status...")
        crawl_status()
        print()

    print("Step 5/7: Enriching transactions...")
    enrich()
    print()

    print("Step 6/7: Generating reports...")
    report()
    print()

    print("Step 7/7: Generating dashboard...")
    visualize()
    print()

    print("Pipeline complete.")


def main():
    parser = argparse.ArgumentParser(description="Run full fund data pipeline")
    parser.add_argument("--skip-nav", action="store_true", help="Skip NAV crawl")
    parser.add_argument("--skip-details", action="store_true", help="Skip details crawl")
    parser.add_argument("--skip-dividends", action="store_true", help="Skip dividend crawl")
    parser.add_argument("--skip-status", action="store_true", help="Skip status crawl")
    args = parser.parse_args()
    run_all(
        skip_nav=args.skip_nav,
        skip_details=args.skip_details,
        skip_dividends=args.skip_dividends,
        skip_status=args.skip_status,
    )


if __name__ == "__main__":
    main()
