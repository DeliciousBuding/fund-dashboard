package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/config"
)

// The legacy X-Fund-Edge-Key browser-write fallback is opt-in: config.Parse
// defaults FUND_EDGE_AUTH_ENABLED to false, so a deployment that never set the
// variable authenticates browser writes with the fund_session cookie plus the
// X-Fund-Request CSRF header only. These tests pin that default at the HTTP
// boundary (not just in the parsed struct) and pin that opting back in still
// works, so the compat layer cannot silently reopen or silently rot.

func edgeAuthTestConfig(t *testing.T, extra map[string]string) config.Config {
	t.Helper()
	env := map[string]string{
		"MCP_API_KEY":   testAdminKey,
		"FUND_EDGE_KEY": testEdgeKey,
	}
	for key, value := range extra {
		env[key] = value
	}
	cfg, err := config.Parse(env)
	if err != nil {
		t.Fatalf("config.Parse: %v", err)
	}
	return cfg
}

func importTransactionsPayload(orderID string) []byte {
	payload, err := json.Marshal(map[string]any{
		"transactions": []map[string]any{
			{
				"order_id":       orderID,
				"fund_code":      "19173",
				"fund_name":      "纳斯达克100指数(QDII)C",
				"trade_time":     "2026-06-03T09:00:00Z",
				"confirm_date":   "2026-06-04",
				"trade_type":     "用户买入",
				"direction":      "buy",
				"confirm_amount": 100,
				"confirm_share":  50,
				"fee":            0,
			},
		},
	})
	if err != nil {
		panic(err)
	}
	return payload
}

// doBrowserWrite posts a transaction import the way a caller would, optionally
// carrying the edge key and/or a session cookie, and returns the status code.
func doBrowserWrite(t *testing.T, router http.Handler, body []byte, withEdgeKey bool, sessionToken string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/transactions/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if withEdgeKey {
		req.Header.Set(edgeKeyHeader, testEdgeKey)
	}
	if sessionToken != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})
		req.Header.Set(csrfHeader, csrfHeaderValue)
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res.Code
}

func TestBrowserWriteEdgeKeyCompatIsOffByDefault(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	svc := newTestAuthService(t, db)
	cfg := edgeAuthTestConfig(t, nil)
	if cfg.EdgeAuthEnabled {
		t.Fatalf("parsed EdgeAuthEnabled = true, want the opt-in default false")
	}
	router := NewRouter(cfg, WithDB(db), WithAuth(svc), WithDBDriver("sqlite"))

	if code := doBrowserWrite(t, router, importTransactionsPayload("EDGEOFF001"), true, ""); code != http.StatusUnauthorized {
		t.Fatalf("edge-key-only write with compat off = %d, want 401", code)
	}

	// The session path is untouched by the flip: cookie + CSRF header still writes.
	token := loginTestUser(t, svc)
	if code := doBrowserWrite(t, router, importTransactionsPayload("EDGEOFF002"), false, token); code != http.StatusOK {
		t.Fatalf("session write with compat off = %d, want 200", code)
	}
}

func TestBrowserWriteEdgeKeyCompatHonoursExplicitEnable(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	svc := newTestAuthService(t, db)
	cfg := edgeAuthTestConfig(t, map[string]string{"FUND_EDGE_AUTH_ENABLED": "true"})
	if !cfg.EdgeAuthEnabled {
		t.Fatalf("parsed EdgeAuthEnabled = false, want true when explicitly enabled")
	}
	router := NewRouter(cfg, WithDB(db), WithAuth(svc), WithDBDriver("sqlite"))

	if code := doBrowserWrite(t, router, importTransactionsPayload("EDGEON001"), true, ""); code != http.StatusOK {
		t.Fatalf("edge-key-only write with compat on = %d, want 200", code)
	}
	if code := doBrowserWrite(t, router, importTransactionsPayload("EDGEON002"), false, ""); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated write with compat on = %d, want 401", code)
	}
}
