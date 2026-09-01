package portfolio

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/DeliciousBuding/fund-dashboard/internal/snapshot"
	"math"
	"strings"
	"time"
)

type AdjustPositionInput struct {
	Code        string
	Shares      float64
	PortfolioID int
	Reason      string
}

type AdjustPositionResult struct {
	OK          bool             `json:"ok"`
	Code        string           `json:"code"`
	Shares      float64          `json:"shares"`
	Security    SecurityListItem `json:"security"`
	PortfolioID int              `json:"portfolio_id"`
	Reason      string           `json:"reason,omitempty"`
	OrderID     string           `json:"order_id,omitempty"`
	DeltaShares float64          `json:"delta_shares,omitempty"`
}

// AdjustPosition sets held shares via a synthetic balancing transaction so the
// ledger (transactions SUM) remains the SSOT. Subsequent price refresh / import /
// DCA recalcs recompute held_shares from transactions and no longer clobber the
// override (#198).
func (s Service) AdjustPosition(ctx context.Context, in AdjustPositionInput) (AdjustPositionResult, error) {
	code := strings.TrimSpace(in.Code)
	if code == "" {
		return AdjustPositionResult{}, fmt.Errorf("fund_code is required")
	}
	if len(code) > 32 {
		return AdjustPositionResult{}, fmt.Errorf("fund_code too long")
	}
	if math.IsNaN(in.Shares) || math.IsInf(in.Shares, 0) {
		return AdjustPositionResult{}, fmt.Errorf("shares must be a finite number")
	}
	if in.Shares < 0 {
		return AdjustPositionResult{}, fmt.Errorf("shares must be >= 0")
	}
	// Bound target shares to avoid absurd ledger deltas via MCP/admin (#223).
	const maxAdjustShares = 1e9
	if in.Shares > maxAdjustShares {
		return AdjustPositionResult{}, fmt.Errorf("shares too large")
	}
	reason := strings.TrimSpace(in.Reason)
	if len(reason) > 200 {
		return AdjustPositionResult{}, fmt.Errorf("reason too long")
	}
	portfolioID := clampPortfolioID(in.PortfolioID)
	target := in.Shares
	if target > 0 && target < 0.001 {
		target = 0
	}

	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM fund_details WHERE fund_code = ?`, code).Scan(&exists); err != nil {
		return AdjustPositionResult{}, fmt.Errorf("lookup security: %w", err)
	}
	if exists == 0 {
		return AdjustPositionResult{}, fmt.Errorf("security not found: %s", code)
	}

	var sumShares sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, `
		SELECT SUM(COALESCE(signed_share_change, 0)) FROM transactions WHERE fund_code = ?
	`, code).Scan(&sumShares); err != nil {
		return AdjustPositionResult{}, fmt.Errorf("sum held shares: %w", err)
	}
	current := 0.0
	if sumShares.Valid {
		current = sumShares.Float64
	}
	delta := target - current
	if math.Abs(delta) < 0.001 {
		if err := snapshot.RecalcForPortfolio(ctx, s.db, code, portfolioID, snapshot.ModeLight); err != nil {
			return AdjustPositionResult{}, fmt.Errorf("recalc snapshot: %w", err)
		}
		item, err := s.getSecurityItem(ctx, code)
		if err != nil {
			return AdjustPositionResult{}, err
		}
		return AdjustPositionResult{
			OK: true, Code: code, Shares: target, Security: item,
			PortfolioID: portfolioID, Reason: strings.TrimSpace(in.Reason), DeltaShares: 0,
		}, nil
	}

	var detailName sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT fund_name FROM fund_details WHERE fund_code = ?
	`, code).Scan(&detailName); err != nil && err != sql.ErrNoRows {
		return AdjustPositionResult{}, fmt.Errorf("lookup fund_details: %w", err)
	}
	fundName := code
	if detailName.Valid && detailName.String != "" {
		fundName = clampPortfolioText(detailName.String, 200)
	}

	now := time.Now().In(time.FixedZone("CST", 8*3600))
	tradeTime := now.Format("2006-01-02T15:04:05-07:00")
	confirmDate := now.Format("2006-01-02")
	orderID := fmt.Sprintf("adj-%s-%d", code, now.UnixNano())
	tradeType := "持仓调整"
	direction := "buy"
	if delta < 0 {
		direction = "sell"
	}
	absShares := math.Abs(delta)

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO transactions
		(order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name,
		 confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
		SELECT ?, ?, ?, ?, ?, ?, ?, 0, ?, 0, 0, ?, 0
		WHERE NOT EXISTS (SELECT 1 FROM transactions WHERE order_id = ? AND fund_code = ?)
	`, orderID, tradeTime, confirmDate, tradeType, direction, code, fundName,
		absShares, delta, orderID, code)
	if err != nil {
		return AdjustPositionResult{}, fmt.Errorf("insert balancing transaction: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return AdjustPositionResult{}, fmt.Errorf("balancing transaction rows affected: %w", raErr)
	}
	if n == 0 {
		// Extremely unlikely (nanosecond order_id) but treat as conflict (#212).
		return AdjustPositionResult{}, fmt.Errorf("balancing transaction not inserted (duplicate order_id)")
	}

	// Recalc from transactions SSOT so held_shares matches the new ledger row.
	if err := snapshot.RecalcForPortfolio(ctx, s.db, code, portfolioID, snapshot.ModeLight); err != nil {
		return AdjustPositionResult{}, fmt.Errorf("recalc snapshot after adjust: %w", err)
	}

	var held sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, `
		SELECT held_shares FROM portfolio_snapshot
		WHERE fund_code = ? AND COALESCE(portfolio_id,1) = ?
	`, code, portfolioID).Scan(&held); err != nil {
		if err == sql.ErrNoRows {
			return AdjustPositionResult{}, fmt.Errorf("adjust verify failed: portfolio_snapshot missing for %s", code)
		}
		return AdjustPositionResult{}, fmt.Errorf("adjust verify read held_shares: %w", err)
	}
	got := 0.0
	if held.Valid {
		got = held.Float64
	}
	if math.Abs(got-target) > 0.01 {
		return AdjustPositionResult{}, fmt.Errorf("adjust verify failed: held_shares=%.6f want=%.6f", got, target)
	}

	item, err := s.getSecurityItem(ctx, code)
	if err != nil {
		return AdjustPositionResult{}, err
	}
	return AdjustPositionResult{
		OK:          true,
		Code:        code,
		Shares:      target,
		Security:    item,
		PortfolioID: portfolioID,
		Reason:      reason,
		OrderID:     orderID,
		DeltaShares: delta,
	}, nil
}
