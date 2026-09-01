// Package snapshot owns the single source of truth for rebuilding a
// portfolio_snapshot row from the transactions ledger + nav_history.
//
// Three historical copies existed (jobs.recalcSnapshot, admin.recalcSnapshotTx,
// portfolio.recalcSnapshotLight) with subtle drift. They are unified here behind
// a Mode that preserves the only intentional divergence: whether identity
// columns (fund_name/security_type) and latest_nav are refreshed.
package snapshot

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// HeldSharesDust is the minimum absolute share count treated as a real holding.
// Float residue after full sells (~1e-15) is not a real position (#90).
const HeldSharesDust = 0.001

// Querier is the minimal SQL surface Recalc needs. *sql.DB and *sql.Tx both
// satisfy it, so a snapshot can be rebuilt inside or outside a transaction.
type Querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Mode selects which columns Recalc refreshes.
type Mode uint8

const (
	// ModeFull refreshes fund_name/security_type and writes latest_nav as-is
	// (NULL when nav_history has no row). Used by price refresh and admin
	// transaction writes.
	ModeFull Mode = iota
	// ModeLight keeps fund_name/security_type untouched and preserves the
	// existing latest_nav when the new NAV is 0/absent. Used by DCA and
	// adjust-position paths that must not clobber identity or wipe NAV.
	ModeLight
)

// Recalc rebuilds one portfolio_snapshot row, resolving portfolio_id from the
// existing snapshot row (default 1). It matches the former jobs/admin helpers.
func Recalc(ctx context.Context, q Querier, code string, mode Mode) error {
	return recalc(ctx, q, code, 0, mode, true)
}

// RecalcForPortfolio rebuilds one row for an explicit portfolioID. It matches
// the former portfolio.recalcSnapshotLight.
func RecalcForPortfolio(ctx context.Context, q Querier, code string, portfolioID int, mode Mode) error {
	return recalc(ctx, q, code, portfolioID, mode, false)
}

