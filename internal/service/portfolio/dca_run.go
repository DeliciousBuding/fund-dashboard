package portfolio

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/DeliciousBuding/fund-dashboard/internal/dialect"
	"github.com/DeliciousBuding/fund-dashboard/internal/snapshot"
	"strings"
	"time"
)

type RunDCAAutoInvestInput struct {
	AsOf        string // YYYY-MM-DD, default today local
	PortfolioID int
	PlanID      int    // optional filter
	FundCode    string // optional filter
	DryRun      bool
	Mode        string // "" = fixed plan amount; "nav_deviation"|"change_pct" via ComputeDCAAmount
	BaseAmount  float64
}

type DCAExecutionItem struct {
	PlanID      int      `json:"plan_id"`
	FundCode    string   `json:"fund_code"`
	FundName    string   `json:"fund_name,omitempty"`
	Amount      float64  `json:"amount"`
	Shares      *float64 `json:"shares,omitempty"`
	NAV         *float64 `json:"nav,omitempty"`
	OrderID     string   `json:"order_id"`
	Status      string   `json:"status"` // executed|skipped_duplicate|skipped_no_nav|skipped_not_due|preview
	Message     string   `json:"message,omitempty"`
	TradeType   string   `json:"trade_type,omitempty"`
	PortfolioID int      `json:"portfolio_id"`
	WeekdayMask string   `json:"weekday_mask,omitempty"`
}

type RunDCAAutoInvestResult struct {
	OK               bool               `json:"ok"`
	AsOf             string             `json:"as_of"`
	DryRun           bool               `json:"dry_run"`
	Executed         int                `json:"executed"`
	Skipped          int                `json:"skipped"`
	Previewed        int                `json:"previewed"`
	Items            []DCAExecutionItem `json:"items"`
	DecisionBoundary string             `json:"decision_boundary"`
	SideEffects      string             `json:"side_effects"`
}

