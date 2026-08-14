package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

)

func TestAdminVerifyReportsAllClearForCleanData(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()

	router := NewRouter(testCfg(), WithDB(db))

	result := doJSONRequest(t, router, http.MethodGet, "/api/admin/verify", nil, http.StatusOK)
	if result["ok"] != true {
		t.Fatalf("ok = %v, want true; body=%s", result["ok"], toJSONString(t, result))
	}
	if !strings.Contains(toJSONString(t, result["issues"]), "all clear") {
		t.Fatalf("issues = %s, want all clear", toJSONString(t, result["issues"]))
	}
}

func TestAdminVerifyDetectsDataQualityIssues(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, "DELETE FROM nav_history WHERE fund_code = '019173'"); err != nil {
		t.Fatalf("delete nav fixture: %v", err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE portfolio_snapshot SET held_shares = -10 WHERE fund_code = 'AAPL'"); err != nil {
		t.Fatalf("make negative position fixture: %v", err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE transactions SET settlement_days = NULL WHERE order_id = 'TX001'"); err != nil {
		t.Fatalf("make missing settlement_days fixture: %v", err)
	}

	router := NewRouter(testCfg(), WithDB(db))

	result := doJSONRequest(t, router, http.MethodGet, "/api/admin/verify", nil, http.StatusOK)
	if result["ok"] != false {
		t.Fatalf("ok = %v, want false; body=%s", result["ok"], toJSONString(t, result))
	}
	issues := toJSONString(t, result["issues"])
	for _, want := range []string{"missing NAV", "negative positions", "missing settlement_days"} {
		if !strings.Contains(issues, want) {
			t.Fatalf("issues = %s, want substring %q", issues, want)
		}
	}
	if strings.Contains(issues, "buy") || strings.Contains(issues, "sell") {
		t.Fatalf("verify issues must stay facts-only, got %s", issues)
	}
}
