package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type ListDCAPlansOptions struct {
	ActiveOnly  bool
	PortfolioID int
}

type DCAPlan struct {
	ID          int     `json:"id"`
	FundCode    string  `json:"fund_code"`
	FundName    *string `json:"fund_name"`
	Amount      float64 `json:"amount"`
	Frequency   string  `json:"frequency"`
	WeekdayMask string  `json:"weekday_mask"`
	TradeType   string  `json:"trade_type"`
	PortfolioID int     `json:"portfolio_id"`
	StartDate   string  `json:"start_date"`
	EndDate     *string `json:"end_date"`
	Active      int     `json:"active"`
	Source      string  `json:"source"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func (s Service) ListDCAPlans(ctx context.Context, opts ListDCAPlansOptions) ([]DCAPlan, error) {
	where := ""
	args := []any{}
	if opts.ActiveOnly {
		where = "WHERE active = 1"
	}
	if opts.PortfolioID > 0 {
		pid := opts.PortfolioID
		if pid > 1000 {
			pid = 1000
		}
		if where == "" {
			where = "WHERE portfolio_id = ?"
		} else {
			where += " AND portfolio_id = ?"
		}
		args = append(args, pid)
	}

	// Soft ceiling (#233): table is small in prod but list must stay bounded.
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT
			id,
			fund_code,
			fund_name,
			amount,
			frequency,
			weekday_mask,
			trade_type,
			portfolio_id,
			start_date,
			end_date,
			active,
			source,
			created_at,
			updated_at
		FROM dca_plans
		%s
		ORDER BY active DESC, fund_code
		LIMIT 5000
	`, where), args...)
	if err != nil {
		return nil, fmt.Errorf("query dca plans: %w", err)
	}
	defer rows.Close()

	var plans []DCAPlan
	for rows.Next() {
		var plan DCAPlan
		var fundName sql.NullString
		var endDate sql.NullString
		if err := rows.Scan(
			&plan.ID,
			&plan.FundCode,
			&fundName,
			&plan.Amount,
			&plan.Frequency,
			&plan.WeekdayMask,
			&plan.TradeType,
			&plan.PortfolioID,
			&plan.StartDate,
			&endDate,
			&plan.Active,
			&plan.Source,
			&plan.CreatedAt,
			&plan.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan dca plan: %w", err)
		}
		plan.FundCode = clampPortfolioText(plan.FundCode, 32)
		plan.Frequency = clampPortfolioText(plan.Frequency, 32)
		plan.WeekdayMask = clampPortfolioText(plan.WeekdayMask, 64)
		plan.TradeType = clampPortfolioText(plan.TradeType, 32)
		plan.Source = clampPortfolioText(plan.Source, 64)
		if fundName.Valid {
			v := clampPortfolioText(fundName.String, 200)
			plan.FundName = &v
		}
		if endDate.Valid {
			v := clampPortfolioText(endDate.String, 40)
			plan.EndDate = &v
		}
		plan.StartDate = clampPortfolioText(plan.StartDate, 40)
		plan.CreatedAt = clampPortfolioText(plan.CreatedAt, 40)
		plan.UpdatedAt = clampPortfolioText(plan.UpdatedAt, 40)
		plans = append(plans, plan)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dca plan rows: %w", err)
	}
	return plans, nil
}

type UpsertDCAPlanInput struct {
	ID          int
	FundCode    string
	FundName    string
	Amount      float64
	Frequency   string
	WeekdayMask string
	TradeType   string
	PortfolioID int
	StartDate   string
	EndDate     string
	Active      *int
	Source      string
}

type UpsertDCAPlanResult struct {
	OK   bool    `json:"ok"`
	Plan DCAPlan `json:"plan"`
}

type DisableDCAPlanResult struct {
	OK      bool `json:"ok"`
	ID      int  `json:"id"`
	Updated bool `json:"updated"`
}

