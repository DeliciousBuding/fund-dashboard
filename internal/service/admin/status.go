package admin

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

var numericSecurityCodePattern = regexp.MustCompile(`^\d+$`)

type StatusByCodeReport struct {
	Code         string            `json:"code"`
	Name         *string           `json:"name,omitempty"`
	Type         *string           `json:"type,omitempty"`
	SecurityType *string           `json:"security_type,omitempty"`
	Market       *string           `json:"market,omitempty"`
	Transactions RangeStats        `json:"transactions"`
	NAV          RangeStats        `json:"nav"`
	Position     PositionStats     `json:"position"`
	Trading      map[string]string `json:"trading"`
}

type RangeStats struct {
	N     int     `json:"n"`
	First *string `json:"first"`
	Last  *string `json:"last"`
}

type PositionStats struct {
	HeldShares    float64  `json:"held_shares"`
	TotalCost     *float64 `json:"total_cost,omitempty"`
	CurrentValue  *float64 `json:"current_value,omitempty"`
	UnrealizedPNL *float64 `json:"unrealized_pnl,omitempty"`
	PNLPct        *float64 `json:"pnl_pct,omitempty"`
}

func (s Service) GetStatusByCode(ctx context.Context, rawCode string) (StatusByCodeReport, error) {
	code := NormalizeSecurityCode(rawCode)
	identity, err := s.querySecurityIdentity(ctx, code)
	if err != nil {
		return StatusByCodeReport{}, err
	}
	transactions, err := s.queryPerCodeRange(ctx, "transactions", "trade_time", code)
	if err != nil {
		return StatusByCodeReport{}, err
	}
	nav, err := s.queryPerCodeRange(ctx, "nav_history", "date", code)
	if err != nil {
		return StatusByCodeReport{}, err
	}
	position, err := s.queryPosition(ctx, code)
	if err != nil {
		return StatusByCodeReport{}, err
	}
	trading, err := s.queryTrading(ctx, code)
	if err != nil {
		return StatusByCodeReport{}, err
	}

	return StatusByCodeReport{
		Code:         code,
		Name:         identity.Name,
		Type:         identity.Type,
		SecurityType: identity.SecurityType,
		Market:       identity.Market,
		Transactions: transactions,
		NAV:          nav,
		Position:     position,
		Trading:      trading,
	}, nil
}

type securityIdentity struct {
	Name         *string
	Type         *string
	SecurityType *string
	Market       *string
}

func (s Service) querySecurityIdentity(ctx context.Context, code string) (securityIdentity, error) {
	var name sql.NullString
	var fundType sql.NullString
	var securityType sql.NullString
	var market sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT fund_name, fund_type, security_type, market
		FROM fund_details
		WHERE fund_code = ?
	`, code).Scan(&name, &fundType, &securityType, &market)
	if err != nil {
		if err == sql.ErrNoRows {
			return securityIdentity{}, nil
		}
		return securityIdentity{}, fmt.Errorf("admin status identity: %w", err)
	}
	if name.Valid {
		name.String = clampAdminText(name.String, 200)
	}
	if fundType.Valid {
		fundType.String = clampAdminText(fundType.String, 64)
	}
	if securityType.Valid {
		securityType.String = clampAdminText(securityType.String, 32)
	}
	if market.Valid {
		market.String = clampAdminText(market.String, 32)
	}
	return securityIdentity{
		Name:         nullableStringPtr(name),
		Type:         nullableStringPtr(fundType),
		SecurityType: nullableStringPtr(securityType),
		Market:       nullableStringPtr(market),
	}, nil
}

func (s Service) queryPerCodeRange(ctx context.Context, table string, dateColumn string, code string) (RangeStats, error) {
	query := fmt.Sprintf(
		"SELECT COUNT(*), MIN(%s), MAX(%s) FROM %s WHERE fund_code = ?",
		quoteSQLiteIdentifier(dateColumn),
		quoteSQLiteIdentifier(dateColumn),
		quoteSQLiteIdentifier(table),
	)
	var stats RangeStats
	var first sql.NullString
	var last sql.NullString
	if err := s.db.QueryRowContext(ctx, query, code).Scan(&stats.N, &first, &last); err != nil {
		return RangeStats{}, fmt.Errorf("admin status %s range: %w", table, err)
	}
	stats.First = nullableStringPtr(first)
	stats.Last = nullableStringPtr(last)
	return stats, nil
}

func (s Service) queryPosition(ctx context.Context, code string) (PositionStats, error) {
	var heldShares sql.NullFloat64
	var totalCost sql.NullFloat64
	var currentValue sql.NullFloat64
	var unrealizedPNL sql.NullFloat64
	var pnlPct sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `
		SELECT held_shares, total_cost, current_value, unrealized_pnl, pnl_pct
		FROM portfolio_snapshot
		WHERE fund_code = ?
	`, code).Scan(&heldShares, &totalCost, &currentValue, &unrealizedPNL, &pnlPct)
	if err != nil {
		if err == sql.ErrNoRows {
			return PositionStats{HeldShares: 0}, nil
		}
		return PositionStats{}, fmt.Errorf("admin status position: %w", err)
	}
	position := PositionStats{HeldShares: 0}
	if heldShares.Valid {
		position.HeldShares = heldShares.Float64
	}
	position.TotalCost = nullableFloatPtr(totalCost)
	position.CurrentValue = nullableFloatPtr(currentValue)
	position.UnrealizedPNL = nullableFloatPtr(unrealizedPNL)
	position.PNLPct = nullableFloatPtr(pnlPct)
	return position, nil
}

func (s Service) queryTrading(ctx context.Context, code string) (map[string]string, error) {
	hasFundStatus, err := s.tableExists(ctx, "fund_status")
	if err != nil {
		return nil, err
	}
	if !hasFundStatus {
		return map[string]string{}, nil
	}

	var purchase sql.NullString
	var redemption sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT purchase_status, redemption_status
		FROM fund_status
		WHERE fund_code = ?
	`, code).Scan(&purchase, &redemption)
	if err != nil {
		if err == sql.ErrNoRows {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("admin status trading: %w", err)
	}
	trading := map[string]string{}
	if purchase.Valid {
		trading["purchase_status"] = purchase.String
	}
	if redemption.Valid {
		trading["redemption_status"] = redemption.String
	}
	return trading, nil
}

func (s Service) tableExists(ctx context.Context, table string) (bool, error) {
	// Cross-dialect table existence check.
	// Try information_schema (PG) first, fall through to sqlite_master.
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = ?
	`, table).Scan(&count)
	if err == nil && count > 0 {
		return true, nil
	}
	// Fall through: sqlite_master
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name = ?
	`, table).Scan(&count); err != nil {
		return false, fmt.Errorf("check table %s exists: %w", table, err)
	}
	return count > 0, nil
}

func NormalizeSecurityCode(rawCode string) string {
	code := strings.TrimSpace(rawCode)
	if numericSecurityCodePattern.MatchString(code) {
		if len(code) >= 6 {
			// Bound absurd numeric strings before DB/crawl use (#226).
			if len(code) > 32 {
				code = code[:32]
			}
			return code
		}
		return strings.Repeat("0", 6-len(code)) + code
	}
	code = strings.ToUpper(code)
	if len(code) > 32 {
		code = code[:32]
	}
	return code
}

func nullableFloatPtr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}
