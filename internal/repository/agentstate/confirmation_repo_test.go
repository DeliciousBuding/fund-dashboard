package agentstate

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/confirmations"
	"github.com/DeliciousBuding/fund-dashboard/internal/repository/sqlitedb"
)

func TestConfirmationRepositoryCreatesTableAndRoundTripsRecord(t *testing.T) {
	ctx := context.Background()
	db := openAgentStateFixture(t)
	defer db.Close()

	repo := NewConfirmationRepository(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema returned error: %v", err)
	}

	record := confirmations.Record{
		Tool:        "add_transaction",
		TokenHash:   "hash-only",
		PayloadHash: "payload-hash",
		CreatedAt:   time.Date(2026, 7, 7, 4, 50, 0, 0, time.UTC),
		ExpiresAt:   time.Date(2026, 7, 7, 5, 5, 0, 0, time.UTC),
	}
	id, err := repo.Save(ctx, record)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if id <= 0 {
		t.Fatalf("id = %d, want positive row id", id)
	}

	got, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got == nil {
		t.Fatalf("Get returned nil, want record")
	}
	if got.Tool != record.Tool ||
		got.TokenHash != record.TokenHash ||
		got.PayloadHash != record.PayloadHash ||
		!got.CreatedAt.Equal(record.CreatedAt) ||
		!got.ExpiresAt.Equal(record.ExpiresAt) {
		t.Fatalf("record = %#v, want %#v", got, record)
	}
	if got.UsedAt != nil {
		t.Fatalf("UsedAt = %#v, want nil before mark used", got.UsedAt)
	}
}

func TestConfirmationRepositoryMarksRecordUsed(t *testing.T) {
	ctx := context.Background()
	db := openAgentStateFixture(t)
	defer db.Close()

	repo := NewConfirmationRepository(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema returned error: %v", err)
	}
	id, err := repo.Save(ctx, confirmations.Record{
		Tool:        "add_transaction",
		TokenHash:   "hash-only",
		PayloadHash: "payload-hash",
		CreatedAt:   time.Date(2026, 7, 7, 4, 50, 0, 0, time.UTC),
		ExpiresAt:   time.Date(2026, 7, 7, 5, 5, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	usedAt := time.Date(2026, 7, 7, 4, 55, 0, 0, time.UTC)
	if err := repo.MarkUsed(ctx, id, usedAt); err != nil {
		t.Fatalf("MarkUsed returned error: %v", err)
	}

	got, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got == nil || got.UsedAt == nil || !got.UsedAt.Equal(usedAt) {
		t.Fatalf("UsedAt = %#v, want %s", got, usedAt)
	}

	// Second mark must fail atomically (single-use / TOCTOU guard).
	if err := repo.MarkUsed(ctx, id, usedAt.Add(time.Minute)); err != confirmations.ErrAlreadyUsed {
		t.Fatalf("second MarkUsed = %v, want ErrAlreadyUsed", err)
	}
}

func TestConfirmationRepositoryDoesNotStoreRawToken(t *testing.T) {
	ctx := context.Background()
	db := openAgentStateFixture(t)
	defer db.Close()

	repo := NewConfirmationRepository(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema returned error: %v", err)
	}
	if _, err := repo.Save(ctx, confirmations.Record{
		Tool:        "add_transaction",
		TokenHash:   "hmac-token-hash",
		PayloadHash: "payload-hash",
		CreatedAt:   time.Date(2026, 7, 7, 4, 50, 0, 0, time.UTC),
		ExpiresAt:   time.Date(2026, 7, 7, 5, 5, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var rawTokenColumns int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM pragma_table_info('agent_confirmations')
		WHERE name IN ('token', 'raw_token', 'confirmation_token')
	`).Scan(&rawTokenColumns); err != nil {
		t.Fatalf("query table info: %v", err)
	}
	if rawTokenColumns != 0 {
		t.Fatalf("raw token columns = %d, want none", rawTokenColumns)
	}
}

func TestConfirmationRepositoryGetMissingReturnsNil(t *testing.T) {
	ctx := context.Background()
	db := openAgentStateFixture(t)
	defer db.Close()

	repo := NewConfirmationRepository(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema returned error: %v", err)
	}
	got, err := repo.Get(ctx, 404)
	if err != nil {
		t.Fatalf("Get missing returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("Get missing = %#v, want nil", got)
	}
}

func openAgentStateFixture(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "fund.db")
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: dbPath})
	if err != nil {
		t.Fatalf("open sqlite fixture: %v", err)
	}
	return db
}

func TestConfirmationRepositoryMarkUsedConcurrentSingleWinner(t *testing.T) {
	ctx := context.Background()
	db := openAgentStateFixture(t)
	defer db.Close()
	repo := NewConfirmationRepository(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	id, err := repo.Save(ctx, confirmations.Record{
		Tool:        "add_transaction",
		TokenHash:   "hash-only",
		PayloadHash: "payload-hash",
		CreatedAt:   time.Date(2026, 7, 19, 3, 0, 0, 0, time.UTC),
		ExpiresAt:   time.Date(2026, 7, 19, 4, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	const n = 16
	errs := make(chan error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	base := time.Date(2026, 7, 19, 3, 5, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			errs <- repo.MarkUsed(ctx, id, base.Add(time.Duration(i)*time.Millisecond))
		}()
	}
	wg.Wait()
	close(errs)

	var wins, already, other int
	for err := range errs {
		switch {
		case err == nil:
			wins++
		case err == confirmations.ErrAlreadyUsed:
			already++
		default:
			other++
			t.Errorf("unexpected: %v", err)
		}
	}
	if wins != 1 || already != n-1 {
		t.Fatalf("wins=%d already=%d other=%d want 1/%d/0", wins, already, other, n-1)
	}
}
