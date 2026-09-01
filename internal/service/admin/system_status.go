package admin

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"
)

type SystemStatusReport struct {
	OK               bool                 `json:"ok"`
	UptimeSec        float64              `json:"uptime_sec"`
	ResponseMS       int64                `json:"response_ms"`
	Transactions     SystemCountLast      `json:"transactions"`
	NAV              SystemNAVStats       `json:"nav"`
	Portfolio        SystemPortfolioStats `json:"portfolio"`
	Securities       SystemSecurityStats  `json:"securities"`
	Anomalies        SystemAnomalyStats   `json:"anomalies"`
	MarketSchedule   MarketSchedule       `json:"market_schedule"`
	ServerTime       string               `json:"server_time"`
	DecisionBoundary string               `json:"decision_boundary"`
	SideEffects      string               `json:"side_effects"`
}

type SystemCountLast struct {
	Count int     `json:"count"`
	Last  *string `json:"last"`
}

type SystemNAVStats struct {
	Count int     `json:"count"`
	Funds int     `json:"funds"`
	First *string `json:"first"`
	Last  *string `json:"last"`
}

type SystemPortfolioStats struct {
	HeldFunds int `json:"held_funds"`
}

type SystemSecurityStats struct {
	Total  int `json:"total"`
	Funds  int `json:"funds"`
	Stocks int `json:"stocks"`
}

type SystemAnomalyStats struct {
	Count int                 `json:"count"`
	Items []SystemAnomalyItem `json:"items"`
}

type SystemAnomalyItem struct {
	Seq       int     `json:"seq"`
	FundCode  string  `json:"fund_code"`
	Direction *string `json:"direction"`
	TradeTime *string `json:"trade_time"`
	Anomaly   string  `json:"anomaly"`
}

type MarketSchedule struct {
	ChinaAShare MarketWindow `json:"china_a_share"`
	HongKong    MarketWindow `json:"hong_kong"`
	US          MarketWindow `json:"us"`
}

type MarketWindow struct {
	Status    string  `json:"status"`
	NextOpen  *string `json:"next_open"`
	NextClose *string `json:"next_close"`
}

func (s Service) GetSystemStatus(ctx context.Context, startedAt time.Time, now time.Time) (SystemStatusReport, error) {
	transactions, err := s.queryTransactions(ctx)
	if err != nil {
		return SystemStatusReport{}, err
	}
	nav, err := s.queryNAV(ctx)
	if err != nil {
		return SystemStatusReport{}, err
	}
	portfolio, err := s.querySystemPortfolio(ctx)
	if err != nil {
		return SystemStatusReport{}, err
	}
	securities, err := s.querySystemSecurities(ctx)
	if err != nil {
		return SystemStatusReport{}, err
	}
	anomalies, err := s.querySystemAnomalies(ctx)
	if err != nil {
		return SystemStatusReport{}, err
	}

	return SystemStatusReport{
		OK:        true,
		UptimeSec: roundTenths(now.Sub(startedAt).Seconds()),
		Transactions: SystemCountLast{
			Count: transactions.Count,
			Last:  transactions.Last,
		},
		NAV: SystemNAVStats{
			Count: nav.Count,
			Funds: nav.Funds,
			First: nav.First,
			Last:  nav.Last,
		},
		Portfolio:        portfolio,
		Securities:       securities,
		Anomalies:        anomalies,
		MarketSchedule:   buildMarketSchedule(now),
		ServerTime:       now.UTC().Format("2006-01-02 15:04:05"),
		DecisionBoundary: "read_only",
		SideEffects:      "none",
	}, nil
}

func (s Service) querySystemPortfolio(ctx context.Context) (SystemPortfolioStats, error) {
	held, err := s.countRows(ctx, "SELECT COUNT(*) FROM portfolio_snapshot WHERE held_shares > 0.001")
	if err != nil {
		return SystemPortfolioStats{}, fmt.Errorf("admin system status portfolio: %w", err)
	}
	return SystemPortfolioStats{HeldFunds: held}, nil
}

