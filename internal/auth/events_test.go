package auth

import (
	"context"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/testutil"
)

// auth_events 审计表(design 06 §2.2):写入、倒序、limit clamp、180d 清扫。

func TestAuthEventsInsertListOrderAndClamp(t *testing.T) {
	db := testutil.OpenTempDB(t)
	t.Cleanup(func() { db.Close() })
	store := NewStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	base := int64(1_800_000_000)
	events := []struct {
		name string
		ts   int64
	}{
		{"login_fail", base},
		{"login_ok", base + 10},
		{"logout", base + 20},
	}
	for _, ev := range events {
		if err := store.InsertAuthEvent(context.Background(), ev.name, "1.2.3.4", "ua-1", "detail-"+ev.name, ev.ts); err != nil {
			t.Fatalf("InsertAuthEvent %s: %v", ev.name, err)
		}
	}

	got, err := store.ListAuthEvents(context.Background(), 100)
	if err != nil {
		t.Fatalf("ListAuthEvents: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	// 倒序:最新在前。
	if got[0].Event != "logout" || got[1].Event != "login_ok" || got[2].Event != "login_fail" {
		t.Fatalf("order = %v %v %v, want logout/login_ok/login_fail", got[0].Event, got[1].Event, got[2].Event)
	}
	if got[0].IP != "1.2.3.4" || got[0].UserAgent != "ua-1" || got[0].Detail != "detail-logout" {
		t.Fatalf("row = %#v", got[0])
	}

	// limit clamp:请求 5000 也只返回 500(先插满 505 行)。
	for i := 0; i < 505; i++ {
		if err := store.InsertAuthEvent(context.Background(), "login_ok", "", "", "", base+100+int64(i)); err != nil {
			t.Fatalf("bulk insert %d: %v", i, err)
		}
	}
	got, err = store.ListAuthEvents(context.Background(), 5000)
	if err != nil {
		t.Fatalf("ListAuthEvents clamp: %v", err)
	}
	if len(got) > 500 {
		t.Fatalf("limit clamp failed: %d > 500", len(got))
	}
	if len(got) != 500 {
		t.Fatalf("clamped len = %d, want 500 (508 rows total)", len(got))
	}

	// limit <= 0 → 空。
	got, err = store.ListAuthEvents(context.Background(), 0)
	if err != nil || len(got) != 0 {
		t.Fatalf("limit 0 = %d, %v; want empty", len(got), err)
	}
}

func TestServiceRecordsAuthEventsWithInjectedClock(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	svc := newTestService(t, Options{Now: func() time.Time { return now }})
	ctx := context.Background()

	svc.RecordAuthEvent(ctx, "login_ok", "5.6.7.8", "cli/1", "")
	svc.RecordAuthEvent(ctx, "lockout", "5.6.7.8", "cli/1", "retry_after=901")

	got, err := svc.ListAuthEvents(ctx, 10)
	if err != nil {
		t.Fatalf("ListAuthEvents: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("events = %d, want 2", len(got))
	}
	if got[0].Event != "lockout" || got[0].TS != now.Unix() || got[0].Detail != "retry_after=901" {
		t.Fatalf("newest = %#v", got[0])
	}
	if got[1].TS != now.Unix() {
		t.Fatalf("ts must come from the injected clock, got %d", got[1].TS)
	}
}

func TestSweepAuthEventsRemovesOldRows(t *testing.T) {
	db := testutil.OpenTempDB(t)
	t.Cleanup(func() { db.Close() })
	store := NewStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	now := time.Now().Unix()
	for i := 0; i < 30; i++ {
		// 200 天前的老行
		if err := store.InsertAuthEvent(context.Background(), "login_fail", "", "", "", now-200*24*3600-int64(i)); err != nil {
			t.Fatalf("old insert: %v", err)
		}
	}
	for i := 0; i < 5; i++ {
		if err := store.InsertAuthEvent(context.Background(), "login_ok", "", "", "", now-int64(i)); err != nil {
			t.Fatalf("new insert: %v", err)
		}
	}
	deleted, err := store.SweepAuthEvents(context.Background(), now-180*24*3600)
	if err != nil {
		t.Fatalf("SweepAuthEvents: %v", err)
	}
	if deleted != 30 {
		t.Fatalf("deleted = %d, want 30", deleted)
	}
	left, err := store.ListAuthEvents(context.Background(), 100)
	if err != nil {
		t.Fatalf("list after sweep: %v", err)
	}
	if len(left) != 5 {
		t.Fatalf("left = %d, want 5", len(left))
	}
}
