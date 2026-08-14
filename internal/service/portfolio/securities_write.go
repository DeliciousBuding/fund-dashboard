package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type UpsertSecurityInput struct {
	Code         string
	Name         string
	FundType     string
	SecurityType string // fund|stock
	Market       string // CN/SH/SZ/HK/US
	Currency     string
	Exchange     string
	Source       string
}

type UpsertSecurityResult struct {
	OK       bool             `json:"ok"`
	Created  bool             `json:"created"`
	Security SecurityListItem `json:"security"`
}

type DeleteSecurityResult struct {
	OK      bool   `json:"ok"`
	Code    string `json:"code"`
	Deleted bool   `json:"deleted"`
}

func (s Service) UpsertSecurity(ctx context.Context, in UpsertSecurityInput) (UpsertSecurityResult, error) {
	code := strings.TrimSpace(in.Code)
	if code == "" {
		return UpsertSecurityResult{}, fmt.Errorf("fund_code is required")
	}
	if len(code) > 32 {
		return UpsertSecurityResult{}, fmt.Errorf("fund_code max 32 chars")
	}
	name := strings.TrimSpace(in.Name)
	if len(name) > 200 {
		return UpsertSecurityResult{}, fmt.Errorf("fund_name max 200 chars")
	}
	secType := strings.TrimSpace(in.SecurityType)
	if secType == "" {
		secType = "fund"
	}
	if secType != "fund" && secType != "stock" {
		return UpsertSecurityResult{}, fmt.Errorf("security_type must be fund or stock")
	}
	market := strings.TrimSpace(in.Market)
	if market == "" {
		if secType == "fund" {
			market = "CN"
		} else {
			return UpsertSecurityResult{}, fmt.Errorf("market is required for stock")
		}
	}
	currency := strings.TrimSpace(in.Currency)
	if currency == "" {
		currency = "CNY"
	}
	exchange := strings.TrimSpace(in.Exchange)
	source := strings.TrimSpace(in.Source)
	if source == "" {
		source = "mcp"
	}
	fundType := strings.TrimSpace(in.FundType)
	if len(market) > 16 {
		return UpsertSecurityResult{}, fmt.Errorf("market max 16 chars")
	}
	if len(currency) > 16 {
		return UpsertSecurityResult{}, fmt.Errorf("currency max 16 chars")
	}
	if len(exchange) > 32 {
		return UpsertSecurityResult{}, fmt.Errorf("exchange max 32 chars")
	}
	if len(source) > 64 {
		return UpsertSecurityResult{}, fmt.Errorf("source max 64 chars")
	}
	if len(fundType) > 64 {
		return UpsertSecurityResult{}, fmt.Errorf("fund_type max 64 chars")
	}

	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM fund_details WHERE fund_code = ?`, code).Scan(&exists); err != nil {
		return UpsertSecurityResult{}, fmt.Errorf("lookup security: %w", err)
	}
	created := exists == 0
	if created && name == "" {
		return UpsertSecurityResult{}, fmt.Errorf("fund_name is required when creating")
	}

	if created {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market, currency, exchange, source)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, code, name, nullIfEmpty(fundType), secType, market, currency, nullIfEmpty(exchange), source); err != nil {
			return UpsertSecurityResult{}, fmt.Errorf("insert security: %w", err)
		}
	} else {
		// partial update: only set provided non-empty fields
		cur, err := s.getSecurityItem(ctx, code)
		if err != nil {
			return UpsertSecurityResult{}, err
		}
		if name == "" {
			name = cur.Name
		}
		if fundType == "" {
			fundType = cur.Type
		}
		if in.SecurityType == "" {
			secType = cur.SecurityType
		}
		if in.Market == "" {
			market = cur.Market
		}
		if _, err := s.db.ExecContext(ctx, `
			UPDATE fund_details SET
				fund_name = ?, fund_type = ?, security_type = ?, market = ?,
				currency = COALESCE(NULLIF(?, ''), currency),
				exchange = COALESCE(NULLIF(?, ''), exchange),
				source = COALESCE(NULLIF(?, ''), source)
			WHERE fund_code = ?
		`, name, nullIfEmpty(fundType), secType, market, currency, exchange, source, code); err != nil {
			return UpsertSecurityResult{}, fmt.Errorf("update security: %w", err)
		}
	}

	item, err := s.getSecurityItem(ctx, code)
	if err != nil {
		return UpsertSecurityResult{}, err
	}
	return UpsertSecurityResult{OK: true, Created: created, Security: item}, nil
}

func (s Service) DeleteSecurity(ctx context.Context, code string) (DeleteSecurityResult, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return DeleteSecurityResult{}, fmt.Errorf("fund_code is required")
	}
	if len(code) > 32 {
		return DeleteSecurityResult{}, fmt.Errorf("fund_code too long")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return DeleteSecurityResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// cascade related operational data (ignore missing optional/legacy tables)
	stmts := []string{
		`DELETE FROM dca_plan_executions WHERE fund_code = ?`,
		`DELETE FROM transactions WHERE fund_code = ?`,
		`DELETE FROM nav_history WHERE fund_code = ?`,
		`DELETE FROM portfolio_snapshot WHERE fund_code = ?`,
		`DELETE FROM fund_holdings WHERE fund_code = ?`,
		`DELETE FROM dca_plans WHERE fund_code = ?`,
		`DELETE FROM fund_status WHERE fund_code = ?`,
		`DELETE FROM summary_by_fund WHERE fund_code = ?`,
		`DELETE FROM crawl_log WHERE fund_code = ?`,
		`DELETE FROM ant_current_positions WHERE fund_code = ?`,
		`DELETE FROM ant_summary_by_fund WHERE fund_code = ?`,
		`DELETE FROM ant_transactions_normalized WHERE fund_code = ?`,
		`DELETE FROM ant_transactions_raw WHERE fund_code_cell = ?`,
		`DELETE FROM source_events WHERE related_security_code = ?`,
		// US stock quote/history/profile cache (optional tables; code key)
		`DELETE FROM stock_realtime WHERE code = ?`,
		`DELETE FROM stock_kline_cache WHERE code = ?`,
		`DELETE FROM stock_profile WHERE code = ?`,
		`DELETE FROM fund_details WHERE fund_code = ?`,
	}
	var deleted bool
	for _, q := range stmts {
		res, err := tx.ExecContext(ctx, q, code)
		if err != nil {
			// ignore missing optional tables
			if strings.Contains(err.Error(), "no such table") || strings.Contains(strings.ToLower(err.Error()), "does not exist") {
				continue
			}
			return DeleteSecurityResult{}, fmt.Errorf("cascade delete (%s): %w", q, err)
		}
		if strings.Contains(q, "fund_details") {
			n, raErr := res.RowsAffected()
			if raErr != nil {
				return DeleteSecurityResult{}, fmt.Errorf("fund_details rows affected: %w", raErr)
			}
			deleted = n > 0
		}
	}
	if err := tx.Commit(); err != nil {
		return DeleteSecurityResult{}, err
	}
	return DeleteSecurityResult{OK: true, Code: code, Deleted: deleted}, nil
}

func (s Service) getSecurityItem(ctx context.Context, code string) (SecurityListItem, error) {
	// Direct lookup only — never scan full ListSecurities (#231).
	var item SecurityListItem
	var heldShares sql.NullFloat64
	var currentValue, unrealized, pnl, latest sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `
		SELECT fd.fund_code, COALESCE(fd.fund_name,''), COALESCE(fd.fund_type,''), COALESCE(fd.security_type,'fund'), COALESCE(fd.market,''),
			ps.held_shares, ps.current_value, ps.unrealized_pnl, ps.pnl_pct, ps.latest_nav
		FROM fund_details fd
		LEFT JOIN portfolio_snapshot ps ON fd.fund_code = ps.fund_code
		WHERE fd.fund_code = ?
	`, code).Scan(&item.Code, &item.Name, &item.Type, &item.SecurityType, &item.Market, &heldShares, &currentValue, &unrealized, &pnl, &latest)
	if err != nil {
		return SecurityListItem{}, fmt.Errorf("get security: %w", err)
	}
	if heldShares.Valid {
		item.HeldShares = heldShares.Float64
	}
	item.CurrentValue = nullableFloat64Ptr(currentValue)
	item.UnrealizedPNL = nullableFloat64Ptr(unrealized)
	item.PNLPct = nullableFloat64Ptr(pnl)
	item.LatestNAV = nullableFloat64Ptr(latest)
	return item, nil
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