// RunDCAAutoInvest materializes due DCA plans as buy transactions (or previews them).
// Local ledger only — no broker placement. Idempotent via order_id.
func (s Service) RunDCAAutoInvest(ctx context.Context, in RunDCAAutoInvestInput) (RunDCAAutoInvestResult, error) {
	asOf := strings.TrimSpace(in.AsOf)
	if asOf == "" {
		asOf = time.Now().Format("2006-01-02")
	}
	if len(asOf) > 10 {
		asOf = asOf[:10]
	}
	day, err := time.ParseInLocation("2006-01-02", asOf, time.Local)
	if err != nil {
		return RunDCAAutoInvestResult{}, NewValidationError("as_of must be YYYY-MM-DD: %v", err)
	}
	portfolioID := in.PortfolioID
	portfolioID = clampPortfolioID(portfolioID)

	maskDay := int(day.Weekday())
	if maskDay == 0 {
		maskDay = 7
	}

	where := "WHERE active = 1 AND portfolio_id = ?"
	args := []any{portfolioID}
	if in.PlanID > 0 {
		where += " AND id = ?"
		args = append(args, in.PlanID)
	}
	if code := strings.TrimSpace(in.FundCode); code != "" {
		where += " AND fund_code = ?"
		args = append(args, code)
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, fund_code, COALESCE(fund_name,''), amount, frequency, weekday_mask,
			trade_type, portfolio_id, start_date, end_date
		FROM dca_plans
		%s
		ORDER BY id
		LIMIT 5000
	`, where), args...)
	if err != nil {
		return RunDCAAutoInvestResult{}, fmt.Errorf("list dca plans: %w", err)
	}
	defer rows.Close()

	type planRow struct {
		id, portfolioID               int
		code, name, freq, mask, ttype string
		amount                        float64
		start, end                    string
		endValid                      bool
	}
	var plans []planRow
	for rows.Next() {
		var p planRow
		var end sql.NullString
		if err := rows.Scan(&p.id, &p.code, &p.name, &p.amount, &p.freq, &p.mask, &p.ttype, &p.portfolioID, &p.start, &end); err != nil {
			return RunDCAAutoInvestResult{}, err
		}
		if end.Valid {
			p.end = end.String
			p.endValid = true
		}
		plans = append(plans, p)
	}
	if err := rows.Err(); err != nil {
		return RunDCAAutoInvestResult{}, err
	}

	result := RunDCAAutoInvestResult{
		OK: true, AsOf: asOf, DryRun: in.DryRun, Items: []DCAExecutionItem{},
		DecisionBoundary: "facts_only",
	}
	if in.DryRun {
		result.SideEffects = "none"
	} else {
		result.SideEffects = "transaction_write_and_snapshot_recalc"
	}

	for _, p := range plans {
		item := DCAExecutionItem{
			PlanID: p.id, FundCode: p.code, FundName: p.name, Amount: p.amount,
			OrderID:   fmt.Sprintf("DCA-%d-%s", p.id, strings.ReplaceAll(asOf, "-", "")),
			TradeType: p.ttype, PortfolioID: p.portfolioID, WeekdayMask: p.mask,
		}
		// date window
		if p.start != "" && asOf < p.start[:minInt(10, len(p.start))] {
			item.Status = "skipped_not_due"
			item.Message = "before start_date"
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		if p.endValid && p.end != "" && asOf > p.end[:minInt(10, len(p.end))] {
			item.Status = "skipped_not_due"
			item.Message = "after end_date"
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		if !weekdayMaskHitLocal(p.mask, maskDay) {
			item.Status = "skipped_not_due"
			item.Message = clampPortfolioText(fmt.Sprintf("weekday %d not in mask %s", maskDay, p.mask), 200)
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}

		// amount mode
		amount := p.amount
		if in.Mode == "nav_deviation" || in.Mode == "change_pct" {
			base := in.BaseAmount
			if base <= 0 {
				base = p.amount
			}
			plan, err := s.ComputeDCAAmount(ctx, ComputeDCAAmountOptions{
				Code: p.code, BaseAmount: base, Mode: in.Mode, PortfolioID: portfolioID,
			})
			if err == nil && plan.ActualAmount > 0 {
				amount = plan.ActualAmount
			}
		}
		item.Amount = amount

		// latest NAV
		var nav sql.NullFloat64
		if err := s.db.QueryRowContext(ctx, `
			SELECT unit_nav FROM nav_history WHERE fund_code = ? ORDER BY date DESC LIMIT 1
		`, p.code).Scan(&nav); err != nil && err != sql.ErrNoRows {
			item.Status = "error"
			item.Message = "internal_error"
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		if !nav.Valid || nav.Float64 <= 0 {
			item.Status = "skipped_no_nav"
			item.Message = "latest NAV unavailable"
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		navVal := nav.Float64
		shares := amount / navVal
		item.NAV = &navVal
		item.Shares = &shares

		// duplicate via transactions.order_id or dca_plan_executions ledger
		var exists int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM transactions WHERE order_id = ? AND fund_code = ?`, item.OrderID, p.code).Scan(&exists); err != nil {
			return result, fmt.Errorf("check order_id %s: %w", item.OrderID, err)
		}
		if exists == 0 {
			if err := s.db.QueryRowContext(ctx, `
				SELECT COUNT(1) FROM dca_plan_executions
				WHERE plan_id = ? AND trade_date = ? AND status IN ('executed','success','ok')
			`, p.id, asOf).Scan(&exists); err != nil {
				// Missing ledger table on older SQLite fixtures — treat as no prior execution.
				if dialect.IsMissingTableError(err) {
					exists = 0
				} else {
					return result, fmt.Errorf("check dca_plan_executions plan=%d date=%s: %w", p.id, asOf, err)
				}
			}
		}
		if exists > 0 {
			item.Status = "skipped_duplicate"
			item.Message = "order or execution already exists"
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}

		if in.DryRun {
			item.Status = "preview"
			item.Message = "dry_run"
			result.Previewed++
			result.Items = append(result.Items, item)
			continue
		}

		// Atomic materialization: claim insert + execution ledger in one TX (#211).
		// Snapshot recalc stays best-effort after commit (rebuilds from transactions SSOT).
		fee := 0.0
		signedCash := -amount
		signedShare := shares
		tradeType := p.ttype
		if tradeType == "" {
			tradeType = "定投买入"
		}
		fundName := p.name
		if fundName == "" {
			fundName = p.code
		}
		tradeTime := asOf + "T09:30:00+08:00"
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			item.Status = "error"
			item.Message = "begin_tx_failed"
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO transactions
			(order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name,
			 confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
			SELECT ?, ?, ?, ?, 'buy', ?, ?, ?, ?, ?, ?, ?, 1
			WHERE NOT EXISTS (SELECT 1 FROM transactions WHERE order_id = ? AND fund_code = ?)
		`, item.OrderID, tradeTime, asOf, tradeType, p.code, fundName,
			amount, shares, fee, signedCash, signedShare, item.OrderID, p.code)
		if err != nil {
			_ = tx.Rollback()
			item.Status = "error"
			item.Message = "internal_error"
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		affected, err := res.RowsAffected()
		if err != nil {
			_ = tx.Rollback()
			item.Status = "error"
			item.Message = "rows_affected_failed"
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		if affected == 0 {
			_ = tx.Rollback()
			item.Status = "skipped_duplicate"
			item.Message = "order or execution already exists"
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		if err := s.recordDCAPlanExecutionTx(ctx, tx, p.id, p.code, asOf, amount, item.OrderID, navVal, "executed", ""); err != nil {
			_ = tx.Rollback()
			item.Status = "error"
			item.Message = "execution_ledger_failed"
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		if err := tx.Commit(); err != nil {
			item.Status = "error"
			item.Message = "commit_failed"
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		var postMsgs []string
		if err := snapshot.RecalcForPortfolio(ctx, s.db, p.code, portfolioID, snapshot.ModeLight); err != nil {
			postMsgs = append(postMsgs, "snapshot_recalc_failed")
		}
		item.Status = "executed"
		if len(postMsgs) > 0 {
			item.Message = strings.Join(postMsgs, "; ")
		}
		result.Executed++
		result.Items = append(result.Items, item)
	}

	return result, nil
}

type dcaExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (s Service) recordDCAPlanExecutionTx(ctx context.Context, ex dcaExecer, planID int, fundCode, tradeDate string, amount float64, orderID string, nav float64, status, message string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := ex.ExecContext(ctx, `
		INSERT INTO dca_plan_executions
			(plan_id, fund_code, trade_date, amount, status, order_id, nav_date, nav, message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, planID, fundCode, tradeDate, amount, status, orderID, tradeDate, nav, nullIfEmpty(message), now, now)
	if err != nil {
		if dialect.IsMissingTableError(err) {
			return nil
		}
	}
	return err
}

func weekdayMaskHitLocal(mask string, day int) bool {
	mask = strings.TrimSpace(mask)
	if mask == "" {
		// empty mask: treat as weekdays 1-5
		return day >= 1 && day <= 5
	}
	parts := strings.Split(mask, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(p, "-") {
			var a, b int
			if _, err := fmt.Sscanf(p, "%d-%d", &a, &b); err == nil && day >= a && day <= b {
				return true
			}
			continue
		}
		var n int
		if _, err := fmt.Sscanf(p, "%d", &n); err == nil && n == day {
			return true
		}
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
