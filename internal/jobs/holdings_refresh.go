package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/datasource"
)

// HoldingsRefresher crawls fund top holdings into fund_holdings.
type HoldingsRefresher struct {
	db     *sql.DB
	source *datasource.EastmoneyHoldings
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
		reportDate = time.Now().Format("2006-01-02")
	}
	n, err := upsertFundHoldings(ctx, r.db, code, reportDate, holdings)
	if err != nil {
		return 0, reportDate, err
	}
	slog.Info("holdings refresh", "code", code, "rows", n, "report_date", reportDate)
	return n, reportDate, nil
}

// CrawlAllHeld refreshes holdings for every held fund in portfolio_snapshot.
func (r *HoldingsRefresher) CrawlAllHeld(ctx context.Context) (funds int, added int, err error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT fund_code FROM portfolio_snapshot
		WHERE held_shares > 0.001 AND COALESCE(security_type, 'fund') = 'fund'
		LIMIT 5000
	`)
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
	for i, code := range codes {
		if err := ctx.Err(); err != nil {
			return funds, added, err
		}
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
	return funds, added, nil
}

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
		LIMIT 500
	`, fundCode, reportDate)
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