func recalc(ctx context.Context, q Querier, code string, portfolioID int, mode Mode, resolveID bool) error {
	if code == "" {
		return nil
	}

	var shares, cost sql.NullFloat64
	var txFundName sql.NullString
	if err := q.QueryRowContext(ctx, `
		SELECT SUM(COALESCE(signed_share_change, 0)), SUM(COALESCE(signed_cash_flow, 0)), MAX(fund_name)
		FROM transactions
		WHERE fund_code = ?
	`, code).Scan(&shares, &cost, &txFundName); err != nil {
		return fmt.Errorf("recalc snapshot transactions: %w", err)
	}

	heldShares := 0.0
	totalCost := 0.0
	if shares.Valid {
		heldShares = shares.Float64
	}
	if cost.Valid {
		totalCost = cost.Float64
	}
	if heldShares > -HeldSharesDust && heldShares < HeldSharesDust {
		heldShares = 0
	}

	fundName := code
	secType := "fund"
	// Full resolves identity up-front; Light defers it to the INSERT branch.
	if mode == ModeFull {
		var err error
		fundName, secType, err = resolveIdentity(ctx, q, code, txFundName)
		if err != nil {
			return err
		}
	}

	var latestNAV sql.NullFloat64
	if err := q.QueryRowContext(ctx, `
		SELECT unit_nav
		FROM nav_history
		WHERE fund_code = ?
		ORDER BY date DESC
		LIMIT 1
	`, code).Scan(&latestNAV); err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("recalc snapshot latest nav: %w", err)
	}
	navVal := 0.0
	var navArg any
	if latestNAV.Valid {
		navVal = latestNAV.Float64
		navArg = navVal
	}

	if resolveID {
		id, err := resolvePortfolioID(ctx, q, code)
		if err != nil {
			return err
		}
		portfolioID = int(id)
	} else if portfolioID <= 0 {
		portfolioID = 1
	}

	// Value math must use the NAV that will actually be visible on the row.
	// ModeLight preserves an existing latest_nav when nav_history has no fresh
	// value, so current_value/unrealized/pnl have to be derived from that
	// preserved NAV — otherwise a Light recalc keeps latest_nav while wiping the
	// valuation columns to zero.
	effNav := navVal
	if mode == ModeLight && navVal == 0 {
		var existing sql.NullFloat64
		if err := q.QueryRowContext(ctx, `
			SELECT latest_nav FROM portfolio_snapshot
			WHERE fund_code = ? AND COALESCE(portfolio_id, 1) = ?
		`, code, portfolioID).Scan(&existing); err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("recalc snapshot existing nav: %w", err)
		}
		if existing.Valid && existing.Float64 > 0 {
			effNav = existing.Float64
		}
	}

	currentValue := 0.0
	if effNav != 0 {
		currentValue = heldShares * effNav
	}
	unrealized := currentValue + totalCost
	pnlPct := 0.0
	if totalCost != 0 {
		denom := totalCost
		if denom < 0 {
			denom = -denom
		}
		pnlPct = unrealized / denom * 100
	}
	if heldShares == 0 {
		currentValue = 0
		unrealized = 0
		pnlPct = 0
	}

	// Portable first-write upsert. UPDATE-then-INSERT works for both *sql.DB
	// and *sql.Tx queriers (a Tx cannot open another transaction) and for every
	// supported portfolio_snapshot PK shape — legacy SQLite keeps a single
	// (fund_code) PRIMARY KEY (deploy/ci-seed.sql) while fresh SQLite/PG use
	// (fund_code, portfolio_id). A single INSERT ... ON CONFLICT would need a
	// dialect-aware conflict target and would still break the legacy
	// single-column PK.
	//
	// Two connections can both see UPDATE affect 0 rows on a first write and
	// then both INSERT; the loser gets a UNIQUE/PK violation. One bounded retry
	// re-runs the UPDATE path after the winner's row is visible, converging on
	// the same row without leaking concurrent-write errors to callers.
	for attempt := 0; ; attempt++ {
		var res sql.Result
		var err error
		if mode == ModeFull {
			res, err = q.ExecContext(ctx, `
				UPDATE portfolio_snapshot SET
					fund_name = ?, held_shares = ?, total_cost = ?, latest_nav = ?,
					current_value = ?, unrealized_pnl = ?, pnl_pct = ?, security_type = ?
				WHERE fund_code = ? AND COALESCE(portfolio_id, 1) = ?
			`, fundName, heldShares, totalCost, navArg, currentValue, unrealized, pnlPct, secType, code, portfolioID)
		} else {
			res, err = q.ExecContext(ctx, `
				UPDATE portfolio_snapshot SET
					held_shares = ?, total_cost = ?, latest_nav = COALESCE(NULLIF(?,0), latest_nav),
					current_value = ?, unrealized_pnl = ?, pnl_pct = ?
				WHERE fund_code = ? AND COALESCE(portfolio_id,1) = ?
			`, heldShares, totalCost, navVal, currentValue, unrealized, pnlPct, code, portfolioID)
		}
		if err != nil {
			return fmt.Errorf("recalc snapshot update: %w", err)
		}
		n, raErr := res.RowsAffected()
		if raErr != nil {
			return fmt.Errorf("recalc snapshot rows affected: %w", raErr)
		}
		if n > 0 {
			return nil
		}

		insertNav := navArg
		if mode == ModeLight {
			var idErr error
			fundName, secType, idErr = resolveIdentity(ctx, q, code, txFundName)
			if idErr != nil {
				return idErr
			}
			insertNav = nullIfZero(navVal)
		}
		_, err = q.ExecContext(ctx, `
			INSERT INTO portfolio_snapshot
				(fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, code, fundName, heldShares, totalCost, insertNav, currentValue, unrealized, pnlPct, secType, portfolioID)
		if err == nil {
			return nil
		}
		if !isUniqueViolation(err) || attempt >= maxSnapshotWriteRetries-1 {
			return fmt.Errorf("recalc snapshot insert: %w", err)
		}
		// The concurrent first writer committed its row; loop back to UPDATE.
	}
}

func resolveIdentity(ctx context.Context, q Querier, code string, txFundName sql.NullString) (string, string, error) {
	var detailName, securityType sql.NullString
	if err := q.QueryRowContext(ctx, `
		SELECT fund_name, security_type
		FROM fund_details
		WHERE fund_code = ?
	`, code).Scan(&detailName, &securityType); err != nil && err != sql.ErrNoRows {
		return "", "", fmt.Errorf("recalc snapshot identity: %w", err)
	}
	fundName := code
	if detailName.Valid && detailName.String != "" {
		fundName = detailName.String
	} else if txFundName.Valid && txFundName.String != "" {
		fundName = txFundName.String
	}
	secType := "fund"
	if securityType.Valid && securityType.String != "" {
		secType = securityType.String
	}
	return fundName, secType, nil
}

func resolvePortfolioID(ctx context.Context, q Querier, code string) (int64, error) {
	portfolioID := int64(1)
	err := q.QueryRowContext(ctx, `
		SELECT portfolio_id FROM portfolio_snapshot
		WHERE fund_code = ?
		ORDER BY portfolio_id
		LIMIT 1
	`, code).Scan(&portfolioID)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("recalc snapshot portfolio_id: %w", err)
	}
	if portfolioID <= 0 {
		portfolioID = 1
	}
	return portfolioID, nil
}

// maxSnapshotWriteRetries bounds the UPDATE→INSERT first-write retry loop. One
// retry suffices for the concurrent-writer case (the conflict is only raised
// after the winning transaction committed); the cap turns a pathological
// concurrent delete/insert livelock into an error instead of a hang.
const maxSnapshotWriteRetries = 3

// isUniqueViolation reports whether err is a PRIMARY KEY/UNIQUE constraint
// conflict. Both supported drivers surface it as a plain error string:
// SQLite "UNIQUE constraint failed: ..." and PostgreSQL "duplicate key value
// violates unique constraint ..." (SQLSTATE 23505). Matching stays narrow so
// every other INSERT failure (NOT NULL, CHECK, ...) keeps surfacing unchanged.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed") ||
		strings.Contains(msg, "duplicate key value violates unique constraint") ||
		strings.Contains(msg, "sqlstate 23505")
}

func nullIfZero(v float64) any {
	if v == 0 {
		return nil
	}
	return v
}
