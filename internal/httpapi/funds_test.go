package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/config"
)

func TestFundDetailRoutesExposeFrontendCompatibleFacts(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()

	router := NewRouter(config.Config{ServiceName: "fund-dashboard-go", Version: "test"}, WithDB(db))

	detail := doJSONRequest(t, router, http.MethodGet, "/api/funds/019173", nil, http.StatusOK)
	if detail["code"] != "019173" ||
		detail["name"] != "纳斯达克100指数(QDII)C" ||
		detail["held_shares"].(float64) != 100 ||
		detail["total_cost"].(float64) != -120 ||
		detail["latest_nav"].(float64) != 1.5 ||
		detail["current_value"].(float64) != 150 ||
		detail["transaction_count"] != nil {
		t.Fatalf("detail response = %s", toJSONString(t, detail))
	}
	transactions := detail["transactions"].([]any)
	if len(transactions) != 1 {
		t.Fatalf("transactions length = %d, want 1: %s", len(transactions), toJSONString(t, detail))
	}
	tx := transactions[0].(map[string]any)
	if tx["trade_time"] != "2026-06-01T09:00:00Z" ||
		tx["trade_type"] != "用户买入" ||
		tx["amount"].(float64) != 120 ||
		tx["shares"].(float64) != 100 ||
		tx["settlement_days"].(float64) != 2 {
		t.Fatalf("transaction response = %#v", tx)
	}

	nav := doJSONArrayRequest(t, router, http.MethodGet, "/api/funds/019173/nav", http.StatusOK)
	if len(nav) != 1 ||
		nav[0]["date"] != "2026-06-18" ||
		nav[0]["unit_nav"].(float64) != 1.5 ||
		nav[0]["daily_change_pct"].(float64) != -4.2 {
		t.Fatalf("nav response = %s", toJSONString(t, nav))
	}

	xirr := doJSONRequest(t, router, http.MethodGet, "/api/funds/019173/xirr", nil, http.StatusOK)
	if xirr["code"] != "019173" {
		t.Fatalf("xirr response = %s", toJSONString(t, xirr))
	}
	if _, ok := xirr["xirr"]; !ok {
		t.Fatalf("xirr response missing frontend xirr field: %s", toJSONString(t, xirr))
	}

	drawdown := doJSONRequest(t, router, http.MethodGet, "/api/funds/019173/drawdown", nil, http.StatusOK)
	if drawdown["code"] != "019173" ||
		drawdown["max_drawdown"].(float64) != 0 ||
		drawdown["peak_date"] != "2026-06-18" ||
		drawdown["trough_date"] != "2026-06-18" {
		t.Fatalf("drawdown response = %s", toJSONString(t, drawdown))
	}

	dca := doJSONRequest(t, router, http.MethodGet, "/api/funds/019173/dca?base=30&mode=nav_deviation", nil, http.StatusOK)
	if dca["fund_code"] != "019173" ||
		dca["mode"] != "nav_deviation" ||
		dca["base_amount"].(float64) != 30 ||
		dca["latest_nav"].(float64) != 1.5 ||
		dca["actual_amount"].(float64) != 15 ||
		dca["range"] == nil {
		t.Fatalf("dca response = %s", toJSONString(t, dca))
	}

	allPayloads := toJSONString(t, map[string]any{
		"detail":   detail,
		"nav":      nav,
		"xirr":     xirr,
		"drawdown": drawdown,
		"dca":      dca,
	})
	if strings.Contains(allPayloads, "backup") ||
		strings.Contains(allPayloads, "建议买入") ||
		strings.Contains(allPayloads, "external_fetch_performed") {
		t.Fatalf("fund detail REST routes should stay read-only facts: %s", allPayloads)
	}
}

func TestFundListRoutesExposeFrontendCompatibleSecurityRows(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()

	router := NewRouter(config.Config{ServiceName: "fund-dashboard-go", Version: "test"}, WithDB(db))

	funds := doJSONArrayRequest(t, router, http.MethodGet, "/api/funds", http.StatusOK)
	if len(funds) != 2 {
		t.Fatalf("funds length = %d, want 2: %s", len(funds), toJSONString(t, funds))
	}
	if funds[0]["code"] != "019173" ||
		funds[0]["security_type"] != "fund" ||
		funds[0]["held_shares"].(float64) != 100 {
		t.Fatalf("first fund row = %#v, want 019173 fund row ordered by code", funds[0])
	}
	if funds[1]["code"] != "AAPL" ||
		funds[1]["security_type"] != "stock" ||
		funds[1]["market"] != "US" ||
		funds[1]["latest_nav"].(float64) != 190 {
		t.Fatalf("second fund row = %#v, want AAPL stock row", funds[1])
	}

	securities := doJSONArrayRequest(t, router, http.MethodGet, "/api/securities", http.StatusOK)
	if toJSONString(t, securities) != toJSONString(t, funds) {
		t.Fatalf("securities alias = %s, want same payload as /api/funds %s", toJSONString(t, securities), toJSONString(t, funds))
	}
}
