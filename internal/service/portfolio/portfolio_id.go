package portfolio

import "github.com/DeliciousBuding/fund-dashboard/internal/textutil"

// clampPortfolioID normalizes portfolio_id for service entrypoints (#230).
// Non-positive -> 1; values above 1000 are capped (matches HTTP/MCP intArgMax).
func clampPortfolioID(id int) int {
	if id <= 0 {
		return 1
	}
	if id > 1000 {
		return 1000
	}
	return id
}

// clampPortfolioText bounds free-text portfolio JSON fields (#246).
// Single implementation lives in internal/textutil.
func clampPortfolioText(s string, max int) string { return textutil.Clamp(s, max) }
