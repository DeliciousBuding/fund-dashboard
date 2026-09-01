// Package admin provides shared service-layer logic for admin diagnostics:
// freshness, verify, db-integrity, system status, per-security status, and transaction
// import/update/delete.
package admin

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/DeliciousBuding/fund-dashboard/internal/dialect"
)

const stalePriceDays = 4

type Service struct {
	db      *sql.DB
	dialect dialect.Dialect
}

// NewServiceWithDriver creates a Service aware of the underlying database driver
// so it can generate the correct SQL dialect (e.g. julianday vs EXTRACT/EPOCH).
func NewServiceWithDriver(db *sql.DB, driver string) Service {
	return Service{db: db, dialect: dialect.New(driver, db)}
}

type FreshnessReport struct {
	LastTransaction               *string         `json:"last_transaction"`
	LastNAVDate                   *string         `json:"last_nav_date"`
	AnomalyCount                  int             `json:"anomaly_count"`
	MissingNAVSecurities          []FreshnessItem `json:"missing_nav_securities"`
	WatchlistMissingNAVSecurities []FreshnessItem `json:"watchlist_missing_nav_securities"`
	StaleSecurities               []StaleSecurity `json:"stale_securities"`
	Actionable                    string          `json:"actionable"`
	Health                        string          `json:"health"`
	DecisionBoundary              string          `json:"decision_boundary"`
}

type FreshnessItem struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type StaleSecurity struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	LastNAV   string `json:"last_nav"`
	StaleDays int    `json:"stale_days"`
}

func (s Service) GetFreshness(ctx context.Context) (FreshnessReport, error) {
	transactions, err := s.queryTransactions(ctx)
	if err != nil {
		return FreshnessReport{}, err
	}
	nav, err := s.queryNAV(ctx)
	if err != nil {
		return FreshnessReport{}, err
	}
	anomalyCount, err := s.queryAnomalyCount(ctx)
	if err != nil {
		return FreshnessReport{}, err
	}
	heldMissing, err := s.queryMissingNAVSecurities(ctx, true)
	if err != nil {
		return FreshnessReport{}, err
	}
	watchlistMissing, err := s.queryMissingNAVSecurities(ctx, false)
	if err != nil {
		return FreshnessReport{}, err
	}
	stale, err := s.queryStaleSecurities(ctx)
	if err != nil {
		return FreshnessReport{}, err
	}

	return FreshnessReport{
		LastTransaction:               transactions.Last,
		LastNAVDate:                   dateOnlyPtr(nav.Last),
		AnomalyCount:                  anomalyCount,
		MissingNAVSecurities:          heldMissing,
		WatchlistMissingNAVSecurities: watchlistMissing,
		StaleSecurities:               stale,
		Actionable:                    freshnessActionable(len(stale), len(heldMissing)),
		Health:                        freshnessHealth(len(stale), len(heldMissing)),
		DecisionBoundary:              "read_only",
	}, nil
}

type countLast struct {
	Count int
	Last  *string
}

type navStats struct {
	Count int
	Funds int
	First *string
	Last  *string
}

func (s Service) queryTransactions(ctx context.Context) (countLast, error) {
	var count int
	var last sql.NullString
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*), MAX(trade_time) FROM transactions").Scan(&count, &last); err != nil {
		return countLast{}, fmt.Errorf("admin freshness transactions: %w", err)
	}
	return countLast{Count: count, Last: nullableStringPtr(last)}, nil
}

