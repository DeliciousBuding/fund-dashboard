package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/chinatime"
	"github.com/DeliciousBuding/fund-dashboard/internal/datasource"
)

// holdingsSource fetches top holdings for one fund; datasource.EastmoneyHoldings
// implements it and tests can substitute a stub.
type holdingsSource interface {
	FetchHoldings(ctx context.Context, code string, topline int) ([]datasource.FundHolding, error)
}

// HoldingsRefresher crawls fund top holdings into fund_holdings.
type HoldingsRefresher struct {
	db     *sql.DB
	source holdingsSource
}

func NewHoldingsRefresher(db *sql.DB) *HoldingsRefresher {
	return &HoldingsRefresher{
		db:     db,
		source: datasource.NewEastmoneyHoldings(),
	}
}

// CrawlCode refreshes holdings for one fund.
func (r *HoldingsRefresher) CrawlCode(ctx context.Context, code string) (added int, reportDate string, err error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return 0, "", fmt.Errorf("code is required")
	}
	if len(code) > 32 {
		return 0, "", fmt.Errorf("code too long")
	}
	holdings, err := r.source.FetchHoldings(ctx, code, 10)
	if err != nil {
		return 0, "", err
	}
	if len(holdings) == 0 {
		return 0, "", nil
	}
	reportDate = holdings[0].ReportDate
	if reportDate == "" {
		// Calendar-day fallback must follow the China fund calendar (chinatime.Loc), not the
		// runner's local timezone.
		reportDate = time.Now().In(chinatime.Loc).Format("2006-01-02")
	}
	n, err := upsertFundHoldings(ctx, r.db, code, reportDate, holdings)
	if err != nil {
		return 0, reportDate, err
	}
	slog.Info("holdings refresh", "code", code, "rows", n, "report_date", reportDate)
	return n, reportDate, nil
}

// holdingsCrawlMaxCodes bounds one CrawlAllHeld batch. Same defense-in-depth
// pattern as getHeldSecurities/recalc: probe limit+1 rows and warn instead of
// silently dropping tail funds.
const holdingsCrawlMaxCodes = 5000

// CrawlAllHeld refreshes holdings for every held fund in portfolio_snapshot.
func (r *HoldingsRefresher) CrawlAllHeld(ctx context.Context) (funds int, added int, err error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT fund_code FROM portfolio_snapshot
		WHERE held_shares > 0.001 AND COALESCE(security_type, 'fund') = 'fund'
		LIMIT ?
	`, holdingsCrawlMaxCodes+1)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return 0, 0, err
		}
		codes = append(codes, code)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	codes, dropped := capCodes(codes, holdingsCrawlMaxCodes)
	if dropped > 0 {
		slog.Warn("holdings crawl code list truncated",
			"limit", holdingsCrawlMaxCodes,
			"processed", len(codes),
			"at_least_dropped", dropped,
		)
	}
	attempted := 0
	for i, code := range codes {
		if err := ctx.Err(); err != nil {
			return funds, added, err
		}
		attempted++
		n, _, err := r.CrawlCode(ctx, code)
		if err != nil {
			slog.Error("holdings refresh failed", "code", code, "error", err)
			continue
		}
		added += n
		funds++
		if i < len(codes)-1 {
			if err := sleepContext(ctx, 1200*time.Millisecond); err != nil {
				return funds, added, err
			}
		}
	}
	// Total-failure must surface: partial crawl parity soft-skips per-fund errors,
	// but a run where every attempted fund failed is an error, not success.
	if attempted > 0 && funds == 0 {
		return funds, added, fmt.Errorf("holdings refresh failed for all %d attempted funds", attempted)
	}
	return funds, added, nil
}

// fundHoldingsMatchMaxRows caps the stored-slice comparison read. A top-10
// holdings payload can never approach it, so it is defense-in-depth only.
const fundHoldingsMatchMaxRows = 500

func upsertFundHoldings(ctx context.Context, db *sql.DB, fundCode, reportDate string, holdings []datasource.FundHolding) (int, error) {
	// Skip rewrite when the stored report slice already matches the crawl payload (#88).
	// DELETE+re-INSERT always looked like added=N even for no-op refreshes.
	same, err := fundHoldingsMatch(ctx, db, fundCode, reportDate, holdings)
	if err != nil {
		return 0, err
	}
	if same {
		return 0, nil
	}

	// Atomic rewrite: DELETE + INSERT in one transaction so crash/partial failure
	// cannot leave an empty/partial report_date slice (#197).
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin holdings rewrite: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM fund_holdings WHERE fund_code = ? AND report_date = ?`, fundCode, reportDate); err != nil {
		return 0, fmt.Errorf("delete old holdings: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO fund_holdings (fund_code, stock_code, stock_name, weight_pct, shares, market_value, report_date)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(fund_code, stock_code, report_date) DO UPDATE SET
			stock_name = excluded.stock_name,
			weight_pct = excluded.weight_pct,
			shares = excluded.shares,
			market_value = excluded.market_value
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	n := 0
	for _, h := range holdings {
		rd := h.ReportDate
		if rd == "" {
			rd = reportDate
		}
		if _, err := stmt.ExecContext(ctx, fundCode, h.StockCode, h.StockName, h.WeightPct, h.Shares, h.MarketValue, rd); err != nil {
			return 0, fmt.Errorf("upsert holding %s/%s: %w", fundCode, h.StockCode, err)
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit holdings rewrite: %w", err)
	}
	return n, nil
}

type holdingKey struct {
	StockCode   string
	StockName   string
	WeightPct   float64
	Shares      float64
	MarketValue float64
}

func fundHoldingsMatch(ctx context.Context, db *sql.DB, fundCode, reportDate string, holdings []datasource.FundHolding) (bool, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT stock_code, COALESCE(stock_name, ''), COALESCE(weight_pct, 0), COALESCE(shares, 0), COALESCE(market_value, 0)
		FROM fund_holdings
		WHERE fund_code = ? AND report_date = ?
		LIMIT ?
	`, fundCode, reportDate, fundHoldingsMatchMaxRows)
	if err != nil {
		return false, fmt.Errorf("query existing holdings: %w", err)
	}
	defer rows.Close()

	existing := map[holdingKey]struct{}{}
	for rows.Next() {
		var k holdingKey
		if err := rows.Scan(&k.StockCode, &k.StockName, &k.WeightPct, &k.Shares, &k.MarketValue); err != nil {
			return false, err
		}
		existing[k] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if len(existing) != len(holdings) {
		return false, nil
	}
	for _, h := range holdings {
		k := holdingKey{
			StockCode:   h.StockCode,
			StockName:   h.StockName,
			WeightPct:   h.WeightPct,
			Shares:      h.Shares,
			MarketValue: h.MarketValue,
		}
		if _, ok := existing[k]; !ok {
			return false, nil
		}
	}
	return true, nil
}
