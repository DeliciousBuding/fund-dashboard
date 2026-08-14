package portfolio

import (
	"context"
	"fmt"
)

type PortfolioDefinition struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s Service) ListPortfolioDefinitions(ctx context.Context) ([]PortfolioDefinition, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, COALESCE(description, '')
		FROM portfolio_definitions
		ORDER BY id
		LIMIT 1000
	`)
	if err != nil {
		return nil, fmt.Errorf("query portfolio definitions: %w", err)
	}
	defer rows.Close()

	var portfolios []PortfolioDefinition
	for rows.Next() {
		var portfolio PortfolioDefinition
		if err := rows.Scan(&portfolio.ID, &portfolio.Name, &portfolio.Description); err != nil {
			return nil, fmt.Errorf("scan portfolio definition: %w", err)
		}
		portfolio.Name = clampPortfolioText(portfolio.Name, 200)
		portfolio.Description = clampPortfolioText(portfolio.Description, 500)
		portfolios = append(portfolios, portfolio)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("portfolio definition rows: %w", err)
	}
	return portfolios, nil
}
