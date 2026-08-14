package portfolio

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestServiceCreateSourceEventStoresDefaultsAndFacts(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()
	ensureSourceEventsTable(t, db)

	service := NewService(db)
	event, err := service.CreateSourceEvent(context.Background(), CreateSourceEventInput{
		Title:               "纳指100 ETF 资金流入创纪录",
		URL:                 stringPtr("https://example.com/nasdaq-inflow"),
		Snippet:             stringPtr("纳斯达克100相关的QDII ETF本周资金净流入达到..."),
		Query:               stringPtr("纳斯达克 QDII 资金流向"),
		RelatedSecurityCode: stringPtr("019173"),
		RelatedSecurityName: stringPtr("纳斯达克100指数(QDII)C"),
	})
	if err != nil {
		t.Fatalf("CreateSourceEvent returned error: %v", err)
	}

	if event.ID == 0 {
		t.Fatalf("ID = 0, want inserted id")
	}
	if event.Source != "websearch" {
		t.Fatalf("Source = %q, want default websearch", event.Source)
	}
	if event.IsRead != 0 || event.IsUseful != 0 {
		t.Fatalf("IsRead/IsUseful = %d/%d, want 0/0", event.IsRead, event.IsUseful)
	}
	if event.FetchedAt == "" || event.CreatedAt == "" {
		t.Fatalf("FetchedAt/CreatedAt = %q/%q, want timestamps", event.FetchedAt, event.CreatedAt)
	}
	if event.RelatedSecurityCode == nil || *event.RelatedSecurityCode != "019173" {
		t.Fatalf("RelatedSecurityCode = %v, want 019173", event.RelatedSecurityCode)
	}
}

func TestServiceGetSourceEventsFiltersAndHidesReadByDefault(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()
	ensureSourceEventsTable(t, db)

	service := NewService(db)
	if _, err := service.CreateSourceEvent(context.Background(), CreateSourceEventInput{
		Title:               "Apple 发布新财报",
		URL:                 stringPtr("https://example.com/apple-earnings"),
		Source:              stringPtr("websearch"),
		Snippet:             stringPtr("Apple Inc. Q3 earnings beat expectations..."),
		Query:               stringPtr("AAPL earnings Q3 2026"),
		RelatedSecurityCode: stringPtr("AAPL"),
		RelatedSecurityName: stringPtr("Apple Inc."),
	}); err != nil {
		t.Fatalf("create apple event: %v", err)
	}
	tencent, err := service.CreateSourceEvent(context.Background(), CreateSourceEventInput{
		Title:               "腾讯游戏业务增长",
		Source:              stringPtr("websearch"),
		Snippet:             stringPtr("腾讯控股游戏收入同比增长..."),
		Query:               stringPtr("00700 腾讯 游戏 收入"),
		RelatedSecurityCode: stringPtr("00700"),
		RelatedSecurityName: stringPtr("腾讯控股"),
	})
	if err != nil {
		t.Fatalf("create tencent event: %v", err)
	}
	if _, err := service.CreateSourceEvent(context.Background(), CreateSourceEventInput{
		Title:               "港股通资金流向变化",
		Source:              stringPtr("eastmoney"),
		Snippet:             stringPtr("南向资金连续3日净流入..."),
		RelatedSecurityCode: stringPtr("00700"),
	}); err != nil {
		t.Fatalf("create eastmoney event: %v", err)
	}

	appleEvents, err := service.GetSourceEvents(context.Background(), GetSourceEventsOptions{
		RelatedSecurityCode: "AAPL",
	})
	if err != nil {
		t.Fatalf("GetSourceEvents apple returned error: %v", err)
	}
	if len(appleEvents) != 1 || !strings.Contains(appleEvents[0].Title, "Apple") {
		t.Fatalf("appleEvents = %#v, want one Apple event", appleEvents)
	}

	emEvents, err := service.GetSourceEvents(context.Background(), GetSourceEventsOptions{Source: "eastmoney"})
	if err != nil {
		t.Fatalf("GetSourceEvents eastmoney returned error: %v", err)
	}
	if len(emEvents) != 1 || emEvents[0].Source != "eastmoney" {
		t.Fatalf("emEvents = %#v, want one eastmoney event", emEvents)
	}

	ok, err := service.MarkSourceEventRead(context.Background(), tencent.ID, MarkSourceEventInput{IsRead: boolPtr(true), IsUseful: boolPtr(true)})
	if err != nil {
		t.Fatalf("MarkSourceEventRead returned error: %v", err)
	}
	if !ok {
		t.Fatalf("MarkSourceEventRead returned false, want true")
	}

	unread, err := service.GetSourceEvents(context.Background(), GetSourceEventsOptions{})
	if err != nil {
		t.Fatalf("GetSourceEvents unread returned error: %v", err)
	}
	if len(unread) != 2 {
		t.Fatalf("unread length = %d, want 2 after marking one read: %#v", len(unread), unread)
	}

	all, err := service.GetSourceEvents(context.Background(), GetSourceEventsOptions{ShowRead: true})
	if err != nil {
		t.Fatalf("GetSourceEvents all returned error: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("all length = %d, want 3", len(all))
	}
	readEvent := findSourceEvent(t, all, tencent.ID)
	if readEvent.IsRead != 1 || readEvent.IsUseful != 1 {
		t.Fatalf("read event flags = %d/%d, want 1/1", readEvent.IsRead, readEvent.IsUseful)
	}
}

func TestServiceSourceEventsDoNotEmitAdviceLanguage(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()
	ensureSourceEventsTable(t, db)

	service := NewService(db)
	if _, err := service.CreateSourceEvent(context.Background(), CreateSourceEventInput{
		Title:               "Market update",
		Source:              stringPtr("websearch"),
		Snippet:             stringPtr("Markets moved..."),
		RelatedSecurityCode: stringPtr("AAPL"),
	}); err != nil {
		t.Fatalf("create market event: %v", err)
	}

	events, err := service.GetSourceEvents(context.Background(), GetSourceEventsOptions{})
	if err != nil {
		t.Fatalf("GetSourceEvents returned error: %v", err)
	}
	payload, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("marshal events: %v", err)
	}
	for _, forbidden := range []string{"买入", "卖出", "加仓", "减仓", "建议", "推荐", "目标价"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("source events contain advice language %q: %s", forbidden, string(payload))
		}
	}
}

func ensureSourceEventsTable(t *testing.T, db execer) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS source_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			url TEXT,
			source TEXT NOT NULL DEFAULT 'websearch',
			snippet TEXT,
			query TEXT,
			related_security_code TEXT,
			related_security_name TEXT,
			is_read INTEGER DEFAULT 0,
			is_useful INTEGER DEFAULT 0,
			fetched_at TEXT NOT NULL DEFAULT (datetime('now')),
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		DELETE FROM source_events;
	`); err != nil {
		t.Fatalf("ensure source_events table: %v", err)
	}
}

func findSourceEvent(t *testing.T, rows []SourceEvent, id int64) SourceEvent {
	t.Helper()
	for _, row := range rows {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("source event %d not found in %#v", id, rows)
	return SourceEvent{}
}

func boolPtr(value bool) *bool {
	return &value
}
