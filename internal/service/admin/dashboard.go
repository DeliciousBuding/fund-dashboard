package admin

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"time"
)

// DashboardReport is the SPA/ops system monitor payload.
type DashboardReport struct {
	OK               bool              `json:"ok"`
	Timestamp        string            `json:"timestamp"`
	ResponseMS       int64             `json:"response_ms"`
	System           DashboardSystem   `json:"system"`
	Database         DashboardDatabase `json:"database"`
	Crawler          DashboardCrawler  `json:"crawler"`
	State            DashboardState    `json:"state"`
	DecisionBoundary string            `json:"decision_boundary"`
}

type DashboardSystem struct {
	UptimeSec    float64         `json:"uptime_sec"`
	UptimeHuman  string          `json:"uptime_human"`
	Memory       DashboardMemory `json:"memory"`
	GoVersion    string          `json:"go_version"`
	// BuildVersion is FUND_VERSION / release pin (auth-gated dashboard only; not public health).
	BuildVersion string `json:"build_version,omitempty"`
	Platform     string `json:"platform"`
}

type DashboardMemory struct {
	RSSMB       float64 `json:"rss_mb"`
	HeapUsedMB  float64 `json:"heap_used_mb"`
	HeapTotalMB float64 `json:"heap_total_mb"`
}

type DashboardDatabase struct {
	SizeBytes int64   `json:"size_bytes"`
	SizeMB    float64 `json:"size_mb"`
}

type DashboardCrawler struct {
	NAVTotal int `json:"nav_total"`
	// NAVFresh is distinct held/nav fund_codes with nav_history.date within the
	// same calendar-day window as freshness stalePriceDays (fund/QDII T+1 reality).
	NAVFresh int `json:"nav_fresh"`
	// NAVFresh24H is a legacy alias of NAVFresh for older SPA clients.
	// It is NOT a strict 24-hour wall-clock window (#73).
	NAVFresh24H    int     `json:"nav_fresh_24h"`
	SuccessRatePct float64 `json:"success_rate_pct"`
	// FreshWindowDays documents the calendar-day window used for NAVFresh.
	FreshWindowDays int `json:"fresh_window_days"`
}

type DashboardState struct {
	TransactionCount int                `json:"transaction_count"`
	LastTransaction  *string            `json:"last_transaction"`
	LastNAVDate      *string            `json:"last_nav_date"`
	HeldFunds        int                `json:"held_funds"`
	NAVRecords       int                `json:"nav_records"`
	NAVFunds         int                `json:"nav_funds"`
	SecuritiesTotal  int                `json:"securities_total"`
	AnomalyCount     int                `json:"anomaly_count"`
	RecentAnomalies  []DashboardAnomaly `json:"recent_anomalies"`
}

type DashboardAnomaly struct {
	Seq       int     `json:"seq"`
	FundCode  string  `json:"fund_code"`
	Direction *string `json:"direction"`
	TradeTime *string `json:"trade_time"`
	Anomaly   string  `json:"anomaly"`
}