func (s Service) queryNAV(ctx context.Context) (navStats, error) {
	var stats navStats
	var first sql.NullString
	var last sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COUNT(DISTINCT fund_code), MIN(date), MAX(date)
		FROM nav_history
	`).Scan(&stats.Count, &stats.Funds, &first, &last); err != nil {
		return navStats{}, fmt.Errorf("admin freshness nav: %w", err)
	}
	stats.First = nullableStringPtr(first)
	stats.Last = nullableStringPtr(last)
	return stats, nil
}

func (s Service) queryAnomalyCount(ctx context.Context) (int, error) {
	hasAnomaly, err := s.dialect.HasColumn(ctx, "transactions", "anomaly")
	if err != nil {
		return 0, err
	}
	if !hasAnomaly {
		return 0, nil
	}
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM transactions WHERE anomaly IS NOT NULL").Scan(&count); err != nil {
		return 0, fmt.Errorf("admin freshness anomalies: %w", err)
	}
	return count, nil
}

func (s Service) queryMissingNAVSecurities(ctx context.Context, heldOnly bool) ([]FreshnessItem, error) {
	join := "JOIN portfolio_snapshot ps ON ps.fund_code = fd.fund_code"
	heldPredicate := "ps.held_shares > 0.001"
	if !heldOnly {
		join = "LEFT JOIN portfolio_snapshot ps ON ps.fund_code = fd.fund_code"
		heldPredicate = "COALESCE(ps.held_shares, 0) <= 0.001"
	}
	query := fmt.Sprintf(`
		SELECT
			fd.fund_code,
			COALESCE(fd.fund_name, fd.fund_code),
			COALESCE(fd.security_type, 'fund')
		FROM fund_details fd
		%s
		WHERE %s
		  AND fd.fund_code NOT IN (SELECT DISTINCT fund_code FROM nav_history)
		GROUP BY fd.fund_code
		ORDER BY fd.fund_code
		LIMIT 5000
	`, join, heldPredicate)
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("admin freshness missing nav: %w", err)
	}
	defer rows.Close()

	items := []FreshnessItem{}
	for rows.Next() {
		var item FreshnessItem
		if err := rows.Scan(&item.Code, &item.Name, &item.Type); err != nil {
			return nil, fmt.Errorf("scan admin freshness missing nav: %w", err)
		}
		item.Code = clampAdminText(item.Code, 32)
		item.Name = clampAdminText(item.Name, 200)
		item.Type = clampAdminText(item.Type, 32)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("admin freshness missing nav rows: %w", err)
	}
	return items, nil
}

func (s Service) queryStaleSecurities(ctx context.Context) ([]StaleSecurity, error) {
	daysSince := s.dialect.DaysSinceExpr("MAX(nh.date)")
	staleSQL := fmt.Sprintf(`
		SELECT
			nh.fund_code,
			COALESCE(fd.fund_name, nh.fund_code),
			MAX(nh.date),
			%s
		FROM nav_history nh
		JOIN fund_details fd ON nh.fund_code = fd.fund_code
		JOIN portfolio_snapshot ps ON ps.fund_code = fd.fund_code
		WHERE ps.held_shares > 0.001
		GROUP BY nh.fund_code, fd.fund_name
		HAVING %s > %d
		ORDER BY %s DESC, nh.fund_code
		LIMIT 5000
	`, daysSince, daysSince, stalePriceDays, daysSince)

	rows, err := s.db.QueryContext(ctx, staleSQL)
	if err != nil {
		return nil, fmt.Errorf("admin freshness stale securities: %w", err)
	}
	defer rows.Close()

	items := []StaleSecurity{}
	for rows.Next() {
		var item StaleSecurity
		if err := rows.Scan(&item.Code, &item.Name, &item.LastNAV, &item.StaleDays); err != nil {
			return nil, fmt.Errorf("scan admin freshness stale security: %w", err)
		}
		item.Code = clampAdminText(item.Code, 32)
		item.Name = clampAdminText(item.Name, 200)
		item.LastNAV = clampAdminText(item.LastNAV, 40)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("admin freshness stale security rows: %w", err)
	}
	return items, nil
}

func freshnessActionable(staleCount int, missingHeldCount int) string {
	if staleCount > 0 {
		return fmt.Sprintf("建议运行 crawl_nav 刷新 %d 只过期证券的价格数据", staleCount)
	}
	if missingHeldCount > 0 {
		return fmt.Sprintf("建议先添加 %d 只持仓证券的价格数据", missingHeldCount)
	}
	return "数据新鲜度正常"
}

func freshnessHealth(staleCount int, missingHeldCount int) string {
	if staleCount == 0 && missingHeldCount == 0 {
		return "fresh"
	}
	if staleCount > 3 {
		return "stale"
	}
	return "degraded"
}

func dateOnlyPtr(value *string) *string {
	if value == nil {
		return nil
	}
	date := *value
	if len(date) > 10 {
		date = date[:10]
	}
	return &date
}

func nullableStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
