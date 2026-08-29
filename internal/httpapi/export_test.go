package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/config"
	"github.com/DeliciousBuding/fund-dashboard/internal/testutil"
	"github.com/xuri/excelize/v2"
)

func sampleExportBody() []byte {
	body, _ := json.Marshal(map[string]any{
		"fundName": "纳指测试",
		"transactions": []map[string]any{
			{
				"trade_time":      "2026-07-01 10:00:00",
				"confirm_date":    "2026-07-02",
				"direction":       "buy",
				"amount":          1000.5,
				"shares":          100.25,
				"nav":             1.2345,
				"inferred_nav":    1.234567,
				"fee":             1.5,
				"settlement_days": 1,
				"trade_day_type":  "交易日",
			},
			{
				"trade_time":   "2026-07-03 11:00:00",
				"confirm_date": "2026-07-04",
				"direction":    "sell",
				"amount":       500.0,
				"shares":       40.0,
				"fee":          0,
			},
		},
	})
	return body
}

func readExportRows(t *testing.T, rec *httptest.ResponseRecorder) [][]string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("Content-Type=%q", ct)
	}
	if rec.Body.Len() < 100 {
		t.Fatalf("xlsx too small: %d", rec.Body.Len())
	}
	f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("open xlsx: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("transactions")
	if err != nil {
		t.Fatalf("rows: %v", err)
	}
	return rows
}

func TestExportTransactionsXLSX(t *testing.T) {
	db := testutil.OpenTempDB(t)
	defer db.Close()
	router := newAuthedRouter(t, config.Config{ServiceName: "fund-dashboard-go", Version: "test"}, db)
	req := httptest.NewRequest(http.MethodPost, "/api/export/transactions-xlsx", bytes.NewReader(sampleExportBody()))
	req.Header.Set("Content-Type", "application/json")
	// No Accept-Language → zh default
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	rows := readExportRows(t, rec)
	if len(rows) != 3 { // header + 2
		t.Fatalf("row count=%d want 3; rows=%v", len(rows), rows)
	}
	if rows[0][0] != "交易时间" || rows[1][2] != "买入" || rows[2][2] != "卖出" {
		t.Fatalf("unexpected zh rows: %v", rows)
	}
}

func TestExportTransactionsXLSXEnglish(t *testing.T) {
	db := testutil.OpenTempDB(t)
	defer db.Close()
	router := newAuthedRouter(t, config.Config{ServiceName: "fund-dashboard-go", Version: "test"}, db)
	req := httptest.NewRequest(http.MethodPost, "/api/export/transactions-xlsx", bytes.NewReader(sampleExportBody()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	rows := readExportRows(t, rec)
	if len(rows) != 3 {
		t.Fatalf("row count=%d want 3; rows=%v", len(rows), rows)
	}
	if rows[0][0] != "Trade time" || rows[1][2] != "Buy" || rows[2][2] != "Sell" {
		t.Fatalf("unexpected en rows: %v", rows)
	}
}

func TestExportTransactionsXLSXRequiresRows(t *testing.T) {
	db := testutil.OpenTempDB(t)
	defer db.Close()
	router := newAuthedRouter(t, config.Config{ServiceName: "fund-dashboard-go", Version: "test"}, db)
	req := httptest.NewRequest(http.MethodPost, "/api/export/transactions-xlsx", bytes.NewReader([]byte(`{"fundName":"x","transactions":[]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", rec.Code, rec.Body.String())
	}
}

func TestSanitizeExportFilename(t *testing.T) {
	got := sanitizeExportFilename("foo\nbar:baz")
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("controls remain: %q", got)
	}
	if strings.Contains(got, ":") {
		t.Fatalf("colon remains: %q", got)
	}
	if got != "foobar_baz" {
		t.Fatalf("got %q want foobar_baz", got)
	}
	long := strings.Repeat("X", 100)
	got = sanitizeExportFilename(long)
	if len([]rune(got)) != 80 {
		t.Fatalf("len=%d want 80", len([]rune(got)))
	}
	if sanitizeExportFilename("   ") != "transactions" {
		t.Fatal("empty")
	}
}

func TestClampExportCell(t *testing.T) {
	if clampExportCell("  ab  ", 10) != "ab" {
		t.Fatal("trim")
	}
	long := strings.Repeat("Z", 100)
	if got := clampExportCell(long, 10); got != strings.Repeat("Z", 10) {
		t.Fatalf("got %q", got)
	}
}
