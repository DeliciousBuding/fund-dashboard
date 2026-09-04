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

// TestStoreListSessionsOrderIsTotal pins the ordering contract of the session
// list: newest last_seen_at first, ties broken by id descending.
//
// Ties are the common case, not an edge case -- last_seen_at is unix seconds, so
// every pair of logins inside the same second ties. The seeds are inserted in
// ascending id order so that insertion (rowid) order is the exact opposite of
// the tiebreak: a plan that returns tied rows in insertion order fails here.
// The repeated reads catch an order that is merely stable-by-accident within one
// connection.
func TestStoreListSessionsOrderIsTotal(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	const tie = int64(1_700_000_000)
	seeds := []struct {
		id         string
		lastSeenAt int64
	}{
		{"aa-tied", tie},
		{"mm-tied", tie},
		{"zz-newest", tie + 10},
	}
	for _, seed := range seeds {
		if err := store.CreateSession(ctx, Session{
			ID:         seed.id,
			CreatedAt:  tie,
			ExpiresAt:  tie + 3600,
			LastSeenAt: seed.lastSeenAt,
			IP:         "192.0.2.1",
			UserAgent:  "store-test-agent",
		}); err != nil {
			t.Fatalf("CreateSession %s: %v", seed.id, err)
		}
	}

	want := []string{"zz-newest", "mm-tied", "aa-tied"}
	for attempt := 1; attempt <= 5; attempt++ {
		list, err := store.ListSessions(ctx)
		if err != nil {
			t.Fatalf("ListSessions attempt %d: %v", attempt, err)
		}
		if len(list.Sessions) != len(want) {
			t.Fatalf("attempt %d: ListSessions = %d rows; want %d", attempt, len(list.Sessions), len(want))
		}
		for i, wantID := range want {
			if list.Sessions[i].ID != wantID {
				got := make([]string, len(list.Sessions))
				for j, sess := range list.Sessions {
					got[j] = sess.ID
				}
				t.Fatalf("attempt %d: order = %v; want %v (last_seen_at DESC, then id DESC)", attempt, got, want)
			}
		}
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
