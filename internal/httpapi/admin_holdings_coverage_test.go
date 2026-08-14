package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestAdminHoldingsCoverageReportsApplicableMissingAndNotApplicableFunds(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()

	for _, stmt := range []string{
		"DELETE FROM fund_holdings",
		"DELETE FROM portfolio_snapshot",
		"DELETE FROM fund_details",
		`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type) VALUES
			('EQ001', 'Equity Fund 1', '指数型-股票', 'fund'),
			('EQ002', 'Equity Fund 2', '指数型-股票', 'fund'),
			('BD001', 'Bond Fund', '债券型-混合一级', 'fund'),
			('AAPL', 'Apple Inc.', '科技股', 'stock')`,
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES
			('EQ001', 'Equity Fund 1', 10, -10, 1, 10, 0, 0, 'fund', 1),
			('EQ002', 'Equity Fund 2', 10, -10, 1, 10, 0, 0, 'fund', 1),
			('BD001', 'Bond Fund', 10, -10, 1, 10, 0, 0, 'fund', 1),
			('AAPL', 'Apple Inc.', 2, -300, 190, 380, 80, 26.67, 'stock', 1),
			('P2EQ', 'Portfolio 2 Fund', 10, -10, 1, 10, 0, 0, 'fund', 2)`,
		`INSERT INTO fund_holdings (fund_code, stock_code, stock_name, weight_pct, shares, market_value, report_date)
			VALUES
			('EQ001', '000001', 'Ping An', 10, 1, 100, '2026-03-31'),
			('P2EQ', '000002', 'Other', 10, 1, 100, '2026-03-31')`,
	} {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("exec fixture statement %q: %v", stmt, err)
		}
	}

	router := NewRouter(testCfg(), WithDB(db))

	result := doJSONRequest(t, router, http.MethodGet, "/api/admin/holdings-coverage", nil, http.StatusOK)
	if result["decision_boundary"] != "read_only" {
		t.Fatalf("decision_boundary = %v, want read_only", result["decision_boundary"])
	}
	if result["total_funds"].(float64) != 4 {
		t.Fatalf("total_funds = %v, want 4", result["total_funds"])
	}
	if result["applicable_funds"].(float64) != 2 || result["funds_with_holdings"].(float64) != 1 {
		t.Fatalf("coverage counts = %s, want applicable=2 with_holdings=1", toJSONString(t, result))
	}
	if result["coverage_pct"].(float64) != 50 || result["applicable_coverage_pct"].(float64) != 50 {
		t.Fatalf("coverage pct = %s, want 50", toJSONString(t, result))
	}
	if result["total_coverage_pct"].(float64) != 25 {
		t.Fatalf("total_coverage_pct = %v, want 25", result["total_coverage_pct"])
	}

	body := toJSONString(t, result)
	if !strings.Contains(body, `"code":"EQ002"`) || !strings.Contains(body, `"source_missing_funds"`) {
		t.Fatalf("missing source-missing EQ002 in %s", body)
	}
	if !strings.Contains(body, `"code":"BD001"`) || !strings.Contains(body, `"code":"AAPL"`) {
		t.Fatalf("missing not-applicable funds in %s", body)
	}
	if strings.Contains(body, "P2EQ") {
		t.Fatalf("portfolio_id filter leaked portfolio 2 fund: %s", body)
	}
	if !strings.Contains(body, `"by_fund_type"`) || !strings.Contains(body, `"coverage_pct":50`) {
		t.Fatalf("missing grouped coverage evidence in %s", body)
	}
}
