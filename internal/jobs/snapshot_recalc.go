package jobs

import (
	"context"
	"database/sql"
)

// SnapshotService exposes portfolio_snapshot rebuild helpers for MCP/admin.
type SnapshotService struct {
	DB *sql.DB
}

func NewSnapshotService(db *sql.DB) *SnapshotService {
	return &SnapshotService{DB: db}
}

func (s *SnapshotService) RecalcCode(ctx context.Context, code string) error {
	return RecalcSnapshot(ctx, s.DB, code)
}

func (s *SnapshotService) RecalcAll(ctx context.Context) (codes int, failed []string, err error) {
	return RecalcAllSnapshots(ctx, s.DB)
}
