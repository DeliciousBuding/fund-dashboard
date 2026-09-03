package portfolio

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// captureHandler records warn records so the XIRR degradation signal can be
// asserted without mutating process-global slog state beyond the test.
type xirrCaptureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *xirrCaptureHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelWarn
}

func (h *xirrCaptureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *xirrCaptureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *xirrCaptureHandler) WithGroup(string) slog.Handler      { return h }

func (h *xirrCaptureHandler) warnings() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]slog.Record(nil), h.records...)
}

// findWarn returns true when a captured warning matches message and attrs.
func findWarn(h *xirrCaptureHandler, messageSubstring string, want map[string]string) bool {
	for _, r := range h.warnings() {
		if !strings.Contains(r.Message, messageSubstring) {
			continue
		}
		got := map[string]string{}
		r.Attrs(func(a slog.Attr) bool {
			got[a.Key] = a.Value.String()
			return true
		})
		match := true
		for k, v := range want {
			if got[k] != v {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func openXIRRDB(t *testing.T, stmts []string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	base := []string{
		`CREATE TABLE fund_details (
			fund_code TEXT PRIMARY KEY,
			fund_name TEXT,
			fund_type TEXT,
			security_type TEXT,
			market TEXT
		)`,
		`CREATE TABLE transactions (
			seq INTEGER PRIMARY KEY AUTOINCREMENT,
			fund_code TEXT,
			direction TEXT,
			trade_time TEXT,
			confirm_amount REAL,
			fee REAL,
			signed_share_change REAL
		)`,
		`CREATE TABLE portfolio_snapshot (
			fund_code TEXT NOT NULL,
			held_shares REAL,
			latest_nav REAL,
			portfolio_id INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (fund_code, portfolio_id)
		)`,
		`CREATE TABLE nav_history (
			fund_code TEXT,
			date TEXT,
			unit_nav REAL
		)`,
	}
	for _, q := range append(base, stmts...) {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	return db
}

// TestGetFundXIRRWarnsWhenNAVMissingAndKeepsComputing pins the T3 contract:
// a live position with no NAV anywhere keeps the historical computation
// (terminal value 0, option (a)) but the silent part is over — the fallback
// must emit a warn carrying the fund code and the reason.
func TestGetFundXIRRWarnsWhenNAVMissingAndKeepsComputing(t *testing.T) {
	db := openXIRRDB(t, []string{
		`INSERT INTO transactions (fund_code, direction, trade_time, confirm_amount, fee, signed_share_change)
			VALUES ('F1', 'buy', '2025-01-01', 1000, 0, 100)`,
		`INSERT INTO transactions (fund_code, direction, trade_time, confirm_amount, fee, signed_share_change)
			VALUES ('F1', 'sell', '2025-06-01', 300, 0, -30)`,
		`INSERT INTO portfolio_snapshot (fund_code, held_shares, latest_nav, portfolio_id)
			VALUES ('F1', 70, NULL, 1)`,
	})
	svc := NewService(db)
	capture := &xirrCaptureHandler{}
	svc.logger = slog.New(capture)

	report, err := svc.GetFundXIRR(context.Background(), "F1", 1)
	if err != nil {
		t.Fatal(err)
	}
	// Option (a): computation unchanged — buy -1000 and sell +300 still solve
	// to an XIRR even though the terminal value degraded to 0.
	if report.XIRRPct == nil {
		t.Fatalf("xirr_pct = nil, want computed value (option a); message=%v", report.Message)
	}
	if report.Message != nil {
		t.Fatalf("message = %q, want empty on computed XIRR", *report.Message)
	}
	if !findWarn(capture, "terminal value degraded", map[string]string{"fund_code": "F1", "reason": "nav_history_empty"}) {
		t.Fatalf("expected nav-missing warn for F1, got %d warnings", len(capture.warnings()))
	}
}

// TestGetFundXIRRNoWarnWhenNAVAvailable: a healthy valuation must stay silent.
func TestGetFundXIRRNoWarnWhenNAVAvailable(t *testing.T) {
	db := openXIRRDB(t, []string{
		`INSERT INTO transactions (fund_code, direction, trade_time, confirm_amount, fee, signed_share_change)
			VALUES ('F1', 'buy', '2025-01-01', 1000, 0, 100)`,
		`INSERT INTO transactions (fund_code, direction, trade_time, confirm_amount, fee, signed_share_change)
			VALUES ('F1', 'sell', '2025-06-01', 300, 0, -30)`,
		`INSERT INTO portfolio_snapshot (fund_code, held_shares, latest_nav, portfolio_id)
			VALUES ('F1', 70, 2.0, 1)`,
	})
	svc := NewService(db)
	capture := &xirrCaptureHandler{}
	svc.logger = slog.New(capture)

	report, err := svc.GetFundXIRR(context.Background(), "F1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if report.XIRRPct == nil {
		t.Fatalf("xirr_pct = nil, want computed value; message=%v", report.Message)
	}
	if len(capture.warnings()) != 0 {
		t.Fatalf("healthy fund must not warn, got %d warnings", len(capture.warnings()))
	}
}

// TestGetFundXIRRWarnsOnZeroSnapshotNAV: a stored 0 NAV marks a live position
// to zero on the snapshot fast path — same degradation, same signal duty.
func TestGetFundXIRRWarnsOnZeroSnapshotNAV(t *testing.T) {
	db := openXIRRDB(t, []string{
		`INSERT INTO transactions (fund_code, direction, trade_time, confirm_amount, fee, signed_share_change)
			VALUES ('F1', 'buy', '2025-01-01', 1000, 0, 100)`,
		`INSERT INTO transactions (fund_code, direction, trade_time, confirm_amount, fee, signed_share_change)
			VALUES ('F1', 'sell', '2025-06-01', 300, 0, -30)`,
		`INSERT INTO portfolio_snapshot (fund_code, held_shares, latest_nav, portfolio_id)
			VALUES ('F1', 70, 0, 1)`,
	})
	svc := NewService(db)
	capture := &xirrCaptureHandler{}
	svc.logger = slog.New(capture)

	if _, err := svc.GetFundXIRR(context.Background(), "F1", 1); err != nil {
		t.Fatal(err)
	}
	if !findWarn(capture, "terminal value degraded", map[string]string{"fund_code": "F1", "reason": "snapshot_nav_not_positive"}) {
		t.Fatalf("expected zero-NAV warn for F1, got %d warnings", len(capture.warnings()))
	}
}

// TestGetPortfolioXIRRWarnsForFundsMissingNAV: the portfolio aggregate skips
// NULL-NAV rows inside SUM, so a live fund can vanish from the terminal value
// without changing any number — each such fund must be warned by code.
func TestGetPortfolioXIRRWarnsForFundsMissingNAV(t *testing.T) {
	db := openXIRRDB(t, []string{
		`INSERT INTO transactions (fund_code, direction, trade_time, confirm_amount, fee, signed_share_change)
			VALUES ('FA', 'buy', '2024-01-01', 100, 0, 10)`,
		`INSERT INTO transactions (fund_code, direction, trade_time, confirm_amount, fee, signed_share_change)
			VALUES ('FB', 'buy', '2024-06-01', 50, 0, 5)`,
		`INSERT INTO portfolio_snapshot (fund_code, held_shares, latest_nav, portfolio_id) VALUES ('FA', 10, 30, 1)`,
		`INSERT INTO portfolio_snapshot (fund_code, held_shares, latest_nav, portfolio_id) VALUES ('FB', 5, NULL, 1)`,
	})
	svc := NewService(db)
	capture := &xirrCaptureHandler{}
	svc.logger = slog.New(capture)

	report, err := svc.GetPortfolioXIRR(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	// Option (a): the aggregate is unchanged — FB contributes nothing to the
	// SUM exactly as before, only the silence is gone.
	if report.CurrentPortfolioValue != 300 {
		t.Fatalf("current_portfolio_value = %v, want 300 (FB excluded by NULL NAV, unchanged)", report.CurrentPortfolioValue)
	}
	if report.XIRRPct == nil {
		t.Fatalf("xirr_pct = nil, want computed value; message=%v", report.Message)
	}
	if !findWarn(capture, "terminal value degraded", map[string]string{"fund_code": "FB", "reason": "snapshot_nav_missing"}) {
		t.Fatalf("expected nav-missing warn for FB, got %d warnings", len(capture.warnings()))
	}
	if findWarn(capture, "terminal value degraded", map[string]string{"fund_code": "FA"}) {
		t.Fatalf("FA has a valid NAV and must not warn: %d warnings", len(capture.warnings()))
	}
}

// TestGetPortfolioXIRRWarnsWhenNothingValued: an all-NULL NAV portfolio makes
// the SUM NULL (early return) — the probe must run before that return.
func TestGetPortfolioXIRRWarnsWhenNothingValued(t *testing.T) {
	db := openXIRRDB(t, []string{
		`INSERT INTO transactions (fund_code, direction, trade_time, confirm_amount, fee, signed_share_change)
			VALUES ('FB', 'buy', '2025-01-02', 50, 0, 5)`,
		`INSERT INTO transactions (fund_code, direction, trade_time, confirm_amount, fee, signed_share_change)
			VALUES ('FB', 'buy', '2025-02-02', 50, 0, 5)`,
		`INSERT INTO portfolio_snapshot (fund_code, held_shares, latest_nav, portfolio_id) VALUES ('FB', 10, NULL, 1)`,
	})
	svc := NewService(db)
	capture := &xirrCaptureHandler{}
	svc.logger = slog.New(capture)

	report, err := svc.GetPortfolioXIRR(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if report.CurrentPortfolioValue != 0 {
		t.Fatalf("current_portfolio_value = %v, want 0 (nothing valued)", report.CurrentPortfolioValue)
	}
	if !findWarn(capture, "terminal value degraded", map[string]string{"fund_code": "FB", "reason": "snapshot_nav_missing"}) {
		t.Fatalf("expected nav-missing warn for FB even with NULL total, got %d warnings", len(capture.warnings()))
	}
}

// TestXIRRTerminalValueTiming pins the cashflow shape used above so the
// degradation tests rest on a verified premise: the terminal cashflow lands
// at the last trade time (Years == 0) with the current value as amount.
func TestXIRRTerminalValueTiming(t *testing.T) {
	last, err := parseXIRRTime("2025-06-01")
	if err != nil {
		t.Fatal(err)
	}
	cashflows := buildXIRRCashflows([]xirrTransaction{
		{Amount: 1000, Direction: "buy", TradeTime: mustParseXIRRTime(t, "2025-01-01")},
		{Amount: 300, Direction: "sell", TradeTime: last},
	}, 140)
	if len(cashflows) != 3 {
		t.Fatalf("cashflows = %d, want 3 (buy, sell, terminal)", len(cashflows))
	}
	terminal := cashflows[2]
	if terminal.Amount != 140 || terminal.Years != 0 {
		t.Fatalf("terminal cashflow = %+v, want amount 140 at years 0", terminal)
	}
}

func mustParseXIRRTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := parseXIRRTime(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
