package admin

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

// ── shared crawl service: code recommendation ───────────────────────────────

func TestRecommendedRefreshCodes(t *testing.T) {
	tests := []struct {
		name   string
		report FreshnessReport
		want   []string
	}{
		{
			name:   "empty report yields no codes",
			report: FreshnessReport{},
			want:   []string{},
		},
		{
			name: "stale only keeps report order",
			report: FreshnessReport{
				StaleSecurities: []StaleSecurity{
					{Code: "019173", LastNAV: "2020-01-01"},
					{Code: "016453", LastNAV: "2020-01-02"},
				},
			},
			want: []string{"019173", "016453"},
		},
		{
			name: "missing only",
			report: FreshnessReport{
				MissingNAVSecurities: []FreshnessItem{{Code: "000001", Type: "fund"}},
			},
			want: []string{"000001"},
		},
		{
			name: "stale listed before missing and overlap deduped",
			report: FreshnessReport{
				StaleSecurities: []StaleSecurity{
					{Code: "019173", LastNAV: "2020-01-01"},
					{Code: "016453", LastNAV: "2020-01-01"},
				},
				MissingNAVSecurities: []FreshnessItem{
					{Code: "019173", Type: "fund"},
					{Code: "000001", Type: "fund"},
				},
			},
			want: []string{"019173", "016453", "000001"},
		},
		{
			name: "codes are normalized and blanks dropped",
			report: FreshnessReport{
				StaleSecurities: []StaleSecurity{
					{Code: "  19173  ", LastNAV: "2020-01-01"},
					{Code: "   ", LastNAV: "2020-01-01"},
					{Code: "aapl", LastNAV: "2020-01-01"},
				},
				MissingNAVSecurities: []FreshnessItem{
					{Code: "", Type: "fund"},
					{Code: "19173", Type: "fund"},
				},
			},
			// "  19173  " normalizes to the 6-digit form and the later raw
			// "19173" is the same code, so it must not appear twice.
			want: []string{"019173", "AAPL"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RecommendedRefreshCodes(tc.report)
			if len(got) != len(tc.want) {
				t.Fatalf("codes = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("codes = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// ── shared crawl service: status vocabulary ─────────────────────────────────

func TestBatchStatus(t *testing.T) {
	tests := []struct {
		name   string
		ok     int
		failed []string
		want   string
	}{
		{name: "no failures is complete", ok: 3, failed: nil, want: "complete"},
		{name: "no work at all is complete", ok: 0, failed: nil, want: "complete"},
		{name: "every attempted code failed is error", ok: 0, failed: []string{"A", "B"}, want: "error"},
		{name: "some failures is partial", ok: 2, failed: []string{"C"}, want: "partial"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := BatchStatus(tc.ok, tc.failed); got != tc.want {
				t.Fatalf("BatchStatus(%d, %v) = %q, want %q", tc.ok, tc.failed, got, tc.want)
			}
			outcome := BatchOutcome{
				Done:   make([]string, tc.ok),
				Failed: tc.failed,
			}
			if got := outcome.Status(); got != tc.want {
				t.Fatalf("BatchOutcome.Status() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ── shared crawl service: batch loop ────────────────────────────────────────

// recordingRefresher records dispatch order and answers from a per-code table.
type recordingRefresher struct {
	calls  []string
	added  map[string]int
	fail   map[string]bool
	onCode func(code string)
}

func newRecordingRefresher() *recordingRefresher {
	return &recordingRefresher{added: map[string]int{}, fail: map[string]bool{}}
}

func (r *recordingRefresher) refresh(_ context.Context, code string) (int, error) {
	r.calls = append(r.calls, code)
	if r.onCode != nil {
		r.onCode(code)
	}
	if r.fail[code] {
		return 0, errors.New("upstream unavailable for " + code)
	}
	return r.added[code], nil
}

func TestRunCodeBatch(t *testing.T) {
	t.Run("full list refresh preserves order and sums added rows", func(t *testing.T) {
		refresher := newRecordingRefresher()
		refresher.added = map[string]int{"A": 2, "B": 3, "C": 5}
		outcome := RunCodeBatch(context.Background(), []string{"A", "B", "C"}, refresher.refresh, BatchPolicy{})
		if outcome.Stopped {
			t.Fatalf("Stopped = true, want false")
		}
		if strings.Join(outcome.Done, ",") != "A,B,C" {
			t.Fatalf("Done = %v, want [A B C]", outcome.Done)
		}
		if outcome.Added != 10 || outcome.Attempted != 3 {
			t.Fatalf("Added = %d Attempted = %d, want 10 and 3", outcome.Added, outcome.Attempted)
		}
		if outcome.Status() != "complete" {
			t.Fatalf("Status = %q, want complete", outcome.Status())
		}
	})

	t.Run("stale subset only refreshes the codes it is given", func(t *testing.T) {
		refresher := newRecordingRefresher()
		refresher.added = map[string]int{"STALE1": 1}
		outcome := RunCodeBatch(context.Background(), []string{"STALE1"}, refresher.refresh, BatchPolicy{})
		if strings.Join(refresher.calls, ",") != "STALE1" {
			t.Fatalf("calls = %v, want only the stale code", refresher.calls)
		}
		if len(outcome.Done) != 1 || outcome.Status() != "complete" {
			t.Fatalf("Done = %v Status = %q", outcome.Done, outcome.Status())
		}
	})

	t.Run("blank codes are skipped and not attempted", func(t *testing.T) {
		refresher := newRecordingRefresher()
		refresher.added = map[string]int{"A": 1}
		outcome := RunCodeBatch(context.Background(), []string{"", "  ", "A"}, refresher.refresh, BatchPolicy{})
		if strings.Join(refresher.calls, ",") != "A" {
			t.Fatalf("calls = %v, want [A]", refresher.calls)
		}
		if outcome.Attempted != 1 {
			t.Fatalf("Attempted = %d, want 1", outcome.Attempted)
		}
	})

	t.Run("upstream failure is soft-skipped and the batch continues", func(t *testing.T) {
		refresher := newRecordingRefresher()
		refresher.added = map[string]int{"A": 1, "C": 4}
		refresher.fail = map[string]bool{"B": true}
		outcome := RunCodeBatch(context.Background(), []string{"A", "B", "C"}, refresher.refresh, BatchPolicy{
			FailureLogMessage: "test crawl code failed",
		})
		if strings.Join(refresher.calls, ",") != "A,B,C" {
			t.Fatalf("calls = %v, want the batch to continue past B", refresher.calls)
		}
		if strings.Join(outcome.Done, ",") != "A,C" {
			t.Fatalf("Done = %v, want [A C]", outcome.Done)
		}
		if strings.Join(outcome.Failed, ",") != "B" {
			t.Fatalf("Failed = %v, want [B]", outcome.Failed)
		}
		if outcome.Added != 5 {
			t.Fatalf("Added = %d, want 5", outcome.Added)
		}
		if outcome.Status() != "partial" {
			t.Fatalf("Status = %q, want partial", outcome.Status())
		}
	})

	t.Run("every code failing is an error, not a success", func(t *testing.T) {
		refresher := newRecordingRefresher()
		refresher.fail = map[string]bool{"A": true, "B": true}
		outcome := RunCodeBatch(context.Background(), []string{"A", "B"}, refresher.refresh, BatchPolicy{})
		if outcome.Status() != "error" {
			t.Fatalf("Status = %q, want error", outcome.Status())
		}
		if len(outcome.Done) != 0 || outcome.Attempted != 2 {
			t.Fatalf("Done = %v Attempted = %d", outcome.Done, outcome.Attempted)
		}
	})

	t.Run("Failed stays nil when nothing failed so payloads keep their shape", func(t *testing.T) {
		refresher := newRecordingRefresher()
		outcome := RunCodeBatch(context.Background(), []string{"A"}, refresher.refresh, BatchPolicy{})
		if outcome.Failed != nil {
			t.Fatalf("Failed = %#v, want nil", outcome.Failed)
		}
		if outcome.Done == nil {
			t.Fatalf("Done = nil, want non-nil slice")
		}
	})

	t.Run("cancelled before any work stops without dispatching", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		refresher := newRecordingRefresher()
		outcome := RunCodeBatch(ctx, []string{"A", "B", "C"}, refresher.refresh, BatchPolicy{})
		if !outcome.Stopped {
			t.Fatalf("Stopped = false, want true")
		}
		if len(refresher.calls) != 0 || len(outcome.Done) != 0 || outcome.Added != 0 {
			t.Fatalf("calls = %v Done = %v Added = %d, want no work", refresher.calls, outcome.Done, outcome.Added)
		}
	})

	t.Run("cancellation mid-batch keeps completed work and reports Stopped", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		refresher := newRecordingRefresher()
		refresher.added = map[string]int{"A": 1}
		refresher.onCode = func(code string) {
			if code == "A" {
				cancel()
			}
		}
		outcome := RunCodeBatch(ctx, []string{"A", "B", "C"}, refresher.refresh, BatchPolicy{})
		if !outcome.Stopped {
			t.Fatalf("Stopped = false, want true")
		}
		if strings.Join(outcome.Done, ",") != "A" {
			t.Fatalf("Done = %v, want [A]", outcome.Done)
		}
		if strings.Join(refresher.calls, ",") != "A" {
			t.Fatalf("calls = %v, want the batch to stop before B", refresher.calls)
		}
	})

	t.Run("backoff is ctx-aware and never blocks past the deadline", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		refresher := newRecordingRefresher()
		refresher.onCode = func(string) { cancel() }
		started := time.Now()
		// A one-hour backoff would hang the suite if the pause were not
		// ctx-aware; cancellation must cut it short immediately.
		outcome := RunCodeBatch(ctx, []string{"A", "B"}, refresher.refresh, BatchPolicy{Backoff: time.Hour})
		if elapsed := time.Since(started); elapsed > 30*time.Second {
			t.Fatalf("batch blocked %v inside backoff, want ctx-aware return", elapsed)
		}
		if !outcome.Stopped {
			t.Fatalf("Stopped = false, want true")
		}
		if strings.Join(outcome.Done, ",") != "A" {
			t.Fatalf("Done = %v, want [A]", outcome.Done)
		}
	})

	t.Run("backoff is not applied after the final code", func(t *testing.T) {
		refresher := newRecordingRefresher()
		started := time.Now()
		outcome := RunCodeBatch(context.Background(), []string{"A"}, refresher.refresh, BatchPolicy{Backoff: time.Hour})
		if elapsed := time.Since(started); elapsed > 30*time.Second {
			t.Fatalf("batch slept %v after the final code", elapsed)
		}
		if outcome.Stopped || len(outcome.Done) != 1 {
			t.Fatalf("Stopped = %v Done = %v", outcome.Stopped, outcome.Done)
		}
	})

	t.Run("backoff is not applied after a failed code", func(t *testing.T) {
		refresher := newRecordingRefresher()
		refresher.fail = map[string]bool{"A": true}
		started := time.Now()
		outcome := RunCodeBatch(context.Background(), []string{"A", "B"}, refresher.refresh, BatchPolicy{Backoff: time.Hour})
		if elapsed := time.Since(started); elapsed > 30*time.Second {
			t.Fatalf("batch throttled a failed code for %v", elapsed)
		}
		if strings.Join(refresher.calls, ",") != "A,B" {
			t.Fatalf("calls = %v, want [A B]", refresher.calls)
		}
		if strings.Join(outcome.Failed, ",") != "A" || strings.Join(outcome.Done, ",") != "B" {
			t.Fatalf("Failed = %v Done = %v", outcome.Failed, outcome.Done)
		}
	})

	t.Run("backoff does pause between successful codes", func(t *testing.T) {
		refresher := newRecordingRefresher()
		started := time.Now()
		RunCodeBatch(context.Background(), []string{"A", "B"}, refresher.refresh, BatchPolicy{Backoff: 20 * time.Millisecond})
		if elapsed := time.Since(started); elapsed < 20*time.Millisecond {
			t.Fatalf("elapsed = %v, want at least one 20ms backoff between codes", elapsed)
		}
	})

	t.Run("missing refresher fails closed instead of pretending to crawl", func(t *testing.T) {
		outcome := RunCodeBatch(context.Background(), []string{"A"}, nil, BatchPolicy{})
		if outcome.Attempted != 0 || len(outcome.Done) != 0 || outcome.Added != 0 {
			t.Fatalf("outcome = %#v, want no work", outcome)
		}
		if outcome.Done == nil {
			t.Fatalf("Done = nil, want non-nil slice")
		}
	})

	t.Run("empty code list is a no-op", func(t *testing.T) {
		refresher := newRecordingRefresher()
		outcome := RunCodeBatch(context.Background(), nil, refresher.refresh, BatchPolicy{})
		if len(refresher.calls) != 0 || outcome.Stopped || outcome.Attempted != 0 {
			t.Fatalf("calls = %v outcome = %#v", refresher.calls, outcome)
		}
		if outcome.Status() != "complete" {
			t.Fatalf("Status = %q, want complete", outcome.Status())
		}
	})
}

// ── shared crawl service: stale-only flow against a real DB ─────────────────

func execCrawlFixture(t *testing.T, db *sql.DB, statements ...string) {
	t.Helper()
	for _, stmt := range statements {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
}

func TestRefreshStaleCodesNoOpWhenFresh(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	execCrawlFixture(t, db,
		`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market)
         VALUES ('019173', 'Fresh Fund', 'fund', 'fund', '')`,
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
         VALUES ('019173', 'Fresh Fund', 10, -100, 1.5, 15, 0, 0, 'fund', 1)`,
		`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type)
         VALUES ('019173', '2099-01-01', 1.5, 0, 'fund')`,
	)

	svc := NewServiceWithDriver(db, "sqlite")
	refresher := newRecordingRefresher()
	result, err := RefreshStaleCodes(context.Background(), svc, refresher.refresh, BatchPolicy{})
	if err != nil {
		t.Fatalf("RefreshStaleCodes: %v", err)
	}
	if len(result.Codes) != 0 {
		t.Fatalf("Codes = %v, want none for a fresh held security", result.Codes)
	}
	if len(refresher.calls) != 0 {
		t.Fatalf("calls = %v, want no upstream call when nothing is stale", refresher.calls)
	}
	if result.Batch.Done == nil {
		t.Fatalf("Batch.Done = nil, want non-nil slice")
	}
	if result.Batch.Status() != "complete" {
		t.Fatalf("Status = %q, want complete", result.Batch.Status())
	}
}

func TestRefreshStaleCodesSelectsStaleAndMissingOnly(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	execCrawlFixture(t, db,
		// Held + fresh NAV: must be skipped entirely.
		`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market)
         VALUES ('010001', 'Fresh Held Fund', 'fund', 'fund', '')`,
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
         VALUES ('010001', 'Fresh Held Fund', 10, -100, 1.5, 15, 0, 0, 'fund', 1)`,
		`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type)
         VALUES ('010001', '2099-01-01', 1.5, 0, 'fund')`,

		// Held + stale NAV: recommended via the stale branch.
		`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market)
         VALUES ('010002', 'Stale Held Fund', 'fund', 'fund', '')`,
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
         VALUES ('010002', 'Stale Held Fund', 10, -100, 1.0, 10, 0, 0, 'fund', 1)`,
		`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type)
         VALUES ('010002', '2020-01-01', 1.0, 0, 'fund')`,

		// Held + no NAV at all: recommended via the missing branch.
		`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market)
         VALUES ('010003', 'Missing Held Fund', 'fund', 'fund', '')`,
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
         VALUES ('010003', 'Missing Held Fund', 10, -100, 0, 0, 0, 0, 'fund', 1)`,

		// Held STOCK + no NAV: the stock source must be selected too, so a
		// stale-only crawl cannot silently skip non-fund holdings.
		`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market)
         VALUES ('AAPL', 'Apple Inc', 'stock', 'stock', 'US')`,
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
         VALUES ('AAPL', 'Apple Inc', 5, -500, 0, 0, 0, 0, 'stock', 1)`,

		// Watchlist only (not held) + no NAV: must NOT be refreshed by a
		// held-only stale crawl.
		`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market)
         VALUES ('010009', 'Unheld Watchlist Fund', 'fund', 'fund', '')`,
	)

	svc := NewServiceWithDriver(db, "sqlite")
	refresher := newRecordingRefresher()
	refresher.added = map[string]int{"010002": 3, "010003": 2, "AAPL": 1}

	result, err := RefreshStaleCodes(context.Background(), svc, refresher.refresh, BatchPolicy{})
	if err != nil {
		t.Fatalf("RefreshStaleCodes: %v", err)
	}

	got := strings.Join(result.Codes, ",")
	if got != "010002,010003,AAPL" {
		t.Fatalf("Codes = %v, want the stale fund, the missing fund and the missing stock", result.Codes)
	}
	if strings.Join(refresher.calls, ",") != got {
		t.Fatalf("calls = %v, want the batch to dispatch exactly the recommended codes", refresher.calls)
	}
	if result.Batch.Added != 6 {
		t.Fatalf("Added = %d, want 6", result.Batch.Added)
	}
	if result.Batch.Status() != "complete" {
		t.Fatalf("Status = %q, want complete", result.Batch.Status())
	}
	// A full refresh is the caller's other mode; the stale flow must never
	// reach the fresh holding or the unheld watchlist entry.
	for _, banned := range []string{"010001", "010009"} {
		for _, call := range refresher.calls {
			if call == banned {
				t.Fatalf("stale-only crawl dispatched %q, which is not held-stale or held-missing", banned)
			}
		}
	}
}

func TestRefreshStaleCodesSoftFailsUpstreamErrors(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	execCrawlFixture(t, db,
		`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market)
         VALUES ('010002', 'Stale Held Fund', 'fund', 'fund', '')`,
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
         VALUES ('010002', 'Stale Held Fund', 10, -100, 1.0, 10, 0, 0, 'fund', 1)`,
		`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type)
         VALUES ('010002', '2020-01-01', 1.0, 0, 'fund')`,
		`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market)
         VALUES ('010003', 'Missing Held Fund', 'fund', 'fund', '')`,
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
         VALUES ('010003', 'Missing Held Fund', 10, -100, 0, 0, 0, 0, 'fund', 1)`,
	)

	svc := NewServiceWithDriver(db, "sqlite")
	refresher := newRecordingRefresher()
	refresher.added = map[string]int{"010003": 2}
	refresher.fail = map[string]bool{"010002": true}

	result, err := RefreshStaleCodes(context.Background(), svc, refresher.refresh, BatchPolicy{
		FailureLogMessage: "test stale refresh code failed",
	})
	if err != nil {
		t.Fatalf("RefreshStaleCodes must soft-fail per-code errors, got err = %v", err)
	}
	if strings.Join(result.Batch.Failed, ",") != "010002" {
		t.Fatalf("Failed = %v, want [010002]", result.Batch.Failed)
	}
	if strings.Join(result.Batch.Done, ",") != "010003" {
		t.Fatalf("Done = %v, want [010003]", result.Batch.Done)
	}
	if result.Batch.Status() != "partial" {
		t.Fatalf("Status = %q, want partial", result.Batch.Status())
	}
}

func TestRefreshStaleCodesPropagatesFreshnessError(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	// Dropping nav_history makes the freshness read fail; the stale flow must
	// surface that instead of reporting an empty, successful crawl.
	if _, err := db.ExecContext(context.Background(), `DROP TABLE nav_history`); err != nil {
		t.Fatalf("drop nav_history: %v", err)
	}

	svc := NewServiceWithDriver(db, "sqlite")
	refresher := newRecordingRefresher()
	_, err := RefreshStaleCodes(context.Background(), svc, refresher.refresh, BatchPolicy{})
	if err == nil {
		t.Fatalf("err = nil, want the freshness failure to propagate")
	}
	if len(refresher.calls) != 0 {
		t.Fatalf("calls = %v, want no crawl after a failed freshness read", refresher.calls)
	}
}
