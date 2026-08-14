package portfolio

import (
	"context"
	"database/sql"
	"fmt"
)

type SecurityListItem struct {
	Code          string   `json:"code"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	SecurityType  string   `json:"security_type"`
	Market        string   `json:"market"`
	HeldShares    float64  `json:"held_shares"`
	CurrentValue  *float64 `json:"current_value"`
	UnrealizedPNL *float64 `json:"unrealized_pnl"`
	PNLPct        *float64 `json:"pnl_pct"`
	LatestNAV     *float64 `json:"latest_nav"`
}

func (s Service) ListSecurities(ctx context.Context, portfolioID int) ([]SecurityListItem, error) {
	portfolioID = clampPortfolioID(portfolioID)
	// Soft cap for SPA/MCP list responses on large masters (#231).
	const maxListSecurities = 5000
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			fd.fund_code,
			COALESCE(fd.fund_name, ''),
			COALESCE(fd.fund_type, ''),
			COALESCE(fd.security_type, 'fund'),
			COALESCE(fd.market, ''),
			ps.held_shares,
			ps.current_value,
			ps.unrealized_pnl,
			ps.pnl_pct,
			ps.latest_nav
		FROM fund_details fd
		LEFT JOIN portfolio_snapshot ps
			ON fd.fund_code = ps.fund_code AND COALESCE(ps.portfolio_id, 1) = ?
		ORDER BY fd.fund_code
		LIMIT ?
	`, portfolioID, maxListSecurities)
	if err != nil {
		return nil, fmt.Errorf("list securities: %w", err)
	}
	defer rows.Close()

	items := []SecurityListItem{}
	for rows.Next() {
		var item SecurityListItem
		var heldShares sql.NullFloat64
		var currentValue sql.NullFloat64
		var unrealizedPNL sql.NullFloat64
		var pnlPct sql.NullFloat64
		var latestNAV sql.NullFloat64
		if err := rows.Scan(
			&item.Code,
			&item.Name,
			&item.Type,
			&item.SecurityType,
			&item.Market,
			&heldShares,
			&currentValue,
			&unrealizedPNL,
			&pnlPct,
			&latestNAV,
		); err != nil {
			return nil, fmt.Errorf("scan security list item: %w", err)
		}
		if heldShares.Valid {
			item.HeldShares = heldShares.Float64
		}
		item.Code = clampPortfolioText(item.Code, 32)
		item.Name = clampPortfolioText(item.Name, 200)
		item.Type = clampPortfolioText(item.Type, 64)
		item.SecurityType = clampPortfolioText(item.SecurityType, 32)
		item.Market = clampPortfolioText(item.Market, 32)
		item.CurrentValue = nullableFloat64Ptr(currentValue)
		item.UnrealizedPNL = nullableFloat64Ptr(unrealizedPNL)
		item.PNLPct = nullableFloat64Ptr(pnlPct)
		item.LatestNAV = nullableFloat64Ptr(latestNAV)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("security list rows: %w", err)
	}
	return items, nil
}
