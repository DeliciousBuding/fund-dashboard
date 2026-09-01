package auth

import (
	"context"
	"fmt"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/testutil"
)

// ListSessions 软上限(200)与截断信号:store 保留上限作为负载/DB 保护,
// 同时把总数与"是否截断"交给上层,设置页才能提示用户而不是静默丢行。

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db := testutil.OpenTempDB(t)
	t.Cleanup(func() { db.Close() })
	store := NewStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return store
}

// seedSessions inserts n sessions with globally unique IDs starting at start
// and monotonically increasing last_seen_at (start+i), so later seeds are
// newer than earlier ones.
func seedSessions(t *testing.T, store *Store, start, n int) {
	t.Helper()
	base := int64(1_700_000_000)
	for i := start; i < start+n; i++ {
		id := fmt.Sprintf("%08x%056x", i, 0)
		if err := store.CreateSession(context.Background(), Session{
			ID:         id,
			CreatedAt:  base,
			ExpiresAt:  base + 3600,
			LastSeenAt: base + int64(i),
			IP:         "192.0.2.1",
			UserAgent:  "store-test-agent",
		}); err != nil {
			t.Fatalf("CreateSession %d: %v", i, err)
		}
	}
}

func TestStoreListSessionsEmpty(t *testing.T) {
	store := newTestStore(t)
	list, err := store.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(list.Sessions) != 0 || list.Total != 0 || list.Truncated {
		t.Fatalf("empty ListSessions = %#v", list)
	}
}

func TestStoreListSessionsTruncationSignals(t *testing.T) {
	store := newTestStore(t)

	// Exactly at the ceiling: full page, no truncation signal.
	seedSessions(t, store, 1, 200)
	list, err := store.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(list.Sessions) != 200 || list.Total != 200 || list.Truncated {
		t.Fatalf("ceiling ListSessions = %d/%d/%v; want 200/200/false", len(list.Sessions), list.Total, list.Truncated)
	}

	// One past the ceiling: newest 200 kept, oldest row cut, signals set.
	seedSessions(t, store, 201, 1)
	list, err = store.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions after +1: %v", err)
	}
	if len(list.Sessions) != 200 || list.Total != 201 || !list.Truncated {
		t.Fatalf("truncated ListSessions = %d/%d/%v; want 200/201/true", len(list.Sessions), list.Total, list.Truncated)
	}
	if list.Sessions[0].LastSeenAt <= list.Sessions[len(list.Sessions)-1].LastSeenAt {
		t.Fatalf("sessions not newest-first: %#v", list.Sessions)
	}
	for _, sess := range list.Sessions {
		if sess.ID == fmt.Sprintf("%08x%056x", 1, 0) {
			t.Fatalf("oldest session unexpectedly in capped page: %#v", sess)
		}
	}
}
