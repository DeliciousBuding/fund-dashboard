package jobs

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// 运行态跟踪(design 06 §2.6):任务执行后 StatusSnapshot 记录 LastRun,
// 失败路径记录 LastError,NextRun 给出下个窗口。

func TestStatusSnapshotRecordsLastRun(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE portfolio_snapshot (fund_code TEXT NOT NULL, held_shares REAL, security_type TEXT, portfolio_id INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (fund_code, portfolio_id))`,
		`CREATE TABLE fund_details (fund_code TEXT PRIMARY KEY, security_type TEXT)`,
		`CREATE TABLE nav_history (fund_code TEXT, date TEXT, unit_nav REAL)`,
		`CREATE TABLE crawl_log (fund_code TEXT, source TEXT, rows_added INTEGER, latest_date TEXT, status TEXT, crawled_at TEXT)`,
		// indices 表(与 schema_pg.go 同列):缺失会让 20:00 窗口聚合记录为失败(见 recordJob)。
		`CREATE TABLE indices (code TEXT PRIMARY KEY, name TEXT, market TEXT, price REAL, change_pct REAL, change_amt REAL, updated_at TEXT)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	stub := &stubDCARunner{}
	s := NewScheduler(NewPriceRefresher(db), db).WithDCARunner(stub)
	now := time.Date(2026, 7, 15, 20, 3, 0, 0, cst) // Wednesday
	s.tick(now)

	snap := s.StatusSnapshot()
	if len(snap) != 4 {
		t.Fatalf("snapshot = %d entries, want 4 (startup_refresh/price_dca/holdings/wal)", len(snap))
	}
	var price *JobStatus
	for i := range snap {
		if snap[i].Name == "price_dca" {
			price = &snap[i]
		}
		if snap[i].Schedule == "" || snap[i].NextRun <= 0 {
			if snap[i].Name != "startup_refresh" { // startup 无下一次循环窗口
				t.Fatalf("%s missing schedule/next_run: %#v", snap[i].Name, snap[i])
			}
		}
	}
	if price == nil {
		t.Fatal("price_dca missing from snapshot")
	}
	if price.LastRun != now.Unix() {
		t.Fatalf("price_dca last_run = %d, want %d", price.LastRun, now.Unix())
	}
	if price.LastError != "" {
		t.Fatalf("price_dca last_error = %q, want empty (claim+run on clean fixture)", price.LastError)
	}
	// 下一个 20:00 窗口:StatusSnapshot 用真实时钟(不可注入),只校验
	// 窗口形状(20:00 CST)且晚于 last_run。
	if price.NextRun <= price.LastRun {
		t.Fatalf("price_dca next_run = %d, want later than last_run %d", price.NextRun, price.LastRun)
	}
	nextTime := time.Unix(price.NextRun, 0).In(cst)
	if nextTime.Hour() != 20 {
		t.Fatalf("price_dca next_run hour = %d, want 20 CST", nextTime.Hour())
	}
}

func TestStatusSnapshotRecordsLastErrorOnFailure(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// 无 fund 表 → 真实 Refresher 会返回错误,被记录为 last_error。
	s := NewScheduler(NewPriceRefresher(db), db)
	now := time.Date(2026, 7, 15, 20, 3, 0, 0, cst)
	s.tick(now)

	snap := s.StatusSnapshot()
	for _, entry := range snap {
		if entry.Name == "price_dca" {
			if entry.LastRun == 0 {
				t.Fatal("last_run must be recorded even on failure")
			}
			if entry.LastError == "" {
				t.Fatalf("missing-table fixture must record last_error: %#v", entry)
			}
		}
	}
}

func TestSweepExpiredStateAuthEventsRetention(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE auth_events (id INTEGER PRIMARY KEY AUTOINCREMENT, ts BIGINT NOT NULL, event TEXT NOT NULL, ip TEXT, user_agent TEXT, detail TEXT)`); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := db.Exec(`INSERT INTO auth_events (ts, event) VALUES (?, 'old')`, now.Add(-200*24*time.Hour).Unix()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO auth_events (ts, event) VALUES (?, 'fresh')`, now.Unix()); err != nil {
		t.Fatal(err)
	}

	// 生产接线是 auth.Store.SweepAuthEvents(见 app.go);测试用等价实现的 stub,
	// 验证 scheduler 注入点与 180d 截止时间片。
	s := NewScheduler(NewPriceRefresher(db), db)
	stub := &stubAuthSweeper{db: db}
	s.WithAuthEventSweeper(stub)
	if err := s.sweepExpiredState(context.Background()); err != nil {
		t.Fatalf("sweepExpiredState: %v", err)
	}
	if len(stub.cutoffs) != 1 {
		t.Fatalf("auth sweep calls = %d, want 1", len(stub.cutoffs))
	}
	if want := now.Add(-180 * 24 * time.Hour).Unix(); stub.cutoffs[0] != want {
		t.Fatalf("auth sweep cutoff = %d, want %d (now-180d)", stub.cutoffs[0], want)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM auth_events`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("remaining auth_events = %d, want 1 (fresh)", n)
	}
}

// stubAuthSweeper mimics auth.Store.SweepAuthEvents (DELETE WHERE ts < cutoff).
type stubAuthSweeper struct {
	db      *sql.DB
	cutoffs []int64
}

func (s *stubAuthSweeper) SweepAuthEvents(ctx context.Context, cutoff int64) (int64, error) {
	s.cutoffs = append(s.cutoffs, cutoff)
	res, err := s.db.ExecContext(ctx, `DELETE FROM auth_events WHERE ts < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
