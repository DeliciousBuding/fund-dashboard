package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
)

type TimelineEntry struct {
	Date       string  `json:"date"`
	TotalValue float64 `json:"total_value"`
	TotalCost  float64 `json:"total_cost"`
	PNL        float64 `json:"pnl"`
	PNLPct     float64 `json:"pnl_pct"`
}

type timelineNAVRow struct {
	Date    string
	Code    string
	UnitNAV float64
}

type timelineTxPoint struct {
	Date   string
	Shares float64
	Cost   float64
}

func (s Service) GetTimeline(ctx context.Context, portfolioID int) ([]TimelineEntry, error) {
	portfolioID = clampPortfolioID(portfolioID)

	navRows, err := s.timelineNAVRows(ctx, portfolioID)
	if err != nil {
		return nil, err
	}
	if len(navRows) == 0 {
		return []TimelineEntry{}, nil
	}

	fundTx, err := s.timelineTransactions(ctx, portfolioID)
	if err != nil {
		return nil, err
	}

	navByFund := map[string][]timelineNAVRow{}
	dateSet := map[string]struct{}{}
	for _, row := range navRows {
		row.Date = dateOnly(row.Date)
		dateSet[row.Date] = struct{}{}
		navByFund[row.Code] = append(navByFund[row.Code], row)
	}

	dates := make([]string, 0, len(dateSet))
	for date := range dateSet {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	fundCodes := make([]string, 0, len(navByFund))
	for code := range navByFund {
		fundCodes = append(fundCodes, code)
	}
	sort.Strings(fundCodes)

	txPointers := map[string]int{}
	navPointers := map[string]int{}
	states := map[string]timelineState{}
	for _, code := range fundCodes {
		states[code] = timelineState{NAV: math.NaN()}
	}

	var timeline []TimelineEntry
	for _, date := range dates {
		var value float64
		var cost float64
		hasPosition := false

		for _, code := range fundCodes {
			state := states[code]
			txs := fundTx[code]
			txPtr := txPointers[code]
			for txPtr < len(txs) && txs[txPtr].Date <= date {
				state.Shares = txs[txPtr].Shares
				state.Cost = txs[txPtr].Cost
				txPtr++
			}
			txPointers[code] = txPtr

			navs := navByFund[code]
			navPtr := navPointers[code]
			for navPtr < len(navs) && navs[navPtr].Date <= date {
				state.NAV = navs[navPtr].UnitNAV
				navPtr++
			}
			navPointers[code] = navPtr
			states[code] = state

			if state.Shares > 0.001 && !math.IsNaN(state.NAV) {
				hasPosition = true
				value += state.Shares * state.NAV
				cost += state.Cost
			}
		}

		if hasPosition {
			costAbs := math.Abs(cost)
			pnl := round2(value + cost)
			pnlPct := 0.0
			if costAbs > 0.01 {
				pnlPct = round2(pnl / costAbs * 100)
			}
			timeline = append(timeline, TimelineEntry{
				Date:       date,
				TotalValue: round2(value),
				TotalCost:  round2(costAbs),
				PNL:        pnl,
				PNLPct:     pnlPct,
			})
		}
	}

	return timeline, nil
}

type timelineState struct {
	Shares float64
	Cost   float64
	NAV    float64
}

func (s Service) timelineNAVRows(ctx context.Context, portfolioID int) ([]timelineNAVRow, error) {
	// Hard cap total NAV rows for multi-fund timelines (#219).
	const maxTimelineNavRows = 100000
	rows, err := s.db.QueryContext(ctx, `
		SELECT n.date, n.fund_code, n.unit_nav
		FROM nav_history n
		JOIN portfolio_snapshot ps ON ps.fund_code = n.fund_code AND COALESCE(ps.portfolio_id, 1) = ?
		WHERE CAST(n.date AS DATE) >= (
			SELECT MIN(CAST(t.trade_time AS DATE))
			FROM transactions t
			JOIN portfolio_snapshot ps2 ON ps2.fund_code = t.fund_code AND COALESCE(ps2.portfolio_id, 1) = ?
		)
		ORDER BY n.date, n.fund_code
		LIMIT ?
	`, portfolioID, portfolioID, maxTimelineNavRows)
	if err != nil {
		return nil, fmt.Errorf("query timeline nav rows: %w", err)
	}
	defer rows.Close()

	var navRows []timelineNAVRow
	for rows.Next() {
		var row timelineNAVRow
		if err := rows.Scan(&row.Date, &row.Code, &row.UnitNAV); err != nil {
			return nil, fmt.Errorf("scan timeline nav row: %w", err)
		}
		navRows = append(navRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("timeline nav rows: %w", err)
	}
	return navRows, nil
}

func (s Service) timelineTransactions(ctx context.Context, portfolioID int) (map[string][]timelineTxPoint, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			t.fund_code,
			date(t.trade_time) as trade_date,
			t.signed_share_change,
			t.signed_cash_flow
		FROM transactions t
		JOIN portfolio_snapshot ps ON ps.fund_code = t.fund_code AND COALESCE(ps.portfolio_id, 1) = ?
		ORDER BY t.fund_code, t.trade_time
		LIMIT 20000
	`, portfolioID)
	if err != nil {
		return nil, fmt.Errorf("query timeline transactions: %w", err)
	}
	defer rows.Close()

	fundTx := map[string][]timelineTxPoint{}
	for rows.Next() {
		var code string
		var date string
		var signedShares sql.NullFloat64
		var signedCash sql.NullFloat64
		if err := rows.Scan(&code, &date, &signedShares, &signedCash); err != nil {
			return nil, fmt.Errorf("scan timeline transaction: %w", err)
		}

		points := fundTx[code]
		point := timelineTxPoint{Date: dateOnly(date)}
		if len(points) > 0 {
			point.Shares = points[len(points)-1].Shares
			point.Cost = points[len(points)-1].Cost
		}
		if signedShares.Valid {
			point.Shares += signedShares.Float64
		}
		if signedCash.Valid {
			point.Cost += signedCash.Float64
		}
		fundTx[code] = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("timeline transaction rows: %w", err)
	}
	return fundTx, nil
}
