package jobs

import (
	"context"
	"log/slog"

	db "github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
)

// ensureNavSchema is a best-effort nav_history column backfill, run once per
// refresher. The implementation lives in internal/repository/db (single
// source, shared with migration 0004_nav_history_security_columns, which runs
// at boot via EnsureSchema and makes this lazy path a pure probe on any
// database that went through startup). It remains here for databases that
// never run EnsureSchema — hand-rolled test fixtures and any embedder that
// opens the DB directly.
func (r *PriceRefresher) ensureNavSchema(ctx context.Context) {
	r.navSchemaOnce.Do(func() {
		if err := db.EnsureNavHistoryColumns(ctx, r.db); err != nil {
			r.navSchemaErr = err
			slog.Error("ensure nav_history schema", "error", err)
		}
	})
}
