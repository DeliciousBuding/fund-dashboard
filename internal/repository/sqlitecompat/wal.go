package sqlitecompat

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
)

type WALConcurrentReport struct {
	JournalMode           string
	ReaderInitialRows     int
	ReaderRowsDuringWrite int
	WriterRowsInserted    int
	FinalProbeRows        int
}

func CheckWALConcurrentReadWrite(ctx context.Context, dbPath string) (WALConcurrentReport, error) {
	if strings.TrimSpace(dbPath) == "" {
		return WALConcurrentReport{}, errors.New("db path is required")
	}
	if _, err := os.Stat(dbPath); err != nil {
		return WALConcurrentReport{}, fmt.Errorf("stat db path: %w", err)
	}

	reader, err := openSingleConn(ctx, dbPath)
	if err != nil {
		return WALConcurrentReport{}, fmt.Errorf("open reader: %w", err)
	}
	defer reader.Close()

	writer, err := openSingleConn(ctx, dbPath)
	if err != nil {
		return WALConcurrentReport{}, fmt.Errorf("open writer: %w", err)
	}
	defer writer.Close()

	observer, err := openSingleConn(ctx, dbPath)
	if err != nil {
		return WALConcurrentReport{}, fmt.Errorf("open observer: %w", err)
	}
	defer observer.Close()

	report := WALConcurrentReport{}
	if err := queryScalar(ctx, writer, "PRAGMA journal_mode=WAL", &report.JournalMode); err != nil {
		return report, err
	}
	report.JournalMode = strings.ToLower(report.JournalMode)

	if _, err := writer.ExecContext(ctx, "DROP TABLE IF EXISTS sqlitecompat_wal_probe"); err != nil {
		return report, fmt.Errorf("reset wal probe table: %w", err)
	}
	if _, err := writer.ExecContext(ctx, "CREATE TABLE sqlitecompat_wal_probe (id INTEGER PRIMARY KEY AUTOINCREMENT, note TEXT NOT NULL)"); err != nil {
		return report, fmt.Errorf("create wal probe table: %w", err)
	}
	defer writer.ExecContext(context.Background(), "DROP TABLE IF EXISTS sqlitecompat_wal_probe")

	tx, err := reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return report, fmt.Errorf("begin read tx: %w", err)
	}
	defer tx.Rollback()

	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM nav_history").Scan(&report.ReaderInitialRows); err != nil {
		return report, fmt.Errorf("read initial nav rows: %w", err)
	}

	result, err := writer.ExecContext(ctx, "INSERT INTO sqlitecompat_wal_probe (note) VALUES ('writer while reader is open')")
	if err != nil {
		return report, fmt.Errorf("write during read tx: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return report, fmt.Errorf("read writer rows affected: %w", err)
	}
	report.WriterRowsInserted = int(rows)

	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM nav_history").Scan(&report.ReaderRowsDuringWrite); err != nil {
		return report, fmt.Errorf("read nav rows during write: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return report, fmt.Errorf("commit read tx: %w", err)
	}

	if err := observer.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlitecompat_wal_probe").Scan(&report.FinalProbeRows); err != nil {
		return report, fmt.Errorf("read final probe rows: %w", err)
	}

	return report, nil
}

func openSingleConn(ctx context.Context, dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	return db, nil
}