func (s Service) querySystemSecurities(ctx context.Context) (SystemSecurityStats, error) {
	var stats SystemSecurityStats
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN security_type = 'fund' OR security_type IS NULL THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN security_type = 'stock' THEN 1 ELSE 0 END), 0)
		FROM fund_details
	`).Scan(&stats.Total, &stats.Funds, &stats.Stocks); err != nil {
		return SystemSecurityStats{}, fmt.Errorf("admin system status securities: %w", err)
	}
	return stats, nil
}

func (s Service) querySystemAnomalies(ctx context.Context) (SystemAnomalyStats, error) {
	hasAnomaly, err := s.dialect.HasColumn(ctx, "transactions", "anomaly")
	if err != nil {
		return SystemAnomalyStats{}, err
	}
	if !hasAnomaly {
		return SystemAnomalyStats{Items: []SystemAnomalyItem{}}, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT seq, fund_code, direction, trade_time, anomaly
		FROM transactions
		WHERE anomaly IS NOT NULL
		ORDER BY seq
		LIMIT ?
	`, maxRecentAnomalies)
	if err != nil {
		return SystemAnomalyStats{}, fmt.Errorf("admin system status anomalies: %w", err)
	}
	defer rows.Close()

	items := []SystemAnomalyItem{}
	for rows.Next() {
		var item SystemAnomalyItem
		var direction sql.NullString
		var tradeTime sql.NullString
		if err := rows.Scan(&item.Seq, &item.FundCode, &direction, &tradeTime, &item.Anomaly); err != nil {
			return SystemAnomalyStats{}, fmt.Errorf("scan admin system anomaly: %w", err)
		}
		// Bound free-text fields (#244; parity with dashboard #243).
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
		return SystemAnomalyStats{}, fmt.Errorf("admin system anomaly rows: %w", err)
	}
	return SystemAnomalyStats{Count: len(items), Items: items}, nil
}

func buildMarketSchedule(now time.Time) MarketSchedule {
	return MarketSchedule{
		ChinaAShare: marketWindow(now, "Asia/Shanghai", 9, 30, 15, 0),
		HongKong:    marketWindow(now, "Asia/Hong_Kong", 9, 30, 16, 0),
		US:          marketWindow(now, "America/New_York", 9, 30, 16, 0),
	}
}

func marketWindow(now time.Time, location string, openHour int, openMinute int, closeHour int, closeMinute int) MarketWindow {
	loc, err := time.LoadLocation(location)
	if err != nil {
		return MarketWindow{Status: "unknown"}
	}
	local := now.In(loc)
	open := time.Date(local.Year(), local.Month(), local.Day(), openHour, openMinute, 0, 0, loc)
	close := time.Date(local.Year(), local.Month(), local.Day(), closeHour, closeMinute, 0, 0, loc)

	if local.Weekday() == time.Saturday || local.Weekday() == time.Sunday {
		next := nextWeekdayOpen(local, loc, openHour, openMinute)
		nextOpen := next.UTC().Format(time.RFC3339)
		return MarketWindow{Status: "closed", NextOpen: &nextOpen}
	}
	if local.Before(open) {
		nextOpen := open.UTC().Format(time.RFC3339)
		nextClose := close.UTC().Format(time.RFC3339)
		return MarketWindow{Status: "closed", NextOpen: &nextOpen, NextClose: &nextClose}
	}
	if local.Before(close) {
		nextClose := close.UTC().Format(time.RFC3339)
		return MarketWindow{Status: "open", NextClose: &nextClose}
	}
	next := nextWeekdayOpen(local, loc, openHour, openMinute)
	nextOpen := next.UTC().Format(time.RFC3339)
	return MarketWindow{Status: "closed", NextOpen: &nextOpen}
}

func nextWeekdayOpen(local time.Time, loc *time.Location, openHour int, openMinute int) time.Time {
	next := local.AddDate(0, 0, 1)
	for next.Weekday() == time.Saturday || next.Weekday() == time.Sunday {
		next = next.AddDate(0, 0, 1)
	}
	return time.Date(next.Year(), next.Month(), next.Day(), openHour, openMinute, 0, 0, loc)
}

func roundTenths(value float64) float64 {
	return math.Round(value*10) / 10
}
