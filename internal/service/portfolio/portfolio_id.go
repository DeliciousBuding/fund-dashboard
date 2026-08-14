package portfolio

import "strings"

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
func clampPortfolioText(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max])
	}
	return s
}
