package admin

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/testutil"
)

// captureHandler records warn-level records so tests can assert the server
// actually logged the drift signal instead of only reading the report.
type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelWarn
}

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }

func (h *captureHandler) warnings() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]slog.Record(nil), h.records...)
}

func openShareDriftDB(t *testing.T, stmts []string) (svc Service, cleanup func()) {
	t.Helper()
	// Production schema via the real boot path; the share-drift probe then
	// runs against the same tables it sees in production.
	db := testutil.OpenTempDBWithProductionSchema(t)
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	return NewServiceWithDriver(db, "sqlite"), func() { _ = db.Close() }
}

func hasShareDriftRecommendation(recs []string) bool {
	for _, rec := range recs {
		if strings.HasPrefix(rec, "share_drift:") {
			return true
		}
	}
	return false
}

// TestShareDriftDetectedReportsAndWarns is the negative test: a hand-crafted
// one-share gap between the transaction ledger and the snapshot must surface
// as a recommendation, degrade the SQLite overall status, and emit a warn.
func TestShareDriftDetectedReportsAndWarns(t *testing.T) {
	svc, cleanup := openShareDriftDB(t, []string{
		`INSERT INTO transactions (fund_code, signed_share_change) VALUES ('F1', 100)`,
		`INSERT INTO portfolio_snapshot (fund_code, held_shares, portfolio_id) VALUES ('F1', 99, 1)`,
	})
	defer cleanup()

	prev := slog.Default()
	handler := &captureHandler{}
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(prev)

	report, err := svc.GetDBIntegrity(context.Background(), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if !hasShareDriftRecommendation(report.Recommendations) {
		t.Fatalf("share drift not reported: %v", report.Recommendations)
	}
	found := false
	for _, rec := range report.Recommendations {
		if strings.HasPrefix(rec, "share_drift:F1 ") &&
			strings.Contains(rec, "ledger_shares=100.0000") &&
			strings.Contains(rec, "snapshot_shares=99.0000") &&
			strings.Contains(rec, "drift=1.0000") {
			found = true
		}
	}
	if !found {
		t.Fatalf("drift recommendation missing expected numbers: %v", report.Recommendations)
	}
	if report.Overall != "degraded" {
		t.Fatalf("overall = %q, want degraded when share drift exists", report.Overall)
	}
	warned := false
	for _, r := range handler.warnings() {
		if strings.Contains(r.Message, "share drift detected") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("expected share drift warn log, got %d warnings", len(handler.warnings()))
	}
}

// TestShareDriftCleanLedgerKeepsOverallOK: consistent ledger/snapshot data
// must not produce findings or flip the report status.
func TestShareDriftCleanLedgerKeepsOverallOK(t *testing.T) {
	svc, cleanup := openShareDriftDB(t, []string{
		`INSERT INTO transactions (fund_code, signed_share_change) VALUES ('F1', 100)`,
		`INSERT INTO transactions (fund_code, signed_share_change) VALUES ('F2', 0.0000001)`,
		`INSERT INTO portfolio_snapshot (fund_code, held_shares, portfolio_id) VALUES ('F1', 100, 1)`,
	})
	defer cleanup()

	report, err := svc.GetDBIntegrity(context.Background(), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if hasShareDriftRecommendation(report.Recommendations) {
		t.Fatalf("clean ledger flagged as drift: %v", report.Recommendations)
	}
	if report.Overall != "ok" {
		t.Fatalf("overall = %q, want ok on consistent data", report.Overall)
	}
}

// TestShareDriftSumsSnapshotsAcrossPortfolios: transactions are fund-wide so
// the snapshot side must aggregate over portfolio rows before comparing.
func TestShareDriftSumsSnapshotsAcrossPortfolios(t *testing.T) {
	svc, cleanup := openShareDriftDB(t, []string{
		`INSERT INTO transactions (fund_code, signed_share_change) VALUES ('F1', 50)`,
		`INSERT INTO portfolio_snapshot (fund_code, held_shares, portfolio_id) VALUES ('F1', 30, 1)`,
		`INSERT INTO portfolio_snapshot (fund_code, held_shares, portfolio_id) VALUES ('F1', 20, 2)`,
	})
	defer cleanup()

	report, err := svc.GetDBIntegrity(context.Background(), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if hasShareDriftRecommendation(report.Recommendations) {
		t.Fatalf("per-portfolio sum (30+20=50) flagged as drift: %v", report.Recommendations)
	}
}

// TestShareDriftWithinDustThresholdIgnored: sub-dust differences — including
// pure float residue — are the dust threshold's job, not drift.
func TestShareDriftWithinDustThresholdIgnored(t *testing.T) {
	svc, cleanup := openShareDriftDB(t, []string{
		`INSERT INTO transactions (fund_code, signed_share_change) VALUES ('F1', 100.0005)`,
		`INSERT INTO portfolio_snapshot (fund_code, held_shares, portfolio_id) VALUES ('F1', 100, 1)`,
		`INSERT INTO transactions (fund_code, signed_share_change) VALUES ('F2', 33.33333333)`,
		`INSERT INTO portfolio_snapshot (fund_code, held_shares, portfolio_id) VALUES ('F2', 33.3333, 1)`,
	})
	defer cleanup()

	report, err := svc.GetDBIntegrity(context.Background(), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if hasShareDriftRecommendation(report.Recommendations) {
		t.Fatalf("sub-dust difference flagged as drift: %v", report.Recommendations)
	}
}

// TestShareDriftLedgerWithoutSnapshotRow: a fund holding shares in the ledger
// but missing from portfolio_snapshot entirely is drift too.
func TestShareDriftLedgerWithoutSnapshotRow(t *testing.T) {
	svc, cleanup := openShareDriftDB(t, []string{
		`INSERT INTO transactions (fund_code, signed_share_change) VALUES ('GHOST', 50)`,
	})
	defer cleanup()

	report, err := svc.GetDBIntegrity(context.Background(), time.Now().UTC())
	if err := err; err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rec := range report.Recommendations {
		if strings.HasPrefix(rec, "share_drift:GHOST ") && strings.Contains(rec, "drift=50.0000") {
			found = true
		}
	}
	if !found {
		t.Fatalf("ledger-only fund not reported: %v", report.Recommendations)
	}
}

// TestShareDriftSkipsMissingTables: an empty database (no user tables) must
// not error or produce findings — same scenario as the sanitize test.
func TestShareDriftSkipsMissingTables(t *testing.T) {
	svc, cleanup := openShareDriftDB(t, nil)
	defer cleanup()

	report, err := svc.GetDBIntegrity(context.Background(), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if hasShareDriftRecommendation(report.Recommendations) {
		t.Fatalf("drift findings without tables: %v", report.Recommendations)
	}
}