// GetDashboard builds the ops dashboard report. startedAt is process start for uptime.
func (s Service) GetDashboard(ctx context.Context, startedAt time.Time, now time.Time) (DashboardReport, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if startedAt.IsZero() {
		startedAt = now
	}

	txCount, txLast, err := s.dashboardTransactions(ctx)
	if err != nil {
		return DashboardReport{}, err
	}
	navCount, navFunds, navLast, err := s.dashboardNAV(ctx)
	if err != nil {
		return DashboardReport{}, err
	}
	heldFunds, err := s.dashboardHeldFunds(ctx)
	if err != nil {
		return DashboardReport{}, err
	}
	secTotal, err := s.dashboardSecuritiesTotal(ctx)
	if err != nil {
		return DashboardReport{}, err
	}
	anomalies, err := s.dashboardAnomalies(ctx)
	if err != nil {
		return DashboardReport{}, err
	}
	fresh, err := s.dashboardNAVFresh(ctx, now)
	if err != nil {
		return DashboardReport{}, err
	}
	dbSize, err := s.dashboardDatabaseSize(ctx)
	if err != nil {
		return DashboardReport{}, err
	}

	uptimeSec := roundTenths(now.Sub(startedAt).Seconds())
	return DashboardReport{
		OK:        true,
		Timestamp: now.Format(time.RFC3339),
		System: DashboardSystem{
			UptimeSec:   uptimeSec,
			UptimeHuman: formatUptime(uptimeSec),
			Memory:      readDashboardMemory(),
			GoVersion:   runtime.Version(),
			Platform:    runtime.GOOS + "/" + runtime.GOARCH,
		},
		Database: DashboardDatabase{
			SizeBytes: dbSize,
			SizeMB:    roundHundredths(float64(dbSize) / 1024 / 1024),
		},
		Crawler: DashboardCrawler{
			NAVTotal:        navFunds,
			NAVFresh:        fresh,
			NAVFresh24H:     fresh, // legacy alias
			SuccessRatePct:  crawlerSuccessRate(fresh, heldFunds),
			FreshWindowDays: stalePriceDays,
		},
		State: DashboardState{
			TransactionCount: txCount,
			LastTransaction:  txLast,
			LastNAVDate:      navLast,
			HeldFunds:        heldFunds,
			NAVRecords:       navCount,
			NAVFunds:         navFunds,
			SecuritiesTotal:  secTotal,
			AnomalyCount:     len(anomalies),
			RecentAnomalies:  anomalies,
		},
		DecisionBoundary: "read_only",
	}, nil
}

func (s Service) dashboardTransactions(ctx context.Context) (int, *string, error) {
	var count int
	var last sql.NullString
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*), MAX(trade_time) FROM transactions").Scan(&count, &last); err != nil {
		return 0, nil, fmt.Errorf("dashboard transactions: %w", err)
	}
	return count, nullableStringPtr(last), nil
}

func (s Service) dashboardNAV(ctx context.Context) (count int, funds int, last *string, err error) {
	var lastNull sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COUNT(DISTINCT fund_code), MAX(date)
		FROM nav_history
	`).Scan(&count, &funds, &lastNull); err != nil {
		return 0, 0, nil, fmt.Errorf("dashboard nav: %w", err)
	}
	return count, funds, nullableStringPtr(lastNull), nil
}

func (s Service) dashboardHeldFunds(ctx context.Context) (int, error) {
	return s.countRows(ctx, "SELECT COUNT(*) FROM portfolio_snapshot WHERE held_shares > 0.001")
}

func (s Service) dashboardSecuritiesTotal(ctx context.Context) (int, error) {
	return s.countRows(ctx, "SELECT COUNT(*) FROM fund_details")
}

func (s Service) dashboardAnomalies(ctx context.Context) ([]DashboardAnomaly, error) {
	hasAnomaly, err := s.tableHasColumn(ctx, "transactions", "anomaly")
	if err != nil {
		return nil, err
	}
	if !hasAnomaly {
		return []DashboardAnomaly{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT seq, fund_code, direction, trade_time, anomaly
		FROM transactions
		WHERE anomaly IS NOT NULL
		ORDER BY seq
		LIMIT 20
	`)
	if err != nil {
		return nil, fmt.Errorf("dashboard anomalies: %w", err)
	}
	defer rows.Close()

	items := []DashboardAnomaly{}
	for rows.Next() {
		var item DashboardAnomaly
		var direction sql.NullString
		var tradeTime sql.NullString
		if err := rows.Scan(&item.Seq, &item.FundCode, &direction, &tradeTime, &item.Anomaly); err != nil {
			return nil, fmt.Errorf("scan dashboard anomaly: %w", err)
		}
		item.FundCode = clampAdminText(item.FundCode, 32)
		item.Anomaly = clampAdminText(item.Anomaly, 500)
		if direction.Valid {
			direction.String = clampAdminText(direction.String, 32)
		}
		if tradeTime.Valid {
			tradeTime.String = clampAdminText(tradeTime.String, 40)
		}
		item.Direction = nullableStringPtr(direction)
		item.TradeTime = nullableStringPtr(tradeTime)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dashboard anomaly rows: %w", err)
	}
	return items, nil
}

