package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/jobs"
	_ "modernc.org/sqlite"
)

func TestCrawlHandlerSanitizesClientError(t *testing.T) {
	// Closed DB forces RefreshAllHeld to fail with a driver-level error.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = db.Close()

	handler := crawlHandler(jobs.NewPriceRefresher(db))
	req := httptest.NewRequest(http.MethodPost, "/api/admin/crawl-nav", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v body=%s", err, rec.Body.String())
	}
	if body["status"] != "error" {
		t.Fatalf("status field=%v", body["status"])
	}
	errMsg, _ := body["error"].(string)
	if errMsg != "internal_error" {
		t.Fatalf("error=%q want internal_error body=%s", errMsg, rec.Body.String())
	}
	raw := rec.Body.String()
	// Must not leak SQL/driver internals to clients (#233).
	for _, frag := range []string{"sql:", "pq:", "database is closed", "no such table", "SQLITE"} {
		if strings.Contains(strings.ToLower(raw), strings.ToLower(frag)) {
			t.Fatalf("leaked detail %q in body: %s", frag, raw)
		}
	}
}
