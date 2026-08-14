package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestMCPRouteHandlesReadOnlyToolCall(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := NewRouter(testCfg(), WithDB(db))

	response := doJSONRequest(t, router, http.MethodPost, "/mcp", map[string]any{
		"jsonrpc": "2.0",
		"id":      "mcp-http-1",
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "get_portfolio_summary",
			"arguments": map[string]any{"portfolio_id": 1},
		},
	}, http.StatusOK)

	if response["error"] != nil {
		t.Fatalf("/mcp response error = %#v", response["error"])
	}
	if response["jsonrpc"] != "2.0" || response["id"] != "mcp-http-1" {
		t.Fatalf("/mcp response envelope = %#v", response)
	}
	if !strings.Contains(toJSONString(t, response), "total_transactions") {
		t.Fatalf("/mcp response missing summary facts: %s", toJSONString(t, response))
	}
}
