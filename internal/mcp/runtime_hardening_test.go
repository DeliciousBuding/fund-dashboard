package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

type panickingNavCrawler struct{}

func (panickingNavCrawler) CrawlAllHeld(context.Context) (int, int, error) {
	panic("crawl_all_held panic")
}

func (panickingNavCrawler) CrawlCode(context.Context, string) (int, string, error) {
	panic("crawl_code panic")
}

func TestHandleRecoversPanicWithoutLeakingInternals(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	portfolio := portfoliosvc.NewService(db)
	admin := adminsvc.NewServiceWithDriver(db, "sqlite")
	server, err := NewServer(ServerDeps{
		Portfolio: &portfolio,
		Admin:     &admin,
		Nav:       panickingNavCrawler{},
		Role:      agenttools.RoleOperator,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"panic"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"crawl_nav","arguments":{"fund_code":"019173"}}`),
	})
	if resp.Error == nil {
		t.Fatalf("response = %#v, want internal_error after panic", resp)
	}
	if resp.Error.Code != -32603 || resp.Error.Message != "internal_error" {
		t.Fatalf("response = %#v, want -32603 internal_error", resp)
	}
	if strings.Contains(resp.Error.Message, "panic") {
		t.Fatalf("response leaks panic details: %#v", resp.Error)
	}
}

func TestImplementedMCPToolsCoversRegistryTools(t *testing.T) {
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry: %v", err)
	}
	implemented := implementedMCPTools()
	for _, tool := range registry.Tools {
		if tool.Capability.Permission == agenttools.PermissionDisabled {
			continue
		}
		if _, ok := implemented[tool.Name]; !ok {
			t.Errorf("registry tool %q has no implemented tools/call case", tool.Name)
		}
	}
}

func TestSanitizedToolErrorMessage(t *testing.T) {
	windowsDrivePath := strings.Join([]string{"C:", "data", "fund.db"}, "\\")
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"postgres relation missing", `ERROR: relation "transactions" does not exist (SQLSTATE 42P01)`, "internal_error"},
		{"sqlite locked", "sqlite: database is locked (5) (SQLITE_BUSY)", "internal_error"},
		{"dial failure", "dial tcp example.com:5432: connect: connection refused", "internal_error"},
		{"relative file open", "open fund.db: no such file or directory", "internal_error"},
		{"windows drive path", "open " + windowsDrivePath + ": The system cannot find the path specified.", "internal_error"},
		{"url detail", "fetch failed: https://example.com/x", "internal_error"},
		{"tls failure", "tls: failed to verify certificate", "internal_error"},
		{"empty message", "", "internal_error"},
		{"overlong message", strings.Repeat("x", 121), "internal_error"},
		{"short validation passthrough", "fund not found", "fund not found"},
		{"amount validation passthrough", "amount must be positive", "amount must be positive"},
		{"code validation passthrough", "invalid market code", "invalid market code"},
	}
	for _, tc := range cases {
		if got := sanitizedToolErrorMessage(tc.in); got != tc.want {
			t.Errorf("%s: sanitizedToolErrorMessage(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestIntArgRejectsOutOfRangeFloat(t *testing.T) {
	if got := intArg(map[string]any{"confirmation_id": 1e300}, "confirmation_id", 0); got != 0 {
		t.Fatalf("intArg(1e300) = %d, want fallback 0", got)
	}
	if got := intArg(map[string]any{"id": -1e300}, "id", 7); got != 7 {
		t.Fatalf("intArg(-1e300) = %d, want fallback 7", got)
	}
}

func TestIntegerFlagParsing(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  int
		valid bool
	}{
		{"int one", 1, 1, true},
		{"int zero", 0, 0, true},
		{"int negative", -1, -1, true},
		{"integral float", 1.0, 1, true},
		{"fractional float", 1.5, 0, false},
		{"huge float", 1e300, 0, false},
		{"json number", json.Number("0"), 0, true},
		{"bool", true, 0, false},
		{"string", "1", 0, false},
	}
	for _, tc := range cases {
		got, valid := integerFlag(tc.value)
		if got != tc.want || valid != tc.valid {
			t.Errorf("%s: integerFlag(%v) = %d, %v; want %d, %v", tc.name, tc.value, got, valid, tc.want, tc.valid)
		}
	}
}

// cancelAfterFirstNav cancels the context on its first CrawlCode call, then
// reports success so the batch loop observes the cancellation on iteration 2.
type cancelAfterFirstNav struct {
	calls     []string
	cancel    context.CancelFunc
	cancelled bool
}

func (n *cancelAfterFirstNav) CrawlAllHeld(context.Context) (int, int, error) { return 0, 0, nil }

func (n *cancelAfterFirstNav) CrawlCode(_ context.Context, code string) (int, string, error) {
	n.calls = append(n.calls, code)
	if !n.cancelled {
		n.cancelled = true
		n.cancel()
	}
	return 1, "2026-07-18", nil
}

func TestCrawlStaleCodesStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	nav := &cancelAfterFirstNav{cancel: cancel}
	done, failed, added, cancelled := crawlStaleCodes(ctx, nav, []string{"a", "b", "c"})
	if !cancelled {
		t.Fatalf("cancelled = false, want true")
	}
	if len(done) != 1 || done[0] != "a" {
		t.Fatalf("done = %v, want [a]", done)
	}
	if len(failed) != 0 || added != 1 || len(nav.calls) != 1 {
		t.Fatalf("failed=%v added=%d calls=%v, want no failures, 1 added, 1 crawl", failed, added, nav.calls)
	}
}

func TestCrawlStaleCodesCancelledBeforeWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	nav := &cancelAfterFirstNav{cancel: func() {}}
	done, failed, added, cancelled := crawlStaleCodes(ctx, nav, []string{"a", "b", "c"})
	if !cancelled {
		t.Fatalf("cancelled = false, want true")
	}
	if len(done) != 0 || len(failed) != 0 || added != 0 || len(nav.calls) != 0 {
		t.Fatalf("done=%v failed=%v added=%d calls=%v, want no work before cancellation", done, failed, added, nav.calls)
	}
}

func TestToolsCallMissingToolNameIsInvalidParams(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)
	for _, params := range []string{`{}`, `null`, `{"arguments":{}}`, `{"name":""}`} {
		resp := server.Handle(context.Background(), Request{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"missing-name"`),
			Method:  "tools/call",
			Params:  json.RawMessage(params),
		})
		if resp.Error == nil {
			t.Fatalf("params %s: error = nil, want invalid_params", params)
		}
		if resp.Error.Code != -32602 || !strings.Contains(resp.Error.Message, "invalid_params") {
			t.Fatalf("params %s: response = %#v, want -32602 invalid_params", params, resp)
		}
	}
}
