package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/config"
)

func TestMCPRouteRequiresBearerAuth(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := NewRouter(testCfg(), WithDB(db))

	// No Authorization header → 401
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":"1","method":"tools/list"}`)))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("no auth status = %d, want 401; body=%s", res.Code, res.Body.String())
	}

	// Wrong key → 401
	req = httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":"1","method":"tools/list"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-key")
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("wrong key status = %d, want 401; body=%s", res.Code, res.Body.String())
	}
}

func TestMCPRouteAllowsPublicKeyForReadTools(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := NewRouter(testCfg(), WithDB(db))

	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "mcp-public-1",
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "get_portfolio_summary",
			"arguments": map[string]any{"portfolio_id": 1},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testPublicMCPKey)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("public key status = %d, want 200; body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "total_transactions") {
		t.Fatalf("public key response missing summary facts: %s", res.Body.String())
	}
}

func TestMCPRouteDeniesWriteForPublicKey(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := NewRouter(testCfg(), WithDB(db))

	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "mcp-public-write",
		"method":  "tools/call",
		"params": map[string]any{
			"name": "add_transaction",
			"arguments": map[string]any{
				"fund_code":      "019173",
				"confirm_amount": 1,
				"confirmed":      true,
			},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testPublicMCPKey)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		// JSON-RPC errors still return HTTP 200 with error payload.
		t.Fatalf("status = %d, want 200 envelope; body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "tool_denied") && !strings.Contains(res.Body.String(), "scope_not_allowed") {
		t.Fatalf("public write should be denied: %s", res.Body.String())
	}
}

func TestMCPRouteFailClosedWhenNoKeysConfigured(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := NewRouter(config.Config{ServiceName: "fund-dashboard-go", Version: "test"}, WithDB(db))

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":"1","method":"tools/list"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer anything")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("no keys status = %d, want 401; body=%s", res.Code, res.Body.String())
	}
}

func TestSPATransactionImportUsesEdgeKeyNotAdminKey(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := NewRouter(testCfg(), WithDB(db))

	// Missing edge key → 401 (edge auth required).
	payload := map[string]any{
		"transactions": []map[string]any{
			{
				"order_id":       "SPA001",
				"fund_code":      "19173",
				"fund_name":      "纳斯达克100",
				"trade_time":     "2026-06-03T09:00:00Z",
				"confirm_date":   "2026-06-04",
				"trade_type":     "用户买入",
				"direction":      "buy",
				"confirm_amount": 100,
				"confirm_share":  50,
				"fee":            0,
			},
		},
	}
	raw, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/transactions/import", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("no edge key status = %d, want 401; body=%s", res.Code, res.Body.String())
	}

	// With edge key (injected by nginx, not MCP_API_KEY) → success.
	result := doJSONRequest(t, router, http.MethodPost, "/api/transactions/import", payload, http.StatusOK)
	if result["ok"] != true || result["imported"].(float64) != 1 {
		t.Fatalf("spa import = %s", toJSONString(t, result))
	}
}

func TestAnalysisCompareRouteReturnsFundsEnvelope(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := newAuthedRouter(t, testCfg(), db)

	result := doJSONRequest(t, router, http.MethodGet, "/api/analysis/compare?codes=019173,aapl", nil, http.StatusOK)
	funds, ok := result["funds"].([]any)
	if !ok || len(funds) == 0 {
		t.Fatalf("compare response = %s, want funds array", toJSONString(t, result))
	}
}

func TestMCPFakeConfirmationWithoutAgentOpsNoPanic(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	// registerMCPRoutes with nil agentOps pointer (typed nil risk).
	router := NewRouter(testCfg(), WithDB(db))
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "check_alerts",
			"arguments": map[string]any{
				"confirmation_id":    1,
				"confirmation_token": "fake",
			},
		},
	}
	// operator key
	res := doJSONRequest(t, router, http.MethodPost, "/mcp", body, http.StatusOK)
	errObj, _ := res["error"].(map[string]any)
	if errObj == nil {
		t.Fatalf("expected error, got %#v", res)
	}
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "confirmation_service_unavailable") && !strings.Contains(msg, "confirmation") {
		t.Fatalf("message=%q want confirmation unavailable", msg)
	}
}
