package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"time"
)

type Service struct {
	db     *sql.DB
	schema *schemaMetaCache // process-local table/column probe cache
}

func NewService(db *sql.DB) Service {
	return Service{db: db, schema: newSchemaMetaCache()}
}

type Summary struct {
	TotalTx       int
	UniqueFunds   int
	UniqueStocks  int
	HeldFunds     int
	TotalBuy      float64
	TotalSell     float64
	TotalFee      float64
	UnrealizedPNL float64
	// InvestedCost is Σ |total_cost| for held rows (ledger uses signed cash-flow; cost is negative).
	InvestedCost float64
	// CurrentValue is Σ current_value for held rows (mark-to-market).
	CurrentValue float64
	// PNLPct is UnrealizedPNL / InvestedCost * 100 when InvestedCost > 0.
	PNLPct float64
	// Insights: top/bottom contributors + optional stale-nav signal (facts-only).
	TopGainer              *HoldingContributor
	TopLoser               *HoldingContributor
	StaleNAVDays           *int
	AutoTx                 int
	ManualTx               int
	AutoAmount             float64
	ManualAmount           float64
	FirstTrade             string
	LastTrade              string
	LastNAVDate            *string
	SettlementDistribution map[string]int
	TradeTypeBreakdown     map[string]int
	BySecurityType         []SecurityTypeBalance
}

// HoldingContributor is a single held position called out on the overview hero.
type HoldingContributor struct {
	Code          string
	Name          string
	UnrealizedPNL float64
	PNLPct        float64
	CurrentValue  float64
}

type SecurityTypeBalance struct {
	SecurityType string
	Count        int
	TotalValue   float64
	TotalPNL     float64
}

