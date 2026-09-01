package portfolio

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	db "github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
)

// TestWriteValidationErrorsAreTyped pins the contract that client-input
// failures from the SPA write services are ValidationError-typed (message
// unchanged), which lets httpapi map them to 400 without string heuristics.
func TestWriteValidationErrorsAreTyped(t *testing.T) {
	ctx := context.Background()
	rawDB, err := db.Open(ctx, db.Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "fund.db")})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer rawDB.Close()
	if _, err := rawDB.Exec(`CREATE TABLE dca_plans (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	fund_code TEXT NOT NULL,
	fund_name TEXT,
	amount REAL NOT NULL,
	frequency TEXT NOT NULL,
	weekday_mask TEXT NOT NULL,
	trade_type TEXT NOT NULL,
	portfolio_id INTEGER NOT NULL,
	start_date TEXT NOT NULL,
	end_date TEXT,
	active INTEGER NOT NULL,
	source TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
)`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	svc := NewService(rawDB)

	cases := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{"upsert missing code", mustErr(svc.UpsertDCAPlan(ctx, UpsertDCAPlanInput{Amount: 100})), "fund_code is required"},
		{"upsert amount zero", mustErr(svc.UpsertDCAPlan(ctx, UpsertDCAPlanInput{FundCode: "019173"})), "amount must be positive"},
		{"upsert amount cap", mustErr(svc.UpsertDCAPlan(ctx, UpsertDCAPlanInput{FundCode: "019173", Amount: 2_000_000})), "amount max 1000000"},
		{"upsert frequency cap", mustErr(svc.UpsertDCAPlan(ctx, UpsertDCAPlanInput{FundCode: "019173", Amount: 100, Frequency: strings.Repeat("w", 33)})), "frequency max 32 chars"},
		{"upsert unknown id", mustErr(svc.UpsertDCAPlan(ctx, UpsertDCAPlanInput{ID: 999, FundCode: "019173", Amount: 100})), "dca plan id 999 not found"},
		{"disable id zero", mustErr(svc.DisableDCAPlan(ctx, 0)), "id is required"},
		{"security missing code", mustErr(svc.UpsertSecurity(ctx, UpsertSecurityInput{Name: "Test"})), "fund_code is required"},
		{"security stock missing market", mustErr(svc.UpsertSecurity(ctx, UpsertSecurityInput{Code: "AAPL", Name: "Apple", SecurityType: "stock"})), "market is required for stock"},
		{"security delete missing code", mustErr(svc.DeleteSecurity(ctx, "  ")), "fund_code is required"},
		{"adjust missing code", mustErr(svc.AdjustPosition(ctx, AdjustPositionInput{Shares: 1})), "fund_code is required"},
		{"adjust negative shares", mustErr(svc.AdjustPosition(ctx, AdjustPositionInput{Code: "019173", Shares: -1})), "shares must be >= 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !IsValidationError(tc.err) {
				t.Fatalf("error %v is not a ValidationError", tc.err)
			}
			var ve *ValidationError
			if !errors.As(tc.err, &ve) {
				t.Fatalf("errors.As failed for %v", tc.err)
			}
			if ve.Error() != tc.wantMsg {
				t.Fatalf("message = %q, want %q", ve.Error(), tc.wantMsg)
			}
		})
	}

	// as_of carries the parse failure text; assert the stable prefix only.
	_, asOfErr := svc.RunDCAAutoInvest(ctx, RunDCAAutoInvestInput{AsOf: "2026-02-30"})
	if !IsValidationError(asOfErr) || !strings.HasPrefix(asOfErr.Error(), "as_of must be YYYY-MM-DD: ") {
		t.Fatalf("as_of error = %v, want typed validation with stable prefix", asOfErr)
	}

	// Validation errors survive wrapping (fmt.Errorf %w) so handler mapping is robust.
	wrapped := fmt.Errorf("context: %w", NewValidationError("bad input"))
	var wrappedErr *ValidationError
	if !IsValidationError(wrapped) || !errors.As(wrapped, &wrappedErr) {
		t.Fatalf("wrapped ValidationError not detected: %v", wrapped)
	}
	if wrappedErr.Error() != "bad input" {
		t.Fatalf("wrapped message = %q, want %q", wrappedErr.Error(), "bad input")
	}
}

// TestWriteDBFailuresAreNotValidation pins the other half of the contract:
// database/infrastructure failures must never masquerade as client validation.
func TestWriteDBFailuresAreNotValidation(t *testing.T) {
	ctx := context.Background()
	rawDB, err := db.Open(ctx, db.Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "fund.db")})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	svc := NewService(rawDB)
	if err := rawDB.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	_, err = svc.UpsertDCAPlan(ctx, UpsertDCAPlanInput{FundCode: "019173", Amount: 100})
	if err == nil {
		t.Fatal("expected DB failure from closed database")
	}
	if IsValidationError(err) {
		t.Fatalf("DB failure misclassified as validation: %v", err)
	}
	var ve *ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("errors.As matched ValidationError for DB failure: %v", err)
	}
}

func mustErr[T any](v T, err error) error {
	return err
}
