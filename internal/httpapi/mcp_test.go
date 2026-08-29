package httpapi

import (
	"net/http"
	"net/http/httptest"
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

// TestMCPRouteNotificationsSilent202 pins JSON-RPC notification semantics:
// messages without an "id" member (notifications/initialized, cancelled,
// progress) are swallowed per spec — 202 Accepted, empty body, no JSON-RPC
// error envelope.
func TestMCPRouteNotificationsSilent202(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := NewRouter(testCfg(), WithDB(db))

	methods := []string{
		"notifications/initialized",
		"notifications/cancelled",
		"notifications/progress",
	}
	for _, method := range methods {
		payload := `{"jsonrpc":"2.0","method":"` + method + `"}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testAdminKey)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("%s: status = %d, want 202; body=%s", method, rec.Code, rec.Body.String())
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("%s: body = %q, want empty", method, rec.Body.String())
		}
	}
}

// TestMCPRouteUnknownMethodReturnsJSONRPCError pins that requests WITH an id
// still get a JSON-RPC -32601 error envelope (HTTP 200), not an HTTP error.
func TestMCPRouteUnknownMethodReturnsJSONRPCError(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := NewRouter(testCfg(), WithDB(db))

	response := doJSONRequest(t, router, http.MethodPost, "/mcp", map[string]any{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "not_a_method",
	}, http.StatusOK)

	if response["error"] == nil {
		t.Fatalf("unknown method response missing error: %s", toJSONString(t, response))
	}
	errObj, ok := response["error"].(map[string]any)
	if !ok || errObj["code"] != float64(-32601) {
		t.Fatalf("unknown method error = %#v, want code -32601", response["error"])
	}
	if response["id"] != float64(7) {
		t.Fatalf("response id = %#v, want 7", response["id"])
	}
}

// TestMCPRouteInitializeShape pins the initialize handshake: result must
// carry protocolVersion + capabilities + serverInfo (MCP §5.1 initialize).
func TestMCPRouteInitializeShape(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := NewRouter(testCfg(), WithDB(db))

	response := doJSONRequest(t, router, http.MethodPost, "/mcp", map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "initialize",
		"params":  map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "test", "version": "0"}},
	}, http.StatusOK)

	if response["error"] != nil {
		t.Fatalf("initialize error = %#v", response["error"])
	}
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("initialize result = %#v", response["result"])
	}
	body := toJSONString(t, result)
	for _, key := range []string{"protocolVersion", "capabilities", "serverInfo"} {
		if _, present := result[key]; !present {
			t.Fatalf("initialize result missing %s: %s", key, body)
		}
	}
	if !strings.Contains(body, `"name":"fund-dashboard-go"`) {
		t.Fatalf("initialize serverInfo missing name: %s", body)
	}
	if !strings.Contains(body, `"tools"`) {
		t.Fatalf("initialize capabilities missing tools: %s", body)
	}
}

// TestMCPRouteToolCallErrorIsJSONRPC pins that tools/call failures surface as
// JSON-RPC errors with HTTP 200, never as an HTTP status code. Unknown tool
// names are denied at authorization (-32001 tool_denied: unknown_tool, see
// agenttools) rather than reaching the tool_not_implemented default branch.
func TestMCPRouteToolCallErrorIsJSONRPC(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := NewRouter(testCfg(), WithDB(db))

	response := doJSONRequest(t, router, http.MethodPost, "/mcp", map[string]any{
		"jsonrpc": "2.0",
		"id":      "mcp-err-1",
		"method":  "tools/call",
		"params":  map[string]any{"name": "no_such_tool", "arguments": map[string]any{}},
	}, http.StatusOK)

	errObj, ok := response["error"].(map[string]any)
	if !ok {
		t.Fatalf("tools/call unknown tool response missing error: %#v", response["error"])
	}
	if errObj["code"] != float64(-32001) {
		t.Fatalf("tools/call unknown tool error code = %#v, want -32001", errObj["code"])
	}
	if message, _ := errObj["message"].(string); !strings.Contains(message, "unknown_tool") {
		t.Fatalf("tools/call unknown tool message = %#v, want unknown_tool", errObj["message"])
	}
}