// dashboardNAVFresh counts distinct fund_codes with a NAV date inside the
// freshness stale window (calendar days). Fund/QDII NAV is typically T+1, so a
// hard 24h wall-clock window falsely reports 0 success mid-morning (#73).
func (s Service) dashboardNAVFresh(ctx context.Context, now time.Time) (int, error) {
	// Inclusive window of stalePriceDays calendar days ending today (UTC date).
	// Example: stalePriceDays=4 and today=2026-07-17 → cutoff date 2026-07-14.
	cutoff := now.UTC().AddDate(0, 0, -(stalePriceDays - 1)).Format("2006-01-02")
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT fund_code)
		FROM nav_history
		WHERE date >= ?
	`, cutoff).Scan(&count); err != nil {
		return 0, fmt.Errorf("dashboard fresh nav: %w", err)
	}
	return count, nil
}

func (s Service) dashboardDatabaseSize(ctx context.Context) (int64, error) {
	if s.driver == "pg" {
		return s.pgDatabaseSize(ctx)
	}
	return s.sqliteDatabaseSize(ctx)
}

func (s Service) pgDatabaseSize(ctx context.Context) (int64, error) {
	var size int64
	if err := s.db.QueryRowContext(ctx,
		"SELECT pg_database_size(current_database())",
	).Scan(&size); err != nil {
		return 0, fmt.Errorf("pg database size: %w", err)
	}
	return size, nil
}

func (s Service) sqliteDatabaseSize(ctx context.Context) (int64, error) {
	rows, err := s.db.QueryContext(ctx, "PRAGMA database_list")
	if err != nil {
		return 0, fmt.Errorf("database_list: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var seq int
		var name string
		var file string
		if err := rows.Scan(&seq, &name, &file); err != nil {
			return 0, fmt.Errorf("scan database_list: %w", err)
		}
		if name != "main" || file == "" {
			continue
		}
		info, err := os.Stat(file)
		if err == nil {
			return info.Size(), nil
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("database_list rows: %w", err)
	}

	pageCount, err := s.querySingleInt(ctx, "PRAGMA page_count")
	if err != nil {
		return 0, fmt.Errorf("page_count: %w", err)
	}
	pageSize, err := s.querySingleInt(ctx, "PRAGMA page_size")
	if err != nil {
		return 0, fmt.Errorf("page_size: %w", err)
	}
	return int64(pageCount * pageSize), nil
}

func readDashboardMemory() DashboardMemory {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return DashboardMemory{
		RSSMB:       roundTenths(float64(mem.Sys) / 1024 / 1024),
		HeapUsedMB:  roundTenths(float64(mem.HeapAlloc) / 1024 / 1024),
		HeapTotalMB: roundTenths(float64(mem.HeapSys) / 1024 / 1024),
	}
}

func crawlerSuccessRate(fresh int, held int) float64 {
	if held <= 0 {
		return 0
	}
	return roundTenths((float64(fresh) / float64(held)) * 100)
}

func formatUptime(seconds float64) string {
	total := int(seconds)
	days := total / 86400
	hours := (total % 86400) / 3600
	minutes := (total % 3600) / 60
	secs := total % 60
	parts := []string{}
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	parts = append(parts, fmt.Sprintf("%ds", secs))
	return strings.Join(parts, " ")
}

func roundHundredths(value float64) float64 {
	return math.Round(value*100) / 100
}

// clampAdminText bounds free-text admin JSON fields (#243/#244).
func clampAdminText(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max])
	}
	return s
}