func (s Service) GetSummary(ctx context.Context, portfolioID int) (*Summary, error) {
	portfolioID = clampPortfolioID(portfolioID)

	// Scope transaction aggregates to funds present in this portfolio's snapshot.
	// transactions table has no portfolio_id column; membership is via portfolio_snapshot.
	row := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) as total_tx,
			COALESCE(SUM(CASE WHEN t.direction='buy' THEN t.confirm_amount ELSE 0 END), 0) as total_buy,
			COALESCE(SUM(CASE WHEN t.direction='sell' THEN t.confirm_amount ELSE 0 END), 0) as total_sell,
			COALESCE(SUM(COALESCE(t.fee,0)), 0) as total_fee,
			COALESCE(SUM(CASE WHEN t.trade_type LIKE '%定投%' THEN 1 ELSE 0 END), 0) as auto_tx,
			COALESCE(SUM(CASE WHEN t.trade_type LIKE '%用户%' THEN 1 ELSE 0 END), 0) as manual_tx,
			COALESCE(SUM(CASE WHEN t.direction='buy' AND t.trade_type LIKE '%定投%' THEN t.confirm_amount ELSE 0 END), 0) as auto_amount,
			COALESCE(SUM(CASE WHEN t.direction='buy' AND t.trade_type LIKE '%用户%' THEN t.confirm_amount ELSE 0 END), 0) as manual_amount,
			COALESCE(MIN(t.trade_time), '') as first_trade,
			COALESCE(MAX(t.trade_time), '') as last_trade
		FROM transactions t
		WHERE EXISTS (
			SELECT 1 FROM portfolio_snapshot ps
			WHERE ps.fund_code = t.fund_code
			  AND COALESCE(ps.portfolio_id, 1) = ?
		)
	`, portfolioID)

	summary := &Summary{
		SettlementDistribution: map[string]int{},
		TradeTypeBreakdown:     map[string]int{},
	}
	if err := row.Scan(
		&summary.TotalTx,
		&summary.TotalBuy,
		&summary.TotalSell,
		&summary.TotalFee,
		&summary.AutoTx,
		&summary.ManualTx,
		&summary.AutoAmount,
		&summary.ManualAmount,
		&summary.FirstTrade,
		&summary.LastTrade,
	); err != nil {
		return nil, fmt.Errorf("scan portfolio summary aggregate: %w", err)
	}

	summary.TotalBuy = round2(summary.TotalBuy)
	summary.TotalSell = round2(summary.TotalSell)
	summary.TotalFee = round2(summary.TotalFee)
	summary.AutoAmount = round2(summary.AutoAmount)
	summary.ManualAmount = round2(summary.ManualAmount)
	summary.FirstTrade = dateOnly(summary.FirstTrade)
	summary.LastTrade = dateOnly(summary.LastTrade)

	if err := s.fillPortfolioScopedCounts(ctx, portfolioID, summary); err != nil {
		return nil, err
	}
	if err := s.fillDistributions(ctx, portfolioID, summary); err != nil {
		return nil, err
	}
	if err := s.fillSecurityTypeBalances(ctx, portfolioID, summary); err != nil {
		return nil, err
	}

	return summary, nil
}

func (s Service) fillPortfolioScopedCounts(ctx context.Context, portfolioID int, summary *Summary) error {
	// One round-trip for held counts + mark-to-market sums (held only).
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN held_shares > 0.001 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN held_shares > 0.001 THEN unrealized_pnl ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN held_shares > 0.001 THEN ABS(total_cost) ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN held_shares > 0.001 THEN current_value ELSE 0 END), 0)
		FROM portfolio_snapshot
		WHERE COALESCE(portfolio_id, 1) = ?
	`, portfolioID).Scan(&summary.HeldFunds, &summary.UnrealizedPNL, &summary.InvestedCost, &summary.CurrentValue); err != nil {
		return fmt.Errorf("sum portfolio value facts: %w", err)
	}
	// Unique fund/stock membership (all snapshot rows with fund_details; preserve INNER JOIN semantics).
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN fd.security_type IS NULL OR fd.security_type != 'stock' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN fd.security_type = 'stock' THEN 1 ELSE 0 END), 0)
		FROM fund_details fd
		JOIN portfolio_snapshot ps ON ps.fund_code = fd.fund_code AND COALESCE(ps.portfolio_id, 1) = ?
	`, portfolioID).Scan(&summary.UniqueFunds, &summary.UniqueStocks); err != nil {
		return fmt.Errorf("count unique securities: %w", err)
	}
	summary.UnrealizedPNL = round2(summary.UnrealizedPNL)
	summary.InvestedCost = round2(summary.InvestedCost)
	summary.CurrentValue = round2(summary.CurrentValue)
	if summary.InvestedCost > 0 {
		summary.PNLPct = round2(summary.UnrealizedPNL / summary.InvestedCost * 100)
	}

	var lastNAV sql.NullString
	if err := s.db.QueryRowContext(ctx, "SELECT MAX(date) FROM nav_history").Scan(&lastNAV); err != nil {
		return fmt.Errorf("read last nav date: %w", err)
	}
	if lastNAV.Valid {
		date := dateOnly(lastNAV.String)
		summary.LastNAVDate = &date
		if days, ok := calendarDaysSince(date); ok && days > 0 {
			d := days
			summary.StaleNAVDays = &d
		}
	}
	if err := s.fillContributors(ctx, portfolioID, summary); err != nil {
		return err
	}
	return nil
}

func (s Service) fillContributors(ctx context.Context, portfolioID int, summary *Summary) error {
	// Single round-trip: top gainer + top loser (only when loser PnL < 0).
	rows, err := s.db.QueryContext(ctx, `
		SELECT kind, fund_code, fund_name, unrealized_pnl, pnl_pct, current_value FROM (
			SELECT 'g' AS kind, fund_code, COALESCE(fund_name, fund_code) AS fund_name,
				COALESCE(unrealized_pnl, 0) AS unrealized_pnl, COALESCE(pnl_pct, 0) AS pnl_pct, COALESCE(current_value, 0) AS current_value
			FROM portfolio_snapshot
			WHERE held_shares > 0.001 AND COALESCE(portfolio_id, 1) = ?
			ORDER BY unrealized_pnl DESC, current_value DESC, fund_code
			LIMIT 1
		)
		UNION ALL
		SELECT kind, fund_code, fund_name, unrealized_pnl, pnl_pct, current_value FROM (
			SELECT 'l' AS kind, fund_code, COALESCE(fund_name, fund_code) AS fund_name,
				COALESCE(unrealized_pnl, 0) AS unrealized_pnl, COALESCE(pnl_pct, 0) AS pnl_pct, COALESCE(current_value, 0) AS current_value
			FROM portfolio_snapshot
			WHERE held_shares > 0.001 AND COALESCE(portfolio_id, 1) = ?
			  AND COALESCE(unrealized_pnl, 0) < 0
			ORDER BY unrealized_pnl ASC, current_value DESC, fund_code
			LIMIT 1
		)
	`, portfolioID, portfolioID)
	if err != nil {
		return fmt.Errorf("contributors: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind, code, name string
		var pnl, pct, val float64
		if err := rows.Scan(&kind, &code, &name, &pnl, &pct, &val); err != nil {
			return fmt.Errorf("scan contributor: %w", err)
		}
		c := &HoldingContributor{
			Code:          clampPortfolioText(code, 32),
			Name:          clampPortfolioText(name, 80),
			UnrealizedPNL: round2(pnl),
			PNLPct:        round2(pct),
			CurrentValue:  round2(val),
		}
		if kind == "g" && c.Code != "" {
			summary.TopGainer = c
		} else if kind == "l" && c.Code != "" {
			if summary.TopGainer == nil || summary.TopGainer.Code != c.Code {
				summary.TopLoser = c
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("contributor rows: %w", err)
	}
	return nil
}

// chinaMarketLoc is the fund NAV calendar (CN A-share / fund industry convention).
// Host may be in another timezone; stale chips must not use host local TZ.
var chinaMarketLoc = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}()

// calendarDaysSince returns whole calendar days from YYYY-MM-DD to Asia/Shanghai today.
func calendarDaysSince(dateYYYYMMDD string) (int, bool) {
	if len(dateYYYYMMDD) < 10 {
		return 0, false
	}
	t, err := time.ParseInLocation("2006-01-02", dateYYYYMMDD[:10], chinaMarketLoc)
	if err != nil {
		return 0, false
	}
	now := time.Now().In(chinaMarketLoc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, chinaMarketLoc)
	d := int(today.Sub(t).Hours() / 24)
	if d < 0 {
		return 0, true
	}
	return d, true
}

func (s Service) fillDistributions(ctx context.Context, portfolioID int, summary *Summary) error {
	// Aggregate in SQL (#234) — avoid scanning every settlement row into Go.
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.settlement_days, COUNT(*)
		FROM transactions t
		WHERE t.settlement_days IS NOT NULL
		  AND EXISTS (
			SELECT 1 FROM portfolio_snapshot ps
			WHERE ps.fund_code = t.fund_code
			  AND COALESCE(ps.portfolio_id, 1) = ?
		  )
		GROUP BY t.settlement_days
		ORDER BY t.settlement_days
		LIMIT 64
	`, portfolioID)
	if err != nil {
		return fmt.Errorf("query settlement distribution: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var days int
		var count int
		if err := rows.Scan(&days, &count); err != nil {
			return fmt.Errorf("scan settlement days: %w", err)
		}
		summary.SettlementDistribution[strconv.Itoa(days)] = count
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("settlement distribution rows: %w", err)
	}

	rows, err = s.db.QueryContext(ctx, `
		SELECT t.trade_type, COUNT(*)
		FROM transactions t
		WHERE EXISTS (
			SELECT 1 FROM portfolio_snapshot ps
			WHERE ps.fund_code = t.fund_code
			  AND COALESCE(ps.portfolio_id, 1) = ?
		)
		GROUP BY t.trade_type
		ORDER BY COUNT(*) DESC, t.trade_type
		LIMIT 64
	`, portfolioID)
	if err != nil {
		return fmt.Errorf("query trade type breakdown: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var tradeType string
		var count int
		if err := rows.Scan(&tradeType, &count); err != nil {
			return fmt.Errorf("scan trade type breakdown: %w", err)
		}
		tradeType = clampPortfolioText(tradeType, 64)
		summary.TradeTypeBreakdown[tradeType] = count
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("trade type breakdown rows: %w", err)
	}
	return nil
}

func (s Service) fillSecurityTypeBalances(ctx context.Context, portfolioID int, summary *Summary) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			COALESCE(security_type, 'fund'),
			COUNT(*),
			COALESCE(SUM(current_value), 0),
			COALESCE(SUM(unrealized_pnl), 0)
		FROM portfolio_snapshot
		WHERE held_shares > 0.001 AND COALESCE(portfolio_id, 1) = ?
		GROUP BY COALESCE(security_type, 'fund')
		ORDER BY COALESCE(SUM(current_value), 0) DESC, COALESCE(security_type, 'fund')
		LIMIT 32
	`, portfolioID)
	if err != nil {
		return fmt.Errorf("query security type balances: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var balance SecurityTypeBalance
		if err := rows.Scan(&balance.SecurityType, &balance.Count, &balance.TotalValue, &balance.TotalPNL); err != nil {
			return fmt.Errorf("scan security type balance: %w", err)
		}
		balance.SecurityType = clampPortfolioText(balance.SecurityType, 32)
		balance.TotalValue = round2(balance.TotalValue)
		balance.TotalPNL = round2(balance.TotalPNL)
		summary.BySecurityType = append(summary.BySecurityType, balance)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("security type balance rows: %w", err)
	}
	return nil
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func dateOnly(value string) string {
	if len(value) >= 10 {
		return value[:10]
	}
	return value
}