func (s Service) UpsertDCAPlan(ctx context.Context, in UpsertDCAPlanInput) (UpsertDCAPlanResult, error) {
	code := strings.TrimSpace(in.FundCode)
	if code == "" {
		return UpsertDCAPlanResult{}, fmt.Errorf("fund_code is required")
	}
	if in.Amount <= 0 {
		return UpsertDCAPlanResult{}, fmt.Errorf("amount must be positive")
	}
	if in.Amount > 1_000_000 {
		return UpsertDCAPlanResult{}, fmt.Errorf("amount max 1000000")
	}
	if len(code) > 32 {
		return UpsertDCAPlanResult{}, fmt.Errorf("fund_code max 32 chars")
	}
	freq := strings.TrimSpace(in.Frequency)
	if freq == "" {
		freq = "weekday"
	}
	if len(freq) > 32 {
		return UpsertDCAPlanResult{}, fmt.Errorf("frequency max 32 chars")
	}
	mask := strings.TrimSpace(in.WeekdayMask)
	if mask == "" {
		mask = "1,2,3,4,5"
	}
	if len(mask) > 64 {
		return UpsertDCAPlanResult{}, fmt.Errorf("weekday_mask max 64 chars")
	}
	tradeType := strings.TrimSpace(in.TradeType)
	if tradeType == "" {
		tradeType = "auto"
	}
	if len(tradeType) > 64 {
		return UpsertDCAPlanResult{}, fmt.Errorf("trade_type max 64 chars")
	}
	pid := in.PortfolioID
	if pid <= 0 {
		pid = 1
	}
	if pid > 1000 {
		pid = 1000
	}
	start := strings.TrimSpace(in.StartDate)
	if start == "" {
		start = time.Now().Format("2006-01-02")
	}
	active := 1
	if in.Active != nil {
		if *in.Active != 0 && *in.Active != 1 {
			return UpsertDCAPlanResult{}, fmt.Errorf("active must be 0 or 1")
		}
		active = *in.Active
	}
	source := strings.TrimSpace(in.Source)
	if source == "" {
		source = "mcp"
	}
	if len(source) > 64 {
		return UpsertDCAPlanResult{}, fmt.Errorf("source max 64 chars")
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	var fundName any
	if strings.TrimSpace(in.FundName) != "" {
		fn := strings.TrimSpace(in.FundName)
		if len(fn) > 200 {
			return UpsertDCAPlanResult{}, fmt.Errorf("fund_name max 200 chars")
		}
		fundName = fn
	}
	var endDate any
	if strings.TrimSpace(in.EndDate) != "" {
		ed := strings.TrimSpace(in.EndDate)
		if len(ed) > 40 {
			return UpsertDCAPlanResult{}, fmt.Errorf("end_date max 40 chars")
		}
		endDate = ed
	}
	if len(start) > 40 {
		return UpsertDCAPlanResult{}, fmt.Errorf("start_date max 40 chars")
	}

	if in.ID > 0 {
		res, err := s.db.ExecContext(ctx, `
			UPDATE dca_plans SET
				fund_code = ?, fund_name = ?, amount = ?, frequency = ?, weekday_mask = ?,
				trade_type = ?, portfolio_id = ?, start_date = ?, end_date = ?, active = ?, source = ?, updated_at = ?
			WHERE id = ?
		`, code, fundName, in.Amount, freq, mask, tradeType, pid, start, endDate, active, source, now, in.ID)
		if err != nil {
			return UpsertDCAPlanResult{}, fmt.Errorf("update dca plan: %w", err)
		}
		n, raErr := res.RowsAffected()
		if raErr != nil {
			return UpsertDCAPlanResult{}, fmt.Errorf("update dca plan rows affected: %w", raErr)
		}
		if n == 0 {
			return UpsertDCAPlanResult{}, fmt.Errorf("dca plan id %d not found", in.ID)
		}
		plan, err := s.getDCAPlanByID(ctx, in.ID)
		if err != nil {
			return UpsertDCAPlanResult{}, err
		}
		return UpsertDCAPlanResult{OK: true, Plan: plan}, nil
	}

	var id int
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO dca_plans
			(fund_code, fund_name, amount, frequency, weekday_mask, trade_type, portfolio_id, start_date, end_date, active, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`, code, fundName, in.Amount, freq, mask, tradeType, pid, start, endDate, active, source, now, now).Scan(&id)
	if err != nil {
		return UpsertDCAPlanResult{}, fmt.Errorf("insert dca plan: %w", err)
	}
	plan, err := s.getDCAPlanByID(ctx, id)
	if err != nil {
		return UpsertDCAPlanResult{}, err
	}
	return UpsertDCAPlanResult{OK: true, Plan: plan}, nil
}

func (s Service) DisableDCAPlan(ctx context.Context, id int) (DisableDCAPlanResult, error) {
	if id <= 0 {
		return DisableDCAPlanResult{}, fmt.Errorf("id is required")
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	res, err := s.db.ExecContext(ctx, `UPDATE dca_plans SET active = 0, updated_at = ? WHERE id = ?`, now, id)
	if err != nil {
		return DisableDCAPlanResult{}, fmt.Errorf("disable dca plan: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return DisableDCAPlanResult{}, fmt.Errorf("disable dca plan rows affected: %w", raErr)
	}
	return DisableDCAPlanResult{OK: true, ID: id, Updated: n > 0}, nil
}

func (s Service) getDCAPlanByID(ctx context.Context, id int) (DCAPlan, error) {
	var plan DCAPlan
	var fundName sql.NullString
	var endDate sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, fund_code, fund_name, amount, frequency, weekday_mask, trade_type, portfolio_id, start_date, end_date, active, source, created_at, updated_at
		FROM dca_plans WHERE id = ?
	`, id).Scan(&plan.ID, &plan.FundCode, &fundName, &plan.Amount, &plan.Frequency, &plan.WeekdayMask, &plan.TradeType, &plan.PortfolioID, &plan.StartDate, &endDate, &plan.Active, &plan.Source, &plan.CreatedAt, &plan.UpdatedAt)
	if err != nil {
		return DCAPlan{}, fmt.Errorf("get dca plan: %w", err)
	}
	plan.FundCode = clampPortfolioText(plan.FundCode, 32)
	plan.Frequency = clampPortfolioText(plan.Frequency, 32)
	plan.WeekdayMask = clampPortfolioText(plan.WeekdayMask, 64)
	plan.TradeType = clampPortfolioText(plan.TradeType, 32)
	plan.Source = clampPortfolioText(plan.Source, 64)
	plan.StartDate = clampPortfolioText(plan.StartDate, 40)
	plan.CreatedAt = clampPortfolioText(plan.CreatedAt, 40)
	plan.UpdatedAt = clampPortfolioText(plan.UpdatedAt, 40)
	if fundName.Valid {
		v := clampPortfolioText(fundName.String, 200)
		plan.FundName = &v
	}
	if endDate.Valid {
		v := clampPortfolioText(endDate.String, 40)
		plan.EndDate = &v
	}
	return plan, nil
}
